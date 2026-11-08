package engine

import (
	"context"
	"strings"
	"testing"
)

func TestPrune_runsSystemPrune(t *testing.T) {
	fakeBins(t, "docker")
	e := &ComposeEngine{binary: "docker"}
	if err := e.Prune(context.Background(), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if got := logOf(t); !strings.Contains(got, "system prune -f") {
		t.Fatalf("log = %q", got)
	}
}

func TestPrune_wrapsFailureWithOutput(t *testing.T) {
	fakeBins(t, "docker")
	t.Setenv("FAKE_FAIL", "prune")
	e := &ComposeEngine{binary: "docker"}
	err := e.Prune(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}
