// Package xlsx provides xlsx file parsing functionality.
package xlsx

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/matweldon/modelmigrate/parser/pkg/model"
)

// Reader handles reading xlsx files.
type Reader struct {
	zipReader     *zip.ReadCloser
	sharedStrings []string
	styles        *styleSheet
	workbook      *workbookXML
}

// Open opens an xlsx file for reading.
func Open(path string) (*Reader, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx file: %w", err)
	}

	r := &Reader{zipReader: zr}

	// Load shared strings
	if err := r.loadSharedStrings(); err != nil {
		zr.Close()
		return nil, fmt.Errorf("failed to load shared strings: %w", err)
	}

	// Load styles for number format detection
	if err := r.loadStyles(); err != nil {
		zr.Close()
		return nil, fmt.Errorf("failed to load styles: %w", err)
	}

	// Load workbook metadata
	if err := r.loadWorkbook(); err != nil {
		zr.Close()
		return nil, fmt.Errorf("failed to load workbook: %w", err)
	}

	return r, nil
}

// Close closes the reader.
func (r *Reader) Close() error {
	return r.zipReader.Close()
}

// Parse reads the entire workbook into a RawWorkbook structure.
func (r *Reader) Parse() (*model.RawWorkbook, error) {
	wb := model.NewRawWorkbook()

	// Get sheet names and order from workbook.xml
	for _, sheet := range r.workbook.Sheets.Sheet {
		wb.SheetOrder = append(wb.SheetOrder, sheet.Name)
	}

	// Parse named ranges
	for _, name := range r.workbook.DefinedNames.Names {
		nr := &model.NamedRange{
			Name:     name.Name,
			RefersTo: name.Value,
		}
		if name.LocalSheetID != "" {
			// Convert sheet ID to name
			id, _ := strconv.Atoi(name.LocalSheetID)
			if id < len(wb.SheetOrder) {
				nr.Scope = wb.SheetOrder[id]
			}
		}
		wb.NamedRanges[name.Name] = nr
	}

	// Parse each sheet
	for i, sheetInfo := range r.workbook.Sheets.Sheet {
		sheetPath := fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
		sheet, err := r.parseSheet(sheetInfo.Name, sheetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse sheet %s: %w", sheetInfo.Name, err)
		}
		wb.Sheets[sheetInfo.Name] = sheet
	}

	return wb, nil
}

func (r *Reader) parseSheet(name, path string) (*model.RawSheet, error) {
	sheet := model.NewRawSheet(name)

	f, err := r.findFile(path)
	if err != nil {
		return nil, err
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	decoder := xml.NewDecoder(rc)
	var inRow bool
	var currentRow int

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch se := token.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "row":
				inRow = true
				for _, attr := range se.Attr {
					if attr.Name.Local == "r" {
						currentRow, _ = strconv.Atoi(attr.Value)
					}
				}
			case "c":
				if inRow {
					cell, err := r.parseCell(decoder, se, name, currentRow)
					if err != nil {
						return nil, err
					}
					if cell != nil {
						sheet.SetCell(cell)
					}
				}
			case "mergeCell":
				// Parse merged regions
				for _, attr := range se.Attr {
					if attr.Name.Local == "ref" {
						if rng := parseRangeRef(name, attr.Value); rng != nil {
							sheet.MergedRegions = append(sheet.MergedRegions, *rng)
						}
					}
				}
			}
		case xml.EndElement:
			if se.Name.Local == "row" {
				inRow = false
			}
		}
	}

	return sheet, nil
}

func (r *Reader) parseCell(decoder *xml.Decoder, start xml.StartElement, sheetName string, rowNum int) (*model.RawCell, error) {
	var cellRef string
	var cellType string
	var styleIndex int

	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "r":
			cellRef = attr.Value
		case "t":
			cellType = attr.Value
		case "s":
			styleIndex, _ = strconv.Atoi(attr.Value)
		}
	}

	row, col := parseCellRef(cellRef)
	cell := &model.RawCell{
		Ref: model.CellRef{
			Sheet: sheetName,
			Row:   row,
			Col:   col,
		},
		Type: model.CellTypeEmpty,
	}

	// Get number format from style
	if styleIndex > 0 && r.styles != nil && styleIndex < len(r.styles.CellXfs.Xf) {
		numFmtID := r.styles.CellXfs.Xf[styleIndex].NumFmtID
		cell.NumberFormat = r.getNumberFormat(numFmtID)
	}

	// Parse cell contents
	var value string
	var formula string

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		switch se := token.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "v":
				// Value element
				var v string
				if err := decoder.DecodeElement(&v, &se); err != nil {
					return nil, err
				}
				value = v
			case "f":
				// Formula element
				var f string
				if err := decoder.DecodeElement(&f, &se); err != nil {
					return nil, err
				}
				formula = f
			}
		case xml.EndElement:
			if se.Name.Local == "c" {
				// End of cell, process the values
				cell.Formula = formula

				switch cellType {
				case "s":
					// Shared string
					idx, _ := strconv.Atoi(value)
					if idx < len(r.sharedStrings) {
						cell.Value = r.sharedStrings[idx]
						cell.Type = model.CellTypeString
					}
				case "b":
					// Boolean
					cell.Value = value == "1"
					cell.Type = model.CellTypeBool
				case "e":
					// Error
					cell.Value = value
					cell.Type = model.CellTypeError
				case "str":
					// Inline string
					cell.Value = value
					cell.Type = model.CellTypeString
				default:
					// Number (or date)
					if value != "" {
						if f, err := strconv.ParseFloat(value, 64); err == nil {
							cell.Value = f
							if cell.IsDate() {
								cell.Type = model.CellTypeDate
							} else {
								cell.Type = model.CellTypeNumber
							}
						} else {
							cell.Value = value
							cell.Type = model.CellTypeString
						}
					}
				}

				if formula != "" {
					cell.Type = model.CellTypeFormula
				}

				if cell.Value == nil && formula == "" {
					return nil, nil // Empty cell, don't store
				}

				return cell, nil
			}
		}
	}
}

