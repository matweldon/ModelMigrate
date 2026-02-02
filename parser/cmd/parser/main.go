// Command parser is the CLI entrypoint for the ModelMigrate parser.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/matweldon/modelmigrate/parser/pkg/inference"
	"github.com/matweldon/modelmigrate/parser/pkg/model"
	"github.com/matweldon/modelmigrate/parser/pkg/xlsx"
)

var algoVersion string

func main() {
	// Parse command line flags
	inputFile := flag.String("input", "", "Path to xlsx file to parse")
	outputFile := flag.String("output", "", "Path to output JSON file (default: stdout)")
	pretty := flag.Bool("pretty", true, "Pretty-print JSON output")
	mode := flag.String("mode", "structural", "Output mode: 'raw' (Layer 1), 'structural' (Layer 2), or 'summary' (human-readable)")
	flag.StringVar(&algoVersion, "algo", "v2", "Array detection algorithm: 'v1' or 'v2' (column-first)")
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -input flag is required")
		fmt.Fprintln(os.Stderr, "Usage: parser -input <file.xlsx> [-output <file.json>] [-pretty=true] [-mode=structural] [-algo=v2]")
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
	case "summary":
		structural := runInference(workbook)
		output = nil // No JSON output for summary mode
		summary = formatDetailedSummary(structural)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown mode '%s'. Use 'raw', 'structural', or 'summary'\n", *mode)
		os.Exit(1)
	}

	// Serialize to JSON (skip for summary mode)
	if output != nil {
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
	}

	// Print summary (to stderr for JSON modes, stdout for summary mode)
	if *mode == "summary" {
		fmt.Print(summary)
	} else {
		fmt.Fprint(os.Stderr, summary)
	}
}

