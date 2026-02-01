This is a fascinating problem - you're essentially building a semantic bridge between two very different computational paradigms. Let me think through this with you.

## Core Tensions to Navigate

First, let me identify what makes this tricky:

1. **Excel's implicit structure** - Arrays aren't declared, they emerge from patterns of formulas and references
2. **Mixed semantics** - The same cell range might be a time series, a lookup table, or a parameter block depending on context
3. **Formula dependencies** - Excel's dependency graph is implicit but crucial for translation order
4. **Named ranges vs positional references** - Some workbooks are well-structured, others are positional spaghetti

## Proposed Intermediate Representation

I'd suggest a layered structure that separates what can be deterministically extracted from what requires interpretation:

```python
@dataclass
class WorkbookModel:
    # Layer 1: Deterministic extraction
    sheets: dict[str, SheetModel]
    named_ranges: dict[str, NamedRange]
    external_links: list[ExternalReference]
    
    # Layer 2: Inferred structure (deterministic heuristics)
    arrays: list[ArrayBlock]
    scalars: list[ScalarVariable]
    lookup_tables: list[LookupTable]
    
    # Layer 3: AI-augmented interpretation
    semantic_annotations: dict[str, SemanticAnnotation]
    business_entities: list[BusinessEntity]
    computation_graph: ComputationGraph

@dataclass
class ArrayBlock:
    """A contiguous region with consistent formula pattern"""
    sheet: str
    top_left: CellRef
    shape: tuple[int, int]
    
    # Deterministic
    orientation: Literal["time_horizontal", "time_vertical", "matrix", "unknown"]
    header_row: list[str] | None
    header_col: list[str] | None
    formula_pattern: FormulaTemplate | None  # Parameterized formula
    value_type: Literal["numeric", "text", "date", "mixed"]
    
    # What varies across the array
    varying_references: list[VaryingReference]
    # What stays constant
    fixed_references: list[CellRef]
    
    # AI-augmented
    semantic_role: str | None  # "cash_flow_projection", "amortization_schedule", etc.
    business_description: str | None
    suggested_variable_name: str | None
    
@dataclass 
class FormulaTemplate:
    """Captures the pattern of formulas in an array"""
    template: str  # e.g., "={row_header} * {fixed_ref_1} + {col_offset:-1}"
    parameters: dict[str, ParameterSpec]
    exceptions: dict[CellRef, str]  # Cells that break the pattern

@dataclass
class VaryingReference:
    """How a reference changes across an array"""
    base_ref: CellRef
    varies_with: Literal["row", "col", "both"]
    offset_pattern: Literal["linear", "fixed_row", "fixed_col", "custom"]
    
@dataclass
class ScalarVariable:
    cell: CellRef
    name: str | None  # From named range or adjacent label
    value: Any
    formula: str | None
    
    # AI-augmented
    semantic_role: str | None  # "discount_rate", "start_date", "tax_rate"
    suggested_variable_name: str | None
    
@dataclass
class ComputationGraph:
    """Dependency structure for determining evaluation order"""
    nodes: dict[str, ComputationNode]  # ArrayBlock or ScalarVariable id -> node
    edges: list[tuple[str, str]]  # (dependency, dependent)
    
    def topological_order(self) -> list[str]:
        """Order for Python code generation"""
        ...

@dataclass
class BusinessEntity:
    """Higher-level semantic grouping"""
    name: str
    entity_type: str  # "loan", "asset", "policy", "scenario"
    components: list[str]  # References to arrays/scalars
    relationships: list[EntityRelationship]
    description: str
```

## Key Design Decisions

**Why separate deterministic from AI-augmented?**

