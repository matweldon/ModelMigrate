// Package inference provides improved structural inference for Excel workbooks.
// This file implements the v2 array detection algorithm with column-first processing.
package inference

import (
	"fmt"
	"sort"

	"github.com/matweldon/modelmigrate/parser/pkg/model"
	"github.com/matweldon/modelmigrate/parser/pkg/xlsx"
)

// CellType represents the inferred type of a cell
type CellType int

const (
	CellTypeUnknown CellType = iota
	CellTypeNumeric
	CellTypeString
)

// CellInfo holds information about a cell (existing or phantom)
type CellInfo struct {
	Sheet       string
	Row         int
	Col         int
	Type        CellType
	Format      string // number format
	Formula     string
	Value       any
	IsPhantom   bool // true if cell doesn't exist in workbook but is referenced
	FormulaHash string // normalized formula structure for congruence checking
}

// ArrayDetectorV2 implements the improved array detection algorithm
type ArrayDetectorV2 struct {
	workbook       *model.RawWorkbook
	parsedFormulas map[string]*xlsx.ParsedFormula

	// Cell universe: all cells (existing + phantom) organized by sheet
	cells map[string]map[string]*CellInfo // sheet -> "row,col" -> CellInfo

	// Assigned cells (already part of an array)
	assigned map[string]bool // "sheet!row,col" -> true

	arrayCounter int
}

// NewArrayDetectorV2 creates a new v2 array detector
func NewArrayDetectorV2(workbook *model.RawWorkbook, parsedFormulas map[string]*xlsx.ParsedFormula) *ArrayDetectorV2 {
	return &ArrayDetectorV2{
		workbook:       workbook,
		parsedFormulas: parsedFormulas,
		cells:          make(map[string]map[string]*CellInfo),
		assigned:       make(map[string]bool),
		arrayCounter:   0,
	}
}

// DetectArrays runs the full detection algorithm
func (d *ArrayDetectorV2) DetectArrays() []*model.InferredArray {
	// Phase 1: Collect cell universe
	d.collectCellUniverse()

	// Phase 2: Build numeric arrays (column-first)
	numericArrays := d.buildNumericArrays()

	// Phase 3: Assign string labels
	d.assignLabels(numericArrays)

	// Phase 3.5: Build formula templates for arrays with formulas
	d.buildFormulaTemplates(numericArrays)

	// Phase 4: Gather remaining cells
	remainingArrays := d.gatherRemainingCells()

	// Combine all arrays
	allArrays := append(numericArrays, remainingArrays...)

	// Phase 5: Resolve cell references to array references in formula templates
	d.resolveArrayReferences(allArrays)

	return allArrays
}

// Phase 1: Collect all cell references into a unified universe
func (d *ArrayDetectorV2) collectCellUniverse() {
	// First, add all existing cells
	for sheetName, sheet := range d.workbook.Sheets {
		if d.cells[sheetName] == nil {
			d.cells[sheetName] = make(map[string]*CellInfo)
		}

		for _, cell := range sheet.Cells {
			key := fmt.Sprintf("%d,%d", cell.Ref.Row, cell.Ref.Col)
			cellType := d.inferCellType(cell)

			info := &CellInfo{
				Sheet:     sheetName,
				Row:       cell.Ref.Row,
				Col:       cell.Ref.Col,
				Type:      cellType,
				Format:    cell.NumberFormat,
				Formula:   cell.Formula,
				Value:     cell.Value,
				IsPhantom: false,
			}

			// Compute formula hash for congruence checking
			if cell.Formula != "" {
				info.FormulaHash = d.computeFormulaHash(sheetName, cell.Ref.Row, cell.Ref.Col)
			}

			d.cells[sheetName][key] = info
		}
	}

	// Second, add phantom cells from formula references
	for sheetName, sheet := range d.workbook.Sheets {
		for _, cell := range sheet.Cells {
			if cell.Formula == "" {
				continue
			}

			parsed := d.parsedFormulas[fmt.Sprintf("%s!%d,%d", sheetName, cell.Ref.Row, cell.Ref.Col)]
			if parsed == nil {
				continue
			}

			// Add phantom cells for direct references
			for _, ref := range parsed.References {
				d.addPhantomCell(ref, cell.Formula)
			}

			// Add phantom cells for range references
			for _, rangeRef := range parsed.RangeReferences {
				for row := rangeRef.TopLeft[0]; row <= rangeRef.BottomRight[0]; row++ {
					for col := rangeRef.TopLeft[1]; col <= rangeRef.BottomRight[1]; col++ {
						ref := model.CellRef{Sheet: rangeRef.Sheet, Row: row, Col: col}
						d.addPhantomCell(ref, cell.Formula)
					}
				}
			}
		}
	}
}

