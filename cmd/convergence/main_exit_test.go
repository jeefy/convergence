package main

import (
	"os"
	"testing"
)

func TestMain_exitsWithRunCode(t *testing.T) {
	origArgs, origExit := os.Args, exit
	t.Cleanup(func() { os.Args, exit = origArgs, origExit })
	os.Args = []string{"convergence", "--help"}
	got := -1
	exit = func(code int) { got = code }

	main()

	if got != 0 {
		t.Fatalf("exit code = %d, want 0", got)
	}
}

func TestRun_returnsZeroOnSuccess(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"convergence", "--help"}
	if code := run(); code != 0 {
		t.Fatalf("code = %d", code)
	}
}
