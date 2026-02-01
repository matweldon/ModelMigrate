package inference

import (
	"fmt"
	"sort"
	"strings"

	"github.com/matweldon/modelmigrate/parser/pkg/model"
	"github.com/matweldon/modelmigrate/parser/pkg/xlsx"
)

// ArrayDetector detects arrays in a workbook.
type ArrayDetector struct {
	workbook       *model.RawWorkbook
	parsedFormulas map[string]*xlsx.ParsedFormula
	arrayCounter   int
}

// NewArrayDetector creates a new array detector.
func NewArrayDetector(workbook *model.RawWorkbook, parsedFormulas map[string]*xlsx.ParsedFormula) *ArrayDetector {
	return &ArrayDetector{
		workbook:       workbook,
		parsedFormulas: parsedFormulas,
		arrayCounter:   0,
	}
}

// DetectArrays finds all arrays in the workbook.
func (d *ArrayDetector) DetectArrays() []*model.InferredArray {
	var arrays []*model.InferredArray

	for sheetName, sheet := range d.workbook.Sheets {
		sheetArrays := d.detectSheetArrays(sheetName, sheet)
		arrays = append(arrays, sheetArrays...)
	}

	return arrays
}

// detectSheetArrays finds arrays in a single sheet.
func (d *ArrayDetector) detectSheetArrays(sheetName string, sheet *model.RawSheet) []*model.InferredArray {
	var arrays []*model.InferredArray

	// Get sheet bounds
	bounds := sheet.Bounds()
	if bounds.MaxRow < 0 || bounds.MaxCol < 0 {
		return arrays
	}

	// Track which cells have been assigned to arrays
	assigned := make(map[string]bool)

	// Find contiguous regions of formula cells
	for row := bounds.MinRow; row <= bounds.MaxRow; row++ {
		for col := bounds.MinCol; col <= bounds.MaxCol; col++ {
			cellKey := fmt.Sprintf("%d,%d", row, col)
			if assigned[cellKey] {
				continue
			}

			cell := sheet.Cells[cellKey]
			if cell == nil {
				continue
			}

			// Try to grow an array from this cell
			array := d.growArray(sheetName, sheet, row, col, assigned)
			if array != nil {
				arrays = append(arrays, array)
			}
		}
	}

	return arrays
}