// addPhantomCell adds a phantom cell if it doesn't already exist
func (d *ArrayDetectorV2) addPhantomCell(ref model.CellRef, formula string) {
	sheetName := ref.Sheet
	if d.cells[sheetName] == nil {
		d.cells[sheetName] = make(map[string]*CellInfo)
	}

	key := fmt.Sprintf("%d,%d", ref.Row, ref.Col)
	if _, exists := d.cells[sheetName][key]; exists {
		return // Cell already exists
	}

	// Infer type from formula context
	cellType := d.inferPhantomType(formula)

	d.cells[sheetName][key] = &CellInfo{
		Sheet:     sheetName,
		Row:       ref.Row,
		Col:       ref.Col,
		Type:      cellType,
		IsPhantom: true,
	}
}

// inferCellType determines the type of an existing cell
func (d *ArrayDetectorV2) inferCellType(cell *model.RawCell) CellType {
	switch cell.Type {
	case "number", "formula":
		return CellTypeNumeric
	case "string":
		return CellTypeString
	default:
		return CellTypeUnknown
	}
}

// inferPhantomType infers the type of a phantom cell from formula context
func (d *ArrayDetectorV2) inferPhantomType(formula string) CellType {
	// Simple heuristic: if formula contains arithmetic operators, assume numeric
	// This could be made more sophisticated by parsing the formula
	for _, op := range []string{"+", "-", "*", "/", "^", "SUM", "AVERAGE", "MIN", "MAX", "COUNT"} {
		if containsString(formula, op) {
			return CellTypeNumeric
		}
	}

	// String functions suggest string type
	for _, fn := range []string{"CONCATENATE", "LEFT", "RIGHT", "MID", "TRIM", "UPPER", "LOWER", "TEXT"} {
		if containsString(formula, fn) {
			return CellTypeString
		}
	}

	return CellTypeUnknown
}

