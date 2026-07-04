package idx

import (
	"context"
	"strings"
)

// MarketIndex is the latest end-of-day summary for one IDX index (e.g.
// COMPOSITE/IHSG, LQ45, IDX30). Source: TradingSummary/GetIndexSummary, which
// returns all indices in a single call.
type MarketIndex struct {
	Code           string  `json:"code"`
	Date           string  `json:"date"`
	Previous       float64 `json:"previous"`
	High           float64 `json:"high"`
	Low            float64 `json:"low"`
	Close          float64 `json:"close"`
	Change         float64 `json:"change"`
	ChangePercent  float64 `json:"change_percent"` // derived: Change / Previous * 100
	Volume         float64 `json:"volume"`
	Value          float64 `json:"value"`
	Frequency      float64 `json:"frequency"`
	NumberOfStocks float64 `json:"number_of_stocks"`
	MarketCap      float64 `json:"market_cap"`
}

type rawIndexResponse struct {
	Data []struct {
		Date          string  `json:"Date"`
		IndexCode     string  `json:"IndexCode"`
		Previous      float64 `json:"Previous"`
		Highest       float64 `json:"Highest"`
		Lowest        float64 `json:"Lowest"`
		Close         float64 `json:"Close"`
		Change        float64 `json:"Change"`
		Volume        float64 `json:"Volume"`
		Value         float64 `json:"Value"`
		Frequency     float64 `json:"Frequency"`
		NumberOfStock float64 `json:"NumberOfStock"`
		MarketCapital float64 `json:"MarketCapital"`
	} `json:"data"`
}

// MarketIndices returns the latest end-of-day summary for IDX indices. If code
// is non-empty (e.g. "COMPOSITE", "LQ45"), only that index is returned;
// otherwise all indices are returned in the API's order. The upstream endpoint
// returns every index in one request, so filtering happens client-side over a
// single cached fetch.
func (c *Client) MarketIndices(ctx context.Context, code string) ([]MarketIndex, error) {
	code = normalizeCode(code)

	// GetIndexSummary is a DataTables-style endpoint that paginates to 10 rows
	// by default; length=100 pulls all ~45 indices in one request.
	u := baseURL + "/primary/TradingSummary/GetIndexSummary?length=100&start=0"
	var raw rawIndexResponse
	if err := c.getJSON(ctx, "index:all", u, ttlIndex, &raw); err != nil {
		return nil, err
	}

	out := make([]MarketIndex, 0, len(raw.Data))
	for _, r := range raw.Data {
		if code != "" && !strings.EqualFold(r.IndexCode, code) {
			continue
		}
		idx := MarketIndex{
			Code:           r.IndexCode,
			Date:           trimDate(r.Date),
			Previous:       r.Previous,
			High:           r.Highest,
			Low:            r.Lowest,
			Close:          r.Close,
			Change:         r.Change,
			Volume:         r.Volume,
			Value:          r.Value,
			Frequency:      r.Frequency,
			NumberOfStocks: r.NumberOfStock,
			MarketCap:      r.MarketCapital,
		}
		if r.Previous != 0 {
			idx.ChangePercent = r.Change / r.Previous * 100
		}
		out = append(out, idx)
	}
	return out, nil
}
