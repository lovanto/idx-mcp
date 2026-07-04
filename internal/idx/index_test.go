package idx

import (
	"context"
	"testing"
)

const indexJSON = `{"draw":0,"recordsTotal":2,"data":[
 {"Date":"2026-07-03T00:00:00","IndexCode":"COMPOSITE","Previous":5744.556,"Highest":5899.302,"Lowest":5805.922,"Close":5875.78,"Change":131.224,"Volume":17151990600,"Value":10535360961366,"Frequency":1363259,"NumberOfStock":913,"MarketCapital":1.0287029795758e+16},
 {"Date":"2026-07-03T00:00:00","IndexCode":"LQ45","Previous":565.493,"Highest":583.893,"Lowest":572.3,"Close":581.783,"Change":16.29,"Volume":4292854948,"Value":5415569922952,"Frequency":423440,"NumberOfStock":45,"MarketCapital":4031453577082980}
]}`

func TestMarketIndicesAll(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetIndexSummary": indexJSON}}
	c := New(f, nil)

	idxs, err := c.MarketIndices(context.Background(), "")
	if err != nil {
		t.Fatalf("MarketIndices: %v", err)
	}
	if len(idxs) != 2 {
		t.Fatalf("got %d indices, want 2", len(idxs))
	}
	comp := idxs[0]
	if comp.Code != "COMPOSITE" || comp.Close != 5875.78 {
		t.Errorf("composite = %+v", comp)
	}
	if comp.Date != "2026-07-03" {
		t.Errorf("date = %q, want trimmed", comp.Date)
	}
	// ChangePercent = 131.224 / 5744.556 * 100 ≈ 2.2843
	if comp.ChangePercent < 2.28 || comp.ChangePercent > 2.29 {
		t.Errorf("change%% = %v, want ~2.284", comp.ChangePercent)
	}
}

func TestMarketIndicesFilter(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetIndexSummary": indexJSON}}
	c := New(f, nil)

	idxs, err := c.MarketIndices(context.Background(), "lq45")
	if err != nil {
		t.Fatalf("MarketIndices: %v", err)
	}
	if len(idxs) != 1 || idxs[0].Code != "LQ45" {
		t.Fatalf("filter by lq45 = %+v, want single LQ45", idxs)
	}
}
