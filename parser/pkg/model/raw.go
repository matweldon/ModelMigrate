// Package model defines the data structures for the intermediate representation.
package model

import "fmt"

// Layer 1: Raw Extraction
// These structures represent the deterministic extraction from xlsx files.

// CellRef represents a cell reference.
type CellRef struct {
	Sheet string `json:"sheet"`
	Row   int    `json:"row"` // 0-indexed
	Col   int    `json:"col"` // 0-indexed
}

// FormulaRef represents a cell reference within a formula, including absolute/relative markers.
// This is used for formula parsing and congruence checking.
type FormulaRef struct {
	Sheet       string `json:"sheet"`
	Row         int    `json:"row"`          // 0-indexed
	Col         int    `json:"col"`          // 0-indexed
	RowAbsolute bool   `json:"row_absolute"` // true if row has $ (e.g., A$1 or $A$1)
	ColAbsolute bool   `json:"col_absolute"` // true if col has $ (e.g., $A1 or $A$1)
}

// ToCellRef converts a FormulaRef to a CellRef (drops absolute info).
func (fr FormulaRef) ToCellRef() CellRef {
	return CellRef{Sheet: fr.Sheet, Row: fr.Row, Col: fr.Col}
}

// FormulaRangeRef represents a range reference within a formula, including absolute/relative markers.
type FormulaRangeRef struct {
	Sheet           string `json:"sheet"`
	TopLeft         [2]int `json:"top_left"`          // [row, col]
	BottomRight     [2]int `json:"bottom_right"`      // [row, col]
	TopRowAbsolute  bool   `json:"top_row_absolute"`  // $ on start row
	TopColAbsolute  bool   `json:"top_col_absolute"`  // $ on start col
	BotRowAbsolute  bool   `json:"bot_row_absolute"`  // $ on end row
	BotColAbsolute  bool   `json:"bot_col_absolute"`  // $ on end col
}

// ToRangeRef converts a FormulaRangeRef to a RangeRef (drops absolute info).
func (frr FormulaRangeRef) ToRangeRef() RangeRef {
	return RangeRef{Sheet: frr.Sheet, TopLeft: frr.TopLeft, BottomRight: frr.BottomRight}
}

// RangeRef represents a range of cells.
type RangeRef struct {
	Sheet       string `json:"sheet"`
	TopLeft     [2]int `json:"top_left"`     // [row, col]
	BottomRight [2]int `json:"bottom_right"` // [row, col]
}

// Shape returns the dimensions of the range.
func (r RangeRef) Shape() (rows, cols int) {
	return r.BottomRight[0] - r.TopLeft[0] + 1, r.BottomRight[1] - r.TopLeft[1] + 1
}

// CellType represents the type of a cell value.
type CellType string

const (
	CellTypeNumber  CellType = "number"
	CellTypeString  CellType = "string"
	CellTypeBool    CellType = "bool"
	CellTypeError   CellType = "error"
	CellTypeDate    CellType = "date"
	CellTypeEmpty   CellType = "empty"
	CellTypeFormula CellType = "formula" // Has formula, value is computed
)

// RawCell represents a single cell as extracted from xlsx.
type RawCell struct {
	Ref          CellRef  `json:"ref"`
	Value        any      `json:"value"`                   // Evaluated value (number, string, bool, or nil)
	Formula      string   `json:"formula,omitempty"`       // Raw formula string if present
	Type         CellType `json:"type"`                    // Value type
	NumberFormat string   `json:"number_format,omitempty"` // For detecting dates, percentages, currency
}

// IsDate returns true if the cell's number format indicates a date.
func (c RawCell) IsDate() bool {
	if c.NumberFormat == "" {
		return false
	}
	// Common date format indicators
	dateIndicators := []string{"yy", "mm", "dd", "yyyy", "date"}
	lower := c.NumberFormat
	for _, ind := range dateIndicators {
		if containsIgnoreCase(lower, ind) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// RawSheet represents a worksheet as extracted from xlsx.
type RawSheet struct {
	Name          string              `json:"name"`
	Cells         map[string]*RawCell `json:"cells"`          // Key is "row,col" for efficient lookup
	MergedRegions []RangeRef          `json:"merged_regions"` // Merged cell ranges
	MaxRow        int                 `json:"max_row"`
	MaxCol        int                 `json:"max_col"`
}

// GetCell returns the cell at the given position, or nil if not found.
func (s *RawSheet) GetCell(row, col int) *RawCell {
	key := cellKey(row, col)
	return s.Cells[key]
}

// SetCell sets a cell at the given position.
func (s *RawSheet) SetCell(cell *RawCell) {
	key := cellKey(cell.Ref.Row, cell.Ref.Col)
	s.Cells[key] = cell
	if cell.Ref.Row > s.MaxRow {
		s.MaxRow = cell.Ref.Row
	}
	if cell.Ref.Col > s.MaxCol {
		s.MaxCol = cell.Ref.Col
	}
}

func cellKey(row, col int) string {
	// String key for map lookup using fmt
	return fmt.Sprintf("%d,%d", row, col)
}

// NamedRange represents a named range in the workbook.
type NamedRange struct {
	Name    string   `json:"name"`
	RefersTo string  `json:"refers_to"` // Raw formula/reference string
	Scope   string   `json:"scope"`     // Sheet name or empty for global
	Range   *RangeRef `json:"range,omitempty"` // Parsed range if it's a simple range reference
}

// RawWorkbook represents the complete Layer 1 extraction.
type RawWorkbook struct {
	Sheets       map[string]*RawSheet `json:"sheets"`
	NamedRanges  map[string]*NamedRange `json:"named_ranges"`
	SheetOrder   []string             `json:"sheet_order"` // Preserve sheet ordering
	ExternalLinks []string            `json:"external_links,omitempty"`
}

// NewRawWorkbook creates an empty RawWorkbook.
func NewRawWorkbook() *RawWorkbook {
	return &RawWorkbook{
		Sheets:      make(map[string]*RawSheet),
		NamedRanges: make(map[string]*NamedRange),
		SheetOrder:  []string{},
	}
}

// NewRawSheet creates an empty RawSheet.
func NewRawSheet(name string) *RawSheet {
	return &RawSheet{
		Name:          name,
		Cells:         make(map[string]*RawCell),
		MergedRegions: []RangeRef{},
	}
}

// SheetBounds holds the min/max row and column for a sheet.
type SheetBounds struct {
	MinRow, MaxRow, MinCol, MaxCol int
}

// Bounds returns the bounding box of non-empty cells in the sheet.
func (s *RawSheet) Bounds() SheetBounds {
	if len(s.Cells) == 0 {
		return SheetBounds{0, -1, 0, -1}
	}

	minRow, maxRow := s.MaxRow, 0
	minCol, maxCol := s.MaxCol, 0

	for _, cell := range s.Cells {
		if cell.Ref.Row < minRow {
			minRow = cell.Ref.Row
		}
		if cell.Ref.Row > maxRow {
			maxRow = cell.Ref.Row
		}
		if cell.Ref.Col < minCol {
			minCol = cell.Ref.Col
		}
		if cell.Ref.Col > maxCol {
			maxCol = cell.Ref.Col
		}
	}

	return SheetBounds{minRow, maxRow, minCol, maxCol}
}
