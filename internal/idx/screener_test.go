package idx

import (
	"encoding/json"
	"testing"
)

func screenerFeed(t *testing.T) rawStockSummary {
	t.Helper()
	const feed = `{"data":[
	 {"Date":"2026-07-03T00:00:00","StockCode":"AAAA","StockName":"Alpha","Previous":100,"Close":110,"Change":10,"Volume":1000,"Value":110000,"Frequency":50,"ForeignBuy":500,"ForeignSell":100},
	 {"Date":"2026-07-03T00:00:00","StockCode":"BBBB","StockName":"Beta","Previous":200,"Close":190,"Change":-10,"Volume":2000,"Value":380000,"Frequency":80,"ForeignBuy":100,"ForeignSell":700},
	 {"Date":"2026-07-03T00:00:00","StockCode":"CCCC","StockName":"Gamma","Previous":50,"Close":51,"Change":1,"Volume":500,"Value":25500,"Frequency":20,"ForeignBuy":50,"ForeignSell":50},
	 {"Date":"2026-07-03T00:00:00","StockCode":"DDDD","StockName":"Delta (untraded)","Previous":80,"Close":80,"Change":0,"Volume":0,"Value":0,"Frequency":0,"ForeignBuy":0,"ForeignSell":0}
	]}`
	var raw rawStockSummary
	if err := json.Unmarshal([]byte(feed), &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return raw
}

func mustOpts(t *testing.T, rankBy, sector string, minValue, minPrice, maxPrice float64, limit int) screenOptions {
	t.Helper()
	opts, err := parseScreenOptions(rankBy, sector, minValue, minPrice, maxPrice, limit)
	if err != nil {
		t.Fatalf("parseScreenOptions: %v", err)
	}
	return opts
}

var screenerSectors = map[string]string{"AAAA": "Energi", "BBBB": "Keuangan", "CCCC": "Energi"}

func TestScreenFeedRankings(t *testing.T) {
	raw := screenerFeed(t)

	res := &ScreenerResult{}
	screenFeed(res, raw, screenerSectors, mustOpts(t, "", "", 0, 0, 0, 0))
	// Default most_active_value: BBBB (380k) first; untraded DDDD excluded.
	if res.Date != "2026-07-03" || res.Matched != 3 || res.Rows[0].Code != "BBBB" {
		t.Errorf("default ranking = %+v", res)
	}
	if res.Rows[0].Sector != "Keuangan" {
		t.Errorf("sector join failed: %+v", res.Rows[0])
	}

	res = &ScreenerResult{}
	screenFeed(res, raw, screenerSectors, mustOpts(t, "top_gainers", "", 0, 0, 0, 0))
	if res.Rows[0].Code != "AAAA" || res.Rows[0].ChangePercent != 10 {
		t.Errorf("top gainer = %+v, want AAAA +10%%", res.Rows[0])
	}

	res = &ScreenerResult{}
	screenFeed(res, raw, screenerSectors, mustOpts(t, "top_foreign_sell", "", 0, 0, 0, 0))
	if res.Rows[0].Code != "BBBB" || res.Rows[0].ForeignNet != -600 {
		t.Errorf("top foreign sell = %+v, want BBBB -600", res.Rows[0])
	}
}

func TestScreenFeedFilters(t *testing.T) {
	raw := screenerFeed(t)

	// Sector filter (case-insensitive substring).
	res := &ScreenerResult{}
	screenFeed(res, raw, screenerSectors, mustOpts(t, "", "energi", 0, 0, 0, 0))
	if res.Matched != 2 {
		t.Errorf("sector filter matched %d, want 2", res.Matched)
	}

	// Price band: only CCCC (51) sits within 40..60.
	res = &ScreenerResult{}
	screenFeed(res, raw, screenerSectors, mustOpts(t, "", "", 0, 40, 60, 0))
	if res.Matched != 1 || res.Rows[0].Code != "CCCC" {
		t.Errorf("price band = %+v, want only CCCC", res.Rows)
	}

	// Min traded value.
	res = &ScreenerResult{}
	screenFeed(res, raw, screenerSectors, mustOpts(t, "", "", 100000, 0, 0, 0))
	if res.Matched != 2 {
		t.Errorf("min value matched %d, want 2 (AAAA+BBBB)", res.Matched)
	}

	// Limit + truncation.
	res = &ScreenerResult{}
	screenFeed(res, raw, screenerSectors, mustOpts(t, "", "", 0, 0, 0, 2))
	if len(res.Rows) != 2 || !res.Truncated || res.Matched != 3 {
		t.Errorf("limit: rows=%d truncated=%v matched=%d", len(res.Rows), res.Truncated, res.Matched)
	}
}

func TestParseScreenOptions(t *testing.T) {
	if _, err := parseScreenOptions("bogus", "", 0, 0, 0, 0); err == nil {
		t.Error("expected error for unknown rank_by")
	}
	opts := mustOpts(t, "", "", 0, 0, 0, 9999)
	if opts.rankBy != "most_active_value" || opts.limit != maxScreenLimit {
		t.Errorf("defaults = %+v", opts)
	}
}
