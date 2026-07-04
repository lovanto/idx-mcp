package idx

import (
	"context"
	"sort"
)

// BrokerActivity is one broker's aggregate trading activity for a day, from
// TradingSummary/GetBrokerSummary. The endpoint reports market-wide totals per
// broker (no per-stock breakdown and no buy/sell split).
type BrokerActivity struct {
	Code      string  `json:"code"` // IDX firm code, e.g. "AD"
	Name      string  `json:"name"`
	Volume    float64 `json:"volume"`
	Value     float64 `json:"value"`
	Frequency float64 `json:"frequency"`
}

type rawBrokerSummary struct {
	Data []struct {
		IDFirm    string  `json:"IDFirm"`
		FirmName  string  `json:"FirmName"`
		Volume    float64 `json:"Volume"`
		Value     float64 `json:"Value"`
		Frequency float64 `json:"Frequency"`
	} `json:"data"`
}

// BrokerSummary returns per-broker trading activity for the latest trading day,
// sorted by traded value (most active first). length=100 pulls all ~88 brokers
// past the DataTables default page.
func (c *Client) BrokerSummary(ctx context.Context) ([]BrokerActivity, error) {
	u := baseURL + "/primary/TradingSummary/GetBrokerSummary?length=100&start=0"
	var raw rawBrokerSummary
	if err := c.getJSON(ctx, "brokersummary:latest", u, ttlIndex, &raw); err != nil {
		return nil, err
	}

	out := make([]BrokerActivity, 0, len(raw.Data))
	for _, b := range raw.Data {
		out = append(out, BrokerActivity{
			Code:      b.IDFirm,
			Name:      b.FirmName,
			Volume:    b.Volume,
			Value:     b.Value,
			Frequency: b.Frequency,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Value > out[j].Value })
	return out, nil
}
