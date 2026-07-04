package idx

import (
	"fmt"
	"sort"
	"strings"

	"context"
)

// Screener result caps, mirroring maxListResults for list_companies.
const (
	defaultScreenLimit = 20
	maxScreenLimit     = 100
)

// ScreenRow is one stock in the screener output.
type ScreenRow struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Sector        string  `json:"sector,omitempty"`
	Close         float64 `json:"close"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	Volume        float64 `json:"volume"`
	Value         float64 `json:"value"`
	Frequency     float64 `json:"frequency"`
	ForeignNet    float64 `json:"foreign_net"`
}

// ScreenerResult is the ranked, filtered slice of the daily per-stock feed.
type ScreenerResult struct {
	Date      string      `json:"date"`
	RankBy    string      `json:"rank_by"`
	Matched   int         `json:"matched"` // stocks passing the filters, before the limit
	Rows      []ScreenRow `json:"rows"`
	Truncated bool        `json:"truncated"`
	Notes     []string    `json:"notes,omitempty"`
}

// screenOptions are the parsed screener parameters.
type screenOptions struct {
	rankBy   string
	sector   string
	minValue float64
	minPrice float64
	maxPrice float64
	limit    int
}

// screenRankings maps each ranking to its sort key. All rankings only include
// stocks that actually traded (Volume > 0).
var screenRankings = map[string]func(a, b ScreenRow) bool{
	"top_gainers":        func(a, b ScreenRow) bool { return a.ChangePercent > b.ChangePercent },
	"top_losers":         func(a, b ScreenRow) bool { return a.ChangePercent < b.ChangePercent },
	"most_active_value":  func(a, b ScreenRow) bool { return a.Value > b.Value },
	"most_active_volume": func(a, b ScreenRow) bool { return a.Volume > b.Volume },
	"top_foreign_buy":    func(a, b ScreenRow) bool { return a.ForeignNet > b.ForeignNet },
	"top_foreign_sell":   func(a, b ScreenRow) bool { return a.ForeignNet < b.ForeignNet },
}

// ScreenStocks ranks and filters the latest daily per-stock feed. rankBy is
// one of: top_gainers, top_losers, most_active_value, most_active_volume,
// top_foreign_buy, top_foreign_sell (default most_active_value). Both feeds it
// joins (stock summary + directory) are already cached by other tools.
func (c *Client) ScreenStocks(ctx context.Context, rankBy, sector string, minValue, minPrice, maxPrice float64, limit int) (*ScreenerResult, error) {
	opts, err := parseScreenOptions(rankBy, sector, minValue, minPrice, maxPrice, limit)
	if err != nil {
		return nil, err
	}

	u := baseURL + "/primary/TradingSummary/GetStockSummary?length=9999&start=0"
	var raw rawStockSummary
	if err := c.getJSON(ctx, "marketsummary:latest", u, ttlIndex, &raw); err != nil {
		return nil, err
	}

	res := &ScreenerResult{RankBy: opts.rankBy}
	sectorOf, err := c.sectorByCode(ctx)
	if err != nil {
		if opts.sector != "" {
			return nil, fmt.Errorf("sector filter unavailable: %w", err)
		}
		sectorOf = nil
		res.Notes = append(res.Notes, "Company directory unavailable; sector column omitted.")
	}

	screenFeed(res, raw, sectorOf, opts)
	return res, nil
}

// parseScreenOptions validates and defaults the screener parameters.
func parseScreenOptions(rankBy, sector string, minValue, minPrice, maxPrice float64, limit int) (screenOptions, error) {
	opts := screenOptions{
		rankBy:   strings.ToLower(strings.TrimSpace(rankBy)),
		sector:   strings.ToLower(strings.TrimSpace(sector)),
		minValue: minValue,
		minPrice: minPrice,
		maxPrice: maxPrice,
		limit:    limit,
	}
	if opts.rankBy == "" {
		opts.rankBy = "most_active_value"
	}
	if _, ok := screenRankings[opts.rankBy]; !ok {
		keys := make([]string, 0, len(screenRankings))
		for k := range screenRankings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return opts, fmt.Errorf("unknown rank_by %q (valid: %s)", rankBy, strings.Join(keys, ", "))
	}
	if opts.limit <= 0 {
		opts.limit = defaultScreenLimit
	}
	if opts.limit > maxScreenLimit {
		opts.limit = maxScreenLimit
	}
	return opts, nil
}

// screenFeed filters, ranks, and caps the feed into res.
func screenFeed(res *ScreenerResult, raw rawStockSummary, sectorOf map[string]string, opts screenOptions) {
	rows := make([]ScreenRow, 0, len(raw.Data))
	for _, r := range raw.Data {
		if res.Date == "" && r.Date != "" {
			res.Date = trimDate(r.Date)
		}
		if r.Volume <= 0 { // untraded stocks carry stale prices; exclude
			continue
		}
		if r.Value < opts.minValue || r.Close < opts.minPrice {
			continue
		}
		if opts.maxPrice > 0 && r.Close > opts.maxPrice {
			continue
		}
		sec := sectorOf[normalizeCode(r.StockCode)]
		if opts.sector != "" && !strings.Contains(strings.ToLower(sec), opts.sector) {
			continue
		}
		row := ScreenRow{
			Code:       r.StockCode,
			Name:       r.StockName,
			Sector:     sec,
			Close:      r.Close,
			Change:     r.Change,
			Volume:     r.Volume,
			Value:      r.Value,
			Frequency:  r.Frequency,
			ForeignNet: r.ForeignBuy - r.ForeignSell,
		}
		if r.Previous > 0 {
			row.ChangePercent = r.Change / r.Previous * 100
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool { return screenRankings[opts.rankBy](rows[i], rows[j]) })
	res.Matched = len(rows)
	if len(rows) > opts.limit {
		rows = rows[:opts.limit]
		res.Truncated = true
	}
	res.Rows = rows
}
