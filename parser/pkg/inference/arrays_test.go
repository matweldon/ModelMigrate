package inference

import (
	"testing"

	"github.com/matweldon/modelmigrate/parser/pkg/model"
	"github.com/matweldon/modelmigrate/parser/pkg/xlsx"
)

// Helper to create a simple workbook for testing
func makeTestWorkbook() *model.RawWorkbook {
	wb := model.NewRawWorkbook()
	wb.SheetOrder = []string{"Sheet1"}
	sheet := model.NewRawSheet("Sheet1")
	wb.Sheets["Sheet1"] = sheet
	return wb
}

func TestArrayDetectorV2_SimpleNumericColumn(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a column of numbers (A1:A5)
	for i := 0; i < 5; i++ {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: i, Col: 0},
			Value: float64(i + 1),
			Type:  model.CellTypeNumber,
		})
	}

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	if len(arrays) == 0 {
		t.Fatal("expected at least one array")
	}

	// Should detect a 5x1 column vector
	found := false
	for _, arr := range arrays {
		rows, cols := arr.RangeRef.Shape()
		if rows == 5 && cols == 1 {
			found = true
			if arr.Orientation != model.OrientationColVector {
				t.Errorf("expected col_vector orientation, got %s", arr.Orientation)
			}
		}
	}
	if !found {
		t.Error("expected to find a 5x1 array")
	}
}

func TestArrayDetectorV2_SimpleNumericRow(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a row of numbers (A1:E1)
	for i := 0; i < 5; i++ {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: i},
			Value: float64(i + 1),
			Type:  model.CellTypeNumber,
		})
	}

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	if len(arrays) == 0 {
		t.Fatal("expected at least one array")
	}

	// Should detect a 1x5 row vector
	found := false
	for _, arr := range arrays {
		rows, cols := arr.RangeRef.Shape()
		if rows == 1 && cols == 5 {
			found = true
			if arr.Orientation != model.OrientationRowVector {
				t.Errorf("expected row_vector orientation, got %s", arr.Orientation)
			}
		}
	}
	if !found {
		t.Error("expected to find a 1x5 array")
	}
}

func TestArrayDetectorV2_Matrix(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a 3x3 matrix of numbers
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			sheet.SetCell(&model.RawCell{
				Ref:   model.CellRef{Sheet: "Sheet1", Row: row, Col: col},
				Value: float64(row*3 + col + 1),
				Type:  model.CellTypeNumber,
			})
		}
	}

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	if len(arrays) == 0 {
		t.Fatal("expected at least one array")
	}

	// Should detect a 3x3 matrix
	found := false
	for _, arr := range arrays {
		rows, cols := arr.RangeRef.Shape()
		if rows == 3 && cols == 3 {
			found = true
			if arr.Orientation != model.OrientationMatrix {
				t.Errorf("expected matrix orientation, got %s", arr.Orientation)
			}
		}
	}
	if !found {
		t.Error("expected to find a 3x3 matrix")
	}
}

func TestArrayDetectorV2_WithHeaders(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Row 0: Headers (strings)
	headers := []string{"Name", "Value", "Total"}
	for i, h := range headers {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: i},
			Value: h,
			Type:  model.CellTypeString,
		})
	}

	// Rows 1-3: Data (numbers)
	for row := 1; row <= 3; row++ {
		for col := 0; col < 3; col++ {
			sheet.SetCell(&model.RawCell{
				Ref:   model.CellRef{Sheet: "Sheet1", Row: row, Col: col},
				Value: float64(row * 10 + col),
				Type:  model.CellTypeNumber,
			})
		}
	}

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	// Find the numeric array
	var numericArray *model.InferredArray
	for _, arr := range arrays {
		if arr.DType == "float64" {
			rows, cols := arr.RangeRef.Shape()
			if rows == 3 && cols == 3 {
				numericArray = arr
				break
			}
		}
	}

	if numericArray == nil {
		t.Fatal("expected to find a 3x3 numeric array")
	}

	// Should have column headers
	if len(numericArray.ColHeaders) != 3 {
		t.Errorf("expected 3 column headers, got %d", len(numericArray.ColHeaders))
	} else {
		for i, expected := range headers {
			if numericArray.ColHeaders[i] != expected {
				t.Errorf("header %d: expected %q, got %q", i, expected, numericArray.ColHeaders[i])
			}
		}
	}
}

