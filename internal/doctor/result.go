// Package doctor implements `convergence doctor`: small independent checks
// that prove the local stack works end to end.
package doctor

import (
	"encoding/json"
	"fmt"
	"io"
)

// Status is the outcome of one check.
type Status string

const (
	Pass Status = "PASS"
	Warn Status = "WARN"
	Fail Status = "FAIL"
	Info Status = "INFO"
)

// Result is what every check returns.
type Result struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

// Summary counts results by status.
type Summary struct {
	Pass int  `json:"pass"`
	Warn int  `json:"warn"`
	Fail int  `json:"fail"`
	Info int  `json:"info"`
	OK   bool `json:"ok"`
}

// Summarize counts results; OK is false when any check FAILed.
func Summarize(rs []Result) Summary {
	var s Summary
	for _, r := range rs {
		switch r.Status {
		case Pass:
			s.Pass++
		case Warn:
			s.Warn++
		case Fail:
			s.Fail++
		case Info:
			s.Info++
		}
	}
	s.OK = s.Fail == 0
	return s
}

// ExitCode is 1 when any check FAILed, else 0 (WARN does not fail).
func ExitCode(rs []Result) int {
	if Summarize(rs).OK {
		return 0
	}
	return 1
}

// WriteText prints one line per check, a hint under each non-PASS, and a summary.
func WriteText(w io.Writer, rs []Result) {
	for _, r := range rs {
		_, _ = fmt.Fprintf(w, "%s %s - %s\n", r.Status, r.Name, r.Detail)
		if r.Hint != "" && r.Status != Pass {
			_, _ = fmt.Fprintf(w, "     hint: %s\n", r.Hint)
		}
	}
	s := Summarize(rs)
	_, _ = fmt.Fprintf(w, "\n%d passed, %d warnings, %d failed, %d info\n", s.Pass, s.Warn, s.Fail, s.Info)
}

// WriteJSON prints the machine-readable report.
func WriteJSON(w io.Writer, rs []Result) error {
	if rs == nil {
		rs = []Result{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Checks  []Result `json:"checks"`
		Summary Summary  `json:"summary"`
	}{rs, Summarize(rs)})
}
