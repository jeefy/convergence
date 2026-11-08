package clientconfig

import (
	"testing"
	"time"
)

// Malformed input must fail or finish, never spin: a stray '}' or ']' inside
// an array made scanner.value consume nothing and loop forever.
func TestScannerValueTerminatesOnMalformedInput(t *testing.T) {
	for _, in := range []string{
		`{"a": [1, 2}`,
		`{"a": [}`,
		`{"a": [1, }`,
		`[1, 2}`,
		`{"a": [1, /`,
		`{"a": [ ) ]}`,
	} {
		t.Run(in, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				s := &scanner{b: []byte(in)}
				_ = s.value()
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("scanner.value did not terminate on %q", in)
			}
		})
	}
}
