package idx

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lovanto/idx-mcp/internal/xlsx"
)

// ttlIdxUploads caches the constituent-announcement listing. Index
// compositions change on IDX's evaluation calendar (roughly quarterly), so a
// day is plenty. The announcement zips themselves are immutable and cached
// forever.
const ttlIdxUploads = 24 * time.Hour

// tickerRe matches an IDX stock code cell in the constituent sheets.
var tickerRe = regexp.MustCompile(`^[A-Z]{4}$`)

// IndexConstituent is one member of a stock index per the latest evaluation
// announcement.
type IndexConstituent struct {
	Code           string  `json:"code"`
	FreeFloatRatio float64 `json:"free_float_ratio,omitempty"`
	IndexShares    float64 `json:"index_shares,omitempty"` // shares used for index calculation (post-evaluation)
	WeightPercent  float64 `json:"weight_pct,omitempty"`   // capped weight (post-evaluation)
	Change         string  `json:"change,omitempty"`       // Tetap / Naik / Turun / Baru
}

// IndexConstituents is the composition of one IDX stock index, sourced from
// the official evaluation announcement's attached workbook.
type IndexConstituents struct {
	IndexCode       string             `json:"index_code"`
	EffectivePeriod string             `json:"effective_period,omitempty"`
	Announcement    string             `json:"announcement,omitempty"`
	AnnouncedDate   string             `json:"announced_date,omitempty"`
	SourceFile      string             `json:"source_file,omitempty"`
	Total           int                `json:"total"`
	Constituents    []IndexConstituent `json:"constituents"`
	Note            string             `json:"note,omitempty"`
}

// rawStockUploader maps StockData/GetStockUploader (modelled on a captured
// response; TypeIndex is space-padded, AttachmentUrl may use backslashes).
type rawStockUploader struct {
	ResultCount int `json:"ResultCount"`
	Results     []struct {
		StockUploaderID int    `json:"StockUploaderID"`
		Date            string `json:"Date"`
		NoPengumuman    string `json:"NoPengumuman"`
		TypeIndex       string `json:"TypeIndex"`
		Description     string `json:"Description"`
		Year            string `json:"Year"`
		AttachmentName  string `json:"AttachmentName"`
		AttachmentUrl   string `json:"AttachmentUrl"`
	} `json:"Results"`
}