// containsString checks if s contains substr (case-insensitive)
func containsString(s, substr string) bool {
	// Simple check - could use strings.Contains with ToUpper
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			// Simple uppercase conversion for ASCII
			if c1 >= 'a' && c1 <= 'z' {
				c1 -= 'a' - 'A'
			}
			if c2 >= 'a' && c2 <= 'z' {
				c2 -= 'a' - 'A'
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

// computeFormulaHash creates a normalized representation of formula structure
// Two formulas are congruent if they have the same hash
func (d *ArrayDetectorV2) computeFormulaHash(sheet string, row, col int) string {
	key := fmt.Sprintf("%s!%d,%d", sheet, row, col)
	parsed := d.parsedFormulas[key]
	if parsed == nil {
		return ""
	}

	// Build a hash based on:
	// 1. Functions used (sorted)
	// 2. Number of references
	// 3. Reference patterns (relative vs absolute)

	funcs := make([]string, len(parsed.Functions))
	copy(funcs, parsed.Functions)
	sort.Strings(funcs)

	hash := fmt.Sprintf("funcs:%v;refs:%d;ranges:%d",
		funcs,
		len(parsed.References),
		len(parsed.RangeReferences))

	return hash
}

// Phase 2: Build numeric arrays using column-first approach
func (d *ArrayDetectorV2) buildNumericArrays() []*model.InferredArray {
	var arrays []*model.InferredArray

	for sheetName, sheetCells := range d.cells {
		// Get bounds of this sheet's cells
		minRow, maxRow, minCol, maxCol := d.getSheetBounds(sheetCells)
		if minRow > maxRow {
			continue
		}

		// Process each column
		for col := minCol; col <= maxCol; col++ {
			// Find maximal vertical runs of numeric cells
			runs := d.findNumericRuns(sheetName, col, minRow, maxRow)

			for _, run := range runs {
				// Try to expand this run horizontally
				array := d.expandRunToArray(sheetName, run, col, maxCol)
				if array != nil {
					arrays = append(arrays, array)
				}
			}
		}
	}

	return arrays
}

// getSheetBounds returns the bounding box of cells in a sheet
func (d *ArrayDetectorV2) getSheetBounds(cells map[string]*CellInfo) (minRow, maxRow, minCol, maxCol int) {
	minRow, maxRow = 1<<30, -1
	minCol, maxCol = 1<<30, -1

	for _, cell := range cells {
		if cell.Row < minRow {
			minRow = cell.Row
		}
		if cell.Row > maxRow {
			maxRow = cell.Row
		}
		if cell.Col < minCol {
			minCol = cell.Col
		}
		if cell.Col > maxCol {
			maxCol = cell.Col
		}
	}

	return
}

// VerticalRun represents a contiguous run of cells in a column
type VerticalRun struct {
	StartRow int
	EndRow   int
	Format   string
	HasFormula bool
	FormulaHash string
}

// findNumericRuns finds all maximal vertical runs of numeric cells in a column
func (d *ArrayDetectorV2) findNumericRuns(sheet string, col, minRow, maxRow int) []VerticalRun {
	var runs []VerticalRun
	sheetCells := d.cells[sheet]

	var currentRun *VerticalRun

	for row := minRow; row <= maxRow; row++ {
		key := fmt.Sprintf("%d,%d", row, col)
		globalKey := fmt.Sprintf("%s!%d,%d", sheet, row, col)

		cell := sheetCells[key]

		// Check if this cell is numeric and not already assigned
		isNumeric := cell != nil && cell.Type == CellTypeNumeric && !d.assigned[globalKey]

		if isNumeric {
			if currentRun == nil {
				// Start a new run
				currentRun = &VerticalRun{
					StartRow:    row,
					EndRow:      row,
					Format:      cell.Format,
					HasFormula:  cell.Formula != "",
					FormulaHash: cell.FormulaHash,
				}
			} else {
				// Check if this cell is compatible with current run
				compatible := d.isCellCompatible(cell, currentRun)

				if compatible {
					// Extend the run
					currentRun.EndRow = row
				} else {
					// End current run, start new one
					if currentRun.EndRow >= currentRun.StartRow {
						runs = append(runs, *currentRun)
					}
					currentRun = &VerticalRun{
						StartRow:    row,
						EndRow:      row,
						Format:      cell.Format,
						HasFormula:  cell.Formula != "",
						FormulaHash: cell.FormulaHash,
					}
				}
			}
		} else {
			// Non-numeric cell - end current run if any
			if currentRun != nil {
				runs = append(runs, *currentRun)
				currentRun = nil
			}
		}
	}

	// Don't forget the last run
	if currentRun != nil {
		runs = append(runs, *currentRun)
	}

	return runs
}

// isCellCompatible checks if a cell is compatible with a run
func (d *ArrayDetectorV2) isCellCompatible(cell *CellInfo, run *VerticalRun) bool {
	// Check format compatibility
	if cell.Format != run.Format {
		return false
	}

	// Check formula compatibility
	cellHasFormula := cell.Formula != ""
	if cellHasFormula != run.HasFormula {
		return false
	}

	// If both have formulas, check congruence
	if cellHasFormula && run.HasFormula {
		if cell.FormulaHash != run.FormulaHash {
			return false
		}
	}

	return true
}

// expandRunToArray tries to expand a vertical run horizontally to form an array
func (d *ArrayDetectorV2) expandRunToArray(sheet string, run VerticalRun, startCol, maxCol int) *model.InferredArray {
	sheetCells := d.cells[sheet]

	endCol := startCol

	// Try to expand right
	for col := startCol + 1; col <= maxCol; col++ {
		// Check if entire column segment matches
		allMatch := true
		for row := run.StartRow; row <= run.EndRow; row++ {
			key := fmt.Sprintf("%d,%d", row, col)
			globalKey := fmt.Sprintf("%s!%d,%d", sheet, row, col)

			cell := sheetCells[key]
			if cell == nil || cell.Type != CellTypeNumeric || d.assigned[globalKey] {
				allMatch = false
				break
			}

			if !d.isCellCompatible(cell, &run) {
				allMatch = false
				break
			}
		}

		if allMatch {
			endCol = col
		} else {
			break
		}
	}

	// Mark cells as assigned
	for row := run.StartRow; row <= run.EndRow; row++ {
		for col := startCol; col <= endCol; col++ {
			globalKey := fmt.Sprintf("%s!%d,%d", sheet, row, col)
			d.assigned[globalKey] = true
		}
	}

	// Create the array
	d.arrayCounter++
	array := &model.InferredArray{
		ID: fmt.Sprintf("arr_%03d", d.arrayCounter),
		RangeRef: model.RangeRef{
			Sheet:       sheet,
			TopLeft:     [2]int{run.StartRow, startCol},
			BottomRight: [2]int{run.EndRow, endCol},
		},
		HasFormulas: run.HasFormula,
		DType:       "float64",
		DataRole:    model.RoleInput, // Will be refined later
	}

	// Determine orientation
	rows := run.EndRow - run.StartRow + 1
	cols := endCol - startCol + 1
	if rows == 1 && cols > 1 {
		array.Orientation = model.OrientationRowVector
	} else if cols == 1 && rows > 1 {
		array.Orientation = model.OrientationColVector
	} else if rows == 1 && cols == 1 {
		array.Orientation = model.OrientationRowVector // scalar
	} else {
		array.Orientation = model.OrientationMatrix
	}

	return array
}

// Phase 3: Assign string labels to numeric arrays
func (d *ArrayDetectorV2) assignLabels(arrays []*model.InferredArray) {
	for _, arr := range arrays {
		// Look for column headers above the array
		arr.ColHeaders = d.findColHeaders(arr)

		// Look for row headers to the left of the array
		arr.RowHeaders = d.findRowHeaders(arr)
	}
}

// findColHeaders looks for string cells above the array that could be column headers
func (d *ArrayDetectorV2) findColHeaders(arr *model.InferredArray) []string {
	sheetCells := d.cells[arr.RangeRef.Sheet]
	if sheetCells == nil {
		return nil
	}

	startRow := arr.RangeRef.TopLeft[0]
	startCol := arr.RangeRef.TopLeft[1]
	endCol := arr.RangeRef.BottomRight[1]

	// Look up to 2 rows above for headers
	for offset := 1; offset <= 2; offset++ {
		headerRow := startRow - offset
		if headerRow < 0 {
			break
		}

		headers := []string{}
		allStrings := true

		for col := startCol; col <= endCol; col++ {
			key := fmt.Sprintf("%d,%d", headerRow, col)
			cell := sheetCells[key]

			if cell == nil || cell.Type != CellTypeString {
				allStrings = false
				break
			}

			headers = append(headers, fmt.Sprintf("%v", cell.Value))
		}

		if allStrings && len(headers) > 0 {
			// Mark header cells as assigned
			for col := startCol; col <= endCol; col++ {
				globalKey := fmt.Sprintf("%s!%d,%d", arr.RangeRef.Sheet, headerRow, col)
				d.assigned[globalKey] = true
			}

			arr.ColHeaderSource = &model.RangeRef{
				Sheet:       arr.RangeRef.Sheet,
				TopLeft:     [2]int{headerRow, startCol},
				BottomRight: [2]int{headerRow, endCol},
			}

			return headers
		}
	}

	return nil
}

// findRowHeaders looks for string cells to the left of the array that could be row headers
func (d *ArrayDetectorV2) findRowHeaders(arr *model.InferredArray) []string {
	sheetCells := d.cells[arr.RangeRef.Sheet]
	if sheetCells == nil {
		return nil
	}

	startRow := arr.RangeRef.TopLeft[0]
	endRow := arr.RangeRef.BottomRight[0]
	startCol := arr.RangeRef.TopLeft[1]

	// Look up to 2 columns to the left for headers
	for offset := 1; offset <= 2; offset++ {
		headerCol := startCol - offset
		if headerCol < 0 {
			break
		}

		headers := []string{}
		allStrings := true

		for row := startRow; row <= endRow; row++ {
			key := fmt.Sprintf("%d,%d", row, headerCol)
			cell := sheetCells[key]

			if cell == nil || cell.Type != CellTypeString {
				allStrings = false
				break
			}

			headers = append(headers, fmt.Sprintf("%v", cell.Value))
		}

		if allStrings && len(headers) > 0 {
			// Mark header cells as assigned
			for row := startRow; row <= endRow; row++ {
				globalKey := fmt.Sprintf("%s!%d,%d", arr.RangeRef.Sheet, row, headerCol)
				d.assigned[globalKey] = true
			}

			arr.RowHeaderSource = &model.RangeRef{
				Sheet:       arr.RangeRef.Sheet,
				TopLeft:     [2]int{startRow, headerCol},
				BottomRight: [2]int{endRow, headerCol},
			}

			return headers
		}
	}

	return nil
}

// Phase 4: Gather remaining unassigned cells into arrays
func (d *ArrayDetectorV2) gatherRemainingCells() []*model.InferredArray {
	var arrays []*model.InferredArray

	for sheetName, sheetCells := range d.cells {
		minRow, maxRow, minCol, maxCol := d.getSheetBounds(sheetCells)
		if minRow > maxRow {
			continue
		}

		// Process remaining string cells column-first
		for col := minCol; col <= maxCol; col++ {
			runs := d.findStringRuns(sheetName, col, minRow, maxRow)

			for _, run := range runs {
				array := d.expandStringRunToArray(sheetName, run, col, maxCol)
				if array != nil {
					arrays = append(arrays, array)
				}
			}
		}
	}

	return arrays
}

// findStringRuns finds all maximal vertical runs of string cells in a column
func (d *ArrayDetectorV2) findStringRuns(sheet string, col, minRow, maxRow int) []VerticalRun {
	var runs []VerticalRun
	sheetCells := d.cells[sheet]

	var currentRun *VerticalRun

	for row := minRow; row <= maxRow; row++ {
		key := fmt.Sprintf("%d,%d", row, col)
		globalKey := fmt.Sprintf("%s!%d,%d", sheet, row, col)

		cell := sheetCells[key]

		// Check if this cell is a string and not already assigned
		isString := cell != nil && cell.Type == CellTypeString && !d.assigned[globalKey]

		if isString {
			if currentRun == nil {
				currentRun = &VerticalRun{
					StartRow: row,
					EndRow:   row,
				}
			} else {
				currentRun.EndRow = row
			}
		} else {
			if currentRun != nil {
				runs = append(runs, *currentRun)
				currentRun = nil
			}
		}
	}

	if currentRun != nil {
		runs = append(runs, *currentRun)
	}

	return runs
}

// expandStringRunToArray expands a string run horizontally
func (d *ArrayDetectorV2) expandStringRunToArray(sheet string, run VerticalRun, startCol, maxCol int) *model.InferredArray {
	sheetCells := d.cells[sheet]

	endCol := startCol

	// Try to expand right
	for col := startCol + 1; col <= maxCol; col++ {
		allMatch := true
		for row := run.StartRow; row <= run.EndRow; row++ {
			key := fmt.Sprintf("%d,%d", row, col)
			globalKey := fmt.Sprintf("%s!%d,%d", sheet, row, col)

			cell := sheetCells[key]
			if cell == nil || cell.Type != CellTypeString || d.assigned[globalKey] {
				allMatch = false
				break
			}
		}

		if allMatch {
			endCol = col
		} else {
			break
		}
	}

	// Mark cells as assigned
	for row := run.StartRow; row <= run.EndRow; row++ {
		for col := startCol; col <= endCol; col++ {
			globalKey := fmt.Sprintf("%s!%d,%d", sheet, row, col)
			d.assigned[globalKey] = true
		}
	}

	// Create the array
	d.arrayCounter++
	array := &model.InferredArray{
		ID: fmt.Sprintf("arr_%03d", d.arrayCounter),
		RangeRef: model.RangeRef{
			Sheet:       sheet,
			TopLeft:     [2]int{run.StartRow, startCol},
			BottomRight: [2]int{run.EndRow, endCol},
		},
		HasFormulas: false,
		DType:       "str",
		DataRole:    model.RoleLabel,
	}

	// Determine orientation
	rows := run.EndRow - run.StartRow + 1
	cols := endCol - startCol + 1
	if rows == 1 && cols > 1 {
		array.Orientation = model.OrientationRowVector
	} else if cols == 1 && rows > 1 {
		array.Orientation = model.OrientationColVector
	} else {
		array.Orientation = model.OrientationMatrix
	}

	return array
}

// buildFormulaTemplates extracts formula patterns for arrays with formulas
func (d *ArrayDetectorV2) buildFormulaTemplates(arrays []*model.InferredArray) {
	for _, arr := range arrays {
		if !arr.HasFormulas {
			continue
		}
		arr.FormulaTemplate = d.extractFormulaTemplate(arr)
	}
}

// extractFormulaTemplate builds a formula template for an array
func (d *ArrayDetectorV2) extractFormulaTemplate(arr *model.InferredArray) *model.FormulaTemplate {
	sheet := arr.RangeRef.Sheet
	startRow := arr.RangeRef.TopLeft[0]
	startCol := arr.RangeRef.TopLeft[1]
	endRow := arr.RangeRef.BottomRight[0]
	endCol := arr.RangeRef.BottomRight[1]

	// Get the first formula cell as template basis
	firstKey := fmt.Sprintf("%s!%d,%d", sheet, startRow, startCol)
	firstParsed := d.parsedFormulas[firstKey]
	if firstParsed == nil {
		return nil
	}

	// Get the raw cell to get the formula string
	rawSheet := d.workbook.Sheets[sheet]
	if rawSheet == nil {
		return nil
	}
	firstCell := rawSheet.Cells[fmt.Sprintf("%d,%d", startRow, startCol)]
	if firstCell == nil || firstCell.Formula == "" {
		return nil
	}

	template := &model.FormulaTemplate{
		TemplateStr:      firstCell.Formula,
		FixedRefs:        make(map[string]model.CellRef),
		RelativePatterns: make(map[string]model.RelativePattern),
		Exceptions:       make(map[string]string),
		Coverage:         1.0,
	}

	// Analyze reference patterns by comparing first cell to others
	for i, ref := range firstParsed.References {
		refName := fmt.Sprintf("ref_%d", i)

		// Check if reference is fixed or relative by sampling other cells
		isRowFixed := true
		isColFixed := true

		// Sample second row if exists
		if endRow > startRow {
			key := fmt.Sprintf("%s!%d,%d", sheet, startRow+1, startCol)
			if parsed := d.parsedFormulas[key]; parsed != nil && len(parsed.References) > i {
				ref2 := parsed.References[i]
				if ref2.Row != ref.Row {
					isRowFixed = false
				}
				if ref2.Col != ref.Col {
					isColFixed = false
				}
			}
		}

		// Sample second column if exists
		if endCol > startCol {
			key := fmt.Sprintf("%s!%d,%d", sheet, startRow, startCol+1)
			if parsed := d.parsedFormulas[key]; parsed != nil && len(parsed.References) > i {
				ref2 := parsed.References[i]
				if ref2.Row != ref.Row {
					isRowFixed = false
				}
				if ref2.Col != ref.Col {
					isColFixed = false
				}
			}
		}

		if isRowFixed && isColFixed {
			template.FixedRefs[refName] = ref
		} else {
			template.RelativePatterns[refName] = model.RelativePattern{
				BaseOffset: [2]int{ref.Row - startRow, ref.Col - startCol},
				RowDelta:   boolToIntV2(!isRowFixed),
				ColDelta:   boolToIntV2(!isColFixed),
			}
		}
	}

	// Count exceptions (cells that don't match the pattern)
	totalCells := (endRow - startRow + 1) * (endCol - startCol + 1)
	matchingCells := 0

	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			if row == startRow && col == startCol {
				matchingCells++
				continue
			}

			if d.formulasCompatibleV2(sheet, startRow, startCol, row, col) {
				matchingCells++
			} else {
				cell := rawSheet.Cells[fmt.Sprintf("%d,%d", row, col)]
				if cell != nil && cell.Formula != "" {
					posKey := fmt.Sprintf("%d,%d", row-startRow, col-startCol)
					template.Exceptions[posKey] = cell.Formula
				}
			}
		}
	}

	template.Coverage = float64(matchingCells) / float64(totalCells)

	return template
}

