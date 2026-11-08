package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GenerateCoachAdvice calls the Convergence gateway's chat completion endpoint to generate coaching tips.
func GenerateCoachAdvice(ctx context.Context, gatewayURL string, a *Assessment) (string, error) {
	if gatewayURL == "" {
		gatewayURL = "http://127.0.0.1:3000"
	}
	endpoint := strings.TrimRight(gatewayURL, "/") + "/v1/chat/completions"

	systemPrompt := `You are an AI Coding Agent Coach and LLM Cost & Performance Optimizer.
Your job is to analyze agent usage patterns and provide concise, actionable coaching to help the user:
1. Improve prompt hygiene and agent effectiveness.
2. Reduce unnecessary token waste and avoid context bloat.
3. Optimize tier routing and model selection.
Keep recommendations realistic, factual, and strictly under 5 bullet points. Do not make up cost savings or numbers.`

	var promptBuf strings.Builder
	fmt.Fprintf(&promptBuf, "User activity over window: %s\n", a.Window)
	fmt.Fprintf(&promptBuf, "- Total requests: %d (Success: %d, Fail: %d)\n", a.TotalRequests, a.SuccessfulRequests, a.FailedRequests)
	fmt.Fprintf(&promptBuf, "- Total tokens: %d (Input: %d, Output: %d)\n", a.TotalTokens, a.InputTokens, a.OutputTokens)
	fmt.Fprintf(&promptBuf, "- Average request latency: %d ms\n", a.AvgDurationMS)
	promptBuf.WriteString("- Tier distribution: ")
	for t, c := range a.Tiers {
		fmt.Fprintf(&promptBuf, "%s=%d ", t, c)
	}
	promptBuf.WriteString("\n- Model distribution: ")
	for m, c := range a.Models {
		fmt.Fprintf(&promptBuf, "%s=%d ", m, c)
	}
	promptBuf.WriteString("\n\nObserved patterns:\n")
	if a.ContextBloatCount > 0 {
		fmt.Fprintf(&promptBuf, "- %d requests had large context (>20k tokens) with tiny outputs (<150 tokens).\n", a.ContextBloatCount)
	}
	if a.HeavyOnSimpleCount > 0 {
		fmt.Fprintf(&promptBuf, "- %d requests triggered heavy tier on very short prompts (<200 chars).\n", a.HeavyOnSimpleCount)
	}
	if a.RapidRetryCount > 0 {
		fmt.Fprintf(&promptBuf, "- %d rapid retries detected within 5 seconds.\n", a.RapidRetryCount)
	}

	if len(a.SampledPrompts) > 0 {
		promptBuf.WriteString("\nSample prompt previews:\n")
		for i, sp := range a.SampledPrompts {
			fmt.Fprintf(&promptBuf, "%d. [%s/%s] %s\n", i+1, sp.Tier, sp.Model, sp.UserPrompt)
		}
	}

	promptBuf.WriteString("\nProvide your assessment and specific coaching advice:")

	reqBody := map[string]any{
		"model": "auto",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": promptBuf.String()},
		},
		"temperature": 0.3,
		"max_tokens":  600,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling coach request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("creating coach request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending coach request to gateway: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading coach response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gateway returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshaling coach response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no completion returned by coach model")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}
