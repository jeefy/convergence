package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRun_ErrorPrintedOnceAndExitsNonZero(t *testing.T) {
	// Given: an invalid subcommand, which makes cobra return an error
	origArgs, origStderr := os.Args, os.Stderr
	t.Cleanup(func() { os.Args, os.Stderr = origArgs, origStderr })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"convergence", "no-such-command"}
	os.Stderr = w

	// When: the command runs
	code := run()
	os.Stderr = origStderr
	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	// Then: exit code is 1 and exactly one error line is printed
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var errLines []string
	for _, l := range lines {
		if strings.HasPrefix(l, "Error: ") {
			errLines = append(errLines, l)
		}
	}
	if len(errLines) != 1 {
		t.Fatalf("got %d Error lines, want 1; stderr:\n%s", len(errLines), out)
	}
}
