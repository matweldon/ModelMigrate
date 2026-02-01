// Command parser is the CLI entrypoint for the ModelMigrate parser.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/matweldon/modelmigrate/parser/pkg/xlsx"
)

func main() {
	// Parse command line flags
	inputFile := flag.String("input", "", "Path to xlsx file to parse")
	outputFile := flag.String("output", "", "Path to output JSON file (default: stdout)")
	pretty := flag.Bool("pretty", true, "Pretty-print JSON output")
	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -input flag is required")
		fmt.Fprintln(os.Stderr, "Usage: parser -input <file.xlsx> [-output <file.json>] [-pretty=true]")
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

	// Serialize to JSON
	var jsonBytes []byte
	if *pretty {
		jsonBytes, err = json.MarshalIndent(workbook, "", "  ")
	} else {
		jsonBytes, err = json.Marshal(workbook)
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
	fmt.Fprintf(os.Stderr, "\nParsed workbook summary:\n")
	fmt.Fprintf(os.Stderr, "  Sheets: %d\n", len(workbook.SheetOrder))
	for _, name := range workbook.SheetOrder {
		sheet := workbook.Sheets[name]
		fmt.Fprintf(os.Stderr, "    - %s: %d cells\n", name, len(sheet.Cells))
	}
	fmt.Fprintf(os.Stderr, "  Named ranges: %d\n", len(workbook.NamedRanges))
}
