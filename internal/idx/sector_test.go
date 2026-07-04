package idx

import (
	"context"
	"testing"
)

const sectorDirectoryJSON = `{"recordsTotal":4,"data":[
 {"KodeEmiten":"BBCA","NamaEmiten":"PT Bank Central Asia Tbk","Sektor":"Keuangan"},
 {"KodeEmiten":"BBRI","NamaEmiten":"PT Bank Rakyat Indonesia Tbk","Sektor":"Keuangan"},
 {"KodeEmiten":"AADI","NamaEmiten":"PT Adaro Andalan Indonesia Tbk","Sektor":"Energi"},
 {"KodeEmiten":"TLKM","NamaEmiten":"PT Telkom Indonesia Tbk","Sektor":"Infrastruktur"}
]}`

const sectorStockJSON = `{"data":[
 {"Date":"2026-06-29T00:00:00","StockCode":"BBCA","Previous":5900,"Close":6000,"Change":100,"Volume":1000,"Value":6000000,"ForeignBuy":5000000,"ForeignSell":1000000},
 {"Date":"2026-06-29T00:00:00","StockCode":"BBRI","Previous":4200,"Close":4100,"Change":-100,"Volume":2000,"Value":8200000,"ForeignBuy":1000000,"ForeignSell":3000000},
 {"Date":"2026-06-29T00:00:00","StockCode":"AADI","Previous":7000,"Close":7000,"Change":0,"Volume":500,"Value":3500000,"ForeignBuy":0,"ForeignSell":0},
 {"Date":"2026-06-29T00:00:00","StockCode":"TLKM","Previous":3000,"Close":3100,"Change":100,"Volume":0,"Value":0,"ForeignBuy":0,"ForeignSell":0},
 {"Date":"2026-06-29T00:00:00","StockCode":"XXXX","Previous":100,"Close":110,"Change":10,"Volume":300,"Value":33000,"ForeignBuy":0,"ForeignSell":0}
]}`

func TestSectorSummary(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{
		"GetCompanyProfiles": sectorDirectoryJSON,
		"GetStockSummary":    sectorStockJSON,
	}}
	c := New(f, nil)

	sum, err := c.SectorSummary(context.Background())
	if err != nil {
		t.Fatalf("SectorSummary: %v", err)
	}
	if sum.Date != "2026-06-29" {
		t.Errorf("Date = %q, want 2026-06-29", sum.Date)
	}

	by := make(map[string]SectorPerformance)
	for _, s := range sum.Sectors {
		by[s.Sector] = s
	}

	// Keuangan: BBCA (up) + BBRI (down), both traded.
	fin := by["Keuangan"]
	if fin.StocksTraded != 2 || fin.Advancing != 1 || fin.Declining != 1 {
		t.Errorf("Keuangan breadth = %+v", fin)
	}
	if fin.TotalValue != 6000000+8200000 {
		t.Errorf("Keuangan value = %v", fin.TotalValue)
	}
	// Foreign net: BBCA (+4,000,000) + BBRI (-2,000,000) = +2,000,000
	if fin.ForeignNet != 2000000 {
		t.Errorf("Keuangan foreign net = %v, want 2000000", fin.ForeignNet)
	}

	// Energi: AADI traded but unchanged.
	en := by["Energi"]
	if en.StocksTraded != 1 || en.Unchanged != 1 {
		t.Errorf("Energi = %+v", en)
	}

	// Infrastruktur: TLKM has Volume 0, so it must not appear.
	if _, ok := by["Infrastruktur"]; ok {
		t.Errorf("Infrastruktur should be absent (only untraded stock)")
	}

	// XXXX is not in the directory -> classified as Unknown.
	unk := by["Unknown"]
	if unk.StocksTraded != 1 || unk.Advancing != 1 {
		t.Errorf("Unknown = %+v", unk)
	}

	// Sorted by total value desc: Keuangan (14.2M) first.
	if sum.Sectors[0].Sector != "Keuangan" {
		t.Errorf("first sector = %q, want Keuangan", sum.Sectors[0].Sector)
	}
}
