package telemetry

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func fillGateway(st *Stats, s []Sample) {
	g := &st.Gateway
	g.Available = true
	g.ByStatus = map[string]float64{}
	for _, x := range s {
		if x.Name != GWRequests {
			continue
		}
		code := x.Labels["status"]
		g.ByStatus[code] += x.Value
		g.Requests += x.Value
		if strings.HasPrefix(code, "2") || strings.HasPrefix(code, "3") {
			g.Success += x.Value
		} else {
			g.Errors += x.Value
		}
	}
	if Has(s, GWTokens) {
		in := Sum(s, GWTokens, map[string]string{GWTokenTypeLabel: "input"})
		out := Sum(s, GWTokens, map[string]string{GWTokenTypeLabel: "output"})
		g.TokensIn, g.TokensOut = &in, &out
	}
	for _, x := range s {
		if strings.HasPrefix(x.Name, GWLatency) {
			if qStr, ok := x.Labels["quantile"]; ok {
				if g.Latency == nil {
					g.Latency = &LatencySummary{}
				}
				ms := x.Value * 1000
				switch qStr {
				case "0.5":
					g.Latency.P50 = ms
				case "0.9":
					g.Latency.P90 = ms
				case "0.99":
					g.Latency.P99 = ms
				}
			}
		}
	}
}

func fillStandIn(st *Stats, s []Sample) {
	v := &st.StandIn
	v.Available = true
	v.Requests = Sum(s, "standin_requests_total", nil)
	v.Tokenized = Sum(s, "standin_matches_total", nil)
	v.Restored = Sum(s, "standin_restored_total", nil)
	v.Errors = Sum(s, "standin_errors_total", nil)
}

func fillPinball(st *Stats, s []Sample) {
	p := &st.Pinball
	p.Available = true
	p.Decisions = Sum(s, "pinball_decisions_total", nil)
	p.ByTier = SumBy(s, "pinball_decisions_total", "tier")
	p.ByModel = SumBy(s, "pinball_decisions_total", "model")
}

func line(w io.Writer, format string, a ...any) { _, _ = fmt.Fprintf(w, format+"\n", a...) }

func unavailable(w io.Writer, sec Section) {
	line(w, "  unavailable (%s)", sec.Error)
}

func breakdown(w io.Writer, title string, m map[string]float64) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	line(w, "  %s:", title)
	for _, k := range keys {
		name := k
		if name == "" {
			name = "(none)"
		}
		line(w, "    %-28s %.0f", name, m[k])
	}
}

// Print renders stats as human-readable text.
func Print(st *Stats, w io.Writer) {
	line(w, "AgentGateway")
	if !st.Gateway.Available {
		unavailable(w, st.Gateway.Section)
	} else {
		line(w, "  Requests:   %.0f (success %.0f, error %.0f)", st.Gateway.Requests, st.Gateway.Success, st.Gateway.Errors)
		if st.Gateway.TokensIn != nil {
			line(w, "  Tokens in:  %.0f", *st.Gateway.TokensIn)
			line(w, "  Tokens out: %.0f", *st.Gateway.TokensOut)
		} else {
			line(w, "  Tokens:     not reported yet")
		}
		if st.Gateway.Latency != nil {
			line(w, "  Latency:    p50: %.1fms, p90: %.1fms, p99: %.1fms", st.Gateway.Latency.P50, st.Gateway.Latency.P90, st.Gateway.Latency.P99)
		}
	}
	line(w, "Stand-In")
	if !st.StandIn.Available {
		unavailable(w, st.StandIn.Section)
	} else {
		line(w, "  Requests inspected:     %.0f", st.StandIn.Requests)
		line(w, "  Credentials tokenized:  %.0f", st.StandIn.Tokenized)
		line(w, "  Credentials restored:   %.0f", st.StandIn.Restored)
		line(w, "  Errors:                 %.0f", st.StandIn.Errors)
	}
	line(w, "Pinball")
	if !st.Pinball.Available {
		unavailable(w, st.Pinball.Section)
	} else {
		line(w, "  Decisions: %.0f", st.Pinball.Decisions)
		breakdown(w, "By tier", st.Pinball.ByTier)
		breakdown(w, "By model", st.Pinball.ByModel)
	}
}
