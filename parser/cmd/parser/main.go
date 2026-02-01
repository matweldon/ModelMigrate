// Command parser is the CLI entrypoint for the ModelMigrate parser.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/matweldon/modelmigrate/parser/pkg/inference"
	"github.com/matweldon/modelmigrate/parser/pkg/model"
	"github.com/matweldon/modelmigrate/parser/pkg/xlsx"
)

func main() {
	// Parse command line flags
	inputFile := flag.String("input", "", "Path to xlsx file to parse")
	outputFile := flag.String("output", "", "Path to output JSON file (default: stdout)")
	pretty := flag.Bool("pretty", true, "Pretty-print JSON output")
	mode := flag.String("mode", "structural", "Output mode: 'raw' (Layer 1) or 'structural' (Layer 2)")
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -input flag is required")
		fmt.Fprintln(os.Stderr, "Usage: parser -input <file.xlsx> [-output <file.json>] [-pretty=true] [-mode=structural]")
		os.Exit(1)
	}

	// Open and parse the xlsx file
	reader, err := xlsx.Open(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer reader.Close()

	workbook, err := reader.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing workbook: %v\n", err)
		os.Exit(1)
	}

	var output interface{}
	var summary string

	switch *mode {
	case "raw":
		output = workbook
		summary = formatRawSummary(workbook)
	case "structural":
		structural := runInference(workbook)
		output = structural
		summary = formatStructuralSummary(structural)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown mode '%s'. Use 'raw' or 'structural'\n", *mode)
		os.Exit(1)
	}

	// Serialize to JSON
	var jsonBytes []byte
	if *pretty {
		jsonBytes, err = json.MarshalIndent(output, "", "  ")
	} else {
		jsonBytes, err = json.Marshal(output)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serializing to JSON: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, jsonBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Wrote output to %s\n", *outputFile)
	} else {
		fmt.Println(string(jsonBytes))
	}

	// Print summary to stderr
	fmt.Fprint(os.Stderr, summary)
}

func runInference(workbook *model.RawWorkbook) *model.StructuralModel {
	// Build dependency graph
	graph, parsedFormulas := inference.BuildDependencyGraph(workbook)

	// Detect arrays
	detector := inference.NewArrayDetector(workbook, parsedFormulas)
	arrays := detector.DetectArrays()

	// Classify data roles based on dependency analysis
	inference.ClassifyDataRoles(arrays, graph)

	// Detect scalars
	scalars := inference.DetectScalars(workbook, graph, arrays)

	// Build structural model
	structural := model.NewStructuralModel(workbook)
	structural.Graph = *graph

	for _, arr := range arrays {
		structural.Arrays[arr.ID] = arr
	}
	for _, scl := range scalars {
		structural.Scalars[scl.ID] = scl
	}

	// Calculate coverage stats
	totalFormulaCells := 0
	for _, sheet := range workbook.Sheets {
		for _, cell := range sheet.Cells {
			if cell.Formula != "" {
				totalFormulaCells++
			}
		}
	}

	cellsInArrays := 0
	for _, arr := range arrays {
		rows := arr.RangeRef.BottomRight[0] - arr.RangeRef.TopLeft[0] + 1
		cols := arr.RangeRef.BottomRight[1] - arr.RangeRef.TopLeft[1] + 1
		cellsInArrays += rows * cols
	}

	structural.Coverage = model.CoverageStats{
		TotalFormulaCells: totalFormulaCells,
		CellsInArrays:     cellsInArrays,
		CellsInScalars:    len(scalars),
		TemplateCoverage:  make(map[string]float64),
	}

	for id, arr := range structural.Arrays {
		if arr.FormulaTemplate != nil {
			structural.Coverage.TemplateCoverage[id] = arr.FormulaTemplate.Coverage
		}
	}

	// Compute topological order
	topoOrder := inference.TopologicalSort(graph)
	if topoOrder != nil {
		structural.Graph.Nodes = topoOrder
	}

	return structural
}

func formatRawSummary(workbook *model.RawWorkbook) string {
	s := "\nParsed workbook summary (Layer 1 - Raw):\n"
	s += fmt.Sprintf("  Sheets: %d\n", len(workbook.SheetOrder))
	for _, name := range workbook.SheetOrder {
		sheet := workbook.Sheets[name]
		s += fmt.Sprintf("    - %s: %d cells\n", name, len(sheet.Cells))
	}
	s += fmt.Sprintf("  Named ranges: %d\n", len(workbook.NamedRanges))
	return s
}

func formatStructuralSummary(structural *model.StructuralModel) string {
	s := "\nStructural analysis summary (Layer 2):\n"
	s += fmt.Sprintf("  Sheets: %d\n", len(structural.Source.SheetOrder))

	// Count arrays by role
	roleCounts := make(map[model.DataRole]int)
	for _, arr := range structural.Arrays {
		roleCounts[arr.DataRole]++
	}

	s += fmt.Sprintf("  Arrays detected: %d\n", len(structural.Arrays))
	s += fmt.Sprintf("    - INPUT: %d, PARAMETER: %d, INTERMEDIATE: %d, OUTPUT: %d\n",
		roleCounts[model.RoleInput], roleCounts[model.RoleParameter],
		roleCounts[model.RoleIntermediate], roleCounts[model.RoleOutput])

	// Show a sample of arrays by role
	s += "  Sample arrays:\n"
	shown := make(map[model.DataRole]int)
	for id, arr := range structural.Arrays {
		if shown[arr.DataRole] >= 2 {
			continue
		}
		rows := arr.RangeRef.BottomRight[0] - arr.RangeRef.TopLeft[0] + 1
		cols := arr.RangeRef.BottomRight[1] - arr.RangeRef.TopLeft[1] + 1
		s += fmt.Sprintf("    - %s [%s]: %s %dx%d (%s)\n", id, arr.DataRole, arr.RangeRef.Sheet, rows, cols, arr.Orientation)
		shown[arr.DataRole]++
	}

	s += fmt.Sprintf("  Scalars detected: %d\n", len(structural.Scalars))
	s += fmt.Sprintf("  Dependency graph: %d nodes, %d edges\n", len(structural.Graph.Nodes), len(structural.Graph.Edges))
	s += fmt.Sprintf("  Coverage: %d formula cells, %d in arrays, %d scalars\n",
		structural.Coverage.TotalFormulaCells, structural.Coverage.CellsInArrays, structural.Coverage.CellsInScalars)
	return s
}
