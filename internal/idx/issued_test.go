package idx

import (
	"context"
	"testing"
)

// issuedJSON is trimmed from a real GetIssuedHistory capture for BBCA
// (July 2026), reordered to exercise the oldest-first sort.
const issuedJSON = `{"draw":0,"recordsTotal":3,"recordsFiltered":3,"data":[
 {"id":18534,"KodeEmiten":"BBCA","TanggalPencatatan":"2021-10-13T00:00:00","JenisTindakan":"stockSplit","JumlahSaham":98620040000.0,"JumlahSahamSetelahTindakan":123275050000.0},
 {"id":1,"KodeEmiten":"BBCA","TanggalPencatatan":"2008-01-31T00:00:00","JenisTindakan":"stockSplit","JumlahSaham":12328760000.0,"JumlahSahamSetelahTindakan":24657520000.0},
 {"id":18533,"KodeEmiten":"BBCA","TanggalPencatatan":"2021-10-13T00:00:00","JenisTindakan":"partialDelisting","JumlahSaham":1232750500.0,"JumlahSahamSetelahTindakan":122042299500.0}
]}`

func TestIssuedHistory(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{"GetIssuedHistory": issuedJSON}}
	c := New(f, nil)

	h, err := c.IssuedHistory(context.Background(), "bbca")
	if err != nil {
		t.Fatalf("IssuedHistory: %v", err)
	}
	if h.Code != "BBCA" {
		t.Errorf("Code = %q, want BBCA", h.Code)
	}
	if h.Total != 3 || len(h.Actions) != 3 {
		t.Fatalf("Total=%d len=%d, want 3/3", h.Total, len(h.Actions))
	}
	if h.Actions[0].Date != "2008-01-31" {
		t.Errorf("actions not sorted oldest first: first date = %s", h.Actions[0].Date)
	}
	last := h.Actions[2]
	if last.Action != "partialDelisting" || last.SharesAfter != 122042299500 {
		t.Errorf("last action = %+v, want partialDelisting ending at 122042299500", last)
	}
}

func TestIssuedHistoryEmptyCode(t *testing.T) {
	c := New(&fakeFetcher{}, nil)
	if _, err := c.IssuedHistory(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty code")
	}
}