// growArray tries to grow an array starting from (row, col).
// Returns nil if the cell doesn't form a meaningful array.
func (d *ArrayDetector) growArray(sheetName string, sheet *model.RawSheet, startRow, startCol int, assigned map[string]bool) *model.InferredArray {
	startKey := fmt.Sprintf("%d,%d", startRow, startCol)
	startCell := sheet.Cells[startKey]
	if startCell == nil {
		return nil
	}

	// Determine if this is a formula cell or value cell
	hasFormula := startCell.Formula != ""

	// Try to expand right
	endCol := startCol
	for col := startCol + 1; col <= sheet.Bounds().MaxCol; col++ {
		cellKey := fmt.Sprintf("%d,%d", startRow, col)
		if assigned[cellKey] {
			break
		}
		cell := sheet.Cells[cellKey]
		if cell == nil {
			break
		}
		// Check if cell is compatible (same formula pattern or same value type)
		if hasFormula {
			if cell.Formula == "" {
				break
			}
			if !d.formulasCompatible(sheetName, startRow, startCol, startRow, col) {
				break
			}
		} else {
			if cell.Formula != "" {
				break
			}
			if cell.Type != startCell.Type {
				break
			}
		}
		endCol = col
	}

	// Try to expand down
	endRow := startRow
	for row := startRow + 1; row <= sheet.Bounds().MaxRow; row++ {
		// Check if entire row matches the pattern
		rowMatches := true
		for col := startCol; col <= endCol; col++ {
			cellKey := fmt.Sprintf("%d,%d", row, col)
			if assigned[cellKey] {
				rowMatches = false
				break
			}
			cell := sheet.Cells[cellKey]
			if cell == nil {
				rowMatches = false
				break
			}
			if hasFormula {
				if cell.Formula == "" {
					rowMatches = false
					break
				}
				// Compare with cell above
				if !d.formulasCompatible(sheetName, row-1, col, row, col) {
					rowMatches = false
					break
				}
			} else {
				if cell.Formula != "" {
					rowMatches = false
					break
				}
				refCell := sheet.Cells[fmt.Sprintf("%d,%d", startRow, col)]
				if refCell == nil || cell.Type != refCell.Type {
					rowMatches = false
					break
				}
			}
		}
		if !rowMatches {
			break
		}
		endRow = row
	}

	// Calculate dimensions
	numRows := endRow - startRow + 1
	numCols := endCol - startCol + 1

	// Skip single cells unless they're scalars we want to track
	if numRows == 1 && numCols == 1 {
		// Mark as assigned so we don't revisit, but don't create array
		assigned[startKey] = true
		return nil
	}

	// Skip very small regions that aren't meaningful
	if numRows*numCols < 2 {
		return nil
	}

	// Mark all cells as assigned
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			assigned[fmt.Sprintf("%d,%d", row, col)] = true
		}
	}

	// Create the array
	d.arrayCounter++
	array := &model.InferredArray{
		ID: fmt.Sprintf("arr_%03d", d.arrayCounter),
		RangeRef: model.RangeRef{
			Sheet:       sheetName,
			TopLeft:     [2]int{startRow, startCol},
			BottomRight: [2]int{endRow, endCol},
		},
		Orientation: d.detectOrientation(numRows, numCols),
		HasFormulas: hasFormula,
		DType:       d.detectDType(sheet, startRow, startCol, endRow, endCol),
		DataRole:    model.RoleIntermediate, // Will be refined later
	}

	// Detect headers
	d.detectHeaders(array, sheet)

	// Build formula template if this is a formula array
	if hasFormula {
		array.FormulaTemplate = d.buildFormulaTemplate(sheetName, sheet, startRow, startCol, endRow, endCol)
	}

	return array
}

// formulasCompatible checks if two formula cells have compatible patterns.
// Two formulas are compatible if they differ only by predictable offsets.
func (d *ArrayDetector) formulasCompatible(sheet string, row1, col1, row2, col2 int) bool {
	key1 := fmt.Sprintf("%s!%d,%d", sheet, row1, col1)
	key2 := fmt.Sprintf("%s!%d,%d", sheet, row2, col2)

	parsed1 := d.parsedFormulas[key1]
	parsed2 := d.parsedFormulas[key2]

	if parsed1 == nil || parsed2 == nil {
		return false
	}

	// Check if same functions are used
	if len(parsed1.Functions) != len(parsed2.Functions) {
		return false
	}
	for i, fn := range parsed1.Functions {
		if fn != parsed2.Functions[i] {
			return false
		}
	}

	// Check if same number of references
	if len(parsed1.References) != len(parsed2.References) {
		return false
	}

	// Check if references move predictably
	rowDiff := row2 - row1
	colDiff := col2 - col1

	for i, ref1 := range parsed1.References {
		ref2 := parsed2.References[i]

		// References should be to the same sheet
		if ref1.Sheet != ref2.Sheet {
			return false
		}

		// Calculate expected offset
		actualRowDiff := ref2.Row - ref1.Row
		actualColDiff := ref2.Col - ref1.Col

		// Reference should either move with the formula or stay fixed
		validRowMove := actualRowDiff == rowDiff || actualRowDiff == 0
		validColMove := actualColDiff == colDiff || actualColDiff == 0

		if !validRowMove || !validColMove {
			return false
		}
	}

	return true
}

// detectOrientation determines the array orientation.
func (d *ArrayDetector) detectOrientation(numRows, numCols int) model.ArrayOrientation {
	if numRows == 1 {
		return model.OrientationRowVector
	}
	if numCols == 1 {
		return model.OrientationColVector
	}
	// Could add heuristics for time series detection here
	return model.OrientationMatrix
}

