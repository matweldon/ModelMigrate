## Task Tracking

Use TODO.md to track the current state of tasks:
- Update TODO.md at the start of each session with current priorities
- Mark tasks as complete ([x]) when done
- Add new tasks discovered during implementation
- Keep it concise - detailed notes go in LOG.md

## Logging

Keep an agent's log in LOG.md with the date and key information such as what was implemented, anything that went wrong, current test coverage etc.
Feel free to add sections like 'TIL's and reflect on anything you want to reflect on. Try to do this regularly, little and often
The log may grow large so be careful about reading it all into context.

## Python

* Use uv (uv add, uv sync). Not uv pip install or uv venv
* Add full docstrings to all public functions, with Google style and examples where appropriate
* Always add type annotations
* Always write tests for every function or method using pytest. If it's too difficult to make it pass write a stub
* Format with ruff - do this regularly
* Check test coverage regularly

## Go

* Write tests in `*_test.go` files alongside the code
* Run tests with `go test ./...` from the parser directory
* Check coverage with `go test ./... -cover`
* Generate detailed coverage report: `go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out`
* Use `go vet ./...` to check for common issues
* Tests run automatically via GitHub Actions on PRs

## Java/Typescript

* Vanilla preferred.
* Use svelte when a component framework is necessary
* Use bun