func TestArrayDetectorV2_WithRowHeaders(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Column 0: Row headers (strings)
	rowHeaders := []string{"Q1", "Q2", "Q3"}
	for i, h := range rowHeaders {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: i, Col: 0},
			Value: h,
			Type:  model.CellTypeString,
		})
	}

	// Columns 1-3: Data (numbers)
	for row := 0; row < 3; row++ {
		for col := 1; col <= 3; col++ {
			sheet.SetCell(&model.RawCell{
				Ref:   model.CellRef{Sheet: "Sheet1", Row: row, Col: col},
				Value: float64(row*10 + col),
				Type:  model.CellTypeNumber,
			})
		}
	}

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	// Find the numeric array
	var numericArray *model.InferredArray
	for _, arr := range arrays {
		if arr.DType == "float64" {
			rows, cols := arr.RangeRef.Shape()
			if rows == 3 && cols == 3 {
				numericArray = arr
				break
			}
		}
	}

	if numericArray == nil {
		t.Fatal("expected to find a 3x3 numeric array")
	}

	// Should have row headers
	if len(numericArray.RowHeaders) != 3 {
		t.Errorf("expected 3 row headers, got %d", len(numericArray.RowHeaders))
	} else {
		for i, expected := range rowHeaders {
			if numericArray.RowHeaders[i] != expected {
				t.Errorf("row header %d: expected %q, got %q", i, expected, numericArray.RowHeaders[i])
			}
		}
	}
}

func TestArrayDetectorV2_WithFormulas(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create input values in column A
	for i := 0; i < 3; i++ {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: i, Col: 0},
			Value: float64(i + 1),
			Type:  model.CellTypeNumber,
		})
	}

	// Create formulas in column B that reference column A
	formulas := []string{"A1*2", "A2*2", "A3*2"}
	parsedFormulas := make(map[string]*xlsx.ParsedFormula)
	for i, f := range formulas {
		sheet.SetCell(&model.RawCell{
			Ref:     model.CellRef{Sheet: "Sheet1", Row: i, Col: 1},
			Value:   float64((i + 1) * 2),
			Formula: f,
			Type:    model.CellTypeFormula,
		})
		key := "Sheet1!" + string(rune('0'+i)) + ",1"
		parsedFormulas[key] = xlsx.ParseFormula(f, "Sheet1")
	}

	detector := NewArrayDetectorV2(wb, parsedFormulas)
	arrays := detector.DetectArrays()

	// Find arrays with formulas
	var formulaArray *model.InferredArray
	for _, arr := range arrays {
		if arr.HasFormulas {
			formulaArray = arr
			break
		}
	}

	if formulaArray == nil {
		t.Fatal("expected to find an array with formulas")
	}
}

func TestArrayDetectorV2_SeparateArrays(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create two separate blocks of numbers
	// Block 1: rows 0-2, cols 0-1
	for row := 0; row < 3; row++ {
		for col := 0; col < 2; col++ {
			sheet.SetCell(&model.RawCell{
				Ref:   model.CellRef{Sheet: "Sheet1", Row: row, Col: col},
				Value: float64(row*2 + col + 1),
				Type:  model.CellTypeNumber,
			})
		}
	}

	// Block 2: rows 5-7, cols 0-1 (gap of 2 rows)
	for row := 5; row < 8; row++ {
		for col := 0; col < 2; col++ {
			sheet.SetCell(&model.RawCell{
				Ref:   model.CellRef{Sheet: "Sheet1", Row: row, Col: col},
				Value: float64(row*2 + col + 100),
				Type:  model.CellTypeNumber,
			})
		}
	}

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	// Should detect two separate arrays
	numericArrays := 0
	for _, arr := range arrays {
		if arr.DType == "float64" {
			rows, cols := arr.RangeRef.Shape()
			if rows == 3 && cols == 2 {
				numericArrays++
			}
		}
	}

	if numericArrays != 2 {
		t.Errorf("expected 2 separate 3x2 arrays, got %d", numericArrays)
	}
}

