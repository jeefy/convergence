package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractPromptAndCompletionText(t *testing.T) {
	// OpenAI messages style
	promptJSON := []any{
		map[string]any{"role": "system", "content": "You are a helpful assistant."},
		map[string]any{"role": "user", "content": "Explain goroutines"},
	}
	extractedPrompt := ExtractPromptText(promptJSON)
	if extractedPrompt != "Explain goroutines" {
		t.Fatalf("expected 'Explain goroutines', got %q", extractedPrompt)
	}

	// Multi-part content
	multiPartJSON := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "Part 1"},
					map[string]any{"type": "text", "text": "Part 2"},
				},
			},
		},
	}
	extractedMulti := ExtractPromptText(multiPartJSON)
	if !strings.Contains(extractedMulti, "Part 1") || !strings.Contains(extractedMulti, "Part 2") {
		t.Fatalf("expected multi-part prompt text, got %q", extractedMulti)
	}

	// Completion
	compJSON := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"role":    "assistant",
					"content": "Goroutines are lightweight threads.",
				},
			},
		},
	}
	extractedComp := ExtractCompletionText(compJSON)
	if extractedComp != "Goroutines are lightweight threads." {
		t.Fatalf("expected completion text, got %q", extractedComp)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	a := Analyze(nil, "7d")
	if a.TotalRequests != 0 {
		t.Fatalf("expected 0 requests, got %d", a.TotalRequests)
	}
	if len(a.Findings) == 0 {
		t.Fatalf("expected findings for empty activity")
	}
}

func TestAnalyzePatterns(t *testing.T) {
	now := time.Now()
	records := []RequestRecord{
		{
			ID:                 "req-1",
			StartedAt:          now.Add(-10 * time.Minute),
			CompletedAt:        now.Add(-10*time.Minute + 500*time.Millisecond),
			DurationMS:         500,
			HTTPStatus:         200,
			GenAIRequestModel:  "claude-opus-5.5",
			GenAIResponseModel: "claude-opus-5.5",
			PinballTier:        "heavy",
			InputTokens:        100,
			OutputTokens:       50,
			TotalTokens:        150,
			UserPromptPreview:  "Short prompt that triggered heavy tier",
		},
		{
			ID:                 "req-2",
			StartedAt:          now.Add(-5 * time.Minute),
			CompletedAt:        now.Add(-5*time.Minute + 2000*time.Millisecond),
			DurationMS:         2000,
			HTTPStatus:         200,
			GenAIRequestModel:  "claude-sonnet-5.5",
			GenAIResponseModel: "claude-sonnet-5.5",
			PinballTier:        "standard",
			InputTokens:        25000,
			OutputTokens:       80,
			TotalTokens:        25080,
			UserPromptPreview:  "Massive prompt dump...",
		},
		{
			ID:                 "req-3",
			StartedAt:          now.Add(-5*time.Minute + 2100*time.Millisecond),
			CompletedAt:        now.Add(-5*time.Minute + 3000*time.Millisecond),
			DurationMS:         900,
			HTTPStatus:         200,
			GenAIRequestModel:  "claude-sonnet-5.5",
			GenAIResponseModel: "claude-sonnet-5.5",
			PinballTier:        "standard",
			InputTokens:        25000,
			OutputTokens:       80,
			TotalTokens:        25080,
			UserPromptPreview:  "Massive prompt dump...", // exact retry within 100ms
		},
		{
			ID:          "req-4",
			StartedAt:   now.Add(-1 * time.Minute),
			CompletedAt: now.Add(-1*time.Minute + 100*time.Millisecond),
			DurationMS:  100,
			HTTPStatus:  500,
		},
	}

	a := Analyze(records, "7d")
	if a.TotalRequests != 4 {
		t.Fatalf("expected 4 requests, got %d", a.TotalRequests)
	}
	if a.SuccessfulRequests != 3 || a.FailedRequests != 1 {
		t.Fatalf("expected 3 success and 1 fail, got %d and %d", a.SuccessfulRequests, a.FailedRequests)
	}
	if a.HeavyOnSimpleCount != 1 {
		t.Fatalf("expected 1 heavy on simple query, got %d", a.HeavyOnSimpleCount)
	}
	if a.ContextBloatCount != 2 {
		t.Fatalf("expected 2 context bloats, got %d", a.ContextBloatCount)
	}
	if a.RapidRetryCount != 1 {
		t.Fatalf("expected 1 rapid retry, got %d", a.RapidRetryCount)
	}

	var buf bytes.Buffer
	if err := PrintAssessment(&buf, &a, "Test coaching advice"); err != nil {
		t.Fatalf("PrintAssessment failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Convergence Usage Insights") {
		t.Fatalf("expected header in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Test coaching advice") {
		t.Fatalf("expected coach advice in output, got:\n%s", out)
	}

	buf.Reset()
	if err := PrintJSON(&buf, &a, "Test coaching advice"); err != nil {
		t.Fatalf("PrintJSON failed: %v", err)
	}
	var js map[string]any
	if err := json.Unmarshal(buf.Bytes(), &js); err != nil {
		t.Fatalf("PrintJSON invalid json: %v", err)
	}
	if js["window"] != "7d" {
		t.Fatalf("expected window 7d in JSON, got %v", js["window"])
	}
}

func TestGenerateCoachAdvice(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "- Optimize prompt sizes\n- Balance tier selection",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	assessment := Assessment{
		Window:        "7d",
		TotalRequests: 10,
		Tiers:         map[string]int{"standard": 8, "heavy": 2},
		Models:        map[string]int{"claude-sonnet-5.5": 8, "claude-opus-5.5": 2},
	}

	advice, err := GenerateCoachAdvice(context.Background(), ts.URL, &assessment)
	if err != nil {
		t.Fatalf("GenerateCoachAdvice failed: %v", err)
	}
	if !strings.Contains(advice, "Optimize prompt sizes") {
		t.Fatalf("unexpected advice: %s", advice)
	}
}
