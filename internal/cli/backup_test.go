package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stubKubectl(t *testing.T, fn func(args []string, in io.Reader, out io.Writer) error) {
	old := kubectl
	kubectl = func(_ string, in io.Reader, out io.Writer, args ...string) error { return fn(args, in, out) }
	t.Cleanup(func() { kubectl = old })
}

func TestBackupRejectsTinyDump(t *testing.T) {
	stubKubectl(t, func(a []string, _ io.Reader, out io.Writer) error {
		if a[0] == "get" {
			_, _ = out.Write([]byte("True"))
			return nil
		}
		_, _ = out.Write([]byte("PGDMP"))
		return nil
	})
	dir := t.TempDir()
	if _, err := backupCluster("ns", dir); err == nil {
		t.Fatal("tiny dump must fail")
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*.dump")); len(m) != 0 {
		t.Errorf("failed dump left behind: %v", m)
	}
}

func TestBackupOKAndOverlayOptional(t *testing.T) {
	stubKubectl(t, func(a []string, _ io.Reader, out io.Writer) error {
		switch {
		case a[0] == "get":
			_, _ = out.Write([]byte("True"))
		case a[2] == "deploy/pinball":
			return fmt.Errorf("no tar")
		default:
			_, _ = out.Write(append([]byte("PGDMP"), make([]byte, 600)...))
		}
		return nil
	})
	p, err := backupCluster("ns", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

func TestBackupNotReady(t *testing.T) {
	stubKubectl(t, func(_ []string, _ io.Reader, out io.Writer) error { _, _ = out.Write([]byte("False")); return nil })
	if _, err := backupCluster("ns", t.TempDir()); err == nil {
		t.Fatal("not-ready pod must fail")
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	d := t.TempDir()
	for i := 1; i <= 7; i++ {
		_ = os.WriteFile(filepath.Join(d, fmt.Sprintf("request_logs-2026010%d.dump", i)), nil, 0o600)
	}
	pruneBackups(d, 5)
	m, _ := filepath.Glob(filepath.Join(d, "*"))
	if len(m) != 5 || !strings.HasSuffix(m[0], "03.dump") {
		t.Errorf("got %v", m)
	}
}

func TestRestoreConfirmation(t *testing.T) {
	dump := filepath.Join(t.TempDir(), "x.dump")
	_ = os.WriteFile(dump, append([]byte("PGDMP"), make([]byte, 600)...), 0o600)
	restored := false
	stubKubectl(t, func(a []string, _ io.Reader, out io.Writer) error {
		if a[0] == "get" {
			_, _ = out.Write([]byte("True"))
		} else {
			restored = true
		}
		return nil
	})
	_ = clusterRestoreCmd.Flags().Set("from", dump)
	_ = clusterRestoreCmd.Flags().Set("yes", "false")
	var o bytes.Buffer
	clusterRestoreCmd.SetOut(&o)
	clusterRestoreCmd.SetIn(strings.NewReader("no\n"))
	if err := clusterRestoreCmd.RunE(clusterRestoreCmd, nil); err == nil || restored {
		t.Fatal("must abort without yes")
	}
	clusterRestoreCmd.SetIn(strings.NewReader("yes\n"))
	if err := clusterRestoreCmd.RunE(clusterRestoreCmd, nil); err != nil || !restored {
		t.Fatalf("restore failed: %v", err)
	}
}