func TestArrayDetectorV2_StringArray(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a column of strings
	strings := []string{"Alpha", "Beta", "Gamma", "Delta"}
	for i, s := range strings {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: i, Col: 0},
			Value: s,
			Type:  model.CellTypeString,
		})
	}

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	// Should detect a string array
	found := false
	for _, arr := range arrays {
		if arr.DType == "str" {
			rows, cols := arr.RangeRef.Shape()
			if rows == 4 && cols == 1 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to find a 4x1 string array")
	}
}

func TestArrayDetectorV2_EmptyWorkbook(t *testing.T) {
	wb := makeTestWorkbook()
	// Don't add any cells

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	if len(arrays) != 0 {
		t.Errorf("expected 0 arrays for empty workbook, got %d", len(arrays))
	}
}

func TestArrayDetectorV2_SingleCell(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 5, Col: 5},
		Value: 42.0,
		Type:  model.CellTypeNumber,
	})

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	if len(arrays) != 1 {
		t.Fatalf("expected 1 array for single cell, got %d", len(arrays))
	}

	arr := arrays[0]
	rows, cols := arr.RangeRef.Shape()
	if rows != 1 || cols != 1 {
		t.Errorf("expected 1x1 array, got %dx%d", rows, cols)
	}
}

func TestClassifyDataRoles(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// A1: input value
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 0},
		Value: 10.0,
		Type:  model.CellTypeNumber,
	})

	// B1: formula referencing A1
	sheet.SetCell(&model.RawCell{
		Ref:     model.CellRef{Sheet: "Sheet1", Row: 0, Col: 1},
		Value:   20.0,
		Formula: "A1*2",
		Type:    model.CellTypeFormula,
	})

	// Build graph
	graph, parsedFormulas := BuildDependencyGraph(wb)

	// Create arrays
	inputArray := &model.InferredArray{
		ID: "arr_001",
		RangeRef: model.RangeRef{
			Sheet:       "Sheet1",
			TopLeft:     [2]int{0, 0},
			BottomRight: [2]int{0, 0},
		},
		HasFormulas: false,
	}
	formulaArray := &model.InferredArray{
		ID: "arr_002",
		RangeRef: model.RangeRef{
			Sheet:       "Sheet1",
			TopLeft:     [2]int{0, 1},
			BottomRight: [2]int{0, 1},
		},
		HasFormulas: true,
	}

	arrays := []*model.InferredArray{inputArray, formulaArray}

	// Classify roles
	ClassifyDataRoles(arrays, graph)

	// Input array should be PARAMETER (referenced by formulas)
	if inputArray.DataRole != model.RoleParameter {
		t.Errorf("expected input array to be PARAMETER, got %s", inputArray.DataRole)
	}

	// Formula array should be OUTPUT (not referenced by anything)
	if formulaArray.DataRole != model.RoleOutput {
		t.Errorf("expected formula array to be OUTPUT, got %s", formulaArray.DataRole)
	}

	_ = parsedFormulas // suppress unused variable warning
}

