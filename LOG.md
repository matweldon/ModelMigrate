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
- `tag-workbook-valuing-dependent-development-workbook.xlsx` - Land valuation model (9 sheets, 340 arrays, 6 formula cells)
- `fcerm-appraisal.xlsx` - UK Environment Agency flood/erosion appraisal (8 sheets, 551 arrays, 1,181 formula cells, 86,675 edges)
- `smartsheet-npv-irr.xlsx` - NPV/IRR calculator (3 sheets, 54 arrays, 24 formula cells)
- `babson-bloomberg.xlsx` - DCF valuation model (1 sheet, 64 arrays, 79 formula cells)

### Pending Tasks for Next Session

1. **Parser improvements**:
   - [ ] Handle formula congruence with absolute refs ($A$1 vs A1)
   - [ ] Improve formula template representation to reference arrays instead of cells
   - [ ] Test stability with different cell orderings (shuffle test)
   - [x] Find more complex test workbooks with formulas

2. **Downstream prototypes**:
   - [ ] Error detection: Look for common spreadsheet errors
   - [ ] Python transpilation: Convert formulas to Python code
   - [ ] AI annotation layer design

3. **Infrastructure**:
   - [x] Added more workbook sources (see data/README.md)
   - [ ] xls format support still out of scope

---

## 2026-02-02: Session 4 - Testing & CI/CD Setup

### Completed

1. **Created TODO.md** - Task tracking file for agent sessions
   - Added instructions to AGENTS.md on how to use it

2. **Go Testing Infrastructure**
   - Created `pkg/xlsx/formulas_test.go` - 10 tests for formula parsing
   - Created `pkg/xlsx/reader_test.go` - Integration tests with real xlsx files
   - Created `pkg/model/raw_test.go` - 5 tests for model utilities
   - Created `pkg/inference/graph_test.go` - 7 tests for dependency graph

3. **Test Coverage**:
   | Package | Coverage |
   |---------|----------|
   | pkg/model | 96% |
   | pkg/xlsx | 87% |
   | pkg/inference | 7% |
   | Total | 32% |

4. **GitHub Actions CI/CD** (`.github/workflows/test.yml`)
   - Runs on push to main and PRs
   - Go test with coverage report
   - Coverage threshold check (30% minimum)
   - Codecov integration ready
   - Go build and vet checks

5. **Fixed Formula Template Extraction in v2 Algorithm**
   - Issue: v2 algorithm was detecting arrays with formulas but not populating `FormulaTemplate`
   - Solution: Added `buildFormulaTemplates()`, `extractFormulaTemplate()`, and `formulasCompatibleV2()` functions to `arrays_v2.go`
   - Result: All 257 arrays with formulas now have formula templates

### Parser Output Sample (fcerm-appraisal.xlsx)

```
Structural analysis summary (Layer 2):
  Sheets: 8
  Arrays detected: 551
    - INPUT: 136, PARAMETER: 158, INTERMEDIATE: 119, OUTPUT: 138
  Scalars detected: 5
  Dependency graph: 4786 nodes, 86675 edges
  Coverage: 1181 formula cells, 9920 in arrays, 5 scalars
```

Sample formula templates now working:
- `CONCATENATE("Total PV Costs ",C8)` - Fixed refs
- `(B16+B17+B18)*($C$9)` - Mixed fixed/relative refs (100% coverage)
- `SUM(B23:B33)` - Range reference

### Files Changed

- **NEW**: `TODO.md` - Task tracking
- **NEW**: `.github/workflows/test.yml` - CI/CD pipeline
- **NEW**: `parser/pkg/xlsx/formulas_test.go` - Formula parsing tests
- **NEW**: `parser/pkg/xlsx/reader_test.go` - xlsx reader integration tests
- **NEW**: `parser/pkg/model/raw_test.go` - Model utility tests
- **NEW**: `parser/pkg/inference/graph_test.go` - Dependency graph tests
- **MODIFIED**: `AGENTS.md` - Added Task Tracking and Go sections
- **MODIFIED**: `parser/pkg/inference/arrays_v2.go` - Added formula template extraction

### Next Steps

1. Increase test coverage for inference package (currently 7%)
2. Continue analyzing parser output for improvements
3. Consider error detection prototypes

---

## 2026-02-04: Session 5 - Documentation Clarification & Test Coverage

### Documentation Updates

Updated SPEC.md to clearly articulate core design principles that were previously implicit or scattered:

1. **Parsimony Goal**: Explicitly stated that the parser aims to find the *fewest, largest arrays possible* rather than treating cells individually. The goal is array-to-array computation, not cell-to-cell.

2. **Formula Congruence**: Added clear definition - two formulas are congruent if they have:
   - Same operators and functions
   - Same number of references
   - References that either stay fixed (absolute `$A$1`) or move consistently (relative `A1`)

   Examples added to illustrate congruent vs non-congruent formulas.

3. **Algorithm Phases**: Documented the 4-phase "bag" algorithm:
   - Phase 1: Collect cell universe (including phantom cells from formula refs)
   - Phase 2: Build numeric arrays column-first
   - Phase 3: Assign string labels to numeric arrays
   - Phase 4: Gather remaining cells

4. **Array-to-Array Templates**: Clarified that formula templates should eventually express array-level operations, not just cell patterns.

### Test Coverage Improvements

Increased inference package test coverage from 56% to 64%:

| Function | Before | After |
|----------|--------|-------|
| containsString | 0% | 100% |
| inferPhantomType | 0% | 100% |
| computeFormulaHash | 88% | 100% |
| resolveNamedRange | 0% | covered |
| formulasCompatibleV2 | 70% | covered |

New tests added:
- `TestContainsString` - case-insensitive string matching
- `TestInferPhantomType` - numeric vs string type inference
- `TestArrayDetectorV2_PhantomCells` - phantom cell detection
- `TestArrayDetectorV2_FormulaTemplateRelative` - relative reference patterns
- `TestFormulasCompatibleV2` - formula compatibility checking
- `TestResolveNamedRange_*` - named range resolution
- `TestBuildDependencyGraph_NamedRange` - named range edges
- `TestBuildDependencyGraph_CrossSheetReference` - cross-sheet edges

### Absolute Reference Handling (Completed)

Added proper handling of absolute (`$A$1`) vs relative (`A1`) references in formula congruence checking:

1. **New model types** (`pkg/model/raw.go`):
   - `FormulaRef` - cell reference with `RowAbsolute`/`ColAbsolute` boolean fields
   - `FormulaRangeRef` - range reference with absolute markers for all four corners

2. **Formula parser updates** (`pkg/xlsx/formulas.go`):
   - `extractFormulaRef()` - extracts cell refs with absolute markers from regex
   - `extractFormulaRangeRef()` - extracts range refs with absolute markers
   - `ParsedFormula` now has `FormulaRefs` and `FormulaRanges` fields (legacy `References`/`RangeReferences` kept for compatibility)

3. **Congruence checking updates** (`pkg/inference/arrays_v2.go`):
   - `formulasCompatibleV2()` now uses `FormulaRefs` and checks:
     - Absolute references must stay fixed across cells
     - Relative references must move by cell offset
     - Both cell and range references are validated

**Example**: `=B1*$A$1` and `=B2*$A$1` are now correctly recognized as congruent because B moves relatively while A stays fixed.

### Pending Tasks

1. **Array-to-array template references** - Convert relative patterns from cell offsets to array references.

2. **Shuffle test** - Verify algorithm stability with different cell orderings.

---
