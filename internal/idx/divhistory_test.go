package idx

import (
	"context"
	"testing"
)

// divHistJSON mimics a GetAnnouncement keyword=dividen response (same envelope
// as the announcement fixture; ResultCount larger than the page to exercise
// the truncation flag).
const divHistJSON = `{"ResultCount":51,"Replies":[
 {"pengumuman":{"NoPengumuman":"PENG-DIV-00001/BEI.PP1/06-2026","TglPengumuman":"2026-06-10T15:30:00","JudulPengumuman":"Pengumuman Jadwal Pembagian Dividen Tunai PT Bank Central Asia Tbk","JenisPengumuman":"STOCK","Kode_Emiten":"BBCA      "},
  "attachments":[{"FullSavePath":"https://www.idx.co.id/StaticData/NewsAndAnnouncement/ANNOUNCEMENTSTOCK/From_EREP/202606/div.pdf","OriginalFilename":"div.pdf"}]},
 {"pengumuman":{"NoPengumuman":"PENG-DIV-00002/BEI.PP1/06-2025","TglPengumuman":"2025-06-11T15:30:00","JudulPengumuman":"Pengumuman Dividen Tunai Tahun Buku 2024","JenisPengumuman":"STOCK","Kode_Emiten":"BBCA      "},
  "attachments":[]}
]}`

func TestDividendHistory(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"keyword=dividen": divHistJSON}}
	c := New(f, nil)

	h, err := c.DividendHistory(context.Background(), "bbca", 0) // 0 -> default years
	if err != nil {
		t.Fatalf("DividendHistory: %v", err)
	}
	if h.Code != "BBCA" {
		t.Errorf("Code = %q, want BBCA", h.Code)
	}
	if len(h.Events) != 2 || h.Total != 51 {
		t.Fatalf("len=%d Total=%d, want 2/51", len(h.Events), h.Total)
	}
	if !h.Truncated {
		t.Error("Truncated = false, want true (51 matches > 2 returned)")
	}
	if h.Events[0].Date != "2026-06-10 15:30:00" {
		t.Errorf("Date = %q, want 2026-06-10 15:30:00", h.Events[0].Date)
	}
	if len(h.Events[0].Attachments) != 1 || h.Events[0].Attachments[0].Filename != "div.pdf" {
		t.Errorf("attachments = %+v, want one div.pdf", h.Events[0].Attachments)
	}
}

func TestDividendHistoryEmptyCode(t *testing.T) {
	c := New(&fakeFetcher{}, nil)
	if _, err := c.DividendHistory(context.Background(), "", 3); err == nil {
		t.Fatal("expected error for empty code")
	}
}
