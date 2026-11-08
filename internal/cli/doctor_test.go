package cli

import (
	"context"
	"testing"

	"github.com/jeefy/convergence/internal/doctor"
)

func TestRunDoctor_noConfigFailsFast(t *testing.T) {
	newFakeEnv(t)
	res := runDoctor(context.Background())
	if len(res) == 0 {
		t.Fatal("no results")
	}
	if doctor.ExitCode(res) == 0 {
		t.Errorf("doctor without a config must fail: %+v", res)
	}
}
