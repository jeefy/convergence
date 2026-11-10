package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	pgPod       = "request-log-postgres-0"
	pgMinDump   = 512 // an empty pg_dump -Fc is larger; smaller means a failed dump
	keepBackups = 5
)

// kubectl runs kubectl with optional stdin/stdout; a var so tests can stub it.
var kubectl = func(ns string, in io.Reader, out io.Writer, args ...string) error {
	c := exec.Command("kubectl", append([]string{"-n", ns}, args...)...)
	c.Stdin, c.Stdout = in, out
	var stderr bytes.Buffer
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("kubectl %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func pgReady(ns string) error {
	var b bytes.Buffer
	if err := kubectl(ns, nil, &b, "get", "pod", pgPod, "-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`); err != nil {
		return fmt.Errorf("request-log Postgres not found: %w", err)
	}
	if b.String() != "True" {
		return fmt.Errorf("pod %s is not ready", pgPod)
	}
	return nil
}

// backupCluster writes a pg_dump -Fc (and, if available, the pinball overlay tar) into dir.
func backupCluster(ns, dir string) (string, error) {
	if err := pgReady(ns); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	dump := filepath.Join(dir, "request_logs-"+ts+".dump")
	f, err := os.OpenFile(dump, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	// Local socket auth in the postgres image is trust, so no password is needed.
	err = kubectl(ns, nil, f, "exec", pgPod, "--", "pg_dump", "-Fc", "-U", "convergence", "-d", "agentgateway")
	_ = f.Close()
	if err == nil {
		err = checkDump(dump)
	}
	if err != nil {
		_ = os.Remove(dump)
		return "", err
	}
	// Overlay is regenerable by the optimizer, so a missing PVC/tar is only a warning.
	tarPath := filepath.Join(dir, "pinball-overlay-"+ts+".tar")
	if tf, err := os.OpenFile(tarPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); err == nil {
		err = kubectl(ns, nil, tf, "exec", "deploy/pinball", "--", "tar", "-C", "/var/lib/pinball", "-cf", "-", ".")
		_ = tf.Close()
		if err != nil {
			_ = os.Remove(tarPath)
			_, _ = fmt.Fprintf(os.Stderr, "warning: pinball overlay not backed up: %v\n", err)
		}
	}
	return dump, nil
}

func checkDump(p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	magic := make([]byte, 5)
	if _, err := io.ReadFull(f, magic); err != nil || st.Size() < pgMinDump || string(magic) != "PGDMP" {
		return fmt.Errorf("dump %s is empty or invalid (%d bytes)", p, st.Size())
	}
	return nil
}

// pruneBackups keeps the newest keep timestamped sets (names sort chronologically).
func pruneBackups(dir string, keep int) {
	for _, prefix := range []string{"request_logs-", "pinball-overlay-"} {
		m, _ := filepath.Glob(filepath.Join(dir, prefix+"*"))
		sort.Strings(m)
		for i := 0; i < len(m)-keep; i++ {
			_ = os.Remove(m[i])
		}
	}
}

func defaultBackupDir() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".convergence", "backups")
}

// autoBackup runs before install when the request-log Postgres already exists.
func autoBackup(ns string) error {
	if err := kubectl(ns, nil, io.Discard, "get", "statefulset", "request-log-postgres"); err != nil {
		return nil // nothing installed yet
	}
	dir := defaultBackupDir()
	p, err := backupCluster(ns, dir)
	if err != nil {
		return fmt.Errorf("pre-install backup failed (use --no-backup to skip): %w", err)
	}
	pruneBackups(dir, keepBackups)
	fmt.Printf("Backed up request log to %s\n", p)
	return nil
}

var clusterBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Back up the in-cluster request-log Postgres (and pinball overlay) to a directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		out, _ := cmd.Flags().GetString("out")
		p, err := backupCluster(ns, out)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), p)
		return nil
	},
}

var clusterRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore the request-log Postgres from a backup dump (replaces existing data)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ns, _ := cmd.Flags().GetString("namespace")
		from, _ := cmd.Flags().GetString("from")
		yes, _ := cmd.Flags().GetBool("yes")
		if err := checkDump(from); err != nil {
			return err
		}
		if err := pgReady(ns); err != nil {
			return err
		}
		if !yes {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "This replaces request_logs data in namespace '%s' with %s. Type 'yes' to continue: ", ns, from)
			line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if strings.TrimSpace(line) != "yes" {
				return fmt.Errorf("aborted")
			}
		}
		f, err := os.Open(from)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if err := kubectl(ns, f, io.Discard, "exec", "-i", pgPod, "--", "pg_restore", "--clean", "--if-exists", "--no-owner", "-U", "convergence", "-d", "agentgateway"); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Restored.")
		return nil
	},
}

func init() {
	clusterBackupCmd.Flags().String("namespace", "convergence", "Kubernetes namespace")
	clusterBackupCmd.Flags().String("out", "", "Output directory")
	_ = clusterBackupCmd.MarkFlagRequired("out")
	clusterRestoreCmd.Flags().String("namespace", "convergence", "Kubernetes namespace")
	clusterRestoreCmd.Flags().String("from", "", "Dump file created by 'cluster backup'")
	_ = clusterRestoreCmd.MarkFlagRequired("from")
	clusterRestoreCmd.Flags().Bool("yes", false, "Skip the confirmation prompt")
	clusterInstallCmd.Flags().Bool("no-backup", false, "Skip the automatic backup taken when request-log Postgres already exists")
	clusterCmd.AddCommand(clusterBackupCmd, clusterRestoreCmd)
}
