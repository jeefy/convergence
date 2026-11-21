// Package testhome points os.UserHomeDir at a scratch directory for the
// duration of a test on every platform.
package testhome

import (
	"path/filepath"
	"testing"
)

// Set makes dir the home directory for the rest of the test. It sets both
// HOME (Unix) and USERPROFILE (Windows) so tests never write into the real
// profile of whoever runs the suite, and pins HERMES_HOME to dir/.hermes so
// fixtures that expect the Unix layout also hold on Windows, where the
// default is %LOCALAPPDATA%\hermes. PASEO_HOME is pinned to dir/.paseo so a
// suite run inside a Paseo agent does not write to the real daemon config, and
// PI_CODING_AGENT_DIR to dir/.omp/agent for the same reason with omp.
func Set(t testing.TB, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HERMES_HOME", filepath.Join(dir, ".hermes"))
	t.Setenv("PASEO_HOME", filepath.Join(dir, ".paseo"))
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(dir, ".omp", "agent"))
}

// New creates a fresh temporary directory, makes it the home directory and
// returns it.
func New(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	Set(t, dir)
	return dir
}
