package insights

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// PrintAssessment writes a human-readable summary of the assessment and coaching tips.
func PrintAssessment(w io.Writer, a *Assessment, coachAdvice string) error {
	var b strings.Builder

	fmt.Fprintf(&b, "=== Convergence Usage Insights (%s) ===\n\n", a.Window)
	fmt.Fprintf(&b, "Requests:      %d (Success: %d, Failed: %d)\n", a.TotalRequests, a.SuccessfulRequests, a.FailedRequests)
	fmt.Fprintf(&b, "Tokens:        %d total (Input: %d, Output: %d)\n", a.TotalTokens, a.InputTokens, a.OutputTokens)
	if a.TotalRequests > 0 {
		fmt.Fprintf(&b, "Avg Latency:   %d ms\n", a.AvgDurationMS)
	}
	b.WriteString("\n")

	// Tiers breakdown
	b.WriteString("Tier Distribution:\n")
	tierOrder := []string{"light", "standard", "heavy", "unrouted"}
	for _, t := range tierOrder {
		if count, ok := a.Tiers[t]; ok && count > 0 {
			pct := float64(count) / float64(a.TotalRequests) * 100
			fmt.Fprintf(&b, "  %-10s %5d (%4.1f%%)\n", t+":", count, pct)
		}
	}
	for t, count := range a.Tiers {
		isKnown := false
		for _, kt := range tierOrder {
			if kt == t {
				isKnown = true
				break
			}
		}
		if !isKnown && count > 0 {
			pct := float64(count) / float64(a.TotalRequests) * 100
			fmt.Fprintf(&b, "  %-10s %5d (%4.1f%%)\n", t+":", count, pct)
		}
	}
	b.WriteString("\n")

	if c := a.Cost; c != nil {
		fmt.Fprintf(&b, "Cost (estimate, USD; %d of %d rows priced):\n", c.PricedRows, c.TotalRows)
		fmt.Fprintf(&b, "  Total:       $%.4f\n", c.TotalCost)
		tiers := make([]string, 0, len(c.CostByTier))
		for t := range c.CostByTier {
			tiers = append(tiers, t)
		}
		sort.Strings(tiers)
		for _, t := range tiers {
			fmt.Fprintf(&b, "  %-10s $%.4f\n", t+":", c.CostByTier[t])
		}
		fmt.Fprintf(&b, "  Same tokens on %s: $%.4f, savings: $%.4f\n\n", c.ReferenceModel, c.ReferenceCost, c.Savings)
	}

	// Models breakdown
	if len(a.Models) > 0 {
		b.WriteString("Top Models:\n")
		type modelCount struct {
			model string
			count int
		}
		var mc []modelCount
		for m, c := range a.Models {
			mc = append(mc, modelCount{model: m, count: c})
		}
		sort.Slice(mc, func(i, j int) bool {
			return mc[i].count > mc[j].count
		})
		for i, m := range mc {
			if i >= 5 {
				break
			}
			pct := float64(m.count) / float64(a.TotalRequests) * 100
			fmt.Fprintf(&b, "  %-28s %5d (%4.1f%%)\n", m.model, m.count, pct)
		}
		b.WriteString("\n")
	}

	// Findings / Observations
	if len(a.Findings) > 0 {
		b.WriteString("Observations & Patterns:\n")
		for _, f := range a.Findings {
			prefix := "[INFO]"
			switch f.Level {
			case "warning":
				prefix = "[WARN]"
			case "tip":
				prefix = "[TIP] "
			}
			fmt.Fprintf(&b, "  %s %s\n      %s\n", prefix, f.Title, f.Description)
		}
		b.WriteString("\n")
	}

	// LLM Coaching Advice if present
	if coachAdvice != "" {
		b.WriteString("Coaching & Recommendations:\n")
		lines := strings.Split(coachAdvice, "\n")
		for _, line := range lines {
			b.WriteString("  " + line + "\n")
		}
		b.WriteString("\n")
	}

	_, err := fmt.Fprint(w, b.String())
	return err
}

// PrintJSON outputs the assessment as JSON.
func PrintJSON(w io.Writer, a *Assessment, coachAdvice string) error {
	type jsonOutput struct {
		Assessment
		CoachAdvice string `json:"coach_advice,omitempty"`
	}
	out := jsonOutput{
		Assessment:  *a,
		CoachAdvice: coachAdvice,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
