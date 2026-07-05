package idx

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"time"
)

// ttlIssued caches the issued-shares history. Corporate actions that change
// the share count (splits, rights issues, buyback retirements) happen at most
// a few times a year per issuer, so a long TTL is safe.
const ttlIssued = 7 * 24 * time.Hour

// IssuedAction is one corporate action that changed an issuer's listed share
// count (ListingActivity/GetIssuedHistory).
type IssuedAction struct {
	Date        string `json:"date"`
	Action      string `json:"action"`       // IDX's own tag, e.g. stockSplit, partialDelisting, rightIssue
	Shares      int64  `json:"shares"`       // shares involved in the action
	SharesAfter int64  `json:"shares_after"` // issued shares after the action
}

// IssuedHistory is the share-count timeline for one emiten, oldest first.
type IssuedHistory struct {
	Code    string         `json:"code"`
	Total   int            `json:"total"`
	Actions []IssuedAction `json:"actions"`
}

// rawIssuedHistory maps GetIssuedHistory (modelled on a captured response;
// DataTables envelope like the company directory).
type rawIssuedHistory struct {
	RecordsTotal int `json:"recordsTotal"`
	Data         []struct {
		KodeEmiten                 string  `json:"KodeEmiten"`
		TanggalPencatatan          string  `json:"TanggalPencatatan"`
		JenisTindakan              string  `json:"JenisTindakan"`
		JumlahSaham                float64 `json:"JumlahSaham"`
		JumlahSahamSetelahTindakan float64 `json:"JumlahSahamSetelahTindakan"`
	} `json:"data"`
}

// IssuedHistory returns the corporate actions that changed `code`'s issued
// share count (splits, rights issues, partial delistings, …), oldest first.
// Useful for spotting dilution or float changes behind a price history.
func (c *Client) IssuedHistory(ctx context.Context, code string) (*IssuedHistory, error) {
	code = normalizeCode(code)
	if code == "" {
		return nil, fmt.Errorf("empty emiten code")
	}
	u := fmt.Sprintf("%s/primary/ListingActivity/GetIssuedHistory?kodeEmiten=%s&indexFrom=0&pageSize=500",
		baseURL, url.QueryEscape(code))
	key := "issued:" + code

	var raw rawIssuedHistory
	if err := c.getJSON(ctx, key, u, ttlIssued, &raw); err != nil {
		return nil, err
	}

	h := &IssuedHistory{Code: code, Total: raw.RecordsTotal}
	for _, r := range raw.Data {
		h.Actions = append(h.Actions, IssuedAction{
			Date:        trimDate(r.TanggalPencatatan),
			Action:      r.JenisTindakan,
			Shares:      int64(math.Round(r.JumlahSaham)),
			SharesAfter: int64(math.Round(r.JumlahSahamSetelahTindakan)),
		})
	}
	sort.SliceStable(h.Actions, func(i, j int) bool { return h.Actions[i].Date < h.Actions[j].Date })
	return h, nil
}
