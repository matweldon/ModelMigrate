// Package xlsx provides formula parsing for Excel formulas.
package xlsx

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/matweldon/modelmigrate/parser/pkg/model"
)

// ParsedFormula represents a parsed Excel formula with extracted references.
type ParsedFormula struct {
	Original        string                  `json:"original"`
	References      []model.CellRef         `json:"references"`       // Legacy: positions only
	RangeReferences []model.RangeRef        `json:"range_references"` // Legacy: positions only
	FormulaRefs     []model.FormulaRef      `json:"formula_refs"`     // With absolute/relative info
	FormulaRanges   []model.FormulaRangeRef `json:"formula_ranges"`   // With absolute/relative info
	NamedReferences []string                `json:"named_references"`
	Functions       []string                `json:"functions"`
	IsArrayFormula  bool                    `json:"is_array_formula"`
}

// Reference patterns for Excel formulas
var (
	// Matches cell references like A1, $A$1, A$1, $A1, with optional sheet prefix
	// Group 1: sheet name (optional, in quotes or plain)
	// Group 2: column absolute marker ($)
	// Group 3: column letters
	// Group 4: row absolute marker ($)
	// Group 5: row number
	cellRefPattern = regexp.MustCompile(`(?:'([^']+)'!|\b([A-Za-z_][A-Za-z0-9_]*!))?(\$?)([A-Z]{1,3})(\$?)([1-9][0-9]*)`)

	// Matches range references like A1:B10, with optional sheet prefix
	rangeRefPattern = regexp.MustCompile(`(?:'([^']+)'!|\b([A-Za-z_][A-Za-z0-9_]*!))?(\$?)([A-Z]{1,3})(\$?)([1-9][0-9]*):(\$?)([A-Z]{1,3})(\$?)([1-9][0-9]*)`)

	// Matches function names like SUM, VLOOKUP, IF
	functionPattern = regexp.MustCompile(`\b([A-Z][A-Z0-9_.]*)\s*\(`)

	// Matches named ranges (identifiers that aren't cell refs or functions)
	namedRefPattern = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\b`)

	// Known Excel functions to exclude from named references
	excelFunctions = map[string]bool{
		"ABS": true, "ACOS": true, "AND": true, "AVERAGE": true, "AVERAGEIF": true,
		"CEILING": true, "CHOOSE": true, "COLUMN": true, "COLUMNS": true, "CONCAT": true,
		"COUNT": true, "COUNTA": true, "COUNTBLANK": true, "COUNTIF": true, "COUNTIFS": true,
		"DATE": true, "DAY": true, "EDATE": true, "EOMONTH": true,
		"EXP": true, "FALSE": true, "FLOOR": true, "FV": true,
		"HLOOKUP": true, "IF": true, "IFERROR": true, "IFNA": true, "IFS": true,
		"INDEX": true, "INDIRECT": true, "INT": true, "IPMT": true, "IRR": true, "ISBLANK": true,
		"ISERROR": true, "ISNA": true, "ISNUMBER": true, "ISTEXT": true,
		"LEFT": true, "LEN": true, "LN": true, "LOG": true, "LOG10": true, "LOOKUP": true, "LOWER": true,
		"MATCH": true, "MAX": true, "MID": true, "MIN": true, "MOD": true, "MONTH": true,
		"NOT": true, "NOW": true, "NPV": true,
		"OFFSET": true, "OR": true,
		"PI": true, "PMT": true, "POWER": true, "PPMT": true, "PV": true,
		"RAND": true, "RANDBETWEEN": true, "RATE": true, "RIGHT": true, "ROUND": true, "ROUNDDOWN": true, "ROUNDUP": true, "ROW": true, "ROWS": true,
		"SQRT": true, "STDEV": true, "SUBSTITUTE": true, "SUM": true, "SUMIF": true, "SUMIFS": true, "SUMPRODUCT": true,
		"TEXT": true, "TODAY": true, "TRANSPOSE": true, "TRIM": true, "TRUE": true,
		"UPPER": true,
		"VALUE": true, "VLOOKUP": true,
		"WEEKDAY": true,
		"XIRR": true, "XNPV": true,
		"YEAR": true,
	}
)

// ParseFormula parses an Excel formula and extracts references.
func ParseFormula(formula string, currentSheet string) *ParsedFormula {
	if formula == "" {
		return nil
	}

	parsed := &ParsedFormula{
		Original:        formula,
		References:      []model.CellRef{},
		RangeReferences: []model.RangeRef{},
		FormulaRefs:     []model.FormulaRef{},
		FormulaRanges:   []model.FormulaRangeRef{},
		NamedReferences: []string{},
		Functions:       []string{},
		IsArrayFormula:  strings.HasPrefix(formula, "{") && strings.HasSuffix(formula, "}"),
	}

	// Extract functions
	funcMatches := functionPattern.FindAllStringSubmatch(formula, -1)
	funcSet := make(map[string]bool)
	for _, match := range funcMatches {
		fn := strings.ToUpper(match[1])
		if !funcSet[fn] {
			funcSet[fn] = true
			parsed.Functions = append(parsed.Functions, fn)
		}
	}

	// Extract range references first (so we can exclude them from cell refs)
	rangePositions := make(map[int]int) // start -> end positions of range matches
	rangeMatches := rangeRefPattern.FindAllStringSubmatchIndex(formula, -1)
	for _, match := range rangeMatches {
		rangePositions[match[0]] = match[1]
		formulaRangeRef := extractFormulaRangeRef(formula, match, currentSheet)
		if formulaRangeRef != nil {
			parsed.FormulaRanges = append(parsed.FormulaRanges, *formulaRangeRef)
			parsed.RangeReferences = append(parsed.RangeReferences, formulaRangeRef.ToRangeRef())
		}
	}

	// Extract cell references (excluding those that are part of ranges)
	cellMatches := cellRefPattern.FindAllStringSubmatchIndex(formula, -1)
	for _, match := range cellMatches {
		// Skip if this is part of a range reference
		inRange := false
		for start, end := range rangePositions {
			if match[0] >= start && match[1] <= end {
				inRange = true
				break
			}
		}
		if inRange {
			continue
		}

		formulaRef := extractFormulaRef(formula, match, currentSheet)
		if formulaRef != nil {
			parsed.FormulaRefs = append(parsed.FormulaRefs, *formulaRef)
			parsed.References = append(parsed.References, formulaRef.ToCellRef())
		}
	}

	// Extract named references (identifiers that aren't cell refs, functions, or sheet names)
	namedMatches := namedRefPattern.FindAllStringSubmatch(formula, -1)
	namedSet := make(map[string]bool)
	for _, match := range namedMatches {
		name := match[1]
		upperName := strings.ToUpper(name)

		// Skip if it's a function
		if excelFunctions[upperName] {
			continue
		}

		// Skip if it looks like a cell reference (1-3 letters followed by digits)
		if isCellRefLike(name) {
			continue
		}

		// Skip common Excel constants
		if upperName == "TRUE" || upperName == "FALSE" {
			continue
		}

		if !namedSet[name] {
			namedSet[name] = true
			parsed.NamedReferences = append(parsed.NamedReferences, name)
		}
	}

	return parsed
}

// extractCellRef extracts a CellRef from regex match indices (legacy, no absolute info)
func extractCellRef(formula string, match []int, currentSheet string) *model.CellRef {
	ref := extractFormulaRef(formula, match, currentSheet)
	if ref == nil {
		return nil
	}
	cellRef := ref.ToCellRef()
	return &cellRef
}

// extractFormulaRef extracts a FormulaRef from regex match indices, including absolute markers
func extractFormulaRef(formula string, match []int, currentSheet string) *model.FormulaRef {
	// Match groups: 0-1: full match, 2-3: quoted sheet, 4-5: plain sheet, 6-7: col$, 8-9: col, 10-11: row$, 12-13: row
	if len(match) < 14 {
		return nil
	}

	// Determine sheet name
	sheet := currentSheet
	if match[2] != -1 && match[3] != -1 {
		// Quoted sheet name
		sheet = formula[match[2]:match[3]]
	} else if match[4] != -1 && match[5] != -1 {
		// Plain sheet name (remove trailing !)
		sheet = strings.TrimSuffix(formula[match[4]:match[5]], "!")
	}

	// Check for column absolute marker ($)
	colAbsolute := false
	if match[6] != -1 && match[7] != -1 && match[7] > match[6] {
		colAbsolute = formula[match[6]:match[7]] == "$"
	}

	// Extract column letters
	if match[8] == -1 || match[9] == -1 {
		return nil
	}
	colStr := formula[match[8]:match[9]]
	col := colLettersToIndex(colStr)

	// Check for row absolute marker ($)
	rowAbsolute := false
	if match[10] != -1 && match[11] != -1 && match[11] > match[10] {
		rowAbsolute = formula[match[10]:match[11]] == "$"
	}

	// Extract row number
	if match[12] == -1 || match[13] == -1 {
		return nil
	}
	rowStr := formula[match[12]:match[13]]
	row, err := strconv.Atoi(rowStr)
	if err != nil {
		return nil
	}
	row-- // Convert to 0-indexed

	return &model.FormulaRef{
		Sheet:       sheet,
		Row:         row,
		Col:         col,
		RowAbsolute: rowAbsolute,
		ColAbsolute: colAbsolute,
	}
}

// extractRangeRef extracts a RangeRef from regex match indices (legacy, no absolute info)
func extractRangeRef(formula string, match []int, currentSheet string) *model.RangeRef {
	ref := extractFormulaRangeRef(formula, match, currentSheet)
	if ref == nil {
		return nil
	}
	rangeRef := ref.ToRangeRef()
	return &rangeRef
}

// extractFormulaRangeRef extracts a FormulaRangeRef from regex match indices, including absolute markers
func extractFormulaRangeRef(formula string, match []int, currentSheet string) *model.FormulaRangeRef {
	// Match groups for range: 0-1: full, 2-3: quoted sheet, 4-5: plain sheet,
	// 6-7: col1$, 8-9: col1, 10-11: row1$, 12-13: row1,
	// 14-15: col2$, 16-17: col2, 18-19: row2$, 20-21: row2
	if len(match) < 22 {
		return nil
	}

	// Determine sheet name
	sheet := currentSheet
	if match[2] != -1 && match[3] != -1 {
		sheet = formula[match[2]:match[3]]
	} else if match[4] != -1 && match[5] != -1 {
		sheet = strings.TrimSuffix(formula[match[4]:match[5]], "!")
	}

	// First cell absolute markers
	topColAbsolute := false
	if match[6] != -1 && match[7] != -1 && match[7] > match[6] {
		topColAbsolute = formula[match[6]:match[7]] == "$"
	}
	topRowAbsolute := false
	if match[10] != -1 && match[11] != -1 && match[11] > match[10] {
		topRowAbsolute = formula[match[10]:match[11]] == "$"
	}

	// Extract first cell (col1, row1)
	if match[8] == -1 || match[12] == -1 {
		return nil
	}
	col1Str := formula[match[8]:match[9]]
	row1Str := formula[match[12]:match[13]]
	col1 := colLettersToIndex(col1Str)
	row1, _ := strconv.Atoi(row1Str)
	row1--

	// Second cell absolute markers
	botColAbsolute := false
	if match[14] != -1 && match[15] != -1 && match[15] > match[14] {
		botColAbsolute = formula[match[14]:match[15]] == "$"
	}
	botRowAbsolute := false
	if match[18] != -1 && match[19] != -1 && match[19] > match[18] {
		botRowAbsolute = formula[match[18]:match[19]] == "$"
	}

	// Extract second cell (col2, row2)
	if match[16] == -1 || match[20] == -1 {
		return nil
	}
	col2Str := formula[match[16]:match[17]]
	row2Str := formula[match[20]:match[21]]
	col2 := colLettersToIndex(col2Str)
	row2, _ := strconv.Atoi(row2Str)
	row2--

	return &model.FormulaRangeRef{
		Sheet:          sheet,
		TopLeft:        [2]int{row1, col1},
		BottomRight:    [2]int{row2, col2},
		TopRowAbsolute: topRowAbsolute,
		TopColAbsolute: topColAbsolute,
		BotRowAbsolute: botRowAbsolute,
		BotColAbsolute: botColAbsolute,
	}
}

// colLettersToIndex converts column letters (A, B, ..., Z, AA, AB, ...) to 0-indexed column number
func colLettersToIndex(letters string) int {
	result := 0
	for _, c := range strings.ToUpper(letters) {
		result = result*26 + int(c-'A') + 1
	}
	return result - 1
}

// isCellRefLike returns true if the string looks like a cell reference
func isCellRefLike(s string) bool {
	if len(s) < 2 {
		return false
	}

	// Check if it starts with 1-3 letters and ends with digits
	letters := 0
	for i, c := range s {
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			letters++
			if letters > 3 {
				return false
			}
		} else if c >= '0' && c <= '9' {
			// Rest should be digits
			for _, d := range s[i:] {
				if d < '0' || d > '9' {
					return false
				}
			}
			return letters > 0
		} else {
			return false
		}
	}
	return false
}
