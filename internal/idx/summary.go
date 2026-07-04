package idx

import (
	"context"
	"sort"
)

// topMoversCount is how many gainers/losers the market summary reports.
const topMoversCount = 5

// MarketSummary is an exchange-wide roll-up for one trading day, aggregated
// from the per-stock GetStockSummary feed (there is no single market-summary
// endpoint). It includes market breadth, turnover, official net foreign flow,
// and the day's biggest movers.
type MarketSummary struct {
	Date           string      `json:"date"`
	StocksTraded   int         `json:"stocks_traded"`
	Advancing      int         `json:"advancing"`
	Declining      int         `json:"declining"`
	Unchanged      int         `json:"unchanged"`
	TotalVolume    float64     `json:"total_volume"`
	TotalValue     float64     `json:"total_value"`
	TotalFrequency float64     `json:"total_frequency"`
	ForeignBuy     float64     `json:"foreign_buy"`
	ForeignSell    float64     `json:"foreign_sell"`
	ForeignNet     float64     `json:"foreign_net"`
	TopGainers     []StockMove `json:"top_gainers"`
	TopLosers      []StockMove `json:"top_losers"`
}

// StockMove is one entry in the top gainers/losers lists.
type StockMove struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Close         float64 `json:"close"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
}

type rawStockSummary struct {
	Data []struct {
		Date        string  `json:"Date"`
		StockCode   string  `json:"StockCode"`
		StockName   string  `json:"StockName"`
		Previous    float64 `json:"Previous"`
		Close       float64 `json:"Close"`
		Change      float64 `json:"Change"`
		Volume      float64 `json:"Volume"`
		Value       float64 `json:"Value"`
		Frequency   float64 `json:"Frequency"`
		ForeignBuy  float64 `json:"ForeignBuy"`
		ForeignSell float64 `json:"ForeignSell"`
	} `json:"data"`
}

// MarketSummary fetches the latest daily per-stock feed and rolls it up into an
// exchange-wide summary. GetStockSummary returns every stock in one call
// regardless of the length parameter; length=9999 is pinned defensively in case
// the default page size ever shrinks.
func (c *Client) MarketSummary(ctx context.Context) (*MarketSummary, error) {
	u := baseURL + "/primary/TradingSummary/GetStockSummary?length=9999&start=0"
	var raw rawStockSummary
	if err := c.getJSON(ctx, "marketsummary:latest", u, ttlIndex, &raw); err != nil {
		return nil, err
	}

	sum := &MarketSummary{}
	gainers := make([]StockMove, 0, len(raw.Data))
	losers := make([]StockMove, 0, len(raw.Data))

	for _, r := range raw.Data {
		if sum.Date == "" && r.Date != "" {
			sum.Date = trimDate(r.Date)
		}
		// Only stocks that actually traded count toward breadth and turnover.
		if r.Volume <= 0 {
			continue
		}
		sum.StocksTraded++
		sum.TotalVolume += r.Volume
		sum.TotalValue += r.Value
		sum.TotalFrequency += r.Frequency
		sum.ForeignBuy += r.ForeignBuy
		sum.ForeignSell += r.ForeignSell

		switch {
		case r.Close > r.Previous:
			sum.Advancing++
		case r.Close < r.Previous:
			sum.Declining++
		default:
			sum.Unchanged++
		}

		if r.Previous > 0 {
			mv := StockMove{
				Code:          r.StockCode,
				Name:          r.StockName,
				Close:         r.Close,
				Change:        r.Change,
				ChangePercent: r.Change / r.Previous * 100,
			}
			if r.Change > 0 {
				gainers = append(gainers, mv)
			} else if r.Change < 0 {
				losers = append(losers, mv)
			}
		}
	}
	sum.ForeignNet = sum.ForeignBuy - sum.ForeignSell

	sort.SliceStable(gainers, func(i, j int) bool { return gainers[i].ChangePercent > gainers[j].ChangePercent })
	sort.SliceStable(losers, func(i, j int) bool { return losers[i].ChangePercent < losers[j].ChangePercent })
	sum.TopGainers = capMoves(gainers, topMoversCount)
	sum.TopLosers = capMoves(losers, topMoversCount)

	return sum, nil
}

func capMoves(m []StockMove, n int) []StockMove {
	if len(m) > n {
		return m[:n]
	}
	return m
}
