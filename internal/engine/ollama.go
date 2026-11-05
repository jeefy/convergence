package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OllamaMissing queries <baseURL>/api/tags and returns the wanted models the
// server does not have. A bare name ("llama3") matches "llama3:latest".
// An error means Ollama was unreachable or answered nonsense.
func OllamaMissing(ctx context.Context, baseURL string, wanted []string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama unreachable at %s: %w", baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }() // read-only response body
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama at %s returned HTTP %d for /api/tags", baseURL, resp.StatusCode)
	}
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("decoding ollama /api/tags: %w", err)
	}
	have := map[string]bool{}
	for _, m := range tags.Models {
		have[m.Name] = true
	}
	var missing []string
	for _, w := range wanted {
		if have[w] || (!strings.Contains(w, ":") && have[w+":latest"]) {
			continue
		}
		missing = append(missing, w)
	}
	return missing, nil
}
