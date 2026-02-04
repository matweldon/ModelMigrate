package inference

import (
	"fmt"
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

// Tests for containsString helper function
func TestContainsString(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"HELLO WORLD", "HELLO", true},
		{"hello world", "HELLO", true},  // Case insensitive
		{"HELLO WORLD", "hello", true},  // Case insensitive
		{"SUM(A1:A10)", "SUM", true},
		{"sum(A1:A10)", "SUM", true},
		{"AVERAGE(B1:B5)", "SUM", false},
		{"A1+B1", "+", true},
		{"A1*B1-C1", "*", true},
		{"A1*B1-C1", "/", false},
		{"", "SUM", false},
		{"SUM", "", true},
		{"CONCATENATE(A1,B1)", "CONCATENATE", true},
		{"LEFT(A1,5)", "LEFT", true},
	}

	for _, tt := range tests {
		got := containsString(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("containsString(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

// Tests for inferPhantomType
func TestInferPhantomType(t *testing.T) {
	detector := NewArrayDetectorV2(makeTestWorkbook(), make(map[string]*xlsx.ParsedFormula))

	tests := []struct {
		formula string
		want    CellType
	}{
		{"A1+B1", CellTypeNumeric},
		{"A1-B1", CellTypeNumeric},
		{"A1*B1", CellTypeNumeric},
		{"A1/B1", CellTypeNumeric},
		{"A1^2", CellTypeNumeric},
		{"SUM(A1:A10)", CellTypeNumeric},
		{"AVERAGE(B1:B5)", CellTypeNumeric},
		{"MIN(C1:C10)", CellTypeNumeric},
		{"MAX(D1:D10)", CellTypeNumeric},
		{"COUNT(E1:E10)", CellTypeNumeric},
		{"CONCATENATE(A1,B1)", CellTypeString},
		{"LEFT(A1,5)", CellTypeString},
		{"RIGHT(A1,3)", CellTypeString},
		{"MID(A1,2,4)", CellTypeString},
		{"TRIM(A1)", CellTypeString},
		{"UPPER(A1)", CellTypeString},
		{"LOWER(A1)", CellTypeString},
		{"TEXT(A1,\"$#,##0\")", CellTypeString},
		{"IF(A1>0,B1,C1)", CellTypeUnknown}, // Ambiguous
		{"VLOOKUP(A1,B1:C10,2)", CellTypeUnknown},
	}

	for _, tt := range tests {
		got := detector.inferPhantomType(tt.formula)
		if got != tt.want {
			t.Errorf("inferPhantomType(%q) = %v, want %v", tt.formula, got, tt.want)
		}
	}
}

// Test phantom cell detection through formula references
func TestArrayDetectorV2_PhantomCells(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a formula that references non-existent cells
	sheet.SetCell(&model.RawCell{
		Ref:     model.CellRef{Sheet: "Sheet1", Row: 5, Col: 5},
		Value:   10.0,
		Formula: "A1+B1+C1", // A1, B1, C1 don't exist
		Type:    model.CellTypeFormula,
	})

	// Parse the formula
	parsedFormulas := make(map[string]*xlsx.ParsedFormula)
	parsedFormulas["Sheet1!5,5"] = xlsx.ParseFormula("A1+B1+C1", "Sheet1")

	detector := NewArrayDetectorV2(wb, parsedFormulas)
	detector.collectCellUniverse()

	// Check that phantom cells were created
	sheetCells := detector.cells["Sheet1"]

	phantomCells := []string{"0,0", "0,1", "0,2"} // A1, B1, C1
	for _, key := range phantomCells {
		cell := sheetCells[key]
		if cell == nil {
			t.Errorf("expected phantom cell at %s", key)
			continue
		}
		if !cell.IsPhantom {
			t.Errorf("expected cell at %s to be phantom", key)
		}
	}
}

// Test formula template extraction with relative references
func TestArrayDetectorV2_FormulaTemplateRelative(t *testing.T) {
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

	// Create formulas in column B with consistent relative references
	formulas := []string{"A1*2", "A2*2", "A3*2"}
	parsedFormulas := make(map[string]*xlsx.ParsedFormula)
	for i, f := range formulas {
		sheet.SetCell(&model.RawCell{
			Ref:     model.CellRef{Sheet: "Sheet1", Row: i, Col: 1},
			Value:   float64((i + 1) * 2),
			Formula: f,
			Type:    model.CellTypeFormula,
		})
		key := fmt.Sprintf("Sheet1!%d,1", i)
		parsedFormulas[key] = xlsx.ParseFormula(f, "Sheet1")
	}

	detector := NewArrayDetectorV2(wb, parsedFormulas)
	arrays := detector.DetectArrays()

	// Find the formula array
	var formulaArray *model.InferredArray
	for _, arr := range arrays {
		if arr.HasFormulas {
			formulaArray = arr
			break
		}
	}

	if formulaArray == nil {
		t.Fatal("expected to find formula array")
	}

	if formulaArray.FormulaTemplate == nil {
		t.Fatal("expected formula template to be populated")
	}

	// Check coverage is 100%
	if formulaArray.FormulaTemplate.Coverage != 1.0 {
		t.Errorf("expected 100%% coverage, got %.1f%%", formulaArray.FormulaTemplate.Coverage*100)
	}

	// Check that relative pattern was detected
	if len(formulaArray.FormulaTemplate.RelativePatterns) == 0 {
		t.Error("expected relative patterns to be detected")
	}
}

// Test formula compatibility check
func TestFormulasCompatibleV2(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a grid of compatible formulas
	formulas := [][]string{
		{"A1*2", "B1*2"},
		{"A2*2", "B2*2"},
	}

	parsedFormulas := make(map[string]*xlsx.ParsedFormula)
	for row := 0; row < 2; row++ {
		for col := 2; col < 4; col++ {
			f := formulas[row][col-2]
			sheet.SetCell(&model.RawCell{
				Ref:     model.CellRef{Sheet: "Sheet1", Row: row, Col: col},
				Value:   float64(row*10 + col),
				Formula: f,
				Type:    model.CellTypeFormula,
			})
			key := fmt.Sprintf("Sheet1!%d,%d", row, col)
			parsedFormulas[key] = xlsx.ParseFormula(f, "Sheet1")
		}
	}

	detector := NewArrayDetectorV2(wb, parsedFormulas)

	// Test compatible formulas
	if !detector.formulasCompatibleV2("Sheet1", 0, 2, 0, 3) {
		t.Error("expected formulas to be compatible horizontally")
	}

	if !detector.formulasCompatibleV2("Sheet1", 0, 2, 1, 2) {
		t.Error("expected formulas to be compatible vertically")
	}

	// Test incompatible case (no formula at target)
	if detector.formulasCompatibleV2("Sheet1", 0, 2, 10, 10) {
		t.Error("expected formulas to be incompatible when target has no formula")
	}
}

// Test string array expansion across multiple columns
func TestArrayDetectorV2_StringArrayMultiColumn(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a 2x3 block of strings
	strings := [][]string{
		{"A", "B", "C"},
		{"D", "E", "F"},
	}
	for row := 0; row < 2; row++ {
		for col := 0; col < 3; col++ {
			sheet.SetCell(&model.RawCell{
				Ref:   model.CellRef{Sheet: "Sheet1", Row: row, Col: col},
				Value: strings[row][col],
				Type:  model.CellTypeString,
			})
		}
	}

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	// Should find a string array
	var stringArray *model.InferredArray
	for _, arr := range arrays {
		if arr.DType == "str" {
			stringArray = arr
			break
		}
	}

	if stringArray == nil {
		t.Fatal("expected to find string array")
	}

	rows, cols := stringArray.RangeRef.Shape()
	if rows != 2 || cols != 3 {
		t.Errorf("expected 2x3 string array, got %dx%d", rows, cols)
	}
}

// Test ClassifyDataRoles with all roles
func TestClassifyDataRoles_AllRoles(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// INPUT: Value with no references to it and no formula
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 10, Col: 10},
		Value: 100.0,
		Type:  model.CellTypeNumber,
	})

	// PARAMETER: Value referenced by formulas but has no formula itself
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 0},
		Value: 10.0,
		Type:  model.CellTypeNumber,
	})

	// INTERMEDIATE: Has formula and is referenced by other formulas
	sheet.SetCell(&model.RawCell{
		Ref:     model.CellRef{Sheet: "Sheet1", Row: 0, Col: 1},
		Value:   20.0,
		Formula: "A1*2",
		Type:    model.CellTypeFormula,
	})

	// OUTPUT: Has formula but not referenced by anything
	sheet.SetCell(&model.RawCell{
		Ref:     model.CellRef{Sheet: "Sheet1", Row: 0, Col: 2},
		Value:   40.0,
		Formula: "B1*2",
		Type:    model.CellTypeFormula,
	})

	graph, _ := BuildDependencyGraph(wb)

	arrays := []*model.InferredArray{
		{ID: "input", RangeRef: model.RangeRef{Sheet: "Sheet1", TopLeft: [2]int{10, 10}, BottomRight: [2]int{10, 10}}, HasFormulas: false},
		{ID: "param", RangeRef: model.RangeRef{Sheet: "Sheet1", TopLeft: [2]int{0, 0}, BottomRight: [2]int{0, 0}}, HasFormulas: false},
		{ID: "inter", RangeRef: model.RangeRef{Sheet: "Sheet1", TopLeft: [2]int{0, 1}, BottomRight: [2]int{0, 1}}, HasFormulas: true},
		{ID: "output", RangeRef: model.RangeRef{Sheet: "Sheet1", TopLeft: [2]int{0, 2}, BottomRight: [2]int{0, 2}}, HasFormulas: true},
	}

	ClassifyDataRoles(arrays, graph)

	expected := map[string]model.DataRole{
		"input":  model.RoleInput,
		"param":  model.RoleParameter,
		"inter":  model.RoleIntermediate,
		"output": model.RoleOutput,
	}

	for _, arr := range arrays {
		if arr.DataRole != expected[arr.ID] {
			t.Errorf("array %s: expected role %s, got %s", arr.ID, expected[arr.ID], arr.DataRole)
		}
	}
}

