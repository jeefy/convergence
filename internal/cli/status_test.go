package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jeefy/convergence/internal/health"
)

func TestWriteStatus_jsonAndBanner(t *testing.T) {
	r := statusReport{Provider: "ollama", Services: []health.ServiceStatus{
		{Name: "agentgateway", Running: true, Healthy: true, Required: true},
		{Name: "stand-in", Container: "not running", Required: true},
	}}
	var b bytes.Buffer
	if err := writeStatus(&b, r, true); err != nil {
		t.Fatal(err)
	}
	var got statusReport
	if err := json.Unmarshal(b.Bytes(), &got); err != nil || len(got.Services) != 2 || got.Services[1].Healthy {
		t.Fatalf("bad json: %v %s", err, b.String())
	}
	b.Reset()
	_ = writeStatus(&b, r, false)
	if !strings.Contains(b.String(), "secret scrubbing is NOT active") || !strings.Contains(b.String(), "REJECTED") {
		t.Errorf("no banner:\n%s", b.String())
	}
}
