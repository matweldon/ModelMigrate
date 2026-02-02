# ModelMigrate TODO

This file tracks the current state of tasks for the AI agent working on this project.
Updated automatically during sessions. Check LOG.md for detailed session notes.

## Current Session Tasks (Session 4 - 2026-02-02)

- [x] Create TODO.md file with current task state
- [x] Update AGENTS.md with TODO.md usage instructions
- [x] Run parser on fcerm-appraisal.xlsx and verify output
- [x] Set up Go testing with coverage
- [x] Configure GitHub Actions for CI/CD
- [x] Fix formula template extraction in v2 algorithm

## Parser Improvements

- [ ] Handle formula congruence with absolute refs ($A$1 vs A1)
- [ ] Improve formula template representation to reference arrays instead of cells
- [ ] Test stability with different cell orderings (shuffle test)
- [x] Find more complex test workbooks with formulas
- [x] Fix formula templates not being populated in v2 algorithm

## Downstream Prototypes

- [ ] Error detection: Look for common spreadsheet errors
- [ ] Python transpilation: Convert formulas to Python code
- [ ] AI annotation layer design

## Infrastructure

- [x] Added test workbooks (see data/README.md)
- [x] Add Go unit tests (model: 96%, xlsx: 87%, inference: 7%)
- [x] Set up CI/CD pipeline (GitHub Actions)
- [ ] xls format support (out of scope for now)
- [ ] Increase test coverage for inference package
