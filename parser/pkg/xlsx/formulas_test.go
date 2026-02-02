package xlsx

import (
	"testing"

	"github.com/matweldon/modelmigrate/parser/pkg/model"
)

func TestParseFormula_SimpleReference(t *testing.T) {
	tests := []struct {
		name          string
		formula       string
		currentSheet  string
		wantRefs      []model.CellRef
		wantFunctions []string
	}{
		{
			name:         "single cell reference",
			formula:      "A1",
			currentSheet: "Sheet1",
			wantRefs:     []model.CellRef{{Sheet: "Sheet1", Row: 0, Col: 0}},
		},
		{
			name:         "absolute column reference",
			formula:      "$A1",
			currentSheet: "Sheet1",
			wantRefs:     []model.CellRef{{Sheet: "Sheet1", Row: 0, Col: 0}},
		},
		{
			name:         "absolute row reference",
			formula:      "A$1",
			currentSheet: "Sheet1",
			wantRefs:     []model.CellRef{{Sheet: "Sheet1", Row: 0, Col: 0}},
		},
		{
			name:         "fully absolute reference",
			formula:      "$A$1",
			currentSheet: "Sheet1",
			wantRefs:     []model.CellRef{{Sheet: "Sheet1", Row: 0, Col: 0}},
		},
		{
			name:         "multi-letter column",
			formula:      "AA100",
			currentSheet: "Sheet1",
			wantRefs:     []model.CellRef{{Sheet: "Sheet1", Row: 99, Col: 26}},
		},
		{
			name:         "cross-sheet reference",
			formula:      "Sheet2!B5",
			currentSheet: "Sheet1",
			wantRefs:     []model.CellRef{{Sheet: "Sheet2", Row: 4, Col: 1}},
		},
		{
			name:         "quoted sheet name with space",
			formula:      "'My Sheet'!C10",
			currentSheet: "Sheet1",
			wantRefs:     []model.CellRef{{Sheet: "My Sheet", Row: 9, Col: 2}},
		},
		{
			name:         "multiple references",
			formula:      "A1+B2*C3",
			currentSheet: "Sheet1",
			wantRefs: []model.CellRef{
				{Sheet: "Sheet1", Row: 0, Col: 0},
				{Sheet: "Sheet1", Row: 1, Col: 1},
				{Sheet: "Sheet1", Row: 2, Col: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseFormula(tt.formula, tt.currentSheet)
			if parsed == nil {
				t.Fatal("ParseFormula returned nil")
			}

			if len(parsed.References) != len(tt.wantRefs) {
				t.Errorf("got %d references, want %d", len(parsed.References), len(tt.wantRefs))
			}

			for i, want := range tt.wantRefs {
				if i >= len(parsed.References) {
					break
				}
				got := parsed.References[i]
				if got.Sheet != want.Sheet || got.Row != want.Row || got.Col != want.Col {
					t.Errorf("ref[%d] = %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

func TestParseFormula_RangeReferences(t *testing.T) {
	tests := []struct {
		name         string
		formula      string
		currentSheet string
		wantRanges   []model.RangeRef
	}{
		{
			name:         "simple range",
			formula:      "SUM(A1:B10)",
			currentSheet: "Sheet1",
			wantRanges: []model.RangeRef{
				{Sheet: "Sheet1", TopLeft: [2]int{0, 0}, BottomRight: [2]int{9, 1}},
			},
		},
		{
			name:         "cross-sheet range",
			formula:      "SUM(Data!A1:C50)",
			currentSheet: "Sheet1",
			wantRanges: []model.RangeRef{
				{Sheet: "Data", TopLeft: [2]int{0, 0}, BottomRight: [2]int{49, 2}},
			},
		},
		{
			name:         "quoted sheet range",
			formula:      "SUM('Sales Data'!A1:Z100)",
			currentSheet: "Sheet1",
			wantRanges: []model.RangeRef{
				{Sheet: "Sales Data", TopLeft: [2]int{0, 0}, BottomRight: [2]int{99, 25}},
			},
		},
		{
			name:         "absolute range",
			formula:      "SUM($A$1:$B$10)",
			currentSheet: "Sheet1",
			wantRanges: []model.RangeRef{
				{Sheet: "Sheet1", TopLeft: [2]int{0, 0}, BottomRight: [2]int{9, 1}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseFormula(tt.formula, tt.currentSheet)
			if parsed == nil {
				t.Fatal("ParseFormula returned nil")
			}

			if len(parsed.RangeReferences) != len(tt.wantRanges) {
				t.Errorf("got %d range references, want %d", len(parsed.RangeReferences), len(tt.wantRanges))
			}

			for i, want := range tt.wantRanges {
				if i >= len(parsed.RangeReferences) {
					break
				}
				got := parsed.RangeReferences[i]
				if got.Sheet != want.Sheet {
					t.Errorf("range[%d].Sheet = %q, want %q", i, got.Sheet, want.Sheet)
				}
				if got.TopLeft != want.TopLeft {
					t.Errorf("range[%d].TopLeft = %v, want %v", i, got.TopLeft, want.TopLeft)
				}
				if got.BottomRight != want.BottomRight {
					t.Errorf("range[%d].BottomRight = %v, want %v", i, got.BottomRight, want.BottomRight)
				}
			}
		})
	}
}

func TestParseFormula_Functions(t *testing.T) {
	tests := []struct {
		name          string
		formula       string
		wantFunctions []string
	}{
		{
			name:          "SUM function",
			formula:       "SUM(A1:A10)",
			wantFunctions: []string{"SUM"},
		},
		{
			name:          "nested functions",
			formula:       "IF(SUM(A1:A10)>100,AVERAGE(B1:B10),0)",
			wantFunctions: []string{"IF", "SUM", "AVERAGE"},
		},
		{
			name:          "VLOOKUP",
			formula:       "VLOOKUP(A1,Data!A:D,2,FALSE)",
			wantFunctions: []string{"VLOOKUP"},
		},
		{
			name:          "arithmetic only",
			formula:       "A1+B1*C1-D1/E1",
			wantFunctions: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseFormula(tt.formula, "Sheet1")
			if parsed == nil {
				t.Fatal("ParseFormula returned nil")
			}

			if len(parsed.Functions) != len(tt.wantFunctions) {
				t.Errorf("got functions %v, want %v", parsed.Functions, tt.wantFunctions)
				return
			}

			for i, want := range tt.wantFunctions {
				if parsed.Functions[i] != want {
					t.Errorf("function[%d] = %q, want %q", i, parsed.Functions[i], want)
				}
			}
		})
	}
}

func TestParseFormula_Empty(t *testing.T) {
	parsed := ParseFormula("", "Sheet1")
	if parsed != nil {
		t.Errorf("expected nil for empty formula, got %+v", parsed)
	}
}

func TestColLettersToIndex(t *testing.T) {
	tests := []struct {
		letters string
		want    int
	}{
		{"A", 0},
		{"B", 1},
		{"Z", 25},
		{"AA", 26},
		{"AB", 27},
		{"AZ", 51},
		{"BA", 52},
		{"ZZ", 701},
		{"AAA", 702},
	}

	for _, tt := range tests {
		t.Run(tt.letters, func(t *testing.T) {
			got := colLettersToIndex(tt.letters)
			if got != tt.want {
				t.Errorf("colLettersToIndex(%q) = %d, want %d", tt.letters, got, tt.want)
			}
		})
	}
}

func TestIsCellRefLike(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"A1", true},
		{"B10", true},
		{"AA100", true},
		{"XYZ123", true},
		{"ABCD1", false}, // more than 3 letters
		{"SUM", false},   // no digits
		{"Revenue", false},
		{"Q", false}, // no digits
		{"123", false}, // no letters
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isCellRefLike(tt.s)
			if got != tt.want {
				t.Errorf("isCellRefLike(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}
