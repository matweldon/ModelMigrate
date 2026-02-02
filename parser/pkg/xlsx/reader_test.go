package xlsx

import (
	"os"
	"path/filepath"
	"testing"
)

// getTestDataPath returns the path to test data files
func getTestDataPath(filename string) string {
	// Try to find the data directory relative to the test
	candidates := []string{
		filepath.Join("..", "..", "..", "data", filename),
		filepath.Join("data", filename),
		filepath.Join("..", "data", filename),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join("..", "..", "..", "data", filename)
}

func TestOpen_ValidXlsx(t *testing.T) {
	testFile := getTestDataPath("smartsheet-npv-irr.xlsx")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", testFile)
	}

	reader, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	if reader == nil {
		t.Fatal("Open() returned nil reader")
	}
}

func TestOpen_NonExistentFile(t *testing.T) {
	_, err := Open("/nonexistent/path/file.xlsx")
	if err == nil {
		t.Error("Open() expected error for non-existent file")
	}
}

func TestOpen_InvalidFile(t *testing.T) {
	// Create a temp file that's not a valid xlsx
	tmpFile, err := os.CreateTemp("", "invalid-*.xlsx")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("not a valid xlsx file")
	tmpFile.Close()

	_, err = Open(tmpFile.Name())
	if err == nil {
		t.Error("Open() expected error for invalid xlsx file")
	}
}

func TestParse_SmartsheetWorkbook(t *testing.T) {
	testFile := getTestDataPath("smartsheet-npv-irr.xlsx")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", testFile)
	}

	reader, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	wb, err := reader.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Check basic structure
	if len(wb.SheetOrder) == 0 {
		t.Error("expected at least one sheet")
	}

	// Check sheets map matches order
	for _, name := range wb.SheetOrder {
		if _, ok := wb.Sheets[name]; !ok {
			t.Errorf("sheet %q in order but not in map", name)
		}
	}
}

func TestParse_TagWorkbook(t *testing.T) {
	testFile := getTestDataPath("tag-workbook-valuing-dependent-development-workbook.xlsx")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", testFile)
	}

	reader, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	wb, err := reader.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Based on LOG.md: 9 sheets, 7 named ranges
	if len(wb.SheetOrder) != 9 {
		t.Errorf("expected 9 sheets, got %d", len(wb.SheetOrder))
	}

	if len(wb.NamedRanges) != 7 {
		t.Errorf("expected 7 named ranges, got %d", len(wb.NamedRanges))
	}

	// Check Calculations sheet has formulas
	calcSheet, ok := wb.Sheets["Calculations"]
	if !ok {
		t.Fatal("Calculations sheet not found")
	}

	formulaCount := 0
	for _, cell := range calcSheet.Cells {
		if cell.Formula != "" {
			formulaCount++
		}
	}
	if formulaCount == 0 {
		t.Error("expected formulas in Calculations sheet")
	}
}

func TestParse_FcermWorkbook(t *testing.T) {
	testFile := getTestDataPath("fcerm-appraisal.xlsx")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", testFile)
	}

	reader, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	wb, err := reader.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Based on LOG.md: 8 sheets
	if len(wb.SheetOrder) != 8 {
		t.Errorf("expected 8 sheets, got %d", len(wb.SheetOrder))
	}

	// Count total formula cells - should be around 1181
	totalFormulas := 0
	for _, sheet := range wb.Sheets {
		for _, cell := range sheet.Cells {
			if cell.Formula != "" {
				totalFormulas++
			}
		}
	}
	if totalFormulas < 1000 {
		t.Errorf("expected at least 1000 formula cells, got %d", totalFormulas)
	}
}

func TestCellTypes(t *testing.T) {
	testFile := getTestDataPath("fcerm-appraisal.xlsx")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", testFile)
	}

	reader, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	wb, err := reader.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Check that we can find different cell types
	hasString := false
	hasNumber := false
	hasFormula := false

	for _, sheet := range wb.Sheets {
		for _, cell := range sheet.Cells {
			switch cell.Type {
			case "string":
				hasString = true
			case "number":
				hasNumber = true
			case "formula":
				hasFormula = true
			}
		}
		if hasString && hasNumber && hasFormula {
			break
		}
	}

	if !hasString {
		t.Error("expected to find string cells")
	}
	if !hasNumber {
		t.Error("expected to find number cells")
	}
	if !hasFormula {
		t.Error("expected to find formula cells")
	}
}
