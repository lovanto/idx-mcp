package idx

import (
	"context"
	"fmt"

	"github.com/lovanto/idx-mcp/internal/xbrl"
)

// XBRL context ids referenced by consolidated headline facts (Phase 1 spike):
// balance-sheet facts use CurrentYearInstant, P&L facts CurrentYearDuration.
const (
	ctxInstant  = "CurrentYearInstant"
	ctxDuration = "CurrentYearDuration"
)

// ValuationRatios combines the latest official price (GetTradingInfoSS, which
// also carries ListedShares) with a parsed XBRL filing into market cap, PER,
// and PBV. All rupiah amounts are full IDR.
type ValuationRatios struct {
	Code                string   `json:"code"`
	Name                string   `json:"name,omitempty"`
	PriceDate           string   `json:"price_date"`
	Price               float64  `json:"price"`
	ListedShares        float64  `json:"listed_shares"`
	MarketCap           float64  `json:"market_cap"`
	ReportYear          string   `json:"report_year"`
	ReportPeriod        string   `json:"report_period"`
	ReportPeriodEnd     string   `json:"report_period_end,omitempty"`
	NetIncome           float64  `json:"net_income"`            // as reported for the period, attributable to parent when available
	NetIncomeAnnualized float64  `json:"net_income_annualized"` // period profit scaled to a full year
	Equity              float64  `json:"equity"`                // attributable to parent when available
	BookValuePerShare   float64  `json:"book_value_per_share"`
	EPSAnnualized       float64  `json:"eps_annualized"`
	PER                 float64  `json:"per"` // 0 when earnings are non-positive (see notes)
	PBV                 float64  `json:"pbv"` // 0 when equity is non-positive (see notes)
	Notes               []string `json:"notes,omitempty"`
}

// annualizationFactor scales an interim P&L figure to a full year. The API
// periods are cumulative year-to-date (TW2 covers H1, TW3 covers 9M).
func annualizationFactor(period string) float64 {
	switch period {
	case "TW1":
		return 4
	case "TW2":
		return 2
	case "TW3":
		return 4.0 / 3
	default: // Audit = full year
		return 1
	}
}

// ValuationRatios computes market cap, PER, and PBV for `code` using the
// latest close and the financial report for `year`/`period` (same semantics as
// FinancialReport). It composes two already-cached fetches, so a warm cache
// serves it without extra network cost.
func (c *Client) ValuationRatios(ctx context.Context, code, year, period string) (*ValuationRatios, error) {
	days, err := c.TradingInfo(ctx, code, 1)
	if err != nil {
		return nil, err
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("no trading data for %s", normalizeCode(code))
	}

	rep, err := c.FinancialReport(ctx, code, year, period)
	if err != nil {
		return nil, err
	}
	return buildValuation(days[0], rep)
}

// buildValuation is the pure ratio computation, separated for testability.
func buildValuation(day TradingDay, rep *FinancialReport) (*ValuationRatios, error) {
	if day.ListedShares <= 0 {
		return nil, fmt.Errorf("listed shares unavailable for %s", day.StockCode)
	}
	if day.Close <= 0 {
		return nil, fmt.Errorf("no valid closing price for %s", day.StockCode)
	}

	v := &ValuationRatios{
		Code:            rep.Code,
		Name:            day.StockName,
		PriceDate:       day.Date,
		Price:           day.Close,
		ListedShares:    day.ListedShares,
		MarketCap:       day.Close * day.ListedShares,
		ReportYear:      rep.Year,
		ReportPeriod:    rep.Period,
		ReportPeriodEnd: rep.Report.Entity.PeriodEndDate,
		Notes: []string{
			"Computed by idx-mcp from the official IDX price feed and XBRL filing; interim profit annualized by simple scaling (TW1 x4, TW2 x2, TW3 x4/3).",
		},
	}

	equity, eqConcept, ok := findAccountIDR(rep.Report, ctxInstant,
		"EquityAttributableToEquityOwnersOfParentEntity", "Equity")
	if !ok {
		return nil, fmt.Errorf("equity not found in %s %s %s filing", rep.Code, rep.Year, rep.Period)
	}
	profit, plConcept, ok := findAccountIDR(rep.Report, ctxDuration,
		"ProfitLossAttributableToParentEntity", "ProfitLoss")
	if !ok {
		return nil, fmt.Errorf("profit/loss not found in %s %s %s filing", rep.Code, rep.Year, rep.Period)
	}
	if eqConcept == "Equity" {
		v.Notes = append(v.Notes, "Parent-entity equity not reported; total equity (including non-controlling interests) used for PBV.")
	}
	if plConcept == "ProfitLoss" {
		v.Notes = append(v.Notes, "Parent-attributable profit not reported; total profit/loss used for PER.")
	}

	v.Equity = equity
	v.NetIncome = profit
	v.NetIncomeAnnualized = profit * annualizationFactor(rep.Period)
	v.BookValuePerShare = equity / day.ListedShares
	v.EPSAnnualized = v.NetIncomeAnnualized / day.ListedShares

	if v.NetIncomeAnnualized > 0 {
		v.PER = v.MarketCap / v.NetIncomeAnnualized
	} else {
		v.Notes = append(v.Notes, "Earnings are non-positive for the period; PER omitted.")
	}
	if equity > 0 {
		v.PBV = v.MarketCap / equity
	} else {
		v.Notes = append(v.Notes, "Equity is non-positive; PBV omitted.")
	}
	return v, nil
}

// findAccountIDR returns the first numeric consolidated fact matching any of
// `concepts` (in preference order) within the given context, plus the concept
// that matched.
func findAccountIDR(rep *xbrl.Report, contextID string, concepts ...string) (float64, string, bool) {
	for _, concept := range concepts {
		for _, a := range rep.Accounts {
			if a.Concept == concept && a.Context == contextID && a.NumericIDR != nil {
				return float64(*a.NumericIDR), concept, true
			}
		}
	}
	return 0, "", false
}
