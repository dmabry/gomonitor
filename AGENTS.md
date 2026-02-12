# AGENTS.md - Guidelines for AI Coding Assistants

This document provides build, lint, and test commands plus code style guidelines for working in this repository.

## Build / Lint / Test Commands

### Core Testing
```bash
go test ./...                           # Run all tests
go test -run TestName                 # Run single test by name
go test -v ./...                      # Verbose test output
```

### Code Quality Checks
```bash
go build -v ./...                       # Build with verbose output
go vet ./...                            # Static analysis checks
gofmt -l .                              # Check formatting (fails if reformat needed)
go mod verify                           # Verify module dependencies
go mod tidy                             # Tidy go.mod/go.sum files
```

### CI/CD Pipeline
The GitHub workflow at `.github/workflows/go-test.yml` runs:
1. `go mod tidy`
2. Code quality: `gofmt -l .`, `go vet ./...`, `go mod verify`
3. Build: `go build -v ./...`

## Code Style Guidelines

### Formatting & Imports
- Run code through `gofmt` before committing (use standard Go formatting)
- Group imports with a blank line between stdlib and third-party packages:
  ```go
  import (
      "fmt"
      "os"

      "github.com/dmabry/gomonitor"
  )
  ```

### Naming Conventions
- Types: `PascalCase` with descriptive names (`ExitCode`, `CheckResult`, `PerformanceMetric`)
- Method receivers: short abbreviations (e.g., `cr *CheckResult`, `ec ExitCode`)
- Variables/parameters: camelCase for local variables, consider using full words
- Constants: group related values in a single `const()` block with iota

### Type Definitions & Documentation
- Add type comments explaining the purpose of public types and their usage patterns
- Document constants describing what each exit code represents (OK=0, Warning=1, Critical=2, Unknown=3)
- Comment complex logic or non-obvious behavior in methods

### Error Handling Patterns
- Return explicit errors rather than panicking where caller may need to handle failures
- Use descriptive error messages that help with debugging: `fmt.Errorf("context: %v", err)`
- In main/CLI tools, wrap unknown states into the `Unknown` exit code when appropriate

### Performance Data & Monitoring
- Performance metrics follow Nagios plugin specification: `'label'=value[UOM];warn;crit;min;max`
- Maintain insertion order for metrics via parallel slices (`PerfOrder`, `perfIndexMap`) for predictable output

## Project Structure Overview

This is a Go library providing Nagios-compatible monitoring:
- **Main types**: `ExitCode` (OK/Warning/Critical/Unknown), `CheckResult`, `PerformanceMetric`
- **Key methods**: `NewCheckResult()`, `SetResult()`, performance data methods (`Add*`, `Update*`, `Delete*`), output methods (`FormatResult()`, `SendResult()`)
- All tests in `gomonitor_test.go` follow table-driven test pattern

## Working with this Codebase

1. Read existing source to understand current patterns before modifying
2. Run `go vet ./...` and `gofmt -l .` after making changes
3. Add tests for new functionality following the table-driven style in `*_test.go`
4. Update documentation (README.md) when API changes affect usage examples

## Version & Compatibility

- Go version: 1.24.4 (from go.mod)
- CI uses Ubuntu latest with actions/checkout@v3, actions/setup-go@v4