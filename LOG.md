# ModelMigrate Development Log

## 2026-02-01: Session 1 - Project Planning

### Context
Initial planning session to design the implementation approach for ModelMigrate.

### Decisions Made

1. **Parser Language: Go**
   - SPEC.md specifies Go for the deterministic parser
   - Benefits: Fast, compiled, minimal dependencies, good for parsing
   - Will implement xlsx parsing first, then add xls support later

2. **Development Priority: Parser First (Layers 1-2)**
   - Layer 1: Raw extraction (cells, formulas, named ranges)
   - Layer 2: Structural inference (arrays, scalars, dependency graph)
   - AI annotation (Layer 3) comes after parser is solid

3. **Test Workbooks Added**
   - `tag-workbook-valuing-dependent-development-workbook.xlsx` (88KB)
     - Land valuation model, 9 sheets
     - Main calculation: 433 rows × 27 cols
     - Large data arrays (Residential: 329 × 240)
   - `15710-gdpcr_0.xls` (625KB)
     - UK Gas Distribution Price Control Review model
     - 31 sheets: P&L, Tax, Cash Flow, Balance Sheet, RAV calculations
     - Regional breakdowns (Scotland, Southern, Northern, etc.)

4. **Format Priority: xlsx first**
   - xlsx is ZIP of XML files - cleaner to parse
   - xls is binary OLE2 format - more complex, defer

### Implementation Plan

#### Phase 1: Go Parser Foundation
- [ ] Set up Go module (`parser/`)
- [ ] xlsx ZIP extraction and XML parsing
- [ ] Cell extraction (values, formulas, types, formats)
- [ ] Sheet structure parsing
- [ ] Named range extraction
- [ ] JSON output format for intermediate representation

#### Phase 2: Structural Inference
- [ ] Formula parsing (references, operators, functions)
- [ ] Array detection (contiguous regions with consistent patterns)
- [ ] Header detection (adjacent text cells)
- [ ] Dependency graph construction
- [ ] Formula template extraction

#### Phase 3: Integration
- [ ] CLI interface for parser
- [ ] Validation against test workbooks
- [ ] Python harness integration

### Completed This Session

1. **Created Go parser skeleton** (`parser/`)
   - Module: `github.com/matweldon/modelmigrate/parser`
   - Structure: `cmd/parser/`, `pkg/xlsx/`, `pkg/model/`, `pkg/inference/`

2. **Implemented Layer 1 xlsx reader** (`pkg/xlsx/reader.go`)
   - ZIP extraction and XML parsing
   - Shared strings handling
   - Style/number format extraction (for date detection)
   - Cell extraction with values, formulas, types
   - Named range extraction
   - Workbook metadata parsing

3. **Defined data structures** (`pkg/model/`)
   - `raw.go`: CellRef, RangeRef, RawCell, RawSheet, RawWorkbook, NamedRange
   - `structural.go`: InferredArray, InferredScalar, FormulaTemplate, DependencyGraph, etc.

4. **Tested against tag-workbook.xlsx**
   - Successfully parsed 9 sheets, 1523 total cells
   - Formulas correctly extracted (e.g., `H7*I7`, `(F7-G7)*I7`)
   - Named ranges captured (7 total)
   - Number formats preserved for date/currency detection

---

## 2026-02-01: Session 2 - Layer 2 Implementation

### Completed

1. **Formula Parser** (`pkg/xlsx/formulas.go`)
   - Regex-based extraction of cell references (A1, $A$1, Sheet1!A1)
   - Range reference parsing (A1:B10)
   - Function detection (SUM, VLOOKUP, etc.)
   - Named reference extraction
   - Handles quoted sheet names