func (r *Reader) loadSharedStrings() error {
	f, err := r.findFile("xl/sharedStrings.xml")
	if err != nil {
		// Shared strings file may not exist
		return nil
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	var sst sharedStringsXML
	if err := xml.NewDecoder(rc).Decode(&sst); err != nil {
		return err
	}

	r.sharedStrings = make([]string, len(sst.SI))
	for i, si := range sst.SI {
		if si.T != "" {
			r.sharedStrings[i] = si.T
		} else if len(si.R) > 0 {
			// Rich text - concatenate all text runs
			var sb strings.Builder
			for _, run := range si.R {
				sb.WriteString(run.T)
			}
			r.sharedStrings[i] = sb.String()
		}
	}

	return nil
}

func (r *Reader) loadStyles() error {
	f, err := r.findFile("xl/styles.xml")
	if err != nil {
		return nil // Styles file may not exist
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	var styles styleSheet
	if err := xml.NewDecoder(rc).Decode(&styles); err != nil {
		return err
	}
	r.styles = &styles

	return nil
}

func (r *Reader) loadWorkbook() error {
	f, err := r.findFile("xl/workbook.xml")
	if err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	var wb workbookXML
	if err := xml.NewDecoder(rc).Decode(&wb); err != nil {
		return err
	}
	r.workbook = &wb

	return nil
}

func (r *Reader) getNumberFormat(numFmtID int) string {
	// Built-in formats
	builtIn := map[int]string{
		0:  "General",
		1:  "0",
		2:  "0.00",
		9:  "0%",
		10: "0.00%",
		14: "mm-dd-yy",
		15: "d-mmm-yy",
		16: "d-mmm",
		17: "mmm-yy",
		18: "h:mm AM/PM",
		19: "h:mm:ss AM/PM",
		20: "h:mm",
		21: "h:mm:ss",
		22: "m/d/yy h:mm",
	}

	if fmt, ok := builtIn[numFmtID]; ok {
		return fmt
	}

	// Custom formats
	if r.styles != nil {
		for _, nf := range r.styles.NumFmts.NumFmt {
			if nf.NumFmtID == numFmtID {
				return nf.FormatCode
			}
		}
	}

	return ""
}

func (r *Reader) findFile(path string) (*zip.File, error) {
	for _, f := range r.zipReader.File {
		if f.Name == path {
			return f, nil
		}
	}
	return nil, fmt.Errorf("file not found in archive: %s", path)
}

// parseCellRef parses an A1-style cell reference into row and column (0-indexed).
func parseCellRef(ref string) (row, col int) {
	re := regexp.MustCompile(`([A-Z]+)(\d+)`)
	matches := re.FindStringSubmatch(ref)
	if len(matches) != 3 {
		return 0, 0
	}

	// Parse column letters
	colStr := matches[1]
	for _, c := range colStr {
		col = col*26 + int(c-'A'+1)
	}
	col-- // 0-indexed

	// Parse row number
	row, _ = strconv.Atoi(matches[2])
	row-- // 0-indexed

	return row, col
}

// parseRangeRef parses an A1:B2 style range reference.
func parseRangeRef(sheet, ref string) *model.RangeRef {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return nil
	}

	row1, col1 := parseCellRef(parts[0])
	row2, col2 := parseCellRef(parts[1])

	return &model.RangeRef{
		Sheet:       sheet,
		TopLeft:     [2]int{row1, col1},
		BottomRight: [2]int{row2, col2},
	}
}

// XML structures for parsing xlsx

type sharedStringsXML struct {
	XMLName xml.Name `xml:"sst"`
	SI      []struct {
		T string `xml:"t"`
		R []struct {
			T string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

type styleSheet struct {
	XMLName xml.Name `xml:"styleSheet"`
	NumFmts struct {
		NumFmt []struct {
			NumFmtID   int    `xml:"numFmtId,attr"`
			FormatCode string `xml:"formatCode,attr"`
		} `xml:"numFmt"`
	} `xml:"numFmts"`
	CellXfs struct {
		Xf []struct {
			NumFmtID int `xml:"numFmtId,attr"`
		} `xml:"xf"`
	} `xml:"cellXfs"`
}

type workbookXML struct {
	XMLName xml.Name `xml:"workbook"`
	Sheets  struct {
		Sheet []struct {
			Name    string `xml:"name,attr"`
			SheetID string `xml:"sheetId,attr"`
			RID     string `xml:"id,attr"`
		} `xml:"sheet"`
	} `xml:"sheets"`
	DefinedNames struct {
		Names []struct {
			Name         string `xml:"name,attr"`
			LocalSheetID string `xml:"localSheetId,attr"`
			Value        string `xml:",chardata"`
		} `xml:"definedName"`
	} `xml:"definedNames"`
}
