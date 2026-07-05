package idx

import (
	"context"
	"strings"
	"testing"
)

func TestBuildCompareRow(t *testing.T) {
	v := &ValuationRatios{
		Code:                "BBCA",
		Name:                "Bank Central Asia Tbk.",
		Price:               6050,
		MarketCap:           738355911975000,
		PER:                 12.6,
		PBV:                 2.85,
		EPSAnnualized:       480,
		BookValuePerShare:   2123,
		DividendYield:       4.9,
		Equity:              259e12,
		NetIncomeAnnualized: 58.6e12,
	}
	row := buildCompareRow(v)
	if row.Code != "BBCA" || row.PER != 12.6 {
		t.Errorf("row = %+v, want BBCA fields carried over", row)
	}
	wantROE := 58.6e12 / 259e12 * 100
	if row.ROE < wantROE-0.01 || row.ROE > wantROE+0.01 {
		t.Errorf("ROE = %f, want ~%f", row.ROE, wantROE)
	}
}

func TestBuildCompareRowNegativeEquity(t *testing.T) {
	row := buildCompareRow(&ValuationRatios{Code: "XXXX", Equity: -1, NetIncomeAnnualized: 5})
	if row.ROE != 0 {
		t.Errorf("ROE = %f, want 0 for non-positive equity", row.ROE)
	}
}

func TestCompareStocksValidation(t *testing.T) {
	c := New(&fakeFetcher{}, nil)

	if _, err := c.CompareStocks(context.Background(), nil, "2026", "tw1"); err == nil {
		t.Error("expected error for no codes")
	}
	if _, err := c.CompareStocks(context.Background(), []string{"bbca", "BBCA", " "}, "2026", "tw1"); err == nil {
		t.Error("expected error for a single (deduped) code")
	}
}

func TestCompareStocksDegradesPerTicker(t *testing.T) {
	// fakeFetcher has no canned responses: every fetch fails, so each row
	// must carry an error instead of the whole call failing.
	c := New(&fakeFetcher{responses: map[string]string{}}, nil)

	cmp, err := c.CompareStocks(context.Background(), []string{"BBCA", "TLKM"}, "2026", "tw1")
	if err != nil {
		t.Fatalf("CompareStocks: %v", err)
	}
	if len(cmp.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(cmp.Rows))
	}
	for _, r := range cmp.Rows {
		if r.Error == "" {
			t.Errorf("row %s: expected per-ticker error", r.Code)
		}
	}
	if cmp.Rows[0].Code != "BBCA" || cmp.Rows[1].Code != "TLKM" {
		t.Errorf("rows out of caller order: %s, %s", cmp.Rows[0].Code, cmp.Rows[1].Code)
	}
}

func TestCompareStocksCapsCodes(t *testing.T) {
	c := New(&fakeFetcher{responses: map[string]string{}}, nil)
	codes := []string{"AAAA", "BBBB", "CCCC", "DDDD", "EEEE", "FFFF", "GGGG"}
	cmp, err := c.CompareStocks(context.Background(), codes, "2026", "tw1")
	if err != nil {
		t.Fatalf("CompareStocks: %v", err)
	}
	if len(cmp.Rows) != maxCompareCodes {
		t.Errorf("rows = %d, want %d", len(cmp.Rows), maxCompareCodes)
	}
	if !strings.Contains(cmp.Note, "first 5") {
		t.Errorf("Note = %q, want truncation note", cmp.Note)
	}
}
