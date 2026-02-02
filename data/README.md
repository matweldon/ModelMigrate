# Test Workbooks

This directory contains Excel workbooks for testing the parser.

## Files

### tag-workbook-valuing-dependent-development-workbook.xlsx
- **Source**: UK Government TAG (Transport Analysis Guidance)
- **Purpose**: Land valuation model for dependent development
- **Complexity**: 9 sheets, 340 arrays, 6 formula cells, 14 dependency edges
- **Roles**: INPUT (337), PARAMETER (1), INTERMEDIATE (2)
- **Best for**: Testing cross-sheet references, phantom cell detection, header association

### fcerm-appraisal.xlsx
- **Source**: UK Environment Agency
- **Download**: https://assets.publishing.service.gov.uk/media/6282186bd3bf7f1f45380252/supporting-spreadsheet-FCERM-economic-appraisal.xlsx
- **Purpose**: Flood and Coastal Erosion Risk Management economic appraisal
- **Complexity**: 8 sheets, 551 arrays, 1,181 formula cells, 86,675 dependency edges
- **Roles**: INPUT, PARAMETER, INTERMEDIATE, OUTPUT all present
- **Best for**: Testing complex multi-sheet models with rich dependency graphs

### smartsheet-npv-irr.xlsx
- **Source**: Smartsheet
- **Download**: https://www.smartsheet.com/sites/default/files/IC-Net-Provit-Value-NPV-and-Internal-Rate-of-Return-IRR-Calculator-9436.xlsx
- **Purpose**: Net Present Value and Internal Rate of Return calculations
- **Complexity**: 3 sheets, 54 arrays, 24 formula cells, 510 dependency edges
- **Roles**: INPUT, INTERMEDIATE, OUTPUT present
- **Best for**: Testing financial functions (NPV, IRR, XNPV)

### babson-bloomberg.xlsx
- **Source**: Babson College Cutler Center
- **Download**: https://www.babson.edu/media/babson/assets/cutler-center/Bloomberg-Practice-Template.xlsx
- **Purpose**: Discounted Cash Flow valuation model
- **Complexity**: 1 sheet, 64 arrays, 79 formula cells, 174 dependency edges
- **Roles**: INPUT, INTERMEDIATE, OUTPUT present
- **Best for**: Testing DCF calculations and simpler single-sheet models

## Getting Summary Stats

The parser prints a summary at the end of stdout. Use `tail` to get stats without loading the full JSON:

```bash
cd parser
go run ./cmd/parser -input ../data/fcerm-appraisal.xlsx -mode structural -algo v2 2>&1 | tail -10
```

## Adding New Workbooks

When adding workbooks for testing:
1. Prefer .xlsx format (xls is out of scope)
2. Look for workbooks with formulas and calculations
3. Cross-sheet references are valuable for testing dependency graphs
4. Include source attribution and download URL where possible