This lets you:
- Validate the AI's interpretations against ground truth
- Show users exactly what was inferred vs interpreted
- Re-run AI interpretation without re-parsing
- Build confidence scores (does the AI's understanding match the structure?)

**Formula templates are crucial**

The pattern `=B5*$C$2+A5` across a column is really `previous_row * constant + row_header`. Extracting this parameterized form is what enables clean Python translation:

```python
# Excel: column of =B5*$C$2+A5, =B6*$C$2+A6, ...
# Python:
result = previous_values * discount_rate + time_index
```

**The "varying reference" concept**

This captures Excel's mixed reference modes ($A$1 vs $A1 vs A$1 vs A1) in a way that maps to numpy broadcasting or pandas operations.

## Validation Hooks

For your "check notebook against structure" requirement:

```python
@dataclass
class ValidationContract:
    """What the generated notebook must satisfy"""
    
    # Shape contracts
    array_shapes: dict[str, tuple[int, ...]]
    
    # Value contracts (spot checks)
    expected_values: list[ValueAssertion]
    
    # Dependency contracts
    must_compute_before: list[tuple[str, str]]
    
    # Formula equivalence (for a sample of cells)
    formula_checks: list[FormulaEquivalenceCheck]

@dataclass
class FormulaEquivalenceCheck:
    original_cell: CellRef
    python_expression: str
    test_inputs: dict[str, Any]
    expected_output: Any
    tolerance: float = 1e-10
```

## Questions to Refine This

A few things that would help me sharpen this:

1. **What's your target Python idiom?** Pure numpy arrays, pandas DataFrames, or a mix? This affects how you'd represent the "array with labeled headers" pattern.

2. **How do you want to handle Excel's date/time arithmetic?** It's notoriously quirky and often needs explicit handling.

3. **Are scenarios typically represented as separate sheets, named ranges, or data tables?** This affects whether "scenario" should be a first-class concept in the IR.

4. **What's the expected workflow?** Does a human review the intermediate representation before notebook generation, or is it more automated with human review at the end?

Would you like me to sketch out what the deterministic parser's output might look like for a concrete example (say, a simple loan amortization schedule)?

1. I'd like it to default to numpy arrays but switch to pandas for the biggest arrays with tall or multi-dimensional structure
2. Just handle it
3. Scenarios are not first-class because they can be represented in different ways. They're just another dimension when present 
4. Eventually there'll be no human in the loop until the end. The AI agent will operate within a harness that includes deterministic tools and hooks

Excellent - those constraints really sharpen things. The "AI agent in a harness" framing is particularly important because it means the IR needs to be machine-verifiable, not just human-readable.

Let me revise with those constraints in mind.

## Revised Core Philosophy

The IR should be:
1. **Round-trippable** - You can regenerate Excel from it (for validation)
2. **Diffable** - Agent iterations can be compared structurally
3. **Constraining** - The agent can't hallucinate structure, only annotate it

## Revised Data Structures

```python
from dataclasses import dataclass, field
from enum import Enum, auto
from typing import Literal, Any, Callable
import numpy as np

# =============================================================================
# LAYER 1: Raw Extraction (Fully Deterministic)
# =============================================================================

@dataclass(frozen=True)
class CellRef:
    sheet: str
    row: int  # 0-indexed
    col: int  # 0-indexed
    
    def to_a1(self) -> str:
        """Convert to A1 notation"""
        col_str = ""
        c = self.col
        while c >= 0:
            col_str = chr(c % 26 + ord('A')) + col_str
            c = c // 26 - 1
        return f"'{self.sheet}'!{col_str}{self.row + 1}"
    
    @classmethod
    def from_a1(cls, ref: str) -> "CellRef":
        ...

@dataclass(frozen=True)
class RangeRef:
    sheet: str
    top_left: tuple[int, int]
    bottom_right: tuple[int, int]
    
    @property
    def shape(self) -> tuple[int, int]:
        return (
            self.bottom_right[0] - self.top_left[0] + 1,
            self.bottom_right[1] - self.top_left[1] + 1
        )

@dataclass
class RawCell:
    ref: CellRef
    value: Any  # Evaluated value
    formula: str | None  # Raw formula string if present
    number_format: str | None  # For detecting dates, percentages, currency
    
    @property
    def is_date(self) -> bool:
        """Heuristic based on format codes"""
        if self.number_format is None:
            return False
        date_indicators = ['yy', 'mm', 'dd', 'date', 'yyyy']
        return any(ind in self.number_format.lower() for ind in date_indicators)

@dataclass
class ParsedFormula:
    """Deterministically parsed formula structure"""
    original: str
    references: list[CellRef | RangeRef]
    named_references: list[str]
    functions_used: list[str]
    operators: list[str]
    constants: list[Any]
    
    # Structural classification
    is_array_formula: bool
    reference_pattern: "ReferencePattern"

@dataclass
class ReferencePattern:
    """How references in a formula relate to the formula's position"""
    relative_refs: list[tuple[int, int]]  # Offsets from formula cell
    absolute_refs: list[CellRef]
    mixed_refs: list[tuple[CellRef, Literal["row", "col"]]]  # Which dimension is fixed

@dataclass
class RawSheet:
    name: str
    cells: dict[tuple[int, int], RawCell]
    merged_regions: list[RangeRef]
    
    def bounds(self) -> tuple[int, int, int, int]:
        """min_row, max_row, min_col, max_col of used range"""
        if not self.cells:
            return (0, 0, 0, 0)
        rows = [k[0] for k in self.cells.keys()]
        cols = [k[1] for k in self.cells.keys()]
        return (min(rows), max(rows), min(cols), max(cols))

@dataclass
class RawWorkbook:
    """Pure extraction, no interpretation"""
    sheets: dict[str, RawSheet]
    named_ranges: dict[str, RangeRef | CellRef]
    defined_names: dict[str, str]  # Name -> formula (for computed names)


# =============================================================================
# LAYER 2: Structural Inference (Deterministic Heuristics)
# =============================================================================

class ArrayOrientation(Enum):
    ROW_VECTOR = auto()      # 1 x n
    COL_VECTOR = auto()      # n x 1  
    TIME_HORIZONTAL = auto() # Rows are entities, cols are time periods
    TIME_VERTICAL = auto()   # Rows are time periods, cols are entities
    MATRIX = auto()          # True 2D structure
    UNKNOWN = auto()

class DataRole(Enum):
    """Deterministically inferable roles"""
    INPUT = auto()           # No formula, no dependents reference it (leaf input)
    PARAMETER = auto()       # No formula, referenced by formulas (constant)
    INTERMEDIATE = auto()    # Has formula, is referenced by others
    OUTPUT = auto()          # Has formula, not referenced by others (terminal)
    LABEL = auto()           # Text used as header/label

@dataclass
class InferredArray:
    """A contiguous block with structural consistency"""
    id: str  # Unique identifier for graph references
    range_ref: RangeRef
    
    # Shape and orientation
    orientation: ArrayOrientation
    
    # Header detection (deterministic: adjacent text cells)
    row_headers: list[str] | None  # Labels for each row
    col_headers: list[str] | None  # Labels for each column
    row_header_source: RangeRef | None
    col_header_source: RangeRef | None
    
    # Data characteristics
    dtype: Literal["float64", "int64", "datetime64", "str", "object"]
    has_formulas: bool
    formula_template: "FormulaTemplate | None"
    
    # Role in computation
    data_role: DataRole
    
    # Values (for validation)
    values: np.ndarray
    
    # Date handling
    time_axis: Literal["row", "col", None]
    time_origin: Any | None  # First date value if detected
    time_freq: str | None  # 'M', 'Q', 'A', 'D', or None

@dataclass
class FormulaTemplate:
    """
    Parameterized formula pattern.
    
    Example: "=B{r}*$C$2+SUM(A$1:A{r})" across rows becomes:
    template = "{prev_col}*{param_0}+SUM({sum_range})"
    """
    template_str: str
    
    # Fixed references (same for all cells in array)
    fixed_refs: dict[str, CellRef]
    
    # Relative references (vary predictably)
    relative_patterns: dict[str, "RelativePattern"]
    
    # Range references that grow/shrink
    dynamic_ranges: dict[str, "DynamicRangePattern"]
    
    # Exceptions to the pattern
    exceptions: dict[tuple[int, int], ParsedFormula]  # (row_offset, col_offset) -> different formula
    
    def matches_cell(self, cell_formula: ParsedFormula, position: tuple[int, int]) -> bool:
        """Verify a cell matches the template at given position"""
        ...
    
    def coverage(self) -> float:
        """What fraction of array cells match the template"""
        ...

@dataclass
class RelativePattern:
    """How a reference varies across array positions"""
    base_offset: tuple[int, int]  # Offset from array top-left
    row_delta: int  # How much row changes per array row (usually 0 or 1)
    col_delta: int  # How much col changes per array col (usually 0 or 1)

@dataclass
class DynamicRangePattern:
    """For ranges like SUM(A$1:A5) that grow as you go down"""
    fixed_corner: CellRef
    growing_dimension: Literal["row", "col"]
    growth_type: Literal["cumulative", "window"]
    window_size: int | None  # For rolling calculations

@dataclass
class InferredScalar:
    """Single-cell value with computational significance"""
    id: str
    cell_ref: CellRef
    
    # Naming (deterministic: from named ranges or adjacent labels)
    excel_name: str | None  # Named range if exists
    inferred_label: str | None  # From adjacent cell
    label_source: CellRef | None
    
    # Value
    value: Any
    dtype: Literal["float64", "int64", "datetime64", "str", "bool"]
    formula: ParsedFormula | None
    
    # Role
    data_role: DataRole
    
    # Date detection
    is_date: bool
    
@dataclass
class LookupTable:
    """VLOOKUP/HLOOKUP/INDEX-MATCH target"""
    id: str
    range_ref: RangeRef
    
    key_column: int  # Relative to range
    key_values: list[Any]
    value_columns: list[int]
    
    is_sorted: bool  # Affects lookup semantics
    
@dataclass 
class DependencyGraph:
    """Computation order and relationships"""
    nodes: dict[str, InferredArray | InferredScalar | LookupTable]
    
    # Edges: (source_id, target_id, reference_type)
    edges: list[tuple[str, str, Literal["direct", "range", "lookup"]]]
    
    # Precomputed for efficiency
    _topo_order: list[str] | None = field(default=None, repr=False)
    _dependents: dict[str, set[str]] = field(default_factory=dict, repr=False)
    _dependencies: dict[str, set[str]] = field(default_factory=dict, repr=False)
    
    def topological_order(self) -> list[str]:
        """Evaluation order for Python code"""
        if self._topo_order is None:
            # Kahn's algorithm
            ...
        return self._topo_order
    
    def dependency_depth(self, node_id: str) -> int:
        """How many layers deep in the computation"""
        ...
    
    def downstream_impact(self, node_id: str) -> set[str]:
        """All nodes affected if this one changes"""
        ...

@dataclass
class StructuralModel:
    """Complete Layer 2 output - fully deterministic"""
    source: RawWorkbook
    
    arrays: dict[str, InferredArray]
    scalars: dict[str, InferredScalar]
    lookups: dict[str, LookupTable]
    
    graph: DependencyGraph
    
    # Validation data
    parse_warnings: list[str]
    coverage_stats: "CoverageStats"

@dataclass
class CoverageStats:
    """How well the structural model covers the workbook"""
    total_formula_cells: int
    cells_in_arrays: int
    cells_in_scalars: int
    uncategorized_cells: list[CellRef]
    
    template_coverage: dict[str, float]  # array_id -> % cells matching template


# =============================================================================
# LAYER 3: Semantic Annotation (AI-Augmented)
# =============================================================================

@dataclass
class SemanticAnnotation:
    """AI-generated interpretation, anchored to structural elements"""
    element_id: str  # References array/scalar/lookup id
    
    # Naming
    suggested_python_name: str
    business_name: str  # Human-readable
    
    # Description
    description: str
    business_logic: str  # What this represents in the domain
    
    # Categorization
    domain_category: str  # "cash_flow", "discount_factor", "premium", etc.
    
    # Confidence
    confidence: float  # 0-1, based on label quality, formula clarity, etc.
    reasoning: str  # Why the AI made this interpretation

@dataclass
class SemanticRelationship:
    """AI-inferred relationship between elements"""
    source_id: str
    target_id: str
    relationship_type: str  # "discounts", "accumulates_into", "allocates_from", etc.
    description: str

@dataclass
class ComputationIntent:
    """AI interpretation of what a formula/array is computing"""
    element_id: str
    
    intent: str  # "present_value", "cumulative_sum", "growth_rate", etc.
    standard_form: str | None  # Canonical formula if recognized
    
    # For validation: if we understand the intent, we can verify the formula
    expected_formula_pattern: str | None

@dataclass
class SemanticModel:
    """Layer 3 output - AI augmented, validated against structure"""
    structure: StructuralModel
    
    annotations: dict[str, SemanticAnnotation]
    relationships: list[SemanticRelationship]
    intents: dict[str, ComputationIntent]
    
    # Global interpretation
    model_description: str
    identified_scenarios: list["ScenarioDimension"] | None
    
    # Validation status
    annotation_coverage: float  # % of elements annotated
    consistency_issues: list[str]  # Where AI interpretation conflicts with structure

@dataclass
class ScenarioDimension:
    """When scenarios are detected (as a dimension, not first-class)"""
    dimension_type: Literal["sheet", "column", "row", "named_range"]
    scenarios: list[str]  # Scenario names/identifiers
    varying_elements: list[str]  # Which element IDs vary by scenario


# =============================================================================
# LAYER 4: Python Translation Spec (Deterministic from Layers 2+3)
# =============================================================================

class PythonContainer(Enum):
    NUMPY = auto()
    PANDAS_SERIES = auto()
    PANDAS_DATAFRAME = auto()
    SCALAR = auto()

@dataclass
class TranslationSpec:
    """How to translate a structural element to Python"""
    element_id: str
    
    # Target representation
    container: PythonContainer
    python_name: str
    
    # For arrays
    use_pandas: bool
    pandas_index: str | None  # Expression for index
    pandas_columns: str | None  # Expression for columns
    
    # Date handling
    date_conversion: "DateConversion | None"
    
    # Generated code template
    initialization_code: str | None  # For inputs
    computation_code: str | None  # For computed values

@dataclass
class DateConversion:
    """Excel date -> Python date translation"""
    excel_epoch: Literal["1900", "1904"]  # Excel's date system
    target_type: Literal["datetime64[D]", "datetime64[M]", "Period"]
    frequency: str | None

@dataclass
class TranslationPlan:
    """Complete plan for generating Python code"""
    semantic_model: SemanticModel
    
    specs: dict[str, TranslationSpec]
    evaluation_order: list[str]
    
    # Code organization
    imports: list[str]
    parameter_block: list[str]  # Variable definitions for inputs
    computation_blocks: list["ComputationBlock"]
    output_block: list[str]  # Final results

@dataclass 
class ComputationBlock:
    """A logical grouping of related computations"""
    name: str
    description: str
    element_ids: list[str]
    code_lines: list[str]
    
    # For notebook structure
    suggested_cell_break: bool


# =============================================================================
# LAYER 5: Validation Contract (For Agent Harness)
# =============================================================================

@dataclass
class ValidationContract:
    """
    The harness uses this to verify agent output.
    All checks are deterministic given the structural model.
    """
    
    # Shape invariants
    shape_assertions: list["ShapeAssertion"]
    
    # Value spot-checks (from original Excel)
    value_assertions: list["ValueAssertion"]
    
    # Computation equivalence
    formula_assertions: list["FormulaAssertion"]
    
    # Dependency ordering
    order_assertions: list["OrderAssertion"]
    
    def validate(self, generated_code: str, namespace: dict) -> "ValidationResult":
        """Execute generated code and verify all assertions"""
        ...

@dataclass
class ShapeAssertion:
    element_id: str
    expected_shape: tuple[int, ...]
    python_expression: str  # e.g., "cash_flows.shape"

@dataclass
class ValueAssertion:
    """Spot-check that specific values match Excel"""
    element_id: str
    position: tuple[int, ...] | None  # None for scalars
    expected_value: Any
    python_expression: str
    tolerance: float = 1e-9
    
@dataclass 
class FormulaAssertion:
    """
    Verify formula equivalence by testing with varied inputs.
    This catches structural errors the agent might make.
    """
    element_id: str
    test_inputs: dict[str, Any]  # python_name -> test value
    expected_outputs: dict[tuple[int, ...], Any]  # position -> expected
    tolerance: float = 1e-9

@dataclass
class OrderAssertion:
    """Verify computation order respects dependencies"""
    before: str  # python_name that must be computed first
    after: str  # python_name that depends on it

@dataclass
class ValidationResult:
    passed: bool
    shape_results: dict[str, bool]
    value_results: dict[str, tuple[bool, Any, Any]]  # (passed, expected, actual)
    formula_results: dict[str, tuple[bool, str]]  # (passed, error_msg)
    order_results: dict[str, bool]
    
    def summary(self) -> str:
        ...
    
    def failing_assertions(self) -> list[str]:
        ...
```

## Date Handling Strategy

Since you said "just handle it," here's the approach baked into the structures:

```python
# In the parser (Layer 1 -> Layer 2)
def infer_date_handling(array: InferredArray, raw: RawWorkbook) -> None:
    """Detect and configure date handling"""
    
    # Check number formats for date indicators
    sample_cells = get_sample_cells(array.range_ref, raw)
    date_formats = [c.number_format for c in sample_cells if c.is_date]
    
    if not date_formats:
        return
    
    # Determine which axis is time
    if array.col_headers:
        # Check if headers parse as dates
        parsed_dates = try_parse_dates(array.col_headers)
        if parsed_dates:
            array.time_axis = "col"
            array.time_origin = parsed_dates[0]
            array.time_freq = infer_frequency(parsed_dates)
    
    if array.row_headers and array.time_axis is None:
        parsed_dates = try_parse_dates(array.row_headers)
        if parsed_dates:
            array.time_axis = "row"
            array.time_origin = parsed_dates[0]
            array.time_freq = infer_frequency(parsed_dates)

def infer_frequency(dates: list) -> str:
    """Infer pandas frequency string from date sequence"""
    if len(dates) < 2:
        return None
    
    deltas = [(dates[i+1] - dates[i]).days for i in range(len(dates)-1)]
    median_delta = sorted(deltas)[len(deltas)//2]
    
    if 28 <= median_delta <= 31:
        return 'ME'  # Month end
    elif 89 <= median_delta <= 92:
        return 'QE'  # Quarter end
    elif 365 <= median_delta <= 366:
        return 'YE'  # Year end
    elif median_delta == 1:
        return 'D'
    elif median_delta == 7:
        return 'W'
    else:
        return None  # Irregular

# In translation (Layer 4)
def generate_date_index(spec: TranslationSpec, array: InferredArray) -> str:
    """Generate pandas DatetimeIndex creation code"""
    
    if array.time_freq and array.time_origin:
        # Can use pd.date_range
        n_periods = array.shape[1] if array.time_axis == "col" else array.shape[0]
        return f"pd.date_range(start='{array.time_origin}', periods={n_periods}, freq='{array.time_freq}')"
    else:
        # Fall back to explicit dates from headers
        dates = array.col_headers if array.time_axis == "col" else array.row_headers
        return f"pd.to_datetime({dates!r})"
```

## Numpy vs Pandas Decision Logic

```python
def choose_container(array: InferredArray, graph: DependencyGraph) -> PythonContainer:
    """
    Deterministic rules for numpy vs pandas.
    """
    rows, cols = array.shape
    
    # Rule 1: Scalars
    if rows == 1 and cols == 1:
        return PythonContainer.SCALAR
    
    # Rule 2: Small vectors stay numpy
    if rows == 1 and cols <= 50:
        return PythonContainer.NUMPY
    if cols == 1 and rows <= 50:
        return PythonContainer.NUMPY
    
    # Rule 3: Time series with labeled index -> pandas
    if array.time_axis is not None:
        if array.shape[0] > 100 or array.shape[1] > 100:
            return PythonContainer.PANDAS_DATAFRAME
    
    # Rule 4: Has meaningful row AND column headers -> DataFrame
    if array.row_headers and array.col_headers:
        if len(set(array.row_headers)) == len(array.row_headers):  # Unique
            return PythonContainer.PANDAS_DATAFRAME
    
    # Rule 5: Tall arrays (likely entity dimension) -> DataFrame
    if rows > 200:
        return PythonContainer.PANDAS_DATAFRAME
    
    # Rule 6: Used in lookups -> DataFrame for .loc access
    if any(is_lookup_target(array.id, graph) for _ in [1]):
        return PythonContainer.PANDAS_DATAFRAME
    
    # Default: numpy for computation efficiency
    return PythonContainer.NUMPY
```

## Agent Harness Interface

Here's how the agent would interact with this:

```python
class ModelTranslationHarness:
    """
    The deterministic cage the AI agent operates within.
    """
    
    def __init__(self, excel_path: str):
        # Layer 1: Parse (fully deterministic)
        self.raw = parse_workbook(excel_path)
        
        # Layer 2: Infer structure (fully deterministic)
        self.structure = infer_structure(self.raw)
        
        # Layer 5: Generate validation contract (fully deterministic)
        self.contract = generate_validation_contract(self.structure)
        
        # Layer 3: Semantic model starts empty, agent fills it
        self.semantic = SemanticModel(
            structure=self.structure,
            annotations={},
            relationships=[],
            intents={},
            model_description="",
            identified_scenarios=None,
            annotation_coverage=0.0,
            consistency_issues=[]
        )
    
    # === Tools the agent can call ===
    
    def get_structure_summary(self) -> str:
        """Agent calls this to understand what it's working with"""
        return format_structure_for_agent(self.structure)
    
    def get_element_detail(self, element_id: str) -> str:
        """Agent calls this to examine a specific array/scalar"""
        element = self.structure.get_element(element_id)
        return format_element_for_agent(element, self.raw)
    
    def get_formula_sample(self, element_id: str, n: int = 3) -> str:
        """Agent calls this to see actual formulas"""
        ...
    
    def annotate_element(
        self, 
        element_id: str, 
        python_name: str,
        business_name: str,
        description: str,
        domain_category: str,
        confidence: float,
        reasoning: str
    ) -> "AnnotationResult":
        """
        Agent submits an annotation.
        Harness validates it against structure.
        """
        # Validate python_name is valid identifier
        if not python_name.isidentifier():
            return AnnotationResult(
                accepted=False, 
                error=f"'{python_name}' is not a valid Python identifier"
            )
        
        # Check for name collisions
        existing_names = {a.suggested_python_name for a in self.semantic.annotations.values()}
        if python_name in existing_names:
            return AnnotationResult(
                accepted=False,
                error=f"'{python_name}' already used for another element"
            )
        
        # Store annotation
        self.semantic.annotations[element_id] = SemanticAnnotation(
            element_id=element_id,
            suggested_python_name=python_name,
            business_name=business_name,
            description=description,
            business_logic="",
            domain_category=domain_category,
            confidence=confidence,
            reasoning=reasoning
        )
        
        return AnnotationResult(accepted=True, error=None)
    
    def add_relationship(
        self,
        source_id: str,
        target_id: str, 
        relationship_type: str,
        description: str
    ) -> "RelationshipResult":
        """Agent describes relationship between elements"""
        
        # Validate: must match actual dependency in graph
        if not self.structure.graph.has_edge(source_id, target_id):
            # Allow semantic relationships that aren't direct formula deps
            # but flag them
            pass
        
        self.semantic.relationships.append(SemanticRelationship(
            source_id=source_id,
            target_id=target_id,
            relationship_type=relationship_type,
            description=description
        ))
        
        return RelationshipResult(accepted=True)
    
    def generate_code(self) -> "GenerationResult":
        """
        Agent requests code generation.
        Uses Layer 4 translation, deterministic given annotations.
        """
        # Check annotation coverage
        coverage = len(self.semantic.annotations) / len(self.structure.all_elements())
        if coverage < 0.8:
            return GenerationResult(
                success=False,
                error=f"Insufficient annotation coverage: {coverage:.1%}. Need 80%+",
                code=None
            )
        
        # Generate translation plan (deterministic)
        plan = generate_translation_plan(self.semantic)
        
        # Generate code (deterministic given plan)
        code = generate_python_code(plan)
        
        return GenerationResult(success=True, error=None, code=code)
    
    def validate_code(self, code: str) -> ValidationResult:
        """
        Agent submits code for validation against contract.
        """
        # Execute in sandbox
        namespace = {}
        try:
            exec(code, namespace)
        except Exception as e:
            return ValidationResult(
                passed=False,
                execution_error=str(e),
                shape_results={},
                value_results={},
                formula_results={},
                order_results={}
            )
        
        # Run contract assertions
        return self.contract.validate(code, namespace)
    
    def get_validation_hints(self, result: ValidationResult) -> str:
        """
        When validation fails, provide structured hints.
        Agent uses these to fix its annotations.
        """
        hints = []
        
        for element_id, (passed, expected, actual) in result.value_results.items():
            if not passed:
                hints.append(
                    f"Value mismatch for {element_id}: "
                    f"expected {expected}, got {actual}. "
                    f"Check formula translation or dependency order."
                )
        
        # ... more hint generation
        
        return "\n".join(hints)
```

## Example Flow

Here's what an actual workbook processing might look like:

```python
# Harness parses and structures automatically
harness = ModelTranslationHarness("loan_model.xlsx")

# Agent examines structure
print(harness.get_structure_summary())
# Output:
# === Arrays (5) ===
# - arr_001: Sheet1!B5:M5 (1x12) [float64, formulas, TIME_HORIZONTAL]
#   Headers: col=['Jan-24','Feb-24',...], row=['Monthly Payment']
#   Template: "=PMT($B$2/12,$B$3,-$B$1)" (100% coverage)
#   Role: OUTPUT
#
# - arr_002: Sheet1!B6:M6 (1x12) [float64, formulas, TIME_HORIZONTAL]  
#   Headers: col=['Jan-24','Feb-24',...], row=['Interest']
#   Template: "={prev_row}*$B$2/12" (100% coverage)
#   Role: INTERMEDIATE
# ...
#
# === Scalars (3) ===
# - scl_001: Sheet1!B1 "Loan Amount" = 100000 [float64, PARAMETER]
# - scl_002: Sheet1!B2 "Annual Rate" = 0.06 [float64, PARAMETER]
# - scl_003: Sheet1!B3 "Term (months)" = 360 [int64, PARAMETER]
#
# === Dependencies ===
# scl_001 -> arr_001, arr_002, arr_003
# scl_002 -> arr_001, arr_002
# ...

# Agent annotates
harness.annotate_element(
    element_id="scl_001",
    python_name="loan_principal",
    business_name="Loan Principal Amount", 
    description="Initial loan amount to be amortized",
    domain_category="loan_parameter",
    confidence=0.95,
    reasoning="Cell labeled 'Loan Amount', used in PMT and balance calculations"
)

# ... more annotations ...

# Agent requests code generation
result = harness.generate_code()

# Harness validates
validation = harness.validate_code(result.code)

if not validation.passed:
    hints = harness.get_validation_hints(validation)
    # Agent iterates based on hints
```

## What Do You Think?

This structure gives you:

1. **Clear separation of concerns** - Parsing, inference, annotation, translation, validation are distinct
2. **Deterministic grounding** - Agent can only annotate what exists; can't hallucinate structure
3. **Verifiable output** - Contract-based validation catches errors automatically
4. **Iterative refinement** - Agent can see exactly why validation failed

Want me to sketch out the actual parser implementation for Layer 1 -> Layer 2, or dive deeper into any particular aspect?