func TestDetectScalars(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create an isolated input cell that will be referenced
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 10, Col: 10},
		Value: 100.0,
		Type:  model.CellTypeNumber,
	})

	// Create a formula that references the isolated cell
	sheet.SetCell(&model.RawCell{
		Ref:     model.CellRef{Sheet: "Sheet1", Row: 20, Col: 20},
		Value:   200.0,
		Formula: "K11*2", // References (10,10) in Excel notation
		Type:    model.CellTypeFormula,
	})

	// Create a block of numbers that will be an array
	for i := 0; i < 3; i++ {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: i, Col: 0},
			Value: float64(i + 1),
			Type:  model.CellTypeNumber,
		})
	}

	graph, _ := BuildDependencyGraph(wb)

	// Create arrays covering A1:A3 and the formula cell
	arrays := []*model.InferredArray{
		{
			ID: "arr_001",
			RangeRef: model.RangeRef{
				Sheet:       "Sheet1",
				TopLeft:     [2]int{0, 0},
				BottomRight: [2]int{2, 0},
			},
		},
		{
			ID: "arr_002",
			RangeRef: model.RangeRef{
				Sheet:       "Sheet1",
				TopLeft:     [2]int{20, 20},
				BottomRight: [2]int{20, 20},
			},
		},
	}

	scalars := DetectScalars(wb, graph, arrays)

	// Should detect the isolated referenced cell as a scalar
	if len(scalars) != 1 {
		t.Errorf("expected 1 scalar, got %d", len(scalars))
	}

	if len(scalars) > 0 {
		scl := scalars[0]
		if scl.CellRef.Row != 10 || scl.CellRef.Col != 10 {
			t.Errorf("expected scalar at (10,10), got (%d,%d)", scl.CellRef.Row, scl.CellRef.Col)
		}
	}
}

func TestGetDependents(t *testing.T) {
	graph := &model.DependencyGraph{
		Nodes: []string{"A", "B", "C", "D"},
		Edges: []model.DependencyEdge{
			{Source: "A", Target: "B"},
			{Source: "A", Target: "C"},
			{Source: "B", Target: "D"},
		},
	}

	dependents := GetDependents(graph, "A")

	// A should have transitive dependents B, C, and D (D through B)
	if len(dependents) != 3 {
		t.Errorf("expected 3 dependents of A (transitive), got %d: %v", len(dependents), dependents)
	}

	expected := map[string]bool{"B": true, "C": true, "D": true}
	for _, d := range dependents {
		if !expected[d] {
			t.Errorf("unexpected dependent: %s", d)
		}
		delete(expected, d)
	}
	if len(expected) > 0 {
		t.Errorf("missing dependents: %v", expected)
	}
}

func TestGetDependents_DirectOnly(t *testing.T) {
	// Test that a node with no transitive deps returns only direct deps
	graph := &model.DependencyGraph{
		Nodes: []string{"A", "B", "C"},
		Edges: []model.DependencyEdge{
			{Source: "A", Target: "B"},
			{Source: "A", Target: "C"},
		},
	}

	dependents := GetDependents(graph, "A")

	// B and C are direct dependents with no further deps
	if len(dependents) != 2 {
		t.Errorf("expected 2 dependents, got %d", len(dependents))
	}
}

func TestGetDependencies(t *testing.T) {
	graph := &model.DependencyGraph{
		Nodes: []string{"A", "B", "C", "D"},
		Edges: []model.DependencyEdge{
			{Source: "A", Target: "C"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "D"},
		},
	}

	dependencies := GetDependencies(graph, "C")

	// C depends on A and B
	if len(dependencies) != 2 {
		t.Errorf("expected 2 dependencies of C, got %d", len(dependencies))
	}

	hasA, hasB := false, false
	for _, d := range dependencies {
		if d == "A" {
			hasA = true
		}
		if d == "B" {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Errorf("expected dependencies [A, B], got %v", dependencies)
	}
}

func TestColIndexToLetters(t *testing.T) {
	tests := []struct {
		col  int
		want string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{701, "ZZ"},
		{702, "AAA"},
	}

	for _, tt := range tests {
		got := colIndexToLetters(tt.col)
		if got != tt.want {
			t.Errorf("colIndexToLetters(%d) = %q, want %q", tt.col, got, tt.want)
		}
	}
}
