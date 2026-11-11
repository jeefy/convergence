package insights

import (
	"bytes"
	"context"
	"database/sql"
	"math"
	"strings"
	"testing"
	"time"
)

func f64(v float64) *float64 { return &v }

func init() {
	now := time.Now().Add(-time.Minute)
	row := func(id, tier string, in, out int64, cost *float64) mockRow {
		return mockRow{
			id: id, startedAt: now, completedAt: now, httpStatus: 200,
			inTokens: in, outTokens: out, totTokens: in + out, cost: cost,
			attributesJSON: []byte(`{"pinball.tier":"` + tier + `"}`),
		}
	}
	sql.Register("mock_pg_cost", &mockDriver{rows: []mockRow{
		row("a", "light", 1_000_000, 0, f64(1)),
		row("b", "heavy", 0, 1_000_000, f64(60)),
		row("c", "heavy", 500, 500, nil), // unpriced: NULL cost
	}})
}

func costFixture(t *testing.T) *CostSummary {
	t.Helper()
	db, err := sql.Open("mock_pg_cost", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	recs, err := QueryRequests(context.Background(), db, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ComputeCost(recs, "claude-opus-5.5")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestComputeCostPricedAndUnpriced(t *testing.T) {
	c := costFixture(t)
	if c.PricedRows != 2 || c.TotalRows != 3 {
		t.Fatalf("coverage = %d of %d, want 2 of 3", c.PricedRows, c.TotalRows)
	}
	if c.TotalCost != 61 || c.CostByTier["light"] != 1 || c.CostByTier["heavy"] != 60 {
		t.Fatalf("unexpected costs: %+v", c)
	}
	// opus-5.5 is $15 in / $75 out per 1M: 15 + 75 = 90 for the two priced rows.
	if math.Abs(c.ReferenceCost-90) > 1e-9 || math.Abs(c.Savings-29) > 1e-9 {
		t.Fatalf("reference=%v savings=%v, want 90 and 29", c.ReferenceCost, c.Savings)
	}
}

func TestComputeCostUnknownReference(t *testing.T) {
	if _, err := ComputeCost(nil, "nope"); err == nil {
		t.Fatal("expected error for unknown reference model")
	}
}

func TestCostPrintAndJSONGolden(t *testing.T) {
	a := Assessment{Window: "1d", Tiers: map[string]int{}, Cost: costFixture(t)}
	var b bytes.Buffer
	if err := PrintAssessment(&b, &a, ""); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"2 of 3 rows priced", "Total:       $61.0000", "claude-opus-5.5: $90.0000, savings: $29.0000"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("text output missing %q:\n%s", want, b.String())
		}
	}
	b.Reset()
	if err := PrintJSON(&b, &a, ""); err != nil {
		t.Fatal(err)
	}
	golden := `  "cost": {
    "total_cost": 61,
    "cost_by_tier": {
      "heavy": 60,
      "light": 1
    },
    "priced_rows": 2,
    "total_rows": 3,
    "reference_model": "claude-opus-5.5",
    "reference_cost": 90,
    "savings": 29
  },`
	if !strings.Contains(b.String(), golden) {
		t.Errorf("json cost block differs:\n%s", b.String())
	}
}
