package idx

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/lovanto/idx-mcp/internal/xlsx"
)

// buildTestXLSX assembles a minimal workbook mirroring the layout of IDX's
// constituent sheets (header block, "Kode" table, trailing removed-members
// section), as captured from the April 2026 LQ45 evaluation file.
func buildTestXLSX(t *testing.T) []byte {
	t.Helper()
	shared := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="9" uniqueCount="9">
<si><t>Periode Efektif Konstituen </t></si>
<si><t>:  04 Mei 2026 s.d. 31 Juli 2026</t></si>
<si><t>Kode</t></si>
<si><t>AADI</t></si>
<si><t>Naik</t></si>
<si><r><t>BB</t></r><r><t>CA</t></r></si>
<si><t>Tetap</t></si>
<si><t>Konstituen yang keluar dari penghitungan indeks</t></si>
<si><t>NCKL</t></si>
</sst>`
	sheet := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>
<row r="4"><c r="B4" t="s"><v>0</v></c><c r="D4" t="s"><v>1</v></c></row>
<row r="9"><c r="C9" t="s"><v>2</v></c></row>
<row r="10"><c r="B10"><v>1</v></c><c r="C10" t="s"><v>3</v></c><c r="D10"><v>0.1934</v></c><c r="E10"><v>1464714340</v></c><c r="F10"><v>1505984866</v></c><c r="G10" t="s"><v>4</v></c><c r="H10"><v>0.0083907</v></c><c r="I10"><v>0.0094857</v></c></row>
<row r="11"><c r="B11"><v>2</v></c><c r="C11" t="s"><v>5</v></c><c r="D11"><v>0.4183</v></c><c r="E11"><v>1</v></c><c r="F11"><v>51043414700</v></c><c r="G11" t="s"><v>6</v></c><c r="H11"><v>0.14</v></c><c r="I11"><v>0.15</v></c></row>
<row r="60"><c r="B60" t="s"><v>7</v></c></row>
<row r="62"><c r="C62" t="s"><v>8</v></c></row>
</sheetData></worksheet>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"xl/sharedStrings.xml":     shared,
		"xl/worksheets/sheet1.xml": sheet,
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close xlsx: %v", err)
	}
	return buf.Bytes()
}

func TestParseConstituentRows(t *testing.T) {
	rows, err := xlsx.Rows(buildTestXLSX(t))
	if err != nil {
		t.Fatalf("xlsx.Rows: %v", err)
	}

	out := parseConstituentRows(rows)
	if out.EffectivePeriod != "04 Mei 2026 s.d. 31 Juli 2026" {
		t.Errorf("EffectivePeriod = %q", out.EffectivePeriod)
	}
	if out.Total != 2 {
		t.Fatalf("Total = %d, want 2 (NCKL is in the removed section and must not count)", out.Total)
	}
	first := out.Constituents[0]
	if first.Code != "AADI" || first.Change != "Naik" {
		t.Errorf("first = %+v, want AADI/Naik", first)
	}
	if first.FreeFloatRatio != 0.1934 || first.IndexShares != 1505984866 {
		t.Errorf("first numbers = %+v", first)
	}
	if first.WeightPercent < 0.948 || first.WeightPercent > 0.949 {
		t.Errorf("WeightPercent = %f, want ~0.9486", first.WeightPercent)
	}
	// BBCA's shared string is split across rich-text runs; the reader must
	// join them.
	if out.Constituents[1].Code != "BBCA" {
		t.Errorf("second code = %q, want BBCA", out.Constituents[1].Code)
	}
}

func TestIndexConstituentsEndToEnd(t *testing.T) {
	xlsxBytes := buildTestXLSX(t)
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	w, _ := zw.Create("2 Lamp Peng - LQ45 - Apr 2026 Mayor.xlsx")
	if _, err := w.Write(xlsxBytes); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	uploaderJSON := fmt.Sprintf(`{"ResultCount":1,"Results":[
	 {"StockUploaderID":6661,"Date":"2026-04-24T19:00:00","Group":"stockIndex","NoPengumuman":"No. Peng-00067/BEI.POP/04-2026","TypeIndex":"LQ45%s","Description":"4 Mei 2026 - 31 Juli 2026 (Evaluasi Mayor)","Year":"2026","AttachmentName":"peng.zip","AttachmentUrl":"\\StaticData\\Exchange\\No. Peng-00067-ID.zip"}]}`,
		"                                ")

	f := &fakeFetcher{responses: map[string]string{
		"GetStockUploader":     uploaderJSON,
		"No.%20Peng-00067-ID.": zbuf.String(),
	}}
	c := New(f, nil)

	got, err := c.IndexConstituents(context.Background(), "lq45")
	if err != nil {
		t.Fatalf("IndexConstituents: %v", err)
	}
	if got.IndexCode != "LQ45" || got.Total != 2 {
		t.Errorf("got %s/%d, want LQ45/2", got.IndexCode, got.Total)
	}
	if got.Announcement != "No. Peng-00067/BEI.POP/04-2026" {
		t.Errorf("Announcement = %q", got.Announcement)
	}
	if got.AnnouncedDate != "2026-04-24" {
		t.Errorf("AnnouncedDate = %q", got.AnnouncedDate)
	}
}

func TestIndexConstituentsUnknownIndex(t *testing.T) {
	f := &fakeFetcher{responses: map[string]string{
		"GetStockUploader": `{"ResultCount":0,"Results":[]}`,
	}}
	c := New(f, nil)
	if _, err := c.IndexConstituents(context.Background(), "NOPE99"); err == nil {
		t.Fatal("expected error for unknown index")
	}
}
