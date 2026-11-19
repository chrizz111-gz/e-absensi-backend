package utils

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ErrInvalidXLSX dikembalikan saat file bukan arsip .xlsx yang valid.
var ErrInvalidXLSX = errors.New("file xlsx tidak valid")

// BuildXLSX membuat file Excel (.xlsx) valid dari matriks baris string.
// Baris pertama otomatis diberi style header tebal (bold) dan lebar kolom
// disesuaikan agar langsung rapi saat dibuka di Microsoft Excel.
func BuildXLSX(sheetName string, rows [][]string) ([]byte, error) {
	if strings.TrimSpace(sheetName) == "" {
		sheetName = "Sheet1"
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := []struct {
		name    string
		content string
	}{
		{"[Content_Types].xml", xlsxContentTypesXML},
		{"_rels/.rels", xlsxRootRelsXML},
		{"xl/workbook.xml", xlsxWorkbookXML(sheetName)},
		{"xl/_rels/workbook.xml.rels", xlsxWorkbookRelsXML},
		{"xl/styles.xml", xlsxStylesXML},
		{"xl/worksheets/sheet1.xml", xlsxSheetXML(rows)},
	}

	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := io.WriteString(w, f.content); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ReadXLSX membaca sheet pertama dari arsip .xlsx dan mengembalikan matriks
// nilai sel per baris. Mendukung sharedStrings maupun inlineStr.
func ReadXLSX(data []byte) ([][]string, error) {
	if len(data) == 0 {
		return nil, ErrInvalidXLSX
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, ErrInvalidXLSX
	}

	var sharedStrings []string
	var sheetBytes []byte

	for _, f := range zr.File {
		switch {
		case f.Name == "xl/sharedStrings.xml":
			raw, err := readZipFile(f)
			if err != nil {
				return nil, ErrInvalidXLSX
			}
			sharedStrings, err = parseSharedStrings(raw)
			if err != nil {
				return nil, ErrInvalidXLSX
			}
		case f.Name == "xl/worksheets/sheet1.xml" || (sheetBytes == nil && strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml")):
			raw, err := readZipFile(f)
			if err != nil {
				return nil, ErrInvalidXLSX
			}
			sheetBytes = raw
		}
	}

	if sheetBytes == nil {
		return nil, ErrInvalidXLSX
	}

	return parseWorksheet(sheetBytes, sharedStrings)
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 20*1024*1024))
}

type xmlSST struct {
	Items []xmlSI `xml:"si"`
}

type xmlSI struct {
	T    string   `xml:"t"`
	Runs []xmlRun `xml:"r"`
}

type xmlRun struct {
	T string `xml:"t"`
}

func parseSharedStrings(data []byte) ([]string, error) {
	var sst xmlSST
	if err := xml.Unmarshal(data, &sst); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sst.Items))
	for _, item := range sst.Items {
		if len(item.Runs) > 0 {
			var b strings.Builder
			for _, r := range item.Runs {
				b.WriteString(r.T)
			}
			out = append(out, b.String())
		} else {
			out = append(out, item.T)
		}
	}
	return out, nil
}

type xmlWorksheet struct {
	SheetData xmlSheetData `xml:"sheetData"`
}

type xmlSheetData struct {
	Rows []xmlRow `xml:"row"`
}

type xmlRow struct {
	R     int       `xml:"r,attr"`
	Cells []xmlCell `xml:"c"`
}

type xmlCell struct {
	Ref       string `xml:"r,attr"`
	Type      string `xml:"t,attr"`
	Value     string `xml:"v"`
	InlineStr xmlSI  `xml:"is"`
}

