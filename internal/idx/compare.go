package idx

import (
	"context"
	"fmt"
)

// maxCompareCodes caps a single comparison. Every uncached ticker costs
// multiple rate-limited fetches (~15s apart), so a small cap keeps one tool
// call from stalling for minutes.
const maxCompareCodes = 5

// CompareRow is one ticker's valuation snapshot in a side-by-side comparison —
// the compact subset of ValuationRatios that is meaningful across issuers,
// plus ROE. A per-ticker failure lands in Error instead of failing the call.
type CompareRow struct {
	Code              string   `json:"code"`
	Name              string   `json:"name,omitempty"`
	Price             float64  `json:"price,omitempty"`
	MarketCap         float64  `json:"market_cap,omitempty"`
	PER               float64  `json:"per,omitempty"`
	PBV               float64  `json:"pbv,omitempty"`
	ROE               float64  `json:"roe_pct,omitempty"` // annualized net income / equity, percent
	EPSAnnualized     float64  `json:"eps_annualized,omitempty"`
	BookValuePerShare float64  `json:"book_value_per_share,omitempty"`
	DividendYield     float64  `json:"dividend_yield_pct,omitempty"`
	Notes             []string `json:"notes,omitempty"`
	Error             string   `json:"error,omitempty"`
}

// StockComparison is a side-by-side valuation of several emitens against the
// same report year/period, in the caller's order.
type StockComparison struct {
	Year   string       `json:"year"`
	Period string       `json:"period"`
	Rows   []CompareRow `json:"rows"`
	Note   string       `json:"note,omitempty"`
}

// CompareStocks computes valuation ratios for up to maxCompareCodes tickers
// against the same year/period and lines them up for comparison. Rows keep
// the caller's order; a ticker whose data cannot be fetched (no filing yet,
// unknown code) degrades to an Error row instead of failing the whole call.
func (c *Client) CompareStocks(ctx context.Context, codes []string, year, period string) (*StockComparison, error) {
	seen := make(map[string]bool, len(codes))
	clean := make([]string, 0, len(codes))
	for _, cd := range codes {
		cd = normalizeCode(cd)
		if cd == "" || seen[cd] {
			continue
		}
		seen[cd] = true
		clean = append(clean, cd)
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("no emiten codes given")
	}
	if len(clean) < 2 {
		return nil, fmt.Errorf("need at least 2 emiten codes to compare (got %d); use get_valuation_ratios for a single ticker", len(clean))
	}

	cmp := &StockComparison{Year: year, Period: period}
	if len(clean) > maxCompareCodes {
		clean = clean[:maxCompareCodes]
		cmp.Note = fmt.Sprintf("more than %d codes given; only the first %d were compared", maxCompareCodes, maxCompareCodes)
	}

	for _, cd := range clean {
		v, err := c.ValuationRatios(ctx, cd, year, period)
		if err != nil {
			cmp.Rows = append(cmp.Rows, CompareRow{Code: cd, Error: err.Error()})
			continue
		}
		cmp.Rows = append(cmp.Rows, buildCompareRow(v))
		// Report the normalized period actually used (e.g. empty -> TW1).
		if cmp.Period == "" || cmp.Period != v.ReportPeriod {
			cmp.Period = v.ReportPeriod
		}
	}
	return cmp, nil
}

// buildCompareRow condenses full ValuationRatios into the comparison row,
// separated for testability.
func buildCompareRow(v *ValuationRatios) CompareRow {
	row := CompareRow{
		Code:              v.Code,
		Name:              v.Name,
		Price:             v.Price,
		MarketCap:         v.MarketCap,
		PER:               v.PER,
		PBV:               v.PBV,
		EPSAnnualized:     v.EPSAnnualized,
		BookValuePerShare: v.BookValuePerShare,
		DividendYield:     v.DividendYield,
		Notes:             v.Notes,
	}
	if v.Equity > 0 && v.NetIncomeAnnualized != 0 {
		row.ROE = v.NetIncomeAnnualized / v.Equity * 100
	}
	return row
}
