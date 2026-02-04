# ModelMigrate

This project builds an agentic AI microservice to manage (audit, document and convert) economic, policy, or accounting models contained in bloated, complex Excel workbooks.

The product will enable civil servants, consultants and other professionals who deal with complex Excel models to:

* Generate documentation for the workbook
* Diagnose and fix errors in the spreadsheet representation of the model
* Convert the model into a reproducible Python representation, augmented by an interactive interface

The big idea here is that there are still too many decision-making, scenario, and policy models implemented in Excel. Many of them are legacy and are too difficult for their current owners to understand and update/convert. AI agents provide an opportunity to do something about this technical debt. However, AI works best when it is constrained by a strong structured framework to verify and focus its reasoning. For that reason, this project will develop an intermediate data structure that the AI can manipulate and reason about.

The product will eventually be an architecture of agentic AI microservice(s) running on a robust, asyncronous cloud architecture to autonomously process Excel models.
At the core of the product will be a suite of tools that parse Excel workbooks to an intermediate data structure (according to a deterministic algorithm), and an agentic harness that gives an AI agent the capabilities to manipulate the intermediate data, to interpret, diagnose and convert to other formats. The agentic harness will also run checks to verify that the outputs maintain fidelity to the inputs.

This repo will be a monorepo that may eventually contain more than one independent package or microservice, including:

* Excel workbook parsing algorithm and microservice: The workbook is parsed to an intermediate representation deterministically to avoid hallucinations.
* AI agent(s) to process the intermediate representation, performing operations such as merging and splitting arrays that preserve the structure's isomorphism while improving its interpretability. The AI agent(s) can then perform a variety of tasks such as auditing, updating data, diagnosing errors, and converting to Python models. The AI agent has tools such as workbook reader and screenshot, that enable it to interpret workbooks

## Intermediate representation

In its internal XML representation, an Excel workbook is a collection of cells with formulas that reference other cells. In the most abstract representation, the models represented in Excel are tensors indexed by meaningful dimensions (such as year, region, cost centre) which form a computational graph. The relationship between the two is many-to-many: for each abstract model, there are many possible ways to represent that as an Excel workbook; and although each Excel workbook has only one initial parsing, the cells can be combined and split into tensors in different ways that preserve fidelity to the workbook, so there are also many abstract models per workbook.

This flexibility, along with the possibility of errors, leads to the need for AI intervention - without this, it'd be enough to build a deterministic parser. The AI can help here in four roles:

1. The Excel workbook might contain errors - e.g. a single cell in the middle of an array that doesn't follow the same formula - the agent can decide whether the workbook does what it's intended to do and flag up any suspected errors
2. The initial parsing of the workbook will be _valid_ but may not be optimal for interpretation. The agent can make isomorphic edits to the abstract structure to improve the interpretability and usability of the model. For example, the parser might return three arrays, that should actually be considered as three slices of one 3d tensor.
3. Documentation and explanation - the agent can add labels and documentation to explain what the model is intended to do
4. Conversion - the agent can reimplement the model in a different form such as a Python notebook

## Architecture and tooling

* The deterministic parser should be implemented using Go, because it's compiled and fast with minimal dependencies and a quick learning curve. It should be a full Go microservice to avoid having to call Go from Python
* All other backend will use Python with the uv tool
* Any frontend should be vanilla typescript/CSS, built using bun
* The parsing and AI agent microservices should be async with a queuing and scheduling system - possibly pub/sub
* The intermediate data structure should be json or yaml (or possibly something else)
* To be completed...

# Plan

## Phase 1: Go Parser Foundation (Layers 1-2)

### 1.1 xlsx Parsing (Layer 1 - Raw Extraction)

Build a Go microservice that extracts raw data from xlsx files:

**Inputs:** xlsx file path
**Outputs:** JSON intermediate representation

Data extracted:
- Sheets (names, dimensions)
- Cells (position, value, formula, number format, type)
- Named ranges (name → range reference mapping)
- Merged regions
- External links

**xlsx format notes:**
- xlsx is a ZIP archive containing XML files
- `xl/workbook.xml` - sheet list and named ranges
- `xl/worksheets/sheet*.xml` - cell data
- `xl/sharedStrings.xml` - deduplicated string values
- `xl/styles.xml` - number formats for date/percentage detection

### 1.2 Structural Inference (Layer 2)

**Core Design Goal: Parsimony**

The parser's goal is to find the most *parsimonious* representation of the workbook - meaning the **fewest, largest arrays possible** that faithfully represent the computational structure. Rather than treating each cell as a node in the computational graph, we want to identify arrays (tensors) and express the computation as **array-to-array operations**.

For example, if cells B1:B10 each contain formulas like `=A1*2`, `=A2*2`, ..., `=A10*2`, we don't want 10 separate scalar operations - we want to recognize that array B depends on array A via the formula `B = A * 2`.