// Test array detection with mixed types that shouldn't merge
func TestArrayDetectorV2_MixedTypesNoMerge(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Numbers in column A, string separator in column B
	// This ensures the string doesn't get used as a header
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 0},
		Value: 1.0,
		Type:  model.CellTypeNumber,
	})
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 1},
		Value: "labelA",
		Type:  model.CellTypeString,
	})
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 5, Col: 0},
		Value: 2.0,
		Type:  model.CellTypeNumber,
	})
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 5, Col: 1},
		Value: "labelB",
		Type:  model.CellTypeString,
	})

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	// Should have numeric arrays and string arrays separately
	numericCount := 0
	stringCount := 0
	for _, arr := range arrays {
		if arr.DType == "float64" {
			numericCount++
		} else if arr.DType == "str" {
			stringCount++
		}
	}

	// At least 2 separate numeric arrays (row 0 and row 5)
	if numericCount < 2 {
		t.Errorf("expected at least 2 numeric arrays, got %d", numericCount)
	}
}

// Test formula congruence with absolute references
func TestFormulasCompatibleV2_AbsoluteRefs(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a parameter cell at A1 (will be referenced with $A$1)
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 0},
		Value: 10.0,
		Type:  model.CellTypeNumber,
	})

	// Create input values in column B
	for i := 0; i < 3; i++ {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: i, Col: 1},
			Value: float64(i + 1),
			Type:  model.CellTypeNumber,
		})
	}

	// Create formulas in column C with mixed absolute/relative references
	// C1 = B1 * $A$1, C2 = B2 * $A$1, C3 = B3 * $A$1
	formulas := []string{"B1*$A$1", "B2*$A$1", "B3*$A$1"}
	parsedFormulas := make(map[string]*xlsx.ParsedFormula)
	for i, f := range formulas {
		sheet.SetCell(&model.RawCell{
			Ref:     model.CellRef{Sheet: "Sheet1", Row: i, Col: 2},
			Value:   float64((i + 1) * 10),
			Formula: f,
			Type:    model.CellTypeFormula,
		})
		key := fmt.Sprintf("Sheet1!%d,2", i)
		parsedFormulas[key] = xlsx.ParseFormula(f, "Sheet1")
	}

	detector := NewArrayDetectorV2(wb, parsedFormulas)

	// These should be compatible - B moves relatively, A is fixed
	if !detector.formulasCompatibleV2("Sheet1", 0, 2, 1, 2) {
		t.Error("expected formulas with mixed abs/rel refs to be compatible")
	}
	if !detector.formulasCompatibleV2("Sheet1", 0, 2, 2, 2) {
		t.Error("expected formulas with mixed abs/rel refs to be compatible")
	}

	// Now test incompatible case - formulas where absolute ref moves (shouldn't happen in real Excel)
	// Create a formula where the "absolute" ref incorrectly moves
	badFormulas := []string{"B1*$A$1", "B2*$A$2"} // Second one has different absolute ref
	badParsedFormulas := make(map[string]*xlsx.ParsedFormula)
	for i, f := range badFormulas {
		key := fmt.Sprintf("Sheet1!%d,3", i)
		badParsedFormulas[key] = xlsx.ParseFormula(f, "Sheet1")
	}

	badDetector := NewArrayDetectorV2(wb, badParsedFormulas)
	if badDetector.formulasCompatibleV2("Sheet1", 0, 3, 1, 3) {
		t.Error("expected formulas with inconsistent absolute refs to be incompatible")
	}
}