func runInference(workbook *model.RawWorkbook) *model.StructuralModel {
	// Build dependency graph
	graph, parsedFormulas := inference.BuildDependencyGraph(workbook)

	// Detect arrays using selected algorithm
	var arrays []*model.InferredArray
	if algoVersion == "v2" {
		detector := inference.NewArrayDetectorV2(workbook, parsedFormulas)
		arrays = detector.DetectArrays()
	} else {
		detector := inference.NewArrayDetector(workbook, parsedFormulas)
		arrays = detector.DetectArrays()
	}

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

func formatDetailedSummary(structural *model.StructuralModel) string {
	var sb strings.Builder

	sb.WriteString("═══════════════════════════════════════════════════════════════════\n")
	sb.WriteString("                    MODELMIGRATE STRUCTURAL ANALYSIS\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════════\n\n")

	// Overview
	sb.WriteString("OVERVIEW\n")
	sb.WriteString("────────────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Sheets:           %d\n", len(structural.Source.SheetOrder)))
	sb.WriteString(fmt.Sprintf("  Arrays:           %d\n", len(structural.Arrays)))
	sb.WriteString(fmt.Sprintf("  Scalars:          %d\n", len(structural.Scalars)))
	sb.WriteString(fmt.Sprintf("  Named ranges:     %d\n", len(structural.Source.NamedRanges)))
	sb.WriteString(fmt.Sprintf("  Formula cells:    %d\n", structural.Coverage.TotalFormulaCells))
	sb.WriteString(fmt.Sprintf("  Dependency edges: %d\n\n", len(structural.Graph.Edges)))

	// Data roles summary
	roleCounts := make(map[model.DataRole]int)
	for _, arr := range structural.Arrays {
		roleCounts[arr.DataRole]++
	}
	sb.WriteString("DATA ROLES\n")
	sb.WriteString("────────────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  INPUT:        %3d  (data entry cells without formulas)\n", roleCounts[model.RoleInput]))
	sb.WriteString(fmt.Sprintf("  PARAMETER:    %3d  (input cells referenced by formulas)\n", roleCounts[model.RoleParameter]))
	sb.WriteString(fmt.Sprintf("  INTERMEDIATE: %3d  (formula cells used by other formulas)\n", roleCounts[model.RoleIntermediate]))
	sb.WriteString(fmt.Sprintf("  OUTPUT:       %3d  (final formula results)\n\n", roleCounts[model.RoleOutput]))

	// Per-sheet breakdown
	sb.WriteString("SHEETS\n")
	sb.WriteString("────────────────────────────────────────────────────────────────────\n")

	// Group arrays by sheet
	bySheet := make(map[string][]*model.InferredArray)
	for _, arr := range structural.Arrays {
		bySheet[arr.RangeRef.Sheet] = append(bySheet[arr.RangeRef.Sheet], arr)
	}

	for _, sheetName := range structural.Source.SheetOrder {
		sheet := structural.Source.Sheets[sheetName]
		arrays := bySheet[sheetName]

		// Count formulas in sheet
		formulaCount := 0
		for _, cell := range sheet.Cells {
			if cell.Formula != "" {
				formulaCount++
			}
		}

		// Array size stats
		totalCells := 0
		singleCells := 0
		for _, arr := range arrays {
			rows := arr.RangeRef.BottomRight[0] - arr.RangeRef.TopLeft[0] + 1
			cols := arr.RangeRef.BottomRight[1] - arr.RangeRef.TopLeft[1] + 1
			size := rows * cols
			totalCells += size
			if size == 1 {
				singleCells++
			}
		}

		sb.WriteString(fmt.Sprintf("\n  %s\n", sheetName))
		sb.WriteString(fmt.Sprintf("    Cells: %d, Formulas: %d\n", len(sheet.Cells), formulaCount))
		sb.WriteString(fmt.Sprintf("    Arrays: %d (%d single-cell)\n", len(arrays), singleCells))

		// Show largest arrays
		if len(arrays) > 0 {
			// Sort by size
			sorted := make([]*model.InferredArray, len(arrays))
			copy(sorted, arrays)
			sort.Slice(sorted, func(i, j int) bool {
				si := (sorted[i].RangeRef.BottomRight[0] - sorted[i].RangeRef.TopLeft[0] + 1) *
					(sorted[i].RangeRef.BottomRight[1] - sorted[i].RangeRef.TopLeft[1] + 1)
				sj := (sorted[j].RangeRef.BottomRight[0] - sorted[j].RangeRef.TopLeft[0] + 1) *
					(sorted[j].RangeRef.BottomRight[1] - sorted[j].RangeRef.TopLeft[1] + 1)
				return si > sj
			})

			sb.WriteString("    Largest arrays:\n")
			for i, arr := range sorted[:min(3, len(sorted))] {
				rows := arr.RangeRef.BottomRight[0] - arr.RangeRef.TopLeft[0] + 1
				cols := arr.RangeRef.BottomRight[1] - arr.RangeRef.TopLeft[1] + 1
				formula := ""
				if arr.HasFormulas {
					formula = " [formulas]"
				}
				headers := ""
				if len(arr.ColHeaders) > 0 {
					headers = fmt.Sprintf(" headers=%v", arr.ColHeaders[:min(2, len(arr.ColHeaders))])
				}
				sb.WriteString(fmt.Sprintf("      %d. %s: %dx%d %s%s%s\n",
					i+1, arr.ID, rows, cols, arr.DataRole, formula, headers))
			}
		}
	}

	// Formula template coverage
	sb.WriteString("\n\nFORMULA TEMPLATES\n")
	sb.WriteString("────────────────────────────────────────────────────────────────────\n")

	withTemplates := 0
	perfect := 0
	lowCoverage := []*model.InferredArray{}

	for _, arr := range structural.Arrays {
		if arr.FormulaTemplate != nil && arr.FormulaTemplate.TemplateStr != "" {
			withTemplates++
			if arr.FormulaTemplate.Coverage == 1.0 {
				perfect++
			} else if arr.FormulaTemplate.Coverage < 0.5 {
				lowCoverage = append(lowCoverage, arr)
			}
		}
	}

	sb.WriteString(fmt.Sprintf("  Arrays with formulas: %d\n", withTemplates))
	sb.WriteString(fmt.Sprintf("  100%% coverage:        %d\n", perfect))
	sb.WriteString(fmt.Sprintf("  <50%% coverage:        %d\n", len(lowCoverage)))

	if len(lowCoverage) > 0 {
		sb.WriteString("\n  Arrays with low template coverage (may contain errors or variations):\n")
		sort.Slice(lowCoverage, func(i, j int) bool {
			return lowCoverage[i].FormulaTemplate.Coverage < lowCoverage[j].FormulaTemplate.Coverage
		})
		for _, arr := range lowCoverage[:min(5, len(lowCoverage))] {
			rows := arr.RangeRef.BottomRight[0] - arr.RangeRef.TopLeft[0] + 1
			cols := arr.RangeRef.BottomRight[1] - arr.RangeRef.TopLeft[1] + 1
			template := arr.FormulaTemplate.TemplateStr
			if len(template) > 40 {
				template = template[:40] + "..."
			}
			sb.WriteString(fmt.Sprintf("    %s (%s, %dx%d): %.0f%% coverage\n",
				arr.ID, arr.RangeRef.Sheet, rows, cols, arr.FormulaTemplate.Coverage*100))
			sb.WriteString(fmt.Sprintf("      Formula: %s\n", template))
		}
	}

	sb.WriteString("\n═══════════════════════════════════════════════════════════════════\n")

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
