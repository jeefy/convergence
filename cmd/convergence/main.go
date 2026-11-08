package main

import (
	"os"

	"github.com/jeefy/convergence/internal/cli"
)

// exit is os.Exit, replaceable so tests can cover main.
var exit = os.Exit

func main() {
	exit(run())
}

// run executes the CLI and returns the process exit code. cobra already prints
// the returned error as "Error: ..." on stderr (SilenceErrors is not set), so
// run must not print it again.
func run() int {
	if err := cli.Execute(); err != nil {
		return 1
	}
	return 0
}
