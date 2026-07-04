package idx

import (
	"context"
	"fmt"

	"github.com/lovanto/idx-mcp/internal/xbrl"
)

// Prior-period XBRL context ids. A single IDX filing carries its own
// comparatives: PriorYearDuration is the same period one year earlier (true
// YoY for the P&L) and PriorEndYearInstant is the prior year-end balance
// sheet (verified against a real BBCA instance).
const (
	ctxPriorDuration   = "PriorYearDuration"
	ctxPriorEndInstant = "PriorEndYearInstant"
)

// GrowthLine compares one concept between the current filing period and its
// in-filing comparative.
type GrowthLine struct {
	Concept       string  `json:"concept"`
	Current       float64 `json:"current"`
	Prior         float64 `json:"prior"`
	GrowthPct     float64 `json:"growth_pct"` // (current-prior)/|prior|*100; 0 when prior is 0
	CurrentPeriod string  `json:"current_period"`
	PriorPeriod   string  `json:"prior_period"`
}

// FinancialGrowth is the YoY comparison derived from a single XBRL filing.
type FinancialGrowth struct {
	Code              string       `json:"code"`
	Name              string       `json:"name,omitempty"`
	Year              string       `json:"year"`
	Period            string       `json:"period"`
	IncomeStatement   []GrowthLine `json:"income_statement"` // vs same period prior year
	BalanceSheet      []GrowthLine `json:"balance_sheet"`    // vs prior year-end
	NetMarginPct      float64      `json:"net_margin_pct"`   // profit / revenue, current period
	NetMarginPriorPct float64      `json:"net_margin_prior_pct"`
	Notes             []string     `json:"notes,omitempty"`
}

// growthDurationConcepts are P&L lines compared YoY, in display order. The
// top line differs by industry (SalesAndRevenue vs InterestIncome for banks);
// whichever exists is included.
var growthDurationConcepts = []string{
	"SalesAndRevenue",
	"InterestIncome",
	"GrossProfit",
	"ProfitLossBeforeIncomeTax",
	"ProfitLoss",
	"ProfitLossAttributableToParentEntity",
}

// growthInstantConcepts are balance-sheet lines compared against the prior
// year-end.
var growthInstantConcepts = []string{
	"Assets",
	"Liabilities",
	"Equity",
	"EquityAttributableToEquityOwnersOfParentEntity",
}

// FinancialGrowth compares the filing for `code`/`year`/`period` against the
// comparatives embedded in the same filing: P&L vs the same period a year
// earlier, balance sheet vs the prior year-end. One fetch, cached forever.
func (c *Client) FinancialGrowth(ctx context.Context, code, year, period string) (*FinancialGrowth, error) {
	rep, err := c.FinancialReport(ctx, code, year, period)
	if err != nil {
		return nil, err
	}
	return buildGrowth(rep)
}

// buildGrowth is the pure comparison, separated for testability.
func buildGrowth(rep *FinancialReport) (*FinancialGrowth, error) {
	g := &FinancialGrowth{
		Code:   rep.Code,
		Name:   rep.Report.Entity.Name,
		Year:   rep.Year,
		Period: rep.Period,
		Notes: []string{
			"Comparatives come from the same filing: P&L vs the same period one year earlier, balance sheet vs the prior year-end.",
		},
	}

	for _, concept := range growthDurationConcepts {
		if line, ok := growthLine(rep.Report, concept, ctxDuration, ctxPriorDuration); ok {
			g.IncomeStatement = append(g.IncomeStatement, line)
		}
	}
	for _, concept := range growthInstantConcepts {
		if line, ok := growthLine(rep.Report, concept, ctxInstant, ctxPriorEndInstant); ok {
			g.BalanceSheet = append(g.BalanceSheet, line)
		}
	}
	if len(g.IncomeStatement) == 0 && len(g.BalanceSheet) == 0 {
		return nil, fmt.Errorf("no comparable accounts found in %s %s %s filing", rep.Code, rep.Year, rep.Period)
	}

	// Net margin from the top line (revenue or interest income) and net profit.
	revenue, hasRev := currentValue(g.IncomeStatement, "SalesAndRevenue", "InterestIncome")
	profit, hasPL := findGrowthLine(g.IncomeStatement, "ProfitLoss")
	if hasRev && hasPL {
		if revenue.Current != 0 {
			g.NetMarginPct = profit.Current / revenue.Current * 100
		}
		if revenue.Prior != 0 {
			g.NetMarginPriorPct = profit.Prior / revenue.Prior * 100
		}
		if revenue.Concept == "InterestIncome" {
			g.Notes = append(g.Notes, "Bank issuer: margin is profit over interest income (gross), not net interest margin.")
		}
	}
	return g, nil
}

// growthLine builds one comparison if both current and prior facts exist.
func growthLine(rep *xbrl.Report, concept, curCtx, priorCtx string) (GrowthLine, bool) {
	cur, ok := factIDR(rep, concept, curCtx)
	if !ok {
		return GrowthLine{}, false
	}
	prior, ok := factIDR(rep, concept, priorCtx)
	if !ok {
		return GrowthLine{}, false
	}
	line := GrowthLine{
		Concept:       concept,
		Current:       cur.value,
		Prior:         prior.value,
		CurrentPeriod: cur.period,
		PriorPeriod:   prior.period,
	}
	if prior.value != 0 {
		line.GrowthPct = (cur.value - prior.value) / abs(prior.value) * 100
	}
	return line, true
}

type idrFact struct {
	value  float64
	period string
}

// factIDR returns the numeric fact for concept in the given context.
func factIDR(rep *xbrl.Report, concept, contextID string) (idrFact, bool) {
	for _, a := range rep.Accounts {
		if a.Concept == concept && a.Context == contextID && a.NumericIDR != nil {
			return idrFact{value: float64(*a.NumericIDR), period: a.Period}, true
		}
	}
	return idrFact{}, false
}

// currentValue returns the first line matching any concept, in order.
func currentValue(lines []GrowthLine, concepts ...string) (GrowthLine, bool) {
	for _, c := range concepts {
		if l, ok := findGrowthLine(lines, c); ok {
			return l, true
		}
	}
	return GrowthLine{}, false
}

func findGrowthLine(lines []GrowthLine, concept string) (GrowthLine, bool) {
	for _, l := range lines {
		if l.Concept == concept {
			return l, true
		}
	}
	return GrowthLine{}, false
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