// IndexConstituents returns the current members of the stock index `indexCode`
// (e.g. LQ45, IDX30, IDX80, KOMPAS100) from the latest evaluation announcement:
// code, free-float ratio, index shares, capped weight, and the evaluation
// change tag. The constituent list lives in an xlsx attached to the
// announcement, so availability follows what IDX publishes per index.
func (c *Client) IndexConstituents(ctx context.Context, indexCode string) (*IndexConstituents, error) {
	indexCode = strings.ToUpper(strings.TrimSpace(indexCode))
	if indexCode == "" {
		return nil, fmt.Errorf("empty index code (e.g. LQ45, IDX30, KOMPAS100)")
	}

	year := time.Now().Year()
	raw, err := c.stockIndexUploads(ctx, indexCode, year)
	if err != nil {
		return nil, err
	}
	if len(raw.Results) == 0 {
		// Early in the year the newest evaluation may still be filed under
		// the previous year.
		if raw, err = c.stockIndexUploads(ctx, indexCode, year-1); err != nil {
			return nil, err
		}
	}
	if len(raw.Results) == 0 {
		return nil, fmt.Errorf("no constituent announcements found for index %q (check the index code, e.g. LQ45, IDX30, IDX80, KOMPAS100)", indexCode)
	}

	newest := raw.Results[0]
	for _, r := range raw.Results[1:] {
		if r.Date > newest.Date {
			newest = r
		}
	}

	zipURL := buildStaticURL(strings.ReplaceAll(newest.AttachmentUrl, `\`, "/"))
	zipKey := fmt.Sprintf("stockidx:zip:%d", newest.StockUploaderID)
	zipBytes, err := c.getRaw(ctx, zipKey, zipURL, 0) // announcement files are immutable
	if err != nil {
		return nil, fmt.Errorf("download constituent attachment: %w", err)
	}

	sheetBytes, sheetName, err := findIndexWorkbook(zipBytes, indexCode)
	if err != nil {
		return nil, fmt.Errorf("%s (announcement %s)", err, newest.NoPengumuman)
	}

	rows, err := xlsx.Rows(sheetBytes)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", sheetName, err)
	}

	out := parseConstituentRows(rows)
	out.IndexCode = indexCode
	out.Announcement = newest.NoPengumuman
	out.AnnouncedDate = trimDate(newest.Date)
	out.SourceFile = sheetName
	out.Note = "composition per IDX's official evaluation announcement (" + newest.Description + "); weights are as of the evaluation, not live"
	if out.Total == 0 {
		return nil, fmt.Errorf("no constituents recognized in %s — the workbook layout may have changed", sheetName)
	}
	return out, nil
}

// stockIndexUploads fetches the constituent-announcement listing for one index
// type and year.
func (c *Client) stockIndexUploads(ctx context.Context, indexCode string, year int) (*rawStockUploader, error) {
	u := fmt.Sprintf("%s/secondary/get/StockData/GetStockUploader?typeIndex=%s&year=%d&table=stockIndex&locale=id",
		baseURL, url.QueryEscape(indexCode), year)
	key := fmt.Sprintf("stockidx:list:%s:%d", indexCode, year)
	var raw rawStockUploader
	if err := c.getJSON(ctx, key, u, ttlIdxUploads, &raw); err != nil {
		return nil, err
	}
	return &raw, nil
}

// findIndexWorkbook picks the xlsx inside the announcement zip that belongs to
// `indexCode`. Evaluation zips bundle one workbook per index (plus the PDF
// announcement); names differ in punctuation ("IDX ESGL", "BISNIS-27"), so
// matching is done on alphanumerics only. A zip holding a single workbook is
// used as-is.
func findIndexWorkbook(zipBytes []byte, indexCode string) ([]byte, string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, "", fmt.Errorf("open attachment zip: %w", err)
	}
	want := alnumUpper(indexCode)
	var xlsxFiles []*zip.File
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".xlsx") {
			xlsxFiles = append(xlsxFiles, f)
		}
	}
	var match *zip.File
	for _, f := range xlsxFiles {
		if strings.Contains(alnumUpper(f.Name), want) {
			match = f
			break
		}
	}
	if match == nil && len(xlsxFiles) == 1 {
		match = xlsxFiles[0]
	}
	if match == nil {
		names := make([]string, len(xlsxFiles))
		for i, f := range xlsxFiles {
			names[i] = f.Name
		}
		return nil, "", fmt.Errorf("no workbook for %s in attachment (found: %s)", indexCode, strings.Join(names, "; "))
	}
	rc, err := match.Open()
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", match.Name, err)
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		return nil, "", fmt.Errorf("read %s: %w", match.Name, err)
	}
	return buf.Bytes(), match.Name, nil
}

// parseConstituentRows extracts the constituent table from a sheet. Layout
// (verified on the April 2026 evaluation workbooks): a header row whose cell
// says "Kode"; data rows follow with, relative to that column, +1 free-float
// ratio, +3 index shares (post-evaluation), +4 change tag, +6 capped weight
// (post-evaluation). Parsing stops at the "Konstituen yang keluar" section so
// removed members are not counted. Separated from fetching for testability.
func parseConstituentRows(rows []xlsx.Row) *IndexConstituents {
	out := &IndexConstituents{}
	kodeCol := -1
	for _, row := range rows {
		for col, v := range row {
			val := strings.TrimSpace(v)
			if strings.HasPrefix(val, "Periode Efektif Konstituen") {
				out.EffectivePeriod = effectiveValue(row, col)
			}
			if kodeCol < 0 && val == "Kode" {
				kodeCol = xlsx.ColIndex(col)
			}
		}
		if kodeCol < 0 {
			continue
		}

		if rowContains(row, "Konstituen yang keluar") {
			break
		}
		code := strings.TrimSpace(row[xlsx.ColName(kodeCol)])
		if !tickerRe.MatchString(code) {
			continue
		}
		member := IndexConstituent{
			Code:           code,
			FreeFloatRatio: cellFloat(row, kodeCol+1),
			IndexShares:    cellFloat(row, kodeCol+3),
			Change:         strings.TrimSpace(row[xlsx.ColName(kodeCol+4)]),
			WeightPercent:  cellFloat(row, kodeCol+6) * 100,
		}
		out.Constituents = append(out.Constituents, member)
	}
	out.Total = len(out.Constituents)
	return out
}

// effectiveValue returns the nearest non-empty cell to the right of `col` in
// the row — the ":  04 Mei 2026 s.d. 31 Juli 2026" companion of the period
// label, whose exact column varies between workbooks.
func effectiveValue(row xlsx.Row, col string) string {
	start := xlsx.ColIndex(col)
	best := ""
	bestIdx := -1
	for c, v := range row {
		i := xlsx.ColIndex(c)
		if i <= start {
			continue
		}
		if v = strings.TrimSpace(v); v == "" {
			continue
		}
		if bestIdx < 0 || i < bestIdx {
			best, bestIdx = v, i
		}
	}
	return strings.TrimSpace(strings.TrimPrefix(best, ":"))
}

func rowContains(row xlsx.Row, substr string) bool {
	for _, v := range row {
		if strings.Contains(v, substr) {
			return true
		}
	}
	return false
}

func cellFloat(row xlsx.Row, colIdx int) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(row[xlsx.ColName(colIdx)]), 64)
	if err != nil {
		return 0
	}
	return f
}

// alnumUpper uppercases and strips everything but letters and digits, so
// "BISNIS-27" matches "Bisnis-27" and "IDXESGL" matches "IDX ESGL".
func alnumUpper(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
