package doctor

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const alnum = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// FakeGitHubToken returns "ghp_" + 36 random alphanumerics.
func FakeGitHubToken() (string, error) {
	var b strings.Builder
	b.WriteString("ghp_")
	max := big.NewInt(int64(len(alnum)))
	for range 36 {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(alnum[n.Int64()])
	}
	return b.String(), nil
}

// CounterDeltaProblems lists the counters that did not increase.
func CounterDeltaProblems(before, after Counters) []string {
	var p []string
	if after.Matches <= before.Matches {
		p = append(p, fmt.Sprintf("standin_matches_total did not increase (%.0f -> %.0f)", before.Matches, after.Matches))
	}
	if after.Restored <= before.Restored {
		p = append(p, fmt.Sprintf("standin_restored_total did not increase (%.0f -> %.0f)", before.Restored, after.Restored))
	}
	if after.Decisions <= before.Decisions {
		p = append(p, fmt.Sprintf("pinball_decisions_total did not increase (%.0f -> %.0f)", before.Decisions, after.Decisions))
	}
	return p
}

// LeakedPlaceholder reports whether a Stand-In placeholder reached the client.
func LeakedPlaceholder(body string) bool { return strings.Contains(body, "<<secret-") }

// RoutedModel extracts the "model" the gateway reports, if any.
func RoutedModel(body []byte) string {
	var r struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &r) != nil {
		return ""
	}
	return r.Model
}

// CheckE2E sends one tiny real request through the gateway and asserts the
// counters moved and no placeholder leaked back.
func CheckE2E(ctx context.Context, e *Env) Result {
	const name = "e2e round trip"
	token, err := FakeGitHubToken()
	if err != nil {
		return Result{name, Fail, "cannot generate token: " + err.Error(), ""}
	}
	before, err := Snapshot(ctx, e)
	if err != nil {
		return Result{name, Fail, "before-snapshot failed: " + shortErr(err), "run 'convergence start'"}
	}
	body, _ := json.Marshal(map[string]any{
		"model":      "auto",
		"max_tokens": 16,
		"messages":   []map[string]string{{"role": "user", "content": "my token is " + token + " . Reply with the word OK"}},
	})
	code, raw, err := e.do(ctx, "POST", e.GatewayURL+"/v1/chat/completions", "application/json", string(body))
	if err != nil {
		return Result{name, Fail, "gateway request failed: " + shortErr(err), "run 'convergence logs agentgateway'"}
	}
	if code != 200 {
		return Result{name, Fail, fmt.Sprintf("gateway returned HTTP %d: %s", code, trunc(string(raw), 160)),
			"check provider credentials/reachability ('convergence logs agentgateway')"}
	}
	if LeakedPlaceholder(string(raw)) {
		return Result{name, Fail, "response contains an unrestored <<secret- placeholder", "Stand-In response restoration is broken; 'convergence logs stand-in'"}
	}
	// counters are updated asynchronously; allow a short settle
	var after Counters
	var problems []string
	for range 5 {
		if after, err = Snapshot(ctx, e); err != nil {
			return Result{name, Fail, "after-snapshot failed: " + shortErr(err), ""}
		}
		if problems = CounterDeltaProblems(before, after); len(problems) == 0 {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	if len(problems) > 0 {
		return Result{name, Fail, strings.Join(problems, "; "), "the request did not traverse Stand-In/Pinball; check agentgateway.yaml extProc/extAuthz and logs"}
	}
	detail := fmt.Sprintf("matches +%.0f, restored +%.0f, decisions +%.0f; no placeholder leaked",
		after.Matches-before.Matches, after.Restored-before.Restored, after.Decisions-before.Decisions)
	if m := RoutedModel(raw); m != "" {
		detail += "; routed model=" + m
	}
	return Result{name, Pass, detail, ""}
}
