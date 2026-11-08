package telemetry

import (
	"bytes"
	"strings"
	"testing"
)

func TestDefaultEndpoints(t *testing.T) {
	e := DefaultEndpoints()
	if e.Gateway != "http://127.0.0.1:15020/metrics" || e.StandIn != "http://127.0.0.1:9102/metrics" || e.Pinball != "http://127.0.0.1:9201/metrics" {
		t.Errorf("unexpected defaults: %+v", e)
	}
}

func TestPrint_when_available_and_unavailable(t *testing.T) {
	var st Stats
	st.Gateway.Available = true
	st.Gateway.Requests, st.Gateway.Success, st.Gateway.Errors = 6, 5, 1
	in, out := 10.0, 20.0
	st.Gateway.TokensIn, st.Gateway.TokensOut = &in, &out
	st.Gateway.Latency = &LatencySummary{P50: 1, P90: 2, P99: 3}
	st.StandIn.Available = true
	st.StandIn.Tokenized = 3
	st.Pinball.Available = true
	st.Pinball.Decisions = 5
	st.Pinball.ByTier = map[string]float64{"light": 4, "": 1}
	var b bytes.Buffer
	Print(&st, &b)
	for _, want := range []string{"Requests:   6 (success 5, error 1)", "Tokens in:  10", "p99: 3.0ms", "Credentials tokenized:  3", "By tier:", "(none)", "light"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %q in:\n%s", want, b.String())
		}
	}

	var down Stats
	down.Gateway.Error = "refused"
	down.StandIn.Error = "refused"
	down.Pinball.Error = "refused"
	b.Reset()
	Print(&down, &b)
	if strings.Count(b.String(), "unavailable (refused)") != 3 {
		t.Errorf("want 3 unavailable sections:\n%s", b.String())
	}

	st.Gateway.TokensIn = nil
	b.Reset()
	Print(&st, &b)
	if !strings.Contains(b.String(), "not reported yet") {
		t.Errorf("missing tokens fallback:\n%s", b.String())
	}
}
