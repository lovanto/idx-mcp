package idx

import (
	"math"
	"strings"
	"testing"

	"github.com/lovanto/idx-mcp/internal/xbrl"
)

func i64(v int64) *int64 { return &v }

// syntheticReport builds a FinancialReport with parent-attributable equity and
// profit plus their totals, mirroring a real consolidated filing.
func syntheticReport(period string) *FinancialReport {
	return &FinancialReport{
		Code:   "TEST",
		Year:   "2026",
		Period: period,
		Report: &xbrl.Report{
			Entity: xbrl.Entity{Code: "TEST", PeriodEndDate: "2026-03-31"},
			Accounts: []xbrl.Account{
				{Concept: "Equity", Context: "CurrentYearInstant", NumericIDR: i64(1_200)},
				{Concept: "EquityAttributableToEquityOwnersOfParentEntity", Context: "CurrentYearInstant", NumericIDR: i64(1_000)},
				{Concept: "ProfitLoss", Context: "CurrentYearDuration", NumericIDR: i64(60)},
				{Concept: "ProfitLossAttributableToParentEntity", Context: "CurrentYearDuration", NumericIDR: i64(50)},
				// Prior-period rows must be ignored.
				{Concept: "ProfitLossAttributableToParentEntity", Context: "PriorYearDuration", NumericIDR: i64(999)},
			},
		},
	}
}

func TestBuildValuation(t *testing.T) {
	day := TradingDay{
		StockCode: "TEST", StockName: "PT Test Tbk.", Date: "2026-07-03",
		Close: 20, ListedShares: 100,
	}
	v, err := buildValuation(day, syntheticReport("TW1"))
	if err != nil {
		t.Fatalf("buildValuation: %v", err)
	}

	if v.MarketCap != 2000 {
		t.Errorf("MarketCap = %v, want 2000", v.MarketCap)
	}
	// Parent-attributable figures win over totals.
	if v.Equity != 1000 || v.NetIncome != 50 {
		t.Errorf("Equity/NetIncome = %v/%v, want 1000/50 (parent-attributable)", v.Equity, v.NetIncome)
	}
	// TW1 annualizes x4: 50 -> 200; PER = 2000/200 = 10.
	if v.NetIncomeAnnualized != 200 || v.PER != 10 {
		t.Errorf("NetIncomeAnnualized/PER = %v/%v, want 200/10", v.NetIncomeAnnualized, v.PER)
	}
	// PBV = 2000/1000 = 2; BVPS = 1000/100 = 10; EPS = 200/100 = 2.
	if v.PBV != 2 || v.BookValuePerShare != 10 || v.EPSAnnualized != 2 {
		t.Errorf("PBV/BVPS/EPS = %v/%v/%v, want 2/10/2", v.PBV, v.BookValuePerShare, v.EPSAnnualized)
	}
	if v.ReportPeriodEnd != "2026-03-31" {
		t.Errorf("ReportPeriodEnd = %q", v.ReportPeriodEnd)
	}
}

func TestBuildValuationFallbackToTotals(t *testing.T) {
	rep := syntheticReport("Audit")
	// Drop the parent-attributable rows; totals must be used with a note.
	var kept []xbrl.Account
	for _, a := range rep.Report.Accounts {
		if !strings.Contains(a.Concept, "Parent") {
			kept = append(kept, a)
		}
	}
	rep.Report.Accounts = kept

	day := TradingDay{StockCode: "TEST", Date: "2026-07-03", Close: 20, ListedShares: 100}
	v, err := buildValuation(day, rep)
	if err != nil {
		t.Fatalf("buildValuation: %v", err)
	}
	if v.Equity != 1200 || v.NetIncome != 60 {
		t.Errorf("Equity/NetIncome = %v/%v, want totals 1200/60", v.Equity, v.NetIncome)
	}
	// Audit period: no annualization.
	if v.NetIncomeAnnualized != 60 {
		t.Errorf("NetIncomeAnnualized = %v, want 60", v.NetIncomeAnnualized)
	}
	notes := strings.Join(v.Notes, " | ")
	if !strings.Contains(notes, "total equity") || !strings.Contains(notes, "total profit/loss") {
		t.Errorf("expected fallback notes, got %q", notes)
	}
}

func TestBuildValuationLossMaker(t *testing.T) {
	rep := syntheticReport("TW2")
	for i := range rep.Report.Accounts {
		if rep.Report.Accounts[i].Concept == "ProfitLossAttributableToParentEntity" &&
			rep.Report.Accounts[i].Context == "CurrentYearDuration" {
			rep.Report.Accounts[i].NumericIDR = i64(-50)
		}
	}
	day := TradingDay{StockCode: "TEST", Date: "2026-07-03", Close: 20, ListedShares: 100}
	v, err := buildValuation(day, rep)
	if err != nil {
		t.Fatalf("buildValuation: %v", err)
	}
	if v.PER != 0 {
		t.Errorf("PER = %v, want 0 for a loss-maker", v.PER)
	}
	if v.NetIncomeAnnualized != -100 { // TW2 x2
		t.Errorf("NetIncomeAnnualized = %v, want -100", v.NetIncomeAnnualized)
	}
	if !strings.Contains(strings.Join(v.Notes, " "), "PER omitted") {
		t.Errorf("expected loss note, got %v", v.Notes)
	}
}

func TestBuildValuationGuards(t *testing.T) {
	rep := syntheticReport("TW1")
	if _, err := buildValuation(TradingDay{Close: 20}, rep); err == nil {
		t.Error("expected error when listed shares are missing")
	}
	if _, err := buildValuation(TradingDay{Close: 0, ListedShares: 100}, rep); err == nil {
		t.Error("expected error when close price is zero")
	}
	rep.Report.Accounts = nil
	if _, err := buildValuation(TradingDay{Close: 20, ListedShares: 100}, rep); err == nil {
		t.Error("expected error when filing has no equity/profit facts")
	}
}

func TestAnnualizationFactor(t *testing.T) {
	cases := map[string]float64{"TW1": 4, "TW2": 2, "TW3": 4.0 / 3, "Audit": 1}
	for p, want := range cases {
		if got := annualizationFactor(p); math.Abs(got-want) > 1e-9 {
			t.Errorf("annualizationFactor(%q) = %v, want %v", p, got, want)
		}
	}
}
