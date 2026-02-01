// Package model defines the data structures for the intermediate representation.
package model

// Layer 2: Structural Inference
// These structures represent the inferred computational structure.

// ArrayOrientation describes how an array is oriented.
type ArrayOrientation string

const (
	OrientationRowVector       ArrayOrientation = "row_vector"       // 1 x n
	OrientationColVector       ArrayOrientation = "col_vector"       // n x 1
	OrientationTimeHorizontal  ArrayOrientation = "time_horizontal"  // Rows are entities, cols are time
	OrientationTimeVertical    ArrayOrientation = "time_vertical"    // Rows are time, cols are entities
	OrientationMatrix          ArrayOrientation = "matrix"           // True 2D structure
	OrientationUnknown         ArrayOrientation = "unknown"
)

// DataRole describes the role of a data element in the computation.
type DataRole string

const (
	RoleInput        DataRole = "input"        // No formula, no dependents (leaf input)
	RoleParameter    DataRole = "parameter"    // No formula, referenced by formulas (constant)
	RoleIntermediate DataRole = "intermediate" // Has formula, is referenced by others
	RoleOutput       DataRole = "output"       // Has formula, not referenced by others (terminal)
	RoleLabel        DataRole = "label"        // Text used as header/label
)

// InferredArray represents a contiguous block with structural consistency.
type InferredArray struct {
	ID       string   `json:"id"` // Unique identifier
	RangeRef RangeRef `json:"range_ref"`

	// Shape and orientation
	Orientation ArrayOrientation `json:"orientation"`

	// Header detection
	RowHeaders      []string  `json:"row_headers,omitempty"`
	ColHeaders      []string  `json:"col_headers,omitempty"`
	RowHeaderSource *RangeRef `json:"row_header_source,omitempty"`
	ColHeaderSource *RangeRef `json:"col_header_source,omitempty"`

	// Data characteristics
	DType          string           `json:"dtype"` // float64, int64, datetime64, str, object
	HasFormulas    bool             `json:"has_formulas"`
	FormulaTemplate *FormulaTemplate `json:"formula_template,omitempty"`

	// Role in computation
	DataRole DataRole `json:"data_role"`

	// Time axis detection
	TimeAxis   string `json:"time_axis,omitempty"`   // "row", "col", or empty
	TimeOrigin string `json:"time_origin,omitempty"` // First date if detected
	TimeFreq   string `json:"time_freq,omitempty"`   // M, Q, A, D, W, or empty
}

// FormulaTemplate captures the pattern of formulas in an array.
type FormulaTemplate struct {
	TemplateStr      string            `json:"template_str"`
	FixedRefs        map[string]CellRef `json:"fixed_refs"`        // References that don't change
	RelativePatterns map[string]RelativePattern `json:"relative_patterns"` // References that vary
	DynamicRanges    map[string]DynamicRangePattern `json:"dynamic_ranges,omitempty"`
	Exceptions       map[string]string `json:"exceptions,omitempty"` // Position -> different formula
	Coverage         float64           `json:"coverage"` // Fraction of cells matching template
}

// RelativePattern describes how a reference varies across array positions.
type RelativePattern struct {
	BaseOffset [2]int `json:"base_offset"` // Offset from array top-left
	RowDelta   int    `json:"row_delta"`   // How much row changes per array row
	ColDelta   int    `json:"col_delta"`   // How much col changes per array col
}

// DynamicRangePattern describes ranges that grow/shrink.
type DynamicRangePattern struct {
	FixedCorner      CellRef `json:"fixed_corner"`
	GrowingDimension string  `json:"growing_dimension"` // "row" or "col"
	GrowthType       string  `json:"growth_type"`       // "cumulative" or "window"
	WindowSize       int     `json:"window_size,omitempty"`
}

// InferredScalar represents a single cell with computational significance.
type InferredScalar struct {
	ID      string  `json:"id"`
	CellRef CellRef `json:"cell_ref"`

	// Naming
	ExcelName     string   `json:"excel_name,omitempty"`     // From named range
	InferredLabel string   `json:"inferred_label,omitempty"` // From adjacent cell
	LabelSource   *CellRef `json:"label_source,omitempty"`

	// Value
	Value   any    `json:"value"`
	DType   string `json:"dtype"`
	Formula string `json:"formula,omitempty"`

	// Role
	DataRole DataRole `json:"data_role"`

	// Date detection
	IsDate bool `json:"is_date"`
}

// LookupTable represents a VLOOKUP/HLOOKUP/INDEX-MATCH target.
type LookupTable struct {
	ID       string   `json:"id"`
	RangeRef RangeRef `json:"range_ref"`

	KeyColumn    int   `json:"key_column"`    // Relative to range
	KeyValues    []any `json:"key_values"`
	ValueColumns []int `json:"value_columns"`

	IsSorted bool `json:"is_sorted"` // Affects lookup semantics
}

// DependencyEdge represents a dependency between elements.
type DependencyEdge struct {
	Source   string `json:"source"`   // Element ID
	Target   string `json:"target"`   // Element ID
	RefType  string `json:"ref_type"` // "direct", "range", "lookup"
}

// DependencyGraph represents the computation order and relationships.
type DependencyGraph struct {
	Nodes []string         `json:"nodes"` // Element IDs in topological order
	Edges []DependencyEdge `json:"edges"`
}

// CoverageStats tracks how well the structural model covers the workbook.
type CoverageStats struct {
	TotalFormulaCells   int       `json:"total_formula_cells"`
	CellsInArrays       int       `json:"cells_in_arrays"`
	CellsInScalars      int       `json:"cells_in_scalars"`
	UncategorizedCells  []CellRef `json:"uncategorized_cells,omitempty"`
	TemplateCoverage    map[string]float64 `json:"template_coverage"` // array_id -> % matching
}

// StructuralModel represents the complete Layer 2 output.
type StructuralModel struct {
	Source  *RawWorkbook `json:"source"`

	Arrays  map[string]*InferredArray  `json:"arrays"`
	Scalars map[string]*InferredScalar `json:"scalars"`
	Lookups map[string]*LookupTable    `json:"lookups"`

	Graph DependencyGraph `json:"graph"`

	// Validation data
	ParseWarnings []string       `json:"parse_warnings,omitempty"`
	Coverage      CoverageStats  `json:"coverage"`
}

// NewStructuralModel creates a new StructuralModel from a RawWorkbook.
func NewStructuralModel(raw *RawWorkbook) *StructuralModel {
	return &StructuralModel{
		Source:  raw,
		Arrays:  make(map[string]*InferredArray),
		Scalars: make(map[string]*InferredScalar),
		Lookups: make(map[string]*LookupTable),
		Graph:   DependencyGraph{Nodes: []string{}, Edges: []DependencyEdge{}},
		Coverage: CoverageStats{TemplateCoverage: make(map[string]float64)},
	}
}
