# ModelMigrate

A tool for parsing, auditing, and converting complex Excel workbooks into structured representations suitable for AI analysis and Python transpilation.

## Overview

ModelMigrate extracts Excel workbooks into an intermediate JSON representation that captures:
- **Arrays** - Contiguous regions with consistent formula patterns
- **Scalars** - Single cells with computational significance
- **Dependency graphs** - How cells reference each other
- **Formula templates** - Array-level computation patterns

The goal is *parsimony*: find the fewest, largest arrays possible and express computation as array-to-array operations rather than cell-by-cell.

## Repository Structure

```
ModelMigrate/
├── parser/                    # Go parser (Layer 1-2)
│   ├── cmd/parser/main.go     # CLI entry point
│   └── pkg/
│       ├── model/             # Data structures
│       │   ├── raw.go         # Layer 1: Raw extraction types
│       │   └── structural.go  # Layer 2: Inferred structure types
│       ├── xlsx/              # Excel file parsing
│       │   ├── reader.go      # xlsx file extraction
│       │   └── formulas.go    # Formula parsing
│       └── inference/         # Structural inference
│           ├── arrays_v2.go   # Array detection algorithm
│           └── graph.go       # Dependency graph construction
├── data/                      # Test workbooks
├── examples/                  # Sample parser output
├── SPEC.md                    # Technical specification
├── LOG.md                     # Development session notes
└── TODO.md                    # Current task tracking
```

## Getting Started

### Prerequisites

- Go 1.21 or later

### Building the Parser

```bash
cd parser
go build -o parser ./cmd/parser
```

### Running the Parser

**Raw extraction (Layer 1):**
```bash
./parser -mode raw path/to/workbook.xlsx
```

**Structural analysis (Layer 2):**
```bash
./parser -mode structural path/to/workbook.xlsx
```

**Save output to file:**
```bash
./parser -mode structural -output result.json path/to/workbook.xlsx
```

### Running Tests

```bash
cd parser
go test ./... -cover
```

## Example Output

For a workbook with formulas like `=A1*$B$1`, `=A2*$B$1`, `=A3*$B$1`:

```json
{
  "id": "arr_003",
  "range_ref": {"sheet": "Sheet1", "top_left": [0, 2], "bottom_right": [2, 2]},
  "orientation": "col_vector",
  "has_formulas": true,
  "formula_template": {
    "template_str": "A1*$B$1",
    "relative_patterns": {
      "ref_0": {
        "target_array_id": "arr_001",
        "row_indexing": "same",
        "col_indexing": "fixed:0"
      }
    },
    "resolved_fixed": {
      "ref_1": {
        "target_array_id": "arr_002",
        "array_row": 0,
        "array_col": 0
      }
    },
    "coverage": 1.0
  }
}
```

This shows that `arr_003` references `arr_001` with matching row indices ("same") and `arr_002` at a fixed position.

## Key Concepts

### Formula Congruence

Two formulas are *congruent* if they have the same structure with predictable reference offsets:
- `=A1*2` and `=A2*2` are congruent (relative reference moves with cell)
- `=A1*$B$1` and `=A2*$B$1` are congruent (A moves, B stays fixed)
- `=A1+B1` and `=A1*B1` are NOT congruent (different operators)

### Detection Algorithm (V2)

1. **Collect cell universe** - All cells including "phantom" cells referenced by formulas
2. **Build numeric arrays** - Column-first, grouping congruent formulas
3. **Assign string labels** - Attach headers to numeric arrays
4. **Gather remaining cells** - Capture unassigned strings
5. **Resolve array references** - Convert cell refs to array-level refs

## Documentation

- [SPEC.md](SPEC.md) - Technical specification and architecture
- [LOG.md](LOG.md) - Development session notes
- [AGENTS.md](AGENTS.md) - Guidelines for AI agents working on this project

## License

MIT
