// Package inference provides structural inference for Excel workbooks.
package inference

import (
	"fmt"
	"sort"

	"github.com/matweldon/modelmigrate/parser/pkg/model"
	"github.com/matweldon/modelmigrate/parser/pkg/xlsx"
)

// BuildDependencyGraph constructs a dependency graph from a raw workbook.
// Returns a map of cell keys to their parsed formulas and the dependency graph.
func BuildDependencyGraph(workbook *model.RawWorkbook) (*model.DependencyGraph, map[string]*xlsx.ParsedFormula) {
	graph := &model.DependencyGraph{
		Nodes: []string{},
		Edges: []model.DependencyEdge{},
	}

	// Map to store parsed formulas
	parsedFormulas := make(map[string]*xlsx.ParsedFormula)

	// Set to track all nodes (cells that are referenced or have formulas)
	nodeSet := make(map[string]bool)

	// First pass: collect all formula cells and parse their formulas
	for sheetName, sheet := range workbook.Sheets {
		for _, cell := range sheet.Cells {
			if cell.Formula == "" {
				continue
			}

			cellKey := cellToKey(cell.Ref)
			nodeSet[cellKey] = true

			// Parse the formula
			parsed := xlsx.ParseFormula(cell.Formula, sheetName)
			if parsed == nil {
				continue
			}
			parsedFormulas[cellKey] = parsed

			// Add edges for cell references
			for _, ref := range parsed.References {
				refKey := cellToKey(ref)
				nodeSet[refKey] = true
				graph.Edges = append(graph.Edges, model.DependencyEdge{
					Source:  refKey,
					Target:  cellKey,
					RefType: "direct",
				})
			}

			// Add edges for range references (each cell in range -> formula cell)
			for _, rangeRef := range parsed.RangeReferences {
				for row := rangeRef.TopLeft[0]; row <= rangeRef.BottomRight[0]; row++ {
					for col := rangeRef.TopLeft[1]; col <= rangeRef.BottomRight[1]; col++ {
						ref := model.CellRef{Sheet: rangeRef.Sheet, Row: row, Col: col}
						refKey := cellToKey(ref)
						nodeSet[refKey] = true
						graph.Edges = append(graph.Edges, model.DependencyEdge{
							Source:  refKey,
							Target:  cellKey,
							RefType: "range",
						})
					}
				}
			}

			// Add edges for named references
			for _, name := range parsed.NamedReferences {
				if namedRange, ok := workbook.NamedRanges[name]; ok {
					// Resolve named range to cell(s)
					refs := resolveNamedRange(*namedRange)
					for _, ref := range refs {
						refKey := cellToKey(ref)
						nodeSet[refKey] = true
						graph.Edges = append(graph.Edges, model.DependencyEdge{
							Source:  refKey,
							Target:  cellKey,
							RefType: "named",
						})
					}
				}
			}
		}
	}

	// Convert node set to sorted list
	for node := range nodeSet {
		graph.Nodes = append(graph.Nodes, node)
	}
	sort.Strings(graph.Nodes)

	return graph, parsedFormulas
}

// TopologicalSort returns nodes in dependency order (dependencies before dependents).
// Returns nil if there's a cycle.
func TopologicalSort(graph *model.DependencyGraph) []string {
	// Build adjacency list and in-degree count
	inDegree := make(map[string]int)
	adjacency := make(map[string][]string)

	for _, node := range graph.Nodes {
		inDegree[node] = 0
		adjacency[node] = []string{}
	}

	for _, edge := range graph.Edges {
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
		inDegree[edge.Target]++
	}

	// Kahn's algorithm
	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}
	sort.Strings(queue) // Deterministic order

	var result []string
	for len(queue) > 0 {
		// Pop from queue
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// Process neighbors
		for _, neighbor := range adjacency[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
				sort.Strings(queue) // Keep deterministic
			}
		}
	}

	// Check for cycle
	if len(result) != len(graph.Nodes) {
		return nil // Cycle detected
	}

	return result
}

// GetDependents returns all cells that depend on the given cell.
func GetDependents(graph *model.DependencyGraph, cellKey string) []string {
	dependents := make(map[string]bool)
	adjacency := make(map[string][]string)

	for _, edge := range graph.Edges {
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
	}

	// BFS to find all dependents
	queue := []string{cellKey}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		for _, dep := range adjacency[node] {
			if !dependents[dep] {
				dependents[dep] = true
				queue = append(queue, dep)
			}
		}
	}

	var result []string
	for dep := range dependents {
		result = append(result, dep)
	}
	sort.Strings(result)
	return result
}

// GetDependencies returns all cells that the given cell depends on.
func GetDependencies(graph *model.DependencyGraph, cellKey string) []string {
	dependencies := make(map[string]bool)

	for _, edge := range graph.Edges {
		if edge.Target == cellKey {
			dependencies[edge.Source] = true
		}
	}

	var result []string
	for dep := range dependencies {
		result = append(result, dep)
	}
	sort.Strings(result)
	return result
}

// cellToKey converts a CellRef to a string key.
func cellToKey(ref model.CellRef) string {
	return fmt.Sprintf("%s!%d,%d", ref.Sheet, ref.Row, ref.Col)
}

// resolveNamedRange attempts to resolve a named range to cell references.
// Returns empty slice if the reference is invalid (e.g., #REF!).
func resolveNamedRange(nr model.NamedRange) []model.CellRef {
	if nr.RefersTo == "" || nr.RefersTo == "#REF!" {
		return nil
	}

	// Parse the RefersTo string (e.g., "'Sheet1'!$A$1" or "'Sheet1'!$A$1:$B$10")
	// This is a simplified parser - a full implementation would use the formula parser
	parsed := xlsx.ParseFormula(nr.RefersTo, "")
	if parsed == nil {
		return nil
	}

	var refs []model.CellRef
	refs = append(refs, parsed.References...)

	for _, rangeRef := range parsed.RangeReferences {
		for row := rangeRef.TopLeft[0]; row <= rangeRef.BottomRight[0]; row++ {
			for col := rangeRef.TopLeft[1]; col <= rangeRef.BottomRight[1]; col++ {
				refs = append(refs, model.CellRef{Sheet: rangeRef.Sheet, Row: row, Col: col})
			}
		}
	}

	return refs
}
