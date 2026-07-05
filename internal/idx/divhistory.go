package idx

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Dividend-history caps. The window is expressed in years because dividend
// events cluster annually; the row cap matches what GetAnnouncement will
// return in one page.
const (
	defaultDivYears = 3
	maxDivYears     = 5
	divPageSize     = 50
)

// ttlDivHistory caches the dividend announcement timeline. The cache key is
// date-stamped, so entries effectively refresh daily regardless of TTL.
const ttlDivHistory = 24 * time.Hour

// DividendEvent is one dividend-related IDX disclosure.
type DividendEvent struct {
	Number      string                   `json:"number"`
	Date        string                   `json:"date"`
	Title       string                   `json:"title"`
	Attachments []AnnouncementAttachment `json:"attachments,omitempty"`
}

// DividendHistory is the multi-year timeline of dividend-related disclosures
// for one emiten, newest first. Titles carry the schedule announcements
// (cash dividend plans, cum/ex dates); the amounts themselves live in the
// attached PDFs, which IDX does not expose in structured form.
type DividendHistory struct {
	Code      string          `json:"code"`
	From      string          `json:"from"`
	To        string          `json:"to"`
	Total     int             `json:"total"` // matches on IDX's side, before the page cap
	Events    []DividendEvent `json:"events"`
	Truncated bool            `json:"truncated"`
	Note      string          `json:"note,omitempty"`
}

// DividendHistory returns dividend-related official disclosures for `code`
// over the last `years` years (default 3, max 5), newest first. It rides on
// GetAnnouncement's own keyword search; use get_dividends for the structured
// details (cash per share, cum/ex/payment dates) of the latest declaration.
func (c *Client) DividendHistory(ctx context.Context, code string, years int) (*DividendHistory, error) {
	code = normalizeCode(code)
	if code == "" {
		return nil, fmt.Errorf("empty emiten code")
	}
	if years < 1 {
		years = defaultDivYears
	}
	if years > maxDivYears {
		years = maxDivYears
	}

	now := time.Now()
	from := now.AddDate(-years, 0, 0)
	u := fmt.Sprintf(
		"%s/primary/ListedCompany/GetAnnouncement?kodeEmiten=%s&emitenType=*&indexFrom=0&pageSize=%d&dateFrom=%s&dateTo=%s&lang=id&keyword=%s",
		baseURL, url.QueryEscape(code), divPageSize,
		from.Format("20060102"), now.Format("20060102"), url.QueryEscape("dividen"))
	key := fmt.Sprintf("divhist:%s:%s:%d", code, now.Format("20060102"), years)

	var raw rawAnnouncementResponse
	if err := c.getJSON(ctx, key, u, ttlDivHistory, &raw); err != nil {
		return nil, err
	}

	h := &DividendHistory{
		Code:  code,
		From:  from.Format("2006-01-02"),
		To:    now.Format("2006-01-02"),
		Total: raw.ResultCount,
		Note:  "timeline of dividend-related disclosures; amounts are in the attached PDFs — use get_dividends for the latest structured declaration",
	}
	for _, r := range raw.Replies {
		e := DividendEvent{
			Number: r.Pengumuman.NoPengumuman,
			Date:   strings.Replace(r.Pengumuman.TglPengumuman, "T", " ", 1),
			Title:  r.Pengumuman.JudulPengumuman,
		}
		for _, att := range r.Attachments {
			if att.FullSavePath == "" {
				continue
			}
			e.Attachments = append(e.Attachments, AnnouncementAttachment{
				Filename: att.OriginalFilename,
				URL:      att.FullSavePath,
			})
		}
		h.Events = append(h.Events, e)
	}
	h.Truncated = h.Total > len(h.Events)
	return h, nil
}