// Test that arrays are correctly detected with absolute reference formulas
func TestArrayDetectorV2_MixedAbsoluteRelativeFormulas(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Create a parameter cell at A1
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 0},
		Value: 1.5,
		Type:  model.CellTypeNumber,
	})

	// Create input values in column B (rows 1-5)
	for i := 1; i <= 5; i++ {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: i, Col: 1},
			Value: float64(i * 100),
			Type:  model.CellTypeNumber,
		})
	}

	// Create formulas in column C with B relative, A absolute
	parsedFormulas := make(map[string]*xlsx.ParsedFormula)
	for i := 1; i <= 5; i++ {
		formula := fmt.Sprintf("B%d*$A$1", i+1)
		sheet.SetCell(&model.RawCell{
			Ref:     model.CellRef{Sheet: "Sheet1", Row: i, Col: 2},
			Value:   float64(i * 100) * 1.5,
			Formula: formula,
			Type:    model.CellTypeFormula,
		})
		key := fmt.Sprintf("Sheet1!%d,2", i)
		parsedFormulas[key] = xlsx.ParseFormula(formula, "Sheet1")
	}

	detector := NewArrayDetectorV2(wb, parsedFormulas)
	arrays := detector.DetectArrays()

	// Find the formula array in column C
	var formulaArray *model.InferredArray
	for _, arr := range arrays {
		if arr.HasFormulas && arr.RangeRef.TopLeft[1] == 2 {
			formulaArray = arr
			break
		}
	}

	if formulaArray == nil {
		t.Fatal("expected to find formula array in column C")
	}

	// Should be a 5-row column vector
	rows, cols := formulaArray.RangeRef.Shape()
	if rows != 5 || cols != 1 {
		t.Errorf("expected 5x1 array, got %dx%d", rows, cols)
	}

	// Formula template should have 100% coverage
	if formulaArray.FormulaTemplate == nil {
		t.Fatal("expected formula template to be populated")
	}
	if formulaArray.FormulaTemplate.Coverage != 1.0 {
		t.Errorf("expected 100%% coverage, got %.1f%%", formulaArray.FormulaTemplate.Coverage*100)
	}
}

