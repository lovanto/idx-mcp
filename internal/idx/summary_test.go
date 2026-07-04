package idx

import (
	"context"
	"testing"
)

// Fixture: 5 stocks. GAIN +25%, MILD +5% (advancing); DROP -20% (declining);
// FLAT unchanged; DEAD has zero volume (untraded, excluded from all counts).
const stockSummaryJSON = `{"recordsTotal":5,"data":[
 {"Date":"2026-07-03T00:00:00","StockCode":"GAIN","StockName":"Gainer","Previous":100,"Close":125,"Change":25,"Volume":1000,"Value":125000,"Frequency":10,"ForeignBuy":800,"ForeignSell":300},
 {"Date":"2026-07-03T00:00:00","StockCode":"MILD","StockName":"Mild Up","Previous":200,"Close":210,"Change":10,"Volume":500,"Value":105000,"Frequency":5,"ForeignBuy":100,"ForeignSell":100},
 {"Date":"2026-07-03T00:00:00","StockCode":"DROP","StockName":"Dropper","Previous":50,"Close":40,"Change":-10,"Volume":2000,"Value":80000,"Frequency":20,"ForeignBuy":0,"ForeignSell":400},
 {"Date":"2026-07-03T00:00:00","StockCode":"FLAT","StockName":"Flatline","Previous":300,"Close":300,"Change":0,"Volume":100,"Value":30000,"Frequency":2,"ForeignBuy":0,"ForeignSell":0},
 {"Date":"2026-07-03T00:00:00","StockCode":"DEAD","StockName":"Untraded","Previous":10,"Close":0,"Change":0,"Volume":0,"Value":0,"Frequency":0,"ForeignBuy":0,"ForeignSell":0}
]}`

func TestMarketSummary(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetStockSummary": stockSummaryJSON}}
	c := New(f, nil)

	s, err := c.MarketSummary(context.Background())
	if err != nil {
		t.Fatalf("MarketSummary: %v", err)
	}
	if s.Date != "2026-07-03" {
		t.Errorf("date = %q", s.Date)
	}
	// DEAD is untraded -> excluded. 4 traded: 2 up, 1 down, 1 flat.
	if s.StocksTraded != 4 || s.Advancing != 2 || s.Declining != 1 || s.Unchanged != 1 {
		t.Errorf("breadth wrong: traded=%d adv=%d dec=%d unch=%d", s.StocksTraded, s.Advancing, s.Declining, s.Unchanged)
	}
	// Turnover excludes DEAD: volume 1000+500+2000+100=3600.
	if s.TotalVolume != 3600 {
		t.Errorf("total volume = %v, want 3600", s.TotalVolume)
	}
	// Foreign net = (800+100+0+0) - (300+100+400+0) = 900 - 800 = 100.
	if s.ForeignNet != 100 {
		t.Errorf("foreign net = %v, want 100", s.ForeignNet)
	}
	// Top gainer is GAIN (+25%) ahead of MILD (+5%).
	if len(s.TopGainers) != 2 || s.TopGainers[0].Code != "GAIN" || s.TopGainers[1].Code != "MILD" {
		t.Errorf("top gainers = %+v", s.TopGainers)
	}
	if len(s.TopLosers) != 1 || s.TopLosers[0].Code != "DROP" {
		t.Errorf("top losers = %+v", s.TopLosers)
	}
	// FLAT (unchanged) must appear in neither movers list.
	for _, m := range append(s.TopGainers, s.TopLosers...) {
		if m.Code == "FLAT" || m.Code == "DEAD" {
			t.Errorf("unexpected mover %s", m.Code)
		}
	}
}
