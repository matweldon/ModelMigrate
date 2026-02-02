# Test Workbooks

This directory contains Excel workbooks for testing the parser.

## Files

### tag-workbook-valuing-dependent-development-workbook.xlsx
- **Source**: UK Government TAG (Transport Analysis Guidance)
- **Purpose**: Land valuation model for dependent development
- **Structure**: 9 sheets with cross-sheet references
- **Key features**:
  - Calculation formulas (Net private value, External impact, Social value)
  - Multiple data sheets (Residential, Commercial, Industrial land values)
  - Named ranges
  - Good for testing formula parsing and dependency graphs

### financial-sample.xlsx
- **Source**: Microsoft Power BI sample data
- **Purpose**: Sales and profit data by segment and country
- **Structure**: 1 sheet, 701 rows, 16 columns
- **Key features**:
  - Flat data table (no formulas)
  - Good for testing data extraction, not computational structure

## Adding New Workbooks

When adding workbooks for testing:
1. Prefer .xlsx format (xls is out of scope)
2. Look for workbooks with formulas and calculations
3. Cross-sheet references are valuable for testing dependency graphs
4. Include source attribution where possible