**Formula Congruence**

Two formulas are *congruent* if they have the same computational structure with predictable reference offsets. Specifically:
- Same operators and functions
- Same number of cell/range references
- References that either stay fixed (absolute, e.g., `$A$1`) or move consistently with the cell position (relative, e.g., `A1`)

Examples:
- `=A1*2` and `=A2*2` are congruent (same structure, row offset +1)
- `=A1+$B$1` and `=A2+$B$1` are congruent (A moves relatively, B is fixed)
- `=SUM(A1:A5)` and `=SUM(B1:B5)` are congruent (same function, column offset +1)
- `=A1+B1` and `=A1*B1` are NOT congruent (different operators)

Congruent formulas indicate that cells belong to the same array with a consistent formula template.

**Array Detection Algorithm (V2 - Column-First)**

The algorithm builds arrays in four phases:

*Phase 1: Collect Cell Universe ("The Bag")*
- Collect ALL cell references into a single "bag", including:
  - Cells that exist in the workbook with data/formulas
  - "Phantom" cells that don't exist but are referenced by formulas (these are implicit inputs)
- Classify each cell by type: numeric, string, or unknown

*Phase 2: Build Numeric Arrays (Column-First)*
- Process numeric cells column-by-column (prioritizing vertical structure)
- Find maximal vertical runs of congruent cells (same type, format, formula pattern)
- Expand runs horizontally only if compatible
- This prioritizes column structure because most financial/data models organize data in columns

*Phase 3: Assign String Labels*
- Look for string cells adjacent to numeric arrays (up to 2 rows above, 2 columns left)
- Attach as column headers (above) or row headers (left)
- Mark attached strings as "assigned" so they're not duplicated

*Phase 4: Gather Remaining Cells*
- Collect any unassigned cells (strings, unknowns) into their own arrays
- Ensures complete coverage - no cell is lost

**Formula Templates (Array-to-Array References)**

Once arrays are identified, formula templates express the computation at the array level:
- `TemplateStr`: The formula pattern (e.g., `(F7-G7)*I7`)
- `FixedRefs`: References that don't change across the array (absolute refs)
- `RelativePatterns`: References that move with cell position
- `Exceptions`: Cells that deviate from the template (potential errors)
- `Coverage`: Percentage of cells matching the template (1.0 = perfect)

*Future goal*: Convert relative patterns to array references (e.g., "ref_0 points to arr_015[same_row, col-3]")

**Arrays:** Contiguous regions with congruent formula patterns
- Detect orientation (row vector, column vector, matrix)
- Extract row/column headers from adjacent text cells
- Build formula templates capturing the array-level computation
- Track exceptions (cells that break the pattern - potential errors)

**Scalars:** Single cells with computational significance
- Link to named ranges if present
- Infer labels from adjacent cells
- Classify role (INPUT, PARAMETER, INTERMEDIATE, OUTPUT)

**Dependency Graph:**
- Parse formula references
- Build directed graph of cell/array dependencies
- Compute topological order for evaluation

### 1.3 Deliverables

```
parser/
├── cmd/
│   └── parser/
│       └── main.go          # CLI entrypoint
├── pkg/
│   ├── xlsx/
│   │   ├── reader.go        # ZIP/XML extraction
│   │   ├── cells.go         # Cell parsing
│   │   ├── formulas.go      # Formula parsing
│   │   └── names.go         # Named range extraction
│   ├── model/
│   │   ├── raw.go           # Layer 1 data structures
│   │   └── structural.go    # Layer 2 data structures
│   └── inference/
│       ├── arrays.go        # Array detection
│       ├── graph.go         # Dependency graph
│       └── templates.go     # Formula template extraction
├── go.mod
└── go.sum
```

## Phase 2: Python Integration

### 2.1 Harness Development
- Call Go parser as subprocess or via gRPC
- Load JSON output into Python dataclasses
- Implement validation contract generation

### 2.2 AI Agent Tools
- Structure summary tool
- Element detail tool
- Annotation submission tool
- Code generation tool

## Phase 3: Frontend & Cloud

### 3.1 Frontend (TypeScript/Bun)
- Upload workbook
- View structural analysis
- Review AI annotations
- Download generated Python

### 3.2 Cloud Architecture
- Async processing with pub/sub
- Workbook storage
- Job queue for AI processing

---

## Test Workbooks

| File | Format | Size | Description |
|------|--------|------|-------------|
| `tag-workbook-valuing-dependent-development-workbook.xlsx` | xlsx | 88KB | Land valuation model (9 sheets) |
| `15710-gdpcr_0.xls` | xls | 625KB | UK Gas Distribution Price Control Review (31 sheets) |

Priority: xlsx format only. xls (legacy binary format) is out of scope.
