package idx

import (
	"context"
	"sort"
)

// SectorPerformance rolls up one IDX sector for the latest trading day: how many
// of its stocks traded, market breadth within the sector, total traded value,
// and net foreign flow.
type SectorPerformance struct {
	Sector       string  `json:"sector"`
	StocksTraded int     `json:"stocks_traded"`
	Advancing    int     `json:"advancing"`
	Declining    int     `json:"declining"`
	Unchanged    int     `json:"unchanged"`
	TotalValue   float64 `json:"total_value"`
	ForeignNet   float64 `json:"foreign_net"`
}

// SectorSummary is the per-sector breakdown of the latest trading day, sorted by
// total traded value (busiest sector first). There is no sector endpoint; it is
// built by joining the company directory (code -> sector) with the daily
// per-stock feed.
type SectorSummary struct {
	Date    string              `json:"date"`
	Sectors []SectorPerformance `json:"sectors"`
}

// SectorSummary joins the listed-company directory with the daily stock feed to
// produce a per-sector market roll-up. Both underlying fetches share the cache
// keys used by list_companies and get_market_summary, so a warm cache serves
// this with no extra network cost.
func (c *Client) SectorSummary(ctx context.Context) (*SectorSummary, error) {
	sectorByCode, err := c.sectorByCode(ctx)
	if err != nil {
		return nil, err
	}

	u := baseURL + "/primary/TradingSummary/GetStockSummary?length=9999&start=0"
	var raw rawStockSummary
	if err := c.getJSON(ctx, "marketsummary:latest", u, ttlIndex, &raw); err != nil {
		return nil, err
	}

	agg := make(map[string]*SectorPerformance)
	date := ""
	for _, r := range raw.Data {
		if date == "" && r.Date != "" {
			date = trimDate(r.Date)
		}
		// Only stocks that actually traded count toward the roll-up.
		if r.Volume <= 0 {
			continue
		}
		sec := sectorByCode[normalizeCode(r.StockCode)]
		if sec == "" {
			sec = "Unknown"
		}
		p := agg[sec]
		if p == nil {
			p = &SectorPerformance{Sector: sec}
			agg[sec] = p
		}
		p.StocksTraded++
		p.TotalValue += r.Value
		p.ForeignNet += r.ForeignBuy - r.ForeignSell
		switch {
		case r.Close > r.Previous:
			p.Advancing++
		case r.Close < r.Previous:
			p.Declining++
		default:
			p.Unchanged++
		}
	}

	out := make([]SectorPerformance, 0, len(agg))
	for _, p := range agg {
		out = append(out, *p)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TotalValue > out[j].TotalValue })
	return &SectorSummary{Date: date, Sectors: out}, nil
}

// sectorByCode fetches the listed-company directory and maps each ticker to its
// sector. It reuses the "listedcompanies:all" cache entry populated by
// list_companies.
func (c *Client) sectorByCode(ctx context.Context) (map[string]string, error) {
	u := baseURL + "/primary/ListedCompany/GetCompanyProfiles?start=0&length=9999"
	var raw rawListedCompanies
	if err := c.getJSON(ctx, "listedcompanies:all", u, ttlListed, &raw); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(raw.Data))
	for _, r := range raw.Data {
		m[normalizeCode(r.KodeEmiten)] = r.Sektor
	}
	return m, nil
}