2. **Dependency Graph** (`pkg/inference/graph.go`)
   - Builds directed graph from formula references
   - Topological sort (Kahn's algorithm) for evaluation order
   - GetDependents/GetDependencies traversal helpers
   - Named range resolution

3. **Array Detection** (`pkg/inference/arrays.go`)
   - Finds contiguous regions with consistent formula patterns
   - Formula compatibility checking (same functions, predictable offsets)
   - Orientation detection (row_vector, col_vector, matrix)
   - Data type inference (float64, str, datetime64)
   - Header detection from adjacent text cells
   - Formula template extraction with fixed/relative pattern classification

4. **CLI Updates** (`cmd/parser/main.go`)
   - Added `-mode` flag: `raw` (Layer 1) or `structural` (Layer 2)
   - Structural summary output with array/scalar counts
   - Coverage statistics

### Test Results

Against `tag-workbook.xlsx`:
- 233 arrays detected
- Formula templates extracted with 100% coverage
- Column/row headers detected from adjacent cells
- Dependency graph: 14 nodes, 14 edges
- Example formula template: `(F7-G7)*I7` with relative patterns

### Example Output

```json
{
  "id": "arr_230",
  "range_ref": {"sheet": "Calculations", "top_left": [6,9], "bottom_right": [7,9]},
  "orientation": "col_vector",
  "col_headers": ["Net private value of development (£'000)"],
  "has_formulas": true,
  "formula_template": {
    "template_str": "(F7-G7)*I7",
    "relative_patterns": {
      "ref_0": {"base_offset": [0,-4], "row_delta": 1, "col_delta": 0}
    },
    "coverage": 1.0
  }
}
```

### Continuation: Data Role Classification

Added `ClassifyDataRoles()` function to analyze dependency graph and classify arrays:

| Role | Description | Count |
|------|-------------|-------|
| INPUT | No formulas, not referenced by formulas | 230 |
| PARAMETER | No formulas, referenced by formulas | 0 |
| INTERMEDIATE | Has formulas, referenced by other formulas | 2 |
| OUTPUT | Has formulas, not referenced | 1 |

**Key finding**: The Calculations sheet has:
- Formula arrays at rows 6-7, cols 9-11 (J7-L8) with formulas like `(F7-G7)*I7`
- These reference cells F7-I8 which are **empty** in the workbook (phantom inputs)
- This is typical of template workbooks waiting for user input

**Array fragmentation**: 233 arrays detected because data sheets (Commercial land values, Industrial land values) have sparse, irregular layouts with many small disjoint regions. This correctly reflects the workbook structure.

### Next Steps
1. Test against the larger gdpcr workbook (requires xls support or conversion)
2. Consider adding formula template merging for related arrays
3. Add "phantom input" detection for referenced but non-existent cells

---

## 2026-02-02: Session 3 - V2 Array Detection Algorithm

### Problem Identified

The v1 algorithm used a greedy "expand right first, then down" approach which caused:
- **Array fragmentation**: 233 arrays when there should be fewer
- **Column data split into rows**: Vertical data columns (like Local Authority names in Residential land values) were being parsed row-by-row instead of recognizing column structure
- **Missed phantom cells**: Cells referenced by formulas but not present in workbook weren't captured

### Solution: 4-Phase Column-First Algorithm

Implemented new `arrays_v2.go` with fundamentally different approach:

**Phase 1: Collect Cell Universe**
- Scan ALL cell references (existing cells + phantom from formula references)
- Split into three groups: numeric, string, unknown

**Phase 2: Build Numeric Arrays (Column-First)**
- Process columns before rows
- Find vertical runs of compatible numeric cells
- Expand horizontally only if compatible
- Prioritizes column structure over row structure

**Phase 3: Assign String Labels**
- Look up to 2 rows/cols away for headers
- Assign row headers (to left) and column headers (above)
- String cells that label arrays aren't duplicated

**Phase 4: Gather Remaining Cells**
- Collect unassigned string/other cells into separate arrays
- Ensures nothing is missed

### Test Results (v2 vs v1)

| Metric | v1 | v2 |
|--------|----|----|
| Arrays detected | 233 | 340 |
| INPUT | 230 | 337 |
| PARAMETER | 0 | 1 |
| INTERMEDIATE | 2 | 2 |
| OUTPUT | 1 | 0 |

**Key improvements**:
- **Phantom cells detected**: arr_117 in Calculations sheet now correctly shows PARAMETER role (2×4 matrix of empty cells referenced by formulas)
- **Headers associated correctly**: Column headers like "Value of land in development use (£'000/ha)" properly linked to arrays
- **Column structure preserved**: Residential land values detected as proper column vectors

### Calculations Sheet Structure (parsed by v2)

```
arr_117 (PARAMETER, 2×4): Input parameters
  - Column headers: ["Value of land in development use", "Value of land in existing use", "Externality Value", "Hecterage"]

arr_118 (INTERMEDIATE, 2×1): Net private value of development
  - Has formulas

arr_120 (INTERMEDIATE, 2×2): Net External impact + Net Social Value
  - Has formulas
```

Dependency graph shows computation flow:
- Cols 5-8 (inputs) → Col 9 (net private value)
- Cols 7-8 (inputs) → Col 10 (external impact)
- Cols 9-10 → Col 11 (net social value)

### Files Changed

- **NEW**: `parser/pkg/inference/arrays_v2.go` (727 lines)
  - `CellInfo` struct with type, format, phantom flag
  - `ArrayDetectorV2` with 4-phase algorithm
  - Column-first numeric array building
  - Label assignment with 2-cell lookahead

- **MODIFIED**: `parser/cmd/parser/main.go`
  - Added `-algo` flag to switch between v1 and v2
  - v2 is now the default

### Data Folder Setup

Added `data/` directory with test workbooks:
- `tag-workbook-valuing-dependent-development-workbook.xlsx` - Land valuation model (9 sheets, formulas)
- `financial-sample.xlsx` - Microsoft Power BI sample (flat data table, no formulas)

### Analysis Notes

The financial-sample.xlsx is a flat data table (701 rows, no formulas) - not ideal for testing computational structure. The tag-workbook remains our primary test case with its calculation sheet and cross-sheet data.

### Pending Tasks for Next Session

1. **Parser improvements**:
   - [ ] Handle formula congruence with absolute refs ($A$1 vs A1)
   - [ ] Improve formula template representation to reference arrays instead of cells
   - [ ] Test stability with different cell orderings (shuffle test)
   - [ ] Find more complex test workbooks with formulas

2. **Downstream prototypes**:
   - [ ] Error detection: Look for common spreadsheet errors
   - [ ] Python transpilation: Convert formulas to Python code
   - [ ] AI annotation layer design

3. **Infrastructure**:
   - [ ] Consider adding more workbook sources
   - [ ] xls format support still out of scope

---
