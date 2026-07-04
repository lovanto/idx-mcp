package idx

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// TradingDay is one daily OHLCV + foreign-flow record. Field set mirrors the
// GetTradingInfoSS response; foreign buy/sell are the official IDX figures that
// yfinance-based competitors lack.
type TradingDay struct {
	Date         string  `json:"date"`
	StockCode    string  `json:"stock_code"`
	StockName    string  `json:"stock_name,omitempty"`
	Previous     float64 `json:"previous"`
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	Close        float64 `json:"close"`
	Change       float64 `json:"change"`
	Volume       float64 `json:"volume"`
	Value        float64 `json:"value"`
	Frequency    float64 `json:"frequency"`
	ListedShares float64 `json:"listed_shares,omitempty"`
	ForeignBuy   float64 `json:"foreign_buy"`
	ForeignSell  float64 `json:"foreign_sell"`
	ForeignNet   float64 `json:"foreign_net"` // derived: buy - sell
}

// rawTradingResponse maps the GetTradingInfoSS payload. Field names match the
// live JSON exactly (verified in the spike).
type rawTradingResponse struct {
	KodeEmiten string `json:"KodeEmiten"`
	Replies    []struct {
		Date         string  `json:"Date"`
		StockCode    string  `json:"StockCode"`
		StockName    string  `json:"StockName"`
		Previous     float64 `json:"Previous"`
		OpenPrice    float64 `json:"OpenPrice"`
		High         float64 `json:"High"`
		Low          float64 `json:"Low"`
		Close        float64 `json:"Close"`
		Change       float64 `json:"Change"`
		Volume       float64 `json:"Volume"`
		Value        float64 `json:"Value"`
		Frequency    float64 `json:"Frequency"`
		ListedShares float64 `json:"ListedShares"`
		ForeignBuy   float64 `json:"ForeignBuy"`
		ForeignSell  float64 `json:"ForeignSell"`
	} `json:"replies"`
}

// TradingInfo returns up to `length` recent trading days for `code` (e.g.
// "BBCA"), most recent first. length is clamped to [1, 365].
func (c *Client) TradingInfo(ctx context.Context, code string, length int) ([]TradingDay, error) {
	code = normalizeCode(code)
	if code == "" {
		return nil, fmt.Errorf("empty emiten code")
	}
	if length < 1 {
		length = 30
	}
	if length > 365 {
		length = 365
	}

	u := fmt.Sprintf("%s/primary/ListedCompany/GetTradingInfoSS?code=%s&length=%d",
		baseURL, url.QueryEscape(code), length)
	key := fmt.Sprintf("trading:%s:%d", code, length)

	var raw rawTradingResponse
	if err := c.getJSON(ctx, key, u, ttlTrading, &raw); err != nil {
		return nil, err
	}

	days := make([]TradingDay, 0, len(raw.Replies))
	for _, r := range raw.Replies {
		days = append(days, TradingDay{
			Date:         trimDate(r.Date),
			StockCode:    r.StockCode,
			StockName:    r.StockName,
			Previous:     r.Previous,
			Open:         r.OpenPrice,
			High:         r.High,
			Low:          r.Low,
			Close:        r.Close,
			Change:       r.Change,
			Volume:       r.Volume,
			Value:        r.Value,
			Frequency:    r.Frequency,
			ListedShares: r.ListedShares,
			ForeignBuy:   r.ForeignBuy,
			ForeignSell:  r.ForeignSell,
			ForeignNet:   r.ForeignBuy - r.ForeignSell,
		})
	}
	return days, nil
}

// normalizeCode upper-cases and trims an emiten ticker.
func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// trimDate drops the time component from IDX's "2026-06-29T00:00:00" dates.
func trimDate(d string) string {
	if i := strings.IndexByte(d, 'T'); i > 0 {
		return d[:i]
	}
	return d
}
