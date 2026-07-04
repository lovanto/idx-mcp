package idx

import (
	"strings"
	"testing"

	"github.com/lovanto/idx-mcp/internal/xbrl"
)

func growthReport() *FinancialReport {
	acc := func(concept, ctx string, v int64) xbrl.Account {
		return xbrl.Account{Concept: concept, Context: ctx, NumericIDR: i64(v)}
	}
	return &FinancialReport{
		Code: "TEST", Year: "2026", Period: "TW1",
		Report: &xbrl.Report{
			Entity: xbrl.Entity{Name: "PT Test Tbk."},
			Accounts: []xbrl.Account{
				acc("SalesAndRevenue", "CurrentYearDuration", 1000),
				acc("SalesAndRevenue", "PriorYearDuration", 800),
				acc("ProfitLoss", "CurrentYearDuration", 150),
				acc("ProfitLoss", "PriorYearDuration", 100),
				acc("Assets", "CurrentYearInstant", 5500),
				acc("Assets", "PriorEndYearInstant", 5000),
				// Equity present only for the current period: line skipped.
				acc("Equity", "CurrentYearInstant", 2000),
			},
		},
	}
}

func TestBuildGrowth(t *testing.T) {
	g, err := buildGrowth(growthReport())
	if err != nil {
		t.Fatalf("buildGrowth: %v", err)
	}

	if len(g.IncomeStatement) != 2 {
		t.Fatalf("income statement lines = %d, want 2: %+v", len(g.IncomeStatement), g.IncomeStatement)
	}
	rev := g.IncomeStatement[0]
	if rev.Concept != "SalesAndRevenue" || rev.GrowthPct != 25 { // (1000-800)/800
		t.Errorf("revenue line = %+v, want +25%%", rev)
	}
	pl := g.IncomeStatement[1]
	if pl.GrowthPct != 50 { // (150-100)/100
		t.Errorf("profit growth = %v, want 50", pl.GrowthPct)
	}

	// Equity lacks a comparative, so only Assets appears.
	if len(g.BalanceSheet) != 1 || g.BalanceSheet[0].Concept != "Assets" || g.BalanceSheet[0].GrowthPct != 10 {
		t.Errorf("balance sheet = %+v, want Assets +10%%", g.BalanceSheet)
	}

	// Margins: 150/1000 = 15%, prior 100/800 = 12.5%.
	if g.NetMarginPct != 15 || g.NetMarginPriorPct != 12.5 {
		t.Errorf("margins = %v/%v, want 15/12.5", g.NetMarginPct, g.NetMarginPriorPct)
	}
}

func TestBuildGrowthBankTopLine(t *testing.T) {
	rep := growthReport()
	for i := range rep.Report.Accounts {
		if rep.Report.Accounts[i].Concept == "SalesAndRevenue" {
			rep.Report.Accounts[i].Concept = "InterestIncome"
		}
	}
	g, err := buildGrowth(rep)
	if err != nil {
		t.Fatalf("buildGrowth: %v", err)
	}
	if g.IncomeStatement[0].Concept != "InterestIncome" {
		t.Errorf("top line = %+v, want InterestIncome", g.IncomeStatement[0])
	}
	if !strings.Contains(strings.Join(g.Notes, " "), "Bank issuer") {
		t.Errorf("expected bank note, got %v", g.Notes)
	}
	if g.NetMarginPct != 15 {
		t.Errorf("margin = %v, want 15", g.NetMarginPct)
	}
}

func TestBuildGrowthNegativePriorDenominator(t *testing.T) {
	rep := &FinancialReport{
		Code: "TEST", Year: "2026", Period: "TW1",
		Report: &xbrl.Report{Accounts: []xbrl.Account{
			{Concept: "ProfitLoss", Context: "CurrentYearDuration", NumericIDR: i64(50)},
			{Concept: "ProfitLoss", Context: "PriorYearDuration", NumericIDR: i64(-100)},
		}},
	}
	g, err := buildGrowth(rep)
	if err != nil {
		t.Fatalf("buildGrowth: %v", err)
	}
	// Turnaround: (50 - -100)/|-100| = +150%, positive because |prior| is used.
	if g.IncomeStatement[0].GrowthPct != 150 {
		t.Errorf("growth = %v, want 150", g.IncomeStatement[0].GrowthPct)
	}
}

func TestBuildGrowthEmpty(t *testing.T) {
	rep := &FinancialReport{Code: "TEST", Year: "2026", Period: "TW1", Report: &xbrl.Report{}}
	if _, err := buildGrowth(rep); err == nil {
		t.Error("expected error for a filing with no comparable accounts")
	}
}
