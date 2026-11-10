package insights

import (
	"fmt"
	"sort"
)

// Price is USD per million tokens.
type Price struct{ Input, Output float64 }

// DefaultPrices mirrors DEFAULT_PRICES in pinball's optimizer
// (src/pinball/optimize/settings.py), the catalog the optimizer uses for cost
// comparisons. They are notional list-price estimates, not invoices; keep the
// two in sync.
var DefaultPrices = map[string]Price{
	"claude-haiku-5.5":  {1, 5},
	"claude-haiku-4.5":  {1, 5},
	"claude-sonnet-5.5": {3, 15},
	"claude-sonnet-5":   {3, 15},
	"gemini-3.8-flash":  {1, 4},
	"claude-opus-5.5":   {15, 75},
	"claude-opus-5":     {15, 75},
	"claude-opus-4.8":   {15, 75},
	"claude-fable-5.1":  {15, 75},
	"claude-fable-5":    {15, 75},
	"gemini-4-argon":    {4, 20},
	"kimi-k3":           {1, 4},
	"judge-opus":        {15, 75},
	"judge-argon":       {4, 20},
}

// CostSummary is the cost section of an Assessment. All money is USD.
type CostSummary struct {
	TotalCost      float64            `json:"total_cost"`
	CostByTier     map[string]float64 `json:"cost_by_tier"`
	PricedRows     int                `json:"priced_rows"`
	TotalRows      int                `json:"total_rows"`
	ReferenceModel string             `json:"reference_model"`
	ReferenceCost  float64            `json:"reference_cost"`
	Savings        float64            `json:"savings"`
}

// ComputeCost sums the logged cost and estimates what the same tokens would
// have cost on the reference model. Reference cost and savings only cover
// priced rows, so both sides compare the same requests.
func ComputeCost(records []RequestRecord, referenceModel string) (*CostSummary, error) {
	p, ok := DefaultPrices[referenceModel]
	if !ok {
		names := make([]string, 0, len(DefaultPrices))
		for n := range DefaultPrices {
			names = append(names, n)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("unknown reference model %q (known: %v)", referenceModel, names)
	}
	c := &CostSummary{CostByTier: map[string]float64{}, TotalRows: len(records), ReferenceModel: referenceModel}
	for i := range records {
		r := &records[i]
		if !r.Priced {
			continue
		}
		tier := r.PinballTier
		if tier == "" {
			tier = "unrouted"
		}
		c.PricedRows++
		c.TotalCost += r.Cost
		c.CostByTier[tier] += r.Cost
		c.ReferenceCost += (float64(r.InputTokens)*p.Input + float64(r.OutputTokens)*p.Output) / 1e6
	}
	c.Savings = c.ReferenceCost - c.TotalCost
	return c, nil
}