// detectDType determines the predominant data type.
func (d *ArrayDetector) detectDType(sheet *model.RawSheet, startRow, startCol, endRow, endCol int) string {
	typeCounts := make(map[string]int)

	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			cell := sheet.Cells[fmt.Sprintf("%d,%d", row, col)]
			if cell == nil {
				continue
			}
			switch cell.Type {
			case "number", "formula":
				if cell.IsDate() {
					typeCounts["datetime64"]++
				} else {
					typeCounts["float64"]++
				}
			case "string":
				typeCounts["str"]++
			case "boolean":
				typeCounts["bool"]++
			default:
				typeCounts["object"]++
			}
		}
	}

	// Return most common type
	maxCount := 0
	result := "object"
	for dtype, count := range typeCounts {
		if count > maxCount {
			maxCount = count
			result = dtype
		}
	}
	return result
}

// detectHeaders looks for row and column headers adjacent to the array.
func (d *ArrayDetector) detectHeaders(array *model.InferredArray, sheet *model.RawSheet) {
	startRow := array.RangeRef.TopLeft[0]
	startCol := array.RangeRef.TopLeft[1]
	endRow := array.RangeRef.BottomRight[0]
	endCol := array.RangeRef.BottomRight[1]

	// Look for column headers above the array
	if startRow > 0 {
		headers := []string{}
		allText := true
		for col := startCol; col <= endCol; col++ {
			cell := sheet.Cells[fmt.Sprintf("%d,%d", startRow-1, col)]
			if cell == nil || cell.Type != "string" {
				allText = false
				break
			}
			headers = append(headers, fmt.Sprintf("%v", cell.Value))
		}
		if allText && len(headers) > 0 {
			array.ColHeaders = headers
			array.ColHeaderSource = &model.RangeRef{
				Sheet:       array.RangeRef.Sheet,
				TopLeft:     [2]int{startRow - 1, startCol},
				BottomRight: [2]int{startRow - 1, endCol},
			}
		}
	}

	// Look for row headers to the left of the array
	if startCol > 0 {
		headers := []string{}
		allText := true
		for row := startRow; row <= endRow; row++ {
			cell := sheet.Cells[fmt.Sprintf("%d,%d", row, startCol-1)]
			if cell == nil || cell.Type != "string" {
				allText = false
				break
			}
			headers = append(headers, fmt.Sprintf("%v", cell.Value))
		}
		if allText && len(headers) > 0 {
			array.RowHeaders = headers
			array.RowHeaderSource = &model.RangeRef{
				Sheet:       array.RangeRef.Sheet,
				TopLeft:     [2]int{startRow, startCol - 1},
				BottomRight: [2]int{endRow, startCol - 1},
			}
		}
	}
}

