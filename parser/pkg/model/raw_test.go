package model

import (
	"testing"
)

func TestRangeRef_Shape(t *testing.T) {
	tests := []struct {
		name     string
		ref      RangeRef
		wantRows int
		wantCols int
	}{
		{
			name:     "single cell",
			ref:      RangeRef{TopLeft: [2]int{0, 0}, BottomRight: [2]int{0, 0}},
			wantRows: 1,
			wantCols: 1,
		},
		{
			name:     "row vector",
			ref:      RangeRef{TopLeft: [2]int{5, 0}, BottomRight: [2]int{5, 9}},
			wantRows: 1,
			wantCols: 10,
		},
		{
			name:     "column vector",
			ref:      RangeRef{TopLeft: [2]int{0, 3}, BottomRight: [2]int{99, 3}},
			wantRows: 100,
			wantCols: 1,
		},
		{
			name:     "matrix",
			ref:      RangeRef{TopLeft: [2]int{5, 5}, BottomRight: [2]int{14, 9}},
			wantRows: 10,
			wantCols: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRows, gotCols := tt.ref.Shape()
			if gotRows != tt.wantRows || gotCols != tt.wantCols {
				t.Errorf("Shape() = (%d, %d), want (%d, %d)", gotRows, gotCols, tt.wantRows, tt.wantCols)
			}
		})
	}
}

func TestRawSheet_GetSetCell(t *testing.T) {
	sheet := NewRawSheet("Test")

	// Initially should be empty
	if got := sheet.GetCell(0, 0); got != nil {
		t.Errorf("GetCell(0,0) = %v, want nil", got)
	}

	// Set a cell
	cell := &RawCell{
		Ref:   CellRef{Sheet: "Test", Row: 5, Col: 3},
		Value: 42.0,
		Type:  CellTypeNumber,
	}
	sheet.SetCell(cell)

	// Should now be retrievable
	got := sheet.GetCell(5, 3)
	if got == nil {
		t.Fatal("GetCell(5,3) = nil after SetCell")
	}
	if got.Value != 42.0 {
		t.Errorf("GetCell(5,3).Value = %v, want 42.0", got.Value)
	}

	// MaxRow and MaxCol should be updated
	if sheet.MaxRow != 5 {
		t.Errorf("MaxRow = %d, want 5", sheet.MaxRow)
	}
	if sheet.MaxCol != 3 {
		t.Errorf("MaxCol = %d, want 3", sheet.MaxCol)
	}
}

func TestRawSheet_Bounds(t *testing.T) {
	sheet := NewRawSheet("Test")

	// Empty sheet
	bounds := sheet.Bounds()
	if bounds.MaxRow != -1 || bounds.MaxCol != -1 {
		t.Errorf("empty sheet bounds = %+v, expected negative max", bounds)
	}

	// Add some cells
	cells := []*RawCell{
		{Ref: CellRef{Sheet: "Test", Row: 2, Col: 3}, Value: "a"},
		{Ref: CellRef{Sheet: "Test", Row: 5, Col: 1}, Value: "b"},
		{Ref: CellRef{Sheet: "Test", Row: 4, Col: 7}, Value: "c"},
	}
	for _, c := range cells {
		sheet.SetCell(c)
	}

	bounds = sheet.Bounds()
	if bounds.MinRow != 2 {
		t.Errorf("MinRow = %d, want 2", bounds.MinRow)
	}
	if bounds.MaxRow != 5 {
		t.Errorf("MaxRow = %d, want 5", bounds.MaxRow)
	}
	if bounds.MinCol != 1 {
		t.Errorf("MinCol = %d, want 1", bounds.MinCol)
	}
	if bounds.MaxCol != 7 {
		t.Errorf("MaxCol = %d, want 7", bounds.MaxCol)
	}
}

func TestRawCell_IsDate(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   bool
	}{
		{"general", "General", false},
		{"number", "0.00", false},
		{"currency", `"$"#,##0.00`, false},
		{"date yyyy-mm-dd", "yyyy-mm-dd", true},
		{"date mm/dd/yy", "mm/dd/yy", true},
		{"date dd-mmm-yyyy", "dd-mmm-yyyy", true},
		{"time hh:mm", "hh:mm", true}, // contains "mm" which triggers date detection
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := RawCell{NumberFormat: tt.format}
			got := cell.IsDate()
			if got != tt.want {
				t.Errorf("IsDate() = %v, want %v for format %q", got, tt.want, tt.format)
			}
		})
	}
}

func TestNewRawWorkbook(t *testing.T) {
	wb := NewRawWorkbook()

	if wb.Sheets == nil {
		t.Error("Sheets map is nil")
	}
	if wb.NamedRanges == nil {
		t.Error("NamedRanges map is nil")
	}
	if wb.SheetOrder == nil {
		t.Error("SheetOrder slice is nil")
	}
	if len(wb.Sheets) != 0 {
		t.Errorf("Sheets has %d entries, want 0", len(wb.Sheets))
	}
}