// Test formula hash computation
func TestComputeFormulaHash(t *testing.T) {
	wb := makeTestWorkbook()
	sheet := wb.Sheets["Sheet1"]

	// Two cells with similar formulas should have same hash
	formulas := []string{"SUM(A1:A5)", "SUM(B1:B5)"}
	parsedFormulas := make(map[string]*xlsx.ParsedFormula)

	for i, f := range formulas {
		sheet.SetCell(&model.RawCell{
			Ref:     model.CellRef{Sheet: "Sheet1", Row: 0, Col: i},
			Value:   10.0,
			Formula: f,
			Type:    model.CellTypeFormula,
		})
		key := fmt.Sprintf("Sheet1!0,%d", i)
		parsedFormulas[key] = xlsx.ParseFormula(f, "Sheet1")
	}

	detector := NewArrayDetectorV2(wb, parsedFormulas)

	hash1 := detector.computeFormulaHash("Sheet1", 0, 0)
	hash2 := detector.computeFormulaHash("Sheet1", 0, 1)

	if hash1 != hash2 {
		t.Errorf("expected same hash for similar formulas, got %q and %q", hash1, hash2)
	}

	// Non-existent cell should return empty hash
	hash3 := detector.computeFormulaHash("Sheet1", 99, 99)
	if hash3 != "" {
		t.Errorf("expected empty hash for non-existent cell, got %q", hash3)
	}
}

