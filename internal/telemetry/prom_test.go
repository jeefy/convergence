package telemetry

import (
	"math"
	"strings"
	"testing"
)

const fixture = `# HELP standin_matches_total m
# TYPE standin_matches_total counter
standin_matches_total{pattern_index="0",tier="a"} 3.0
standin_matches_total{pattern_index="1",tier="b"} 2.0
standin_restored_total 3.0
standin_held_bytes_bucket{le="+Inf"} 3.0
weird{a="say \"hi\" \\ x",b="v"} 1e2
nan_metric NaN
inf_metric +Inf
agentgateway_requests_total{status="200",method="POST"} 5
agentgateway_requests_total{status="500",method="POST"} 1
pinball_decisions_total{model="m1",tier="light"} 4
pinball_decisions_total{model="m2",tier="deep"} 1
`

func TestParseProm_when_labels_and_special_values(t *testing.T) {
	// Given a fixture with labels, escapes, NaN, +Inf and histogram buckets
	s, err := ParseProm(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	// Then series are parsed and NaN/buckets dropped
	if got := Sum(s, "standin_matches_total", nil); got != 5 {
		t.Errorf("matches = %v, want 5", got)
	}
	if got := Sum(s, "standin_matches_total", map[string]string{"tier": "b"}); got != 2 {
		t.Errorf("tier b = %v, want 2", got)
	}
	if Has(s, "nan_metric") || Has(s, "standin_held_bytes_bucket") {
		t.Error("NaN and bucket series must be skipped")
	}
	if !Has(s, "inf_metric") || Sum(s, "inf_metric", nil) != 0 {
		t.Error("+Inf parsed but excluded from sums")
	}
	for _, x := range s {
		if x.Name == "weird" {
			if x.Labels["a"] != `say "hi" \ x` || x.Value != 100 {
				t.Errorf("weird = %#v", x)
			}
		}
	}
}

func TestSumBy_when_multiple_series(t *testing.T) {
	s, _ := ParseProm(strings.NewReader(fixture))
	m := SumBy(s, "pinball_decisions_total", "tier")
	if m["light"] != 4 || m["deep"] != 1 {
		t.Errorf("got %v", m)
	}
}

func TestParseProm_when_garbage(t *testing.T) {
	s, err := ParseProm(strings.NewReader("<html>nope</html>\nfoo{bar\nfoo{a=\"x} 1\nfoo abc\n{} 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 0 {
		t.Errorf("garbage yielded %v", s)
	}
}

func TestFill_when_metrics_missing(t *testing.T) {
	st := &Stats{}
	fillGateway(st, nil)
	fillStandIn(st, nil)
	fillPinball(st, nil)
	if st.Gateway.TokensIn != nil || st.Gateway.Requests != 0 || st.StandIn.Tokenized != 0 || st.Pinball.Decisions != 0 {
		t.Errorf("expected zero values: %+v", st)
	}
}

func TestFillGateway_when_success_and_error(t *testing.T) {
	s, _ := ParseProm(strings.NewReader(fixture +
		`agentgateway_gen_ai_client_token_usage_sum{gen_ai_token_type="input"} 21
agentgateway_gen_ai_client_token_usage_sum{gen_ai_token_type="output"} 15
agentgateway_request_duration_seconds{quantile="0.5"} 0.045
agentgateway_request_duration_seconds{quantile="0.9"} 0.120
agentgateway_request_duration_seconds{quantile="0.99"} 0.350
`))
	st := &Stats{}
	fillGateway(st, s)
	if st.Gateway.Requests != 6 || st.Gateway.Success != 5 || st.Gateway.Errors != 1 {
		t.Errorf("gateway = %+v", st.Gateway)
	}
	if st.Gateway.TokensIn == nil || *st.Gateway.TokensIn != 21 || *st.Gateway.TokensOut != 15 {
		t.Errorf("tokens = %+v", st.Gateway)
	}
	if st.Gateway.Latency == nil || st.Gateway.Latency.P50 != 45 || st.Gateway.Latency.P90 != 120 || st.Gateway.Latency.P99 != 350 {
		t.Errorf("latency = %+v", st.Gateway.Latency)
	}
	if math.IsNaN(st.Gateway.Requests) {
		t.Error("nan")
	}
}

func TestCollect_when_all_down(t *testing.T) {
	ep := NewEndpoints("127.0.0.1", 1, 1, 1)
	st := Collect(t.Context(), ep)
	if !st.AllDown() || st.Gateway.Error != "connection refused" {
		t.Errorf("got %+v", st.Gateway)
	}
}
