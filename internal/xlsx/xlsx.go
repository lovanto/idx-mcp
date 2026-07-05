// Package xlsx is a minimal read-only .xlsx sheet extractor: it resolves
// shared strings and returns the first worksheet as sparse rows. It exists so
// idx-mcp can read IDX's index-constituent workbooks without pulling in a
// full spreadsheet dependency; it supports only what those files use
// (sharedStrings, inline strings, and plain values).
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Row is one worksheet row: column letter (e.g. "C") to the cell's resolved
// string value. Empty cells are absent.
type Row map[string]string

// Rows extracts the first worksheet of an .xlsx workbook, in sheet order.
func Rows(data []byte) ([]Row, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}

	var shared []string
	if f := findFile(zr, "xl/sharedStrings.xml"); f != nil {
		if shared, err = parseSharedStrings(f); err != nil {
			return nil, err
		}
	}

	sheet := findFile(zr, "xl/worksheets/sheet1.xml")
	if sheet == nil {
		return nil, fmt.Errorf("xlsx has no xl/worksheets/sheet1.xml")
	}
	return parseSheet(sheet, shared)
}

func findFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, name) {
			return f
		}
	}
	return nil
}

// parseSharedStrings flattens each <si> item (which may hold several rich-text
// <t> runs) into one string.
func parseSharedStrings(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open sharedStrings: %w", err)
	}
	defer rc.Close()

	var out []string
	var cur strings.Builder
	inT := false
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse sharedStrings: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				cur.Reset()
			case "t":
				inT = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "si":
				out = append(out, cur.String())
			case "t":
				inT = false
			}
		case xml.CharData:
			if inT {
				cur.Write(t)
			}
		}
	}
	return out, nil
}

// parseSheet streams sheetData rows, resolving cell values against the shared
// string table.
func parseSheet(f *zip.File, shared []string) ([]Row, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open sheet: %w", err)
	}
	defer rc.Close()

	type cell struct {
		Ref    string `xml:"r,attr"`
		Type   string `xml:"t,attr"`
		Value  string `xml:"v"`
		Inline string `xml:"is>t"`
	}
	type row struct {
		Cells []cell `xml:"c"`
	}

	var rows []Row
	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse sheet: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}
		var r row
		if err := dec.DecodeElement(&r, &start); err != nil {
			return nil, fmt.Errorf("parse row: %w", err)
		}
		out := Row{}
		for _, c := range r.Cells {
			v := c.Value
			switch c.Type {
			case "s":
				idx := -1
				fmt.Sscanf(c.Value, "%d", &idx)
				if idx >= 0 && idx < len(shared) {
					v = shared[idx]
				}
			case "inlineStr":
				v = c.Inline
			}
			if v == "" {
				continue
			}
			out[colLetter(c.Ref)] = v
		}
		rows = append(rows, out)
	}
	return rows, nil
}

// colLetter strips the row digits off a cell reference ("C10" -> "C").
func colLetter(ref string) string {
	for i, r := range ref {
		if r >= '0' && r <= '9' {
			return ref[:i]
		}
	}
	return ref
}

// ColIndex converts a column letter to a 0-based index ("A" -> 0, "AA" -> 26).
func ColIndex(col string) int {
	n := 0
	for _, r := range col {
		n = n*26 + int(r-'A'+1)
	}
	return n - 1
}

// ColName converts a 0-based index back to a column letter.
func ColName(idx int) string {
	name := ""
	for idx >= 0 {
		name = string(rune('A'+idx%26)) + name
		idx = idx/26 - 1
	}
	return name
}