// formulasCompatibleV2 checks if two cells have compatible formulas
// Uses absolute/relative reference information for accurate congruence checking
func (d *ArrayDetectorV2) formulasCompatibleV2(sheet string, baseRow, baseCol, targetRow, targetCol int) bool {
	baseKey := fmt.Sprintf("%s!%d,%d", sheet, baseRow, baseCol)
	targetKey := fmt.Sprintf("%s!%d,%d", sheet, targetRow, targetCol)

	baseParsed := d.parsedFormulas[baseKey]
	targetParsed := d.parsedFormulas[targetKey]

	if baseParsed == nil || targetParsed == nil {
		return baseParsed == targetParsed
	}

	// Check if same functions are used
	if len(baseParsed.Functions) != len(targetParsed.Functions) {
		return false
	}
	for i, fn := range baseParsed.Functions {
		if i >= len(targetParsed.Functions) || targetParsed.Functions[i] != fn {
			return false
		}
	}

	// Check if same number of references (use FormulaRefs for absolute info)
	if len(baseParsed.FormulaRefs) != len(targetParsed.FormulaRefs) {
		return false
	}

	// Check if references follow consistent offset pattern based on absolute/relative markers
	rowDiff := targetRow - baseRow
	colDiff := targetCol - baseCol

	for i, baseRef := range baseParsed.FormulaRefs {
		targetRef := targetParsed.FormulaRefs[i]

		// Check absolute/relative markers match
		if baseRef.RowAbsolute != targetRef.RowAbsolute || baseRef.ColAbsolute != targetRef.ColAbsolute {
			return false
		}

		rowRefDiff := targetRef.Row - baseRef.Row
		colRefDiff := targetRef.Col - baseRef.Col

		// For absolute references ($), the position should stay the same
		// For relative references, the position should move with the cell
		if baseRef.RowAbsolute {
			// Absolute row: should not change
			if rowRefDiff != 0 {
				return false
			}
		} else {
			// Relative row: should move by cell row offset
			if rowRefDiff != rowDiff {
				return false
			}
		}

		if baseRef.ColAbsolute {
			// Absolute column: should not change
			if colRefDiff != 0 {
				return false
			}
		} else {
			// Relative column: should move by cell column offset
			if colRefDiff != colDiff {
				return false
			}
		}
	}

	// Also check range references for consistency
	if len(baseParsed.FormulaRanges) != len(targetParsed.FormulaRanges) {
		return false
	}

	for i, baseRange := range baseParsed.FormulaRanges {
		targetRange := targetParsed.FormulaRanges[i]

		// Check absolute markers match
		if baseRange.TopRowAbsolute != targetRange.TopRowAbsolute ||
			baseRange.TopColAbsolute != targetRange.TopColAbsolute ||
			baseRange.BotRowAbsolute != targetRange.BotRowAbsolute ||
			baseRange.BotColAbsolute != targetRange.BotColAbsolute {
			return false
		}

		// Check top-left corner
		if baseRange.TopRowAbsolute {
			if targetRange.TopLeft[0] != baseRange.TopLeft[0] {
				return false
			}
		} else {
			if targetRange.TopLeft[0]-baseRange.TopLeft[0] != rowDiff {
				return false
			}
		}

		if baseRange.TopColAbsolute {
			if targetRange.TopLeft[1] != baseRange.TopLeft[1] {
				return false
			}
		} else {
			if targetRange.TopLeft[1]-baseRange.TopLeft[1] != colDiff {
				return false
			}
		}

		// Check bottom-right corner
		if baseRange.BotRowAbsolute {
			if targetRange.BottomRight[0] != baseRange.BottomRight[0] {
				return false
			}
		} else {
			if targetRange.BottomRight[0]-baseRange.BottomRight[0] != rowDiff {
				return false
			}
		}

		if baseRange.BotColAbsolute {
			if targetRange.BottomRight[1] != baseRange.BottomRight[1] {
				return false
			}
		} else {
			if targetRange.BottomRight[1]-baseRange.BottomRight[1] != colDiff {
				return false
			}
		}
	}

	return true
}

