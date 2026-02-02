package inference

import (
	"testing"

	"github.com/matweldon/modelmigrate/parser/pkg/model"
)

func TestBuildDependencyGraph_Simple(t *testing.T) {
	// Create a simple workbook with some formulas
	wb := model.NewRawWorkbook()
	wb.SheetOrder = []string{"Sheet1"}

	sheet := model.NewRawSheet("Sheet1")
	wb.Sheets["Sheet1"] = sheet

	// A1 = 10 (value)
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 0},
		Value: 10.0,
		Type:  model.CellTypeNumber,
	})

	// B1 = 20 (value)
	sheet.SetCell(&model.RawCell{
		Ref:   model.CellRef{Sheet: "Sheet1", Row: 0, Col: 1},
		Value: 20.0,
		Type:  model.CellTypeNumber,
	})

	// C1 = A1 + B1 (formula)
	sheet.SetCell(&model.RawCell{
		Ref:     model.CellRef{Sheet: "Sheet1", Row: 0, Col: 2},
		Value:   30.0,
		Formula: "A1+B1",
		Type:    model.CellTypeFormula,
	})

	graph, parsedFormulas := BuildDependencyGraph(wb)

	// Should have parsed formula for C1
	if _, ok := parsedFormulas["Sheet1!0,2"]; !ok {
		t.Error("expected parsed formula for Sheet1!0,2")
	}

	// Should have 3 nodes (A1, B1, C1)
	if len(graph.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(graph.Nodes))
	}

	// Should have 2 edges (A1->C1, B1->C1)
	if len(graph.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(graph.Edges))
	}
}

func TestBuildDependencyGraph_RangeReference(t *testing.T) {
	wb := model.NewRawWorkbook()
	wb.SheetOrder = []string{"Sheet1"}

	sheet := model.NewRawSheet("Sheet1")
	wb.Sheets["Sheet1"] = sheet

	// A1:A3 = values
	for i := 0; i < 3; i++ {
		sheet.SetCell(&model.RawCell{
			Ref:   model.CellRef{Sheet: "Sheet1", Row: i, Col: 0},
			Value: float64(i + 1),
			Type:  model.CellTypeNumber,
		})
	}

	// B1 = SUM(A1:A3)
	sheet.SetCell(&model.RawCell{
		Ref:     model.CellRef{Sheet: "Sheet1", Row: 0, Col: 1},
		Value:   6.0,
		Formula: "SUM(A1:A3)",
		Type:    model.CellTypeFormula,
	})

	graph, _ := BuildDependencyGraph(wb)

	// Should have 4 nodes (A1, A2, A3, B1)
	if len(graph.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(graph.Nodes))
	}

	// Should have 3 edges (A1->B1, A2->B1, A3->B1)
	if len(graph.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d: %+v", len(graph.Edges), graph.Edges)
	}

	// All edges should be "range" type
	for _, edge := range graph.Edges {
		if edge.RefType != "range" {
			t.Errorf("expected range ref type, got %q", edge.RefType)
		}
		if edge.Target != "Sheet1!0,1" {
			t.Errorf("expected target Sheet1!0,1, got %q", edge.Target)
		}
	}
}

func TestTopologicalSort_Simple(t *testing.T) {
	graph := &model.DependencyGraph{
		Nodes: []string{"A", "B", "C"},
		Edges: []model.DependencyEdge{
			{Source: "A", Target: "C"},
			{Source: "B", Target: "C"},
		},
	}

	sorted := TopologicalSort(graph)

	if sorted == nil {
		t.Fatal("TopologicalSort returned nil")
	}

	// A and B should come before C
	posA := -1
	posB := -1
	posC := -1
	for i, node := range sorted {
		switch node {
		case "A":
			posA = i
		case "B":
			posB = i
		case "C":
			posC = i
		}
	}

	if posA >= posC || posB >= posC {
		t.Errorf("invalid topological order: %v", sorted)
	}
}

func TestTopologicalSort_Chain(t *testing.T) {
	graph := &model.DependencyGraph{
		Nodes: []string{"A", "B", "C", "D"},
		Edges: []model.DependencyEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "D"},
		},
	}

	sorted := TopologicalSort(graph)

	// Should be exactly A, B, C, D in that order
	expected := []string{"A", "B", "C", "D"}
	if len(sorted) != len(expected) {
		t.Fatalf("expected %d nodes, got %d", len(expected), len(sorted))
	}
	for i, node := range expected {
		if sorted[i] != node {
			t.Errorf("position %d: expected %s, got %s", i, node, sorted[i])
		}
	}
}

func TestTopologicalSort_Cycle(t *testing.T) {
	graph := &model.DependencyGraph{
		Nodes: []string{"A", "B", "C"},
		Edges: []model.DependencyEdge{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "A"}, // Creates cycle
		},
	}

	sorted := TopologicalSort(graph)

	// Should return nil for cyclic graph
	if sorted != nil {
		t.Errorf("expected nil for cyclic graph, got %v", sorted)
	}
}

func TestTopologicalSort_Empty(t *testing.T) {
	graph := &model.DependencyGraph{
		Nodes: []string{},
		Edges: []model.DependencyEdge{},
	}

	sorted := TopologicalSort(graph)

	if len(sorted) != 0 {
		t.Errorf("expected empty result, got %v", sorted)
	}
}

func TestCellToKey(t *testing.T) {
	ref := model.CellRef{Sheet: "Data Sheet", Row: 5, Col: 3}
	key := cellToKey(ref)

	// Should be in format "Sheet!row,col"
	expected := "Data Sheet!5,3"
	if key != expected {
		t.Errorf("cellToKey = %q, want %q", key, expected)
	}
}