// buildFormulaTemplate extracts the formula pattern for an array.
func (d *ArrayDetector) buildFormulaTemplate(sheetName string, sheet *model.RawSheet, startRow, startCol, endRow, endCol int) *model.FormulaTemplate {
	// Get the first formula as the template basis
	firstKey := fmt.Sprintf("%s!%d,%d", sheetName, startRow, startCol)
	firstParsed := d.parsedFormulas[firstKey]
	if firstParsed == nil {
		return nil
	}

	firstCell := sheet.Cells[fmt.Sprintf("%d,%d", startRow, startCol)]
	if firstCell == nil {
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
			key := fmt.Sprintf("%s!%d,%d", sheetName, startRow+1, startCol)
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
			key := fmt.Sprintf("%s!%d,%d", sheetName, startRow, startCol+1)
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
				RowDelta:   boolToInt(!isRowFixed),
				ColDelta:   boolToInt(!isColFixed),
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

			if d.formulasCompatible(sheetName, startRow, startCol, row, col) {
				matchingCells++
			} else {
				cell := sheet.Cells[fmt.Sprintf("%d,%d", row, col)]
				if cell != nil {
					posKey := fmt.Sprintf("%d,%d", row-startRow, col-startCol)
					template.Exceptions[posKey] = cell.Formula
				}
			}
		}
	}

	template.Coverage = float64(matchingCells) / float64(totalCells)

	return template
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DetectScalars finds scalar values (single cells) that are referenced by formulas.
func DetectScalars(workbook *model.RawWorkbook, graph *model.DependencyGraph, arrays []*model.InferredArray) []*model.InferredScalar {
	var scalars []*model.InferredScalar
	scalarCounter := 0

	// Build set of cells that are in arrays
	inArray := make(map[string]bool)
	for _, arr := range arrays {
		for row := arr.RangeRef.TopLeft[0]; row <= arr.RangeRef.BottomRight[0]; row++ {
			for col := arr.RangeRef.TopLeft[1]; col <= arr.RangeRef.BottomRight[1]; col++ {
				key := fmt.Sprintf("%s!%d,%d", arr.RangeRef.Sheet, row, col)
				inArray[key] = true
			}
		}
	}

	// Find cells that are referenced but not in arrays
	referenced := make(map[string]bool)
	for _, edge := range graph.Edges {
		referenced[edge.Source] = true
	}

	for cellKey := range referenced {
		if inArray[cellKey] {
			continue
		}

		// Parse the cell key
		parts := strings.SplitN(cellKey, "!", 2)
		if len(parts) != 2 {
			continue
		}
		sheetName := parts[0]
		var row, col int
		fmt.Sscanf(parts[1], "%d,%d", &row, &col)

		sheet := workbook.Sheets[sheetName]
		if sheet == nil {
			continue
		}

		localKey := fmt.Sprintf("%d,%d", row, col)
		cell := sheet.Cells[localKey]
		if cell == nil {
			continue
		}

		scalarCounter++
		scalar := &model.InferredScalar{
			ID:      fmt.Sprintf("scl_%03d", scalarCounter),
			CellRef: model.CellRef{Sheet: sheetName, Row: row, Col: col},
			Value:   cell.Value,
			DType:   cellTypeToDType(cell),
			IsDate:  cell.IsDate(),
		}

		if cell.Formula != "" {
			scalar.Formula = cell.Formula
			scalar.DataRole = model.RoleIntermediate
		} else {
			// Check if it's referenced by others
			deps := GetDependents(graph, cellKey)
			if len(deps) > 0 {
				scalar.DataRole = model.RoleParameter
			} else {
				scalar.DataRole = model.RoleInput
			}
		}

		// Try to find label from adjacent cells
		scalar.InferredLabel, scalar.LabelSource = findAdjacentLabel(sheet, row, col, sheetName)

		// Check named ranges
		for name, nr := range workbook.NamedRanges {
			if strings.Contains(nr.RefersTo, fmt.Sprintf("$%s$%d", colIndexToLetters(col), row+1)) {
				scalar.ExcelName = name
				break
			}
		}

		scalars = append(scalars, scalar)
	}

	// Sort by ID for deterministic output
	sort.Slice(scalars, func(i, j int) bool {
		return scalars[i].ID < scalars[j].ID
	})

	return scalars
}

func cellTypeToDType(cell *model.RawCell) string {
	if cell.IsDate() {
		return "datetime64"
	}
	switch cell.Type {
	case "number", "formula":
		return "float64"
	case "string":
		return "str"
	case "boolean":
		return "bool"
	default:
		return "object"
	}
}

func findAdjacentLabel(sheet *model.RawSheet, row, col int, sheetName string) (string, *model.CellRef) {
	// Check cell to the left
	if col > 0 {
		leftKey := fmt.Sprintf("%d,%d", row, col-1)
		if cell := sheet.Cells[leftKey]; cell != nil && cell.Type == "string" {
			return fmt.Sprintf("%v", cell.Value), &model.CellRef{Sheet: sheetName, Row: row, Col: col - 1}
		}
	}

	// Check cell above
	if row > 0 {
		aboveKey := fmt.Sprintf("%d,%d", row-1, col)
		if cell := sheet.Cells[aboveKey]; cell != nil && cell.Type == "string" {
			return fmt.Sprintf("%v", cell.Value), &model.CellRef{Sheet: sheetName, Row: row - 1, Col: col}
		}
	}

	return "", nil
}

func colIndexToLetters(col int) string {
	result := ""
	col++ // Convert to 1-indexed
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}
