package telemetry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Endpoints are the three metrics URLs.
type Endpoints struct {
	Gateway, StandIn, Pinball string
}

// DefaultEndpoints returns loopback URLs on the default ports.
func DefaultEndpoints() Endpoints { return NewEndpoints("127.0.0.1", 15020, 9102, 9201) }

// NewEndpoints builds endpoints for a bind host and ports.
func NewEndpoints(host string, gw, standin, pinball int) Endpoints {
	u := func(p int) string { return fmt.Sprintf("http://%s:%d/metrics", host, p) }
	return Endpoints{Gateway: u(gw), StandIn: u(standin), Pinball: u(pinball)}
}

// Scrape fetches and parses one Prometheus endpoint.
func Scrape(ctx context.Context, client *http.Client, url string) ([]Sample, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() // read-only response body
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return ParseProm(resp.Body)
}

// Section is the result for one source: either data or an error.
type Section struct {
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// Stats is the aggregated view.
type Stats struct {
	Gateway struct {
		Section
		Requests  float64            `json:"requests"`
		ByStatus  map[string]float64 `json:"by_status,omitempty"`
		Success   float64            `json:"success"`
		Errors    float64            `json:"errors"`
		TokensIn  *float64           `json:"tokens_in,omitempty"`
		TokensOut *float64           `json:"tokens_out,omitempty"`
	} `json:"gateway"`
	StandIn struct {
		Section
		Requests  float64 `json:"requests"`
		Tokenized float64 `json:"credentials_tokenized"`
		Restored  float64 `json:"credentials_restored"`
		Errors    float64 `json:"errors"`
	} `json:"standin"`
	Pinball struct {
		Section
		Decisions float64            `json:"decisions"`
		ByTier    map[string]float64 `json:"by_tier,omitempty"`
		ByModel   map[string]float64 `json:"by_model,omitempty"`
	} `json:"pinball"`
}

// AllDown reports whether every source failed.
func (s *Stats) AllDown() bool {
	return !s.Gateway.Available && !s.StandIn.Available && !s.Pinball.Available
}

// Collect scrapes all three sources, never failing on a single one.
func Collect(ctx context.Context, ep Endpoints) *Stats {
	client := &http.Client{Timeout: 3 * time.Second}
	st := &Stats{}
	if smp, err := Scrape(ctx, client, ep.Gateway); err != nil {
		st.Gateway.Error = errText(err)
	} else {
		fillGateway(st, smp)
	}
	if smp, err := Scrape(ctx, client, ep.StandIn); err != nil {
		st.StandIn.Error = errText(err)
	} else {
		fillStandIn(st, smp)
	}
	if smp, err := Scrape(ctx, client, ep.Pinball); err != nil {
		st.Pinball.Error = errText(err)
	} else {
		fillPinball(st, smp)
	}
	return st
}

func errText(err error) string {
	switch {
	case strings.Contains(err.Error(), "connection refused"):
		return "connection refused"
	case strings.Contains(err.Error(), "deadline exceeded"), strings.Contains(err.Error(), "Timeout"):
		return "timeout"
	}
	return err.Error()
}