func parseWorksheet(data []byte, sharedStrings []string) ([][]string, error) {
	var ws xmlWorksheet
	if err := xml.Unmarshal(data, &ws); err != nil {
		return nil, ErrInvalidXLSX
	}

	rows := make([][]string, 0, len(ws.SheetData.Rows))
	for _, r := range ws.SheetData.Rows {
		maxCol := -1
		colValues := make(map[int]string, len(r.Cells))
		for idx, c := range r.Cells {
			colIdx := columnIndexFromCellRef(c.Ref)
			if colIdx < 0 {
				colIdx = idx
			}
			if colIdx > maxCol {
				maxCol = colIdx
			}
			colValues[colIdx] = resolveCellValue(c, sharedStrings)
		}
		if maxCol < 0 {
			rows = append(rows, []string{})
			continue
		}
		rowSlice := make([]string, maxCol+1)
		for colIdx, val := range colValues {
			if colIdx >= 0 && colIdx < len(rowSlice) {
				rowSlice[colIdx] = val
			}
		}
		rows = append(rows, rowSlice)
	}
	return rows, nil
}

func resolveCellValue(c xmlCell, sharedStrings []string) string {
	switch c.Type {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(c.Value))
		if err == nil && idx >= 0 && idx < len(sharedStrings) {
			return sharedStrings[idx]
		}
		return ""
	case "inlineStr":
		if len(c.InlineStr.Runs) > 0 {
			var b strings.Builder
			for _, r := range c.InlineStr.Runs {
				b.WriteString(r.T)
			}
			return b.String()
		}
		return c.InlineStr.T
	default:
		return c.Value
	}
}

// columnIndexFromCellRef menerjemahkan referensi sel seperti "A1" -> 0, "F2" -> 5.
func columnIndexFromCellRef(ref string) int {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return -1
	}
	col := 0
	foundLetter := false
	for i := 0; i < len(ref); i++ {
		ch := ref[i]
		if ch >= 'A' && ch <= 'Z' {
			foundLetter = true
			col = col*26 + int(ch-'A'+1)
		} else if ch >= 'a' && ch <= 'z' {
			foundLetter = true
			col = col*26 + int(ch-'a'+1)
		} else {
			break
		}
	}
	if !foundLetter {
		return -1
	}
	return col - 1
}

// columnNameFromIndex menerjemahkan index 0 -> "A", 5 -> "F", 26 -> "AA".
func columnNameFromIndex(idx int) string {
	var b []byte
	n := idx + 1
	for n > 0 {
		rem := (n - 1) % 26
		b = append([]byte{byte('A' + rem)}, b...)
		n = (n - 1) / 26
	}
	return string(b)
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func xlsxWorkbookXML(sheetName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="%s" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`, xmlEscape(sheetName))
}

func xlsxSheetXML(rows [][]string) string {
	maxCols := 6
	for _, r := range rows {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` + "\n")
	b.WriteString(fmt.Sprintf(`  <cols><col min="1" max="%d" width="25" customWidth="1"/></cols>`+"\n", maxCols))
	b.WriteString(`  <sheetData>` + "\n")

	for rIdx, row := range rows {
		rowNum := rIdx + 1
		styleAttr := ""
		if rIdx == 0 {
			styleAttr = ` s="1"`
		}
		b.WriteString(fmt.Sprintf(`    <row r="%d">`, rowNum))
		for cIdx, val := range row {
			cellRef := fmt.Sprintf("%s%d", columnNameFromIndex(cIdx), rowNum)
			b.WriteString(fmt.Sprintf(`<c r="%s"%s t="inlineStr"><is><t>%s</t></is></c>`, cellRef, styleAttr, xmlEscape(val)))
		}
		b.WriteString(`</row>` + "\n")
	}

	b.WriteString(`  </sheetData>` + "\n")
	b.WriteString(`</worksheet>`)
	return b.String()
}

const xlsxContentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const xlsxRootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const xlsxWorkbookRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const xlsxStylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="2">
    <font><sz val="11"/><name val="Calibri"/></font>
    <font><b/><sz val="11"/><name val="Calibri"/></font>
  </fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1">
    <border><left/><right/><top/><bottom/><diagonal/></border>
  </borders>
  <cellStyleXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
  </cellStyleXfs>
  <cellXfs count="2">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/>
  </cellXfs>
</styleSheet>`