func boolToIntV2(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Phase 5: Resolve cell references in formula templates to array references
func (d *ArrayDetectorV2) resolveArrayReferences(arrays []*model.InferredArray) {
	// Build a map from cell position to array ID for quick lookup
	cellToArray := make(map[string]string) // "sheet!row,col" -> array ID

	for _, arr := range arrays {
		for row := arr.RangeRef.TopLeft[0]; row <= arr.RangeRef.BottomRight[0]; row++ {
			for col := arr.RangeRef.TopLeft[1]; col <= arr.RangeRef.BottomRight[1]; col++ {
				key := fmt.Sprintf("%s!%d,%d", arr.RangeRef.Sheet, row, col)
				cellToArray[key] = arr.ID
			}
		}
	}

	// Create a map for quick array lookup by ID
	arrayByID := make(map[string]*model.InferredArray)
	for _, arr := range arrays {
		arrayByID[arr.ID] = arr
	}

	// Now resolve references in each array's formula template
	for _, arr := range arrays {
		if arr.FormulaTemplate == nil {
			continue
		}

		// Resolve relative patterns to array references
		for refName, pattern := range arr.FormulaTemplate.RelativePatterns {
			// Calculate the actual cell position for the first cell of this array
			refRow := arr.RangeRef.TopLeft[0] + pattern.BaseOffset[0]
			refCol := arr.RangeRef.TopLeft[1] + pattern.BaseOffset[1]
			refKey := fmt.Sprintf("%s!%d,%d", arr.RangeRef.Sheet, refRow, refCol)

			if targetArrayID, ok := cellToArray[refKey]; ok {
				pattern.TargetArrayID = targetArrayID

				// Determine indexing pattern
				targetArr := arrayByID[targetArrayID]
				if targetArr != nil {
					// Calculate how the reference indexes into the target array
					pattern.RowIndexing = d.calculateIndexing(
						pattern.RowDelta,
						refRow-targetArr.RangeRef.TopLeft[0],
						arr.RangeRef.TopLeft[0]-targetArr.RangeRef.TopLeft[0],
					)
					pattern.ColIndexing = d.calculateIndexing(
						pattern.ColDelta,
						refCol-targetArr.RangeRef.TopLeft[1],
						arr.RangeRef.TopLeft[1]-targetArr.RangeRef.TopLeft[1],
					)
				}

				// Update the pattern in the map
				arr.FormulaTemplate.RelativePatterns[refName] = pattern
			}
		}

		// Resolve fixed refs to array references
		if len(arr.FormulaTemplate.FixedRefs) > 0 {
			arr.FormulaTemplate.ResolvedFixed = make(map[string]model.FixedRefResolved)

			for refName, cellRef := range arr.FormulaTemplate.FixedRefs {
				refKey := fmt.Sprintf("%s!%d,%d", cellRef.Sheet, cellRef.Row, cellRef.Col)

				resolved := model.FixedRefResolved{CellRef: cellRef}

				if targetArrayID, ok := cellToArray[refKey]; ok {
					resolved.TargetArrayID = targetArrayID

					targetArr := arrayByID[targetArrayID]
					if targetArr != nil {
						resolved.ArrayRow = cellRef.Row - targetArr.RangeRef.TopLeft[0]
						resolved.ArrayCol = cellRef.Col - targetArr.RangeRef.TopLeft[1]
					}
				}

				arr.FormulaTemplate.ResolvedFixed[refName] = resolved
			}
		}
	}
}

// calculateIndexing determines how a reference indexes into a target array
func (d *ArrayDetectorV2) calculateIndexing(delta int, baseOffset int, sourceArrayOffset int) string {
	if delta == 0 {
		// Fixed index within the target array
		return fmt.Sprintf("fixed:%d", baseOffset)
	} else if delta == 1 {
		// Moves with the source array
		if sourceArrayOffset == 0 {
			return "same"
		}
		return fmt.Sprintf("same%+d", -sourceArrayOffset)
	} else {
		// More complex pattern
		return fmt.Sprintf("delta:%d,base:%d", delta, baseOffset)
	}
}
