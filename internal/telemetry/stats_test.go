package telemetry

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAccessLogs(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "access.jsonl")

	sampleLogs := `{"http.status":200,"gen_ai.usage.input_tokens":1000,"gen_ai.usage.cache_read.input_tokens":500,"gen_ai.usage.output_tokens":50,"agw.ai.usage.cost.total":0.005,"gen_ai.response.model":"claude-haiku-5.5","pinball.tier":"light","st_tokenized":2}
{"http.status":200,"gen_ai.usage.input_tokens":5000,"gen_ai.usage.cache_read.input_tokens":2000,"gen_ai.usage.output_tokens":200,"agw.ai.usage.cost.total":0.025,"gen_ai.response.model":"claude-opus-5.5","pinball.tier":"deep","st_tokenized":1}
`
	if err := os.WriteFile(logFile, []byte(sampleLogs), 0644); err != nil {
		t.Fatalf("writing sample log: %v", err)
	}

	summary, err := ParseAccessLogs(tempDir)
	if err != nil {
		t.Fatalf("ParseAccessLogs failed: %v", err)
	}

	if summary.TotalRequests != 2 {
		t.Errorf("expected 2 requests, got %d", summary.TotalRequests)
	}
	if summary.TotalInput != 6000 {
		t.Errorf("expected 6000 input tokens, got %d", summary.TotalInput)
	}
	if summary.TotalCache != 2500 {
		t.Errorf("expected 2500 cache tokens, got %d", summary.TotalCache)
	}
	if summary.TotalOutput != 250 {
		t.Errorf("expected 250 output tokens, got %d", summary.TotalOutput)
	}
	if summary.TotalSecrets != 3 {
		t.Errorf("expected 3 secrets scrubbed, got %d", summary.TotalSecrets)
	}
	if summary.TierCounts["light"] != 1 || summary.TierCounts["deep"] != 1 {
		t.Errorf("unexpected tier counts: %+v", summary.TierCounts)
	}

	var buf bytes.Buffer
	PrintStats(summary, &buf)
	out := buf.String()
	if !strings.Contains(out, "Estimated Cost:   $0.0300 USD") {
		t.Errorf("expected cost output, got: %s", out)
	}
}
