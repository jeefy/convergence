package insights

import (
	"sort"
	"strings"
	"time"
)

// Assessment summarizes usage statistics, health indicators, and behavioral patterns.
type Assessment struct {
	Window             string          `json:"window"`
	TotalRequests      int             `json:"total_requests"`
	SuccessfulRequests int             `json:"successful_requests"`
	FailedRequests     int             `json:"failed_requests"`
	TotalTokens        int64           `json:"total_tokens"`
	InputTokens        int64           `json:"input_tokens"`
	OutputTokens       int64           `json:"output_tokens"`
	AvgDurationMS      int64           `json:"avg_duration_ms"`
	Tiers              map[string]int  `json:"tiers"`
	Models             map[string]int  `json:"models"`
	Providers          map[string]int  `json:"providers"`
	ContextBloatCount  int             `json:"context_bloat_count"`
	RapidRetryCount    int             `json:"rapid_retry_count"`
	HeavyOnSimpleCount int             `json:"heavy_on_simple_count"`
	Findings           []Finding       `json:"findings"`
	SampledPrompts     []SampledPrompt `json:"sampled_prompts,omitempty"`
}

// Finding represents an actionable observation or pattern detected in the window.
type Finding struct {
	Level       string `json:"level"`    // "info", "warning", "tip"
	Category    string `json:"category"` // "tiering", "context", "reliability", "prompting"
	Title       string `json:"title"`
	Description string `json:"description"`
}

// SampledPrompt holds a representative prompt preview with metadata for coaching.
type SampledPrompt struct {
	Model      string `json:"model"`
	Tier       string `json:"tier"`
	TokensIn   int64  `json:"tokens_in"`
	TokensOut  int64  `json:"tokens_out"`
	UserPrompt string `json:"user_prompt"`
}

// Analyze processes raw records and produces an Assessment.
func Analyze(records []RequestRecord, window string) Assessment {
	a := Assessment{
		Window:    window,
		Tiers:     make(map[string]int),
		Models:    make(map[string]int),
		Providers: make(map[string]int),
	}

	a.TotalRequests = len(records)
	if a.TotalRequests == 0 {
		a.Findings = append(a.Findings, Finding{
			Level:       "info",
			Category:    "usage",
			Title:       "No activity recorded",
			Description: "No request log records found in the specified window.",
		})
		return a
	}

	var totalDuration int64
	var prevRecord *RequestRecord

	for i := range records {
		r := &records[i]
		if r.HTTPStatus >= 200 && r.HTTPStatus < 400 && r.Error == nil {
			a.SuccessfulRequests++
		} else {
			a.FailedRequests++
		}

		a.TotalTokens += r.TotalTokens
		a.InputTokens += r.InputTokens
		a.OutputTokens += r.OutputTokens
		totalDuration += r.DurationMS

		// Tiers
		tier := r.PinballTier
		if tier == "" {
			tier = "unrouted"
		}
		a.Tiers[tier]++

		// Models
		model := r.GenAIResponseModel
		if model == "" {
			model = r.GenAIRequestModel
		}
		if model == "" {
			model = "unknown"
		}
		a.Models[model]++

		// Providers
		provider := r.GenAIProviderName
		if provider == "" {
			provider = "default"
		}
		a.Providers[provider]++

		// Pattern 1: Context Bloat (massive input with tiny output)
		if r.InputTokens > 20000 && r.OutputTokens < 150 {
			a.ContextBloatCount++
		}

		// Pattern 2: Heavy tier on simple/short queries
		if r.PinballTier == "heavy" && r.InputTokens < 500 && len(r.UserPromptPreview) < 200 {
			a.HeavyOnSimpleCount++
		}

		// Pattern 3: Rapid retries (sub-5s repeats or errors)
		if prevRecord != nil {
			timeDiff := r.StartedAt.Sub(prevRecord.CompletedAt)
			if timeDiff >= 0 && timeDiff < 5*time.Second {
				if r.UserPromptPreview != "" && prevRecord.UserPromptPreview != "" &&
					strings.EqualFold(strings.TrimSpace(r.UserPromptPreview), strings.TrimSpace(prevRecord.UserPromptPreview)) {
					a.RapidRetryCount++
				}
			}
		}
		prevRecord = r

		// Sample interesting prompts (long prompts or heavy tier)
		if len(a.SampledPrompts) < 5 && r.UserPromptPreview != "" {
			preview := r.UserPromptPreview
			if len(preview) > 300 {
				preview = preview[:300] + "..."
			}
			a.SampledPrompts = append(a.SampledPrompts, SampledPrompt{
				Model:      model,
				Tier:       tier,
				TokensIn:   r.InputTokens,
				TokensOut:  r.OutputTokens,
				UserPrompt: preview,
			})
		}
	}

	a.AvgDurationMS = totalDuration / int64(a.TotalRequests)

	// Build Findings
	// 1. Success rate
	if a.FailedRequests > 0 {
		failPct := (float64(a.FailedRequests) / float64(a.TotalRequests)) * 100
		if failPct > 10.0 {
			a.Findings = append(a.Findings, Finding{
				Level:       "warning",
				Category:    "reliability",
				Title:       "High failure rate detected",
				Description: strings.TrimSpace(strings.ReplaceAll("Failure rate is currently at XX%, check provider logs or upstream rate limits.", "XX%", "")) + string(rune(0)),
			})
		}
	}

	// 2. Tier usage
	heavyCount := a.Tiers["heavy"]
	lightCount := a.Tiers["light"]
	standardCount := a.Tiers["standard"]
	totalRouted := heavyCount + lightCount + standardCount

	if totalRouted > 0 {
		heavyRatio := float64(heavyCount) / float64(totalRouted)
		if heavyRatio > 0.6 {
			a.Findings = append(a.Findings, Finding{
				Level:       "tip",
				Category:    "tiering",
				Title:       "Heavy tier dominant (>60% of routed turns)",
				Description: "Most requests are routed to heavy-tier models. Consider tuning Pinball thresholds or breaking tasks into smaller steps to route routine turns to light/standard tiers.",
			})
		}
		if lightCount == 0 && totalRouted > 20 {
			a.Findings = append(a.Findings, Finding{
				Level:       "tip",
				Category:    "tiering",
				Title:       "Zero light-tier utilization",
				Description: "No requests qualified for the light tier. If your agents send tools or long system prompts, consider tuning `pinball.scoring.tools: 0` in models.yaml.",
			})
		}
	}

	// 3. Context bloat
	if a.ContextBloatCount > 0 {
		a.Findings = append(a.Findings, Finding{
			Level:       "tip",
			Category:    "context",
			Title:       "Large context with minimal output observed",
			Description: "Multiple requests sent large prompts (>20k tokens) for small completions (<150 tokens). Use targeted file reads or smaller windows to save tokens and avoid prompt cache evictions.",
		})
	}

	// 4. Heavy tier on simple queries
	if a.HeavyOnSimpleCount > 0 {
		a.Findings = append(a.Findings, Finding{
			Level:       "tip",
			Category:    "tiering",
			Title:       "Heavy tier triggered on short prompts",
			Description: "Several short prompts were routed to heavy tier models. Check if heavy keywords (e.g., 'refactor', 'architect') in system instructions are skewing the scoring.",
		})
	}

	// Sort models for clean display
	type kv struct {
		k string
		v int
	}
	var sortedModels []kv
	for k, v := range a.Models {
		sortedModels = append(sortedModels, kv{k, v})
	}
	sort.Slice(sortedModels, func(i, j int) bool {
		return sortedModels[i].v > sortedModels[j].v
	})

	return a
}
