package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AccessLogEntry struct {
	Timestamp      string  `json:"timestamp"`
	HTTPStatus     int     `json:"http.status"`
	DurationMs     float64 `json:"duration"`
	InputTokens    int     `json:"gen_ai.usage.input_tokens"`
	CacheTokens    int     `json:"gen_ai.usage.cache_read.input_tokens"`
	OutputTokens   int     `json:"gen_ai.usage.output_tokens"`
	CostUSD        float64 `json:"agw.ai.usage.cost.total"`
	RequestModel   string  `json:"gen_ai.request.model"`
	ResponseModel  string  `json:"gen_ai.response.model"`
	PinballTier    string  `json:"pinball.tier"`
	PinballModel   string  `json:"pinball.model"`
	StandinMatches int     `json:"st_tokenized"`
}

type StatsSummary struct {
	TotalRequests int
	SuccessCount  int
	ErrorCount    int
	TotalInput    int
	TotalCache    int
	TotalOutput   int
	TotalCostUSD  float64
	TierCounts    map[string]int
	ModelCounts   map[string]int
	TotalSecrets  int
}

func NewStatsSummary() *StatsSummary {
	return &StatsSummary{
		TierCounts:  make(map[string]int),
		ModelCounts: make(map[string]int),
	}
}

func ParseAccessLogs(logDir string) (*StatsSummary, error) {
	summary := NewStatsSummary()
	logPath := filepath.Join(logDir, "access.jsonl")

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return summary, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry AccessLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		summary.TotalRequests++
		if entry.HTTPStatus >= 200 && entry.HTTPStatus < 400 {
			summary.SuccessCount++
		} else {
			summary.ErrorCount++
		}

		summary.TotalInput += entry.InputTokens
		summary.TotalCache += entry.CacheTokens
		summary.TotalOutput += entry.OutputTokens
		summary.TotalCostUSD += entry.CostUSD
		summary.TotalSecrets += entry.StandinMatches

		if entry.PinballTier != "" {
			summary.TierCounts[entry.PinballTier]++
		}
		if entry.ResponseModel != "" {
			summary.ModelCounts[entry.ResponseModel]++
		}
	}

	return summary, scanner.Err()
}

func FetchPrometheusMetrics(adminPort int) (map[string]float64, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	url := fmt.Sprintf("http://localhost:%d/stats/prometheus", adminPort)

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	metrics := make(map[string]float64)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			if idx := strings.Index(name, "{"); idx != -1 {
				name = name[:idx]
			}
			val, err := strconv.ParseFloat(parts[1], 64)
			if err == nil {
				metrics[name] += val
			}
		}
	}
	return metrics, nil
}

func PrintStats(summary *StatsSummary, out io.Writer) {
	fmt.Fprintln(out, "================ Convergence Usage & Telemetry ================")
	fmt.Fprintf(out, "Total Requests:   %d (Success: %d, Errors: %d)\n", summary.TotalRequests, summary.SuccessCount, summary.ErrorCount)
	fmt.Fprintf(out, "Estimated Cost:   $%.4f USD\n", summary.TotalCostUSD)

	totalTokens := summary.TotalInput + summary.TotalCache + summary.TotalOutput
	cacheRatio := 0.0
	if summary.TotalInput+summary.TotalCache > 0 {
		cacheRatio = float64(summary.TotalCache) / float64(summary.TotalInput+summary.TotalCache) * 100
	}

	fmt.Fprintf(out, "Total Tokens:     %d\n", totalTokens)
	fmt.Fprintf(out, "  - Input:        %d\n", summary.TotalInput)
	fmt.Fprintf(out, "  - Cache Read:   %d (%.1f%% cache hit rate)\n", summary.TotalCache, cacheRatio)
	fmt.Fprintf(out, "  - Output:       %d\n", summary.TotalOutput)

	fmt.Fprintln(out, "\n--- Stand-In Privacy Protection ---")
	fmt.Fprintf(out, "Secrets Scrubbed: %d credential strings tokenized/restored\n", summary.TotalSecrets)

	fmt.Fprintln(out, "\n--- Pinball Routing Breakdown ---")
	if len(summary.TierCounts) == 0 {
		fmt.Fprintln(out, "No routed turns recorded yet.")
	} else {
		for tier, count := range summary.TierCounts {
			fmt.Fprintf(out, "  - %-12s: %d decisions\n", tier, count)
		}
	}

	if len(summary.ModelCounts) > 0 {
		fmt.Fprintln(out, "\n--- Models Served ---")
		for model, count := range summary.ModelCounts {
			fmt.Fprintf(out, "  - %-20s: %d turns\n", model, count)
		}
	}
	fmt.Fprintln(out, "===============================================================")
}