// Test algorithm stability with different cell orderings (shuffle test)
// The algorithm should produce identical results regardless of the order cells are added
func TestArrayDetectorV2_ShuffleStability(t *testing.T) {
	// Define a consistent set of cells to add
	type cellData struct {
		row     int
		col     int
		value   any
		formula string
		typ     model.CellType
	}

	// Create a complex workbook structure:
	// - Header row in row 0 (strings)
	// - Data in rows 1-5, columns 0-2 (numbers)
	// - Formulas in column 3 referencing column 2
	cells := []cellData{
		// Headers
		{0, 0, "Name", "", model.CellTypeString},
		{0, 1, "Value1", "", model.CellTypeString},
		{0, 2, "Value2", "", model.CellTypeString},
		{0, 3, "Total", "", model.CellTypeString},
		// Data rows
		{1, 0, "A", "", model.CellTypeString},
		{1, 1, 10.0, "", model.CellTypeNumber},
		{1, 2, 20.0, "", model.CellTypeNumber},
		{1, 3, 30.0, "B2+C2", model.CellTypeFormula},
		{2, 0, "B", "", model.CellTypeString},
		{2, 1, 15.0, "", model.CellTypeNumber},
		{2, 2, 25.0, "", model.CellTypeNumber},
		{2, 3, 40.0, "B3+C3", model.CellTypeFormula},
		{3, 0, "C", "", model.CellTypeString},
		{3, 1, 20.0, "", model.CellTypeNumber},
		{3, 2, 30.0, "", model.CellTypeNumber},
		{3, 3, 50.0, "B4+C4", model.CellTypeFormula},
		{4, 0, "D", "", model.CellTypeString},
		{4, 1, 25.0, "", model.CellTypeNumber},
		{4, 2, 35.0, "", model.CellTypeNumber},
		{4, 3, 60.0, "B5+C5", model.CellTypeFormula},
		{5, 0, "E", "", model.CellTypeString},
		{5, 1, 30.0, "", model.CellTypeNumber},
		{5, 2, 40.0, "", model.CellTypeNumber},
		{5, 3, 70.0, "B6+C6", model.CellTypeFormula},
	}

	// Helper to create workbook with cells in given order
	createWorkbook := func(order []int) (*model.RawWorkbook, map[string]*xlsx.ParsedFormula) {
		wb := model.NewRawWorkbook()
		wb.SheetOrder = []string{"Sheet1"}
		sheet := model.NewRawSheet("Sheet1")
		wb.Sheets["Sheet1"] = sheet
		parsedFormulas := make(map[string]*xlsx.ParsedFormula)

		for _, idx := range order {
			c := cells[idx]
			cell := &model.RawCell{
				Ref:     model.CellRef{Sheet: "Sheet1", Row: c.row, Col: c.col},
				Value:   c.value,
				Formula: c.formula,
				Type:    c.typ,
			}
			sheet.SetCell(cell)

			if c.formula != "" {
				key := fmt.Sprintf("Sheet1!%d,%d", c.row, c.col)
				parsedFormulas[key] = xlsx.ParseFormula(c.formula, "Sheet1")
			}
		}
		return wb, parsedFormulas
	}

	// Helper to extract array signature for comparison
	type arraySignature struct {
		topLeft     [2]int
		bottomRight [2]int
		hasFormulas bool
		dtype       string
		orientation model.ArrayOrientation
	}

	getSignatures := func(arrays []*model.InferredArray) []arraySignature {
		sigs := make([]arraySignature, len(arrays))
		for i, arr := range arrays {
			sigs[i] = arraySignature{
				topLeft:     arr.RangeRef.TopLeft,
				bottomRight: arr.RangeRef.BottomRight,
				hasFormulas: arr.HasFormulas,
				dtype:       arr.DType,
				orientation: arr.Orientation,
			}
		}
		// Sort by position for consistent comparison
		for i := 0; i < len(sigs); i++ {
			for j := i + 1; j < len(sigs); j++ {
				if sigs[i].topLeft[0] > sigs[j].topLeft[0] ||
					(sigs[i].topLeft[0] == sigs[j].topLeft[0] && sigs[i].topLeft[1] > sigs[j].topLeft[1]) {
					sigs[i], sigs[j] = sigs[j], sigs[i]
				}
			}
		}
		return sigs
	}

	// Create baseline with natural order
	naturalOrder := make([]int, len(cells))
	for i := range naturalOrder {
		naturalOrder[i] = i
	}
	baseWb, baseParsed := createWorkbook(naturalOrder)
	baseDetector := NewArrayDetectorV2(baseWb, baseParsed)
	baseArrays := baseDetector.DetectArrays()
	baseSignatures := getSignatures(baseArrays)

	// Test with different orderings
	orderings := [][]int{
		// Reverse order
		func() []int {
			o := make([]int, len(cells))
			for i := range o {
				o[i] = len(cells) - 1 - i
			}
			return o
		}(),
		// Column-major order (all col 0, then col 1, etc.)
		{0, 4, 8, 12, 16, 20, 1, 5, 9, 13, 17, 21, 2, 6, 10, 14, 18, 22, 3, 7, 11, 15, 19, 23},
		// Interleaved (every other cell)
		{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21, 23},
		// Random-ish shuffle
		{12, 3, 18, 7, 0, 15, 22, 9, 4, 20, 11, 1, 17, 6, 23, 8, 14, 2, 19, 5, 21, 10, 16, 13},
	}

	for orderIdx, order := range orderings {
		wb, parsed := createWorkbook(order)
		detector := NewArrayDetectorV2(wb, parsed)
		arrays := detector.DetectArrays()
		signatures := getSignatures(arrays)

		// Compare with baseline
		if len(signatures) != len(baseSignatures) {
			t.Errorf("order %d: got %d arrays, want %d", orderIdx, len(signatures), len(baseSignatures))
			continue
		}

		for i, sig := range signatures {
			base := baseSignatures[i]
			if sig.topLeft != base.topLeft || sig.bottomRight != base.bottomRight {
				t.Errorf("order %d, array %d: range mismatch - got %v:%v, want %v:%v",
					orderIdx, i, sig.topLeft, sig.bottomRight, base.topLeft, base.bottomRight)
			}
			if sig.hasFormulas != base.hasFormulas {
				t.Errorf("order %d, array %d: hasFormulas mismatch - got %v, want %v",
					orderIdx, i, sig.hasFormulas, base.hasFormulas)
			}
			if sig.dtype != base.dtype {
				t.Errorf("order %d, array %d: dtype mismatch - got %s, want %s",
					orderIdx, i, sig.dtype, base.dtype)
			}
		}
	}
}

// Test that the algorithm handles empty sheets gracefully
func TestArrayDetectorV2_EmptySheet(t *testing.T) {
	wb := model.NewRawWorkbook()
	wb.SheetOrder = []string{"Sheet1", "Sheet2"}
	wb.Sheets["Sheet1"] = model.NewRawSheet("Sheet1")
	wb.Sheets["Sheet2"] = model.NewRawSheet("Sheet2")

	// Add data only to Sheet1
	wb.Sheets["Sheet1"].SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 0},
		Value: 1.0,
		Type:  model.CellTypeNumber,
	})

	detector := NewArrayDetectorV2(wb, make(map[string]*xlsx.ParsedFormula))
	arrays := detector.DetectArrays()

	// Should only find arrays in Sheet1, not crash on empty Sheet2
	if len(arrays) == 0 {
		t.Error("expected at least one array from Sheet1")
	}

	for _, arr := range arrays {
		if arr.RangeRef.Sheet == "Sheet2" {
			t.Error("should not find arrays in empty Sheet2")
		}
	}
}
