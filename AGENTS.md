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
1. `go mod tidy -diff` (fails if go.mod/go.sum is un-tidied)
2. Code quality: `gofmt -l .` (prints unformatted files), `go vet ./...`, `go mod verify`
3. Tests: `go test -race -cover ./...`
4. Build: `go build -v ./...`

The job has a 10-minute timeout. The workflow also runs as a reusable workflow (`workflow_call`), and `release.yml` calls it as its gate, so PRs and releases always run the same checks. Change the gate in `go-test.yml` only.

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
- Maintain insertion order for metrics via the `PerfOrder` slice for predictable output
- The goal is 100% compatibility with upstream Icinga 2's plugin output parsing. Check behavior against the Icinga 2 sources (`lib/icinga/pluginutility.cpp` `ParseCheckOutput`/`SplitPerfdata`, `lib/base/perfdatavalue.cpp` `Parse`/`Format`, `lib/base/convert.cpp`, `lib/methods/pluginchecktask.cpp`), and prefer Icinga's behavior where it differs from the Nagios guidelines, as long as the output stays valid under the guidelines (e.g. `'` is stripped from labels because no encoding of it is correct for both)
- Normal output renders numbers losslessly; only `NormalizeUnits` output uses Icinga's six-decimal display format

## Project Structure Overview

This is a Go library providing Nagios-compatible monitoring:
- **Main types**: `ExitCode` (OK/Warning/Critical/Unknown), `CheckResult`, `PerformanceMetric`
- **Key methods**: `NewCheckResult()`, `SetResult()`, performance data methods (`Add*`, `Update*`, `Delete*`), output methods (`FormatResult()`, `SendResult()`)
- Tests across the `*_test.go` files follow table-driven patterns where applicable

## Working with this Codebase

1. Read existing source to understand current patterns before modifying
2. Run `go vet ./...` and `gofmt -l .` after making changes
3. Add tests for new functionality following the table-driven style in `*_test.go`
4. Update documentation (README.md) when API changes affect usage examples

## Version & Compatibility

- Go version: 1.27 (from go.mod)
- CI uses Ubuntu latest with actions/checkout@v7, actions/setup-go@v7

## Releasing a New Version

### Automated release (recommended)

Pushing a tag starts the workflow at `.github/workflows/release.yml`. It first validates the tag, then runs the quality gate from `go-test.yml` (tidy -diff, gofmt, vet, mod verify, `go test -race -cover`, build), and only then creates the GitHub release with generated notes:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Tag validation rejects the release when:
- the tag is not `vMAJOR.MINOR.PATCH` with an optional `-PRERELEASE` suffix (build metadata `+...` is not allowed in Go module versions)
- the tagged commit is not on `main`
- the major version does not match the module path (v2 and above need the `/vN` suffix in `go.mod`; v0 and v1 must not have one)

A tag with a pre-release suffix (e.g. `v1.2.0-rc.1`) is published as a GitHub pre-release, so it is not marked Latest. Only the final job has write access, and it runs no repository code.

### Manual release (fallback)

Use this only when the Release workflow failed for reasons unrelated to the code (e.g. a GitHub outage) and the tag is already pushed. Do not push the tag again: pushing it is what starts the workflow, and a second `gh release create` for the same tag fails.

```bash
# 1. Confirm the workflow did not create the release
gh release view v1.0.0 --repo dmabry/gomonitor   # must report "release not found"

# 2. Run the quality gate locally on the tagged commit
git checkout v1.0.0
go mod tidy -diff
gofmt -l .
go vet ./...
go mod verify
go test -race -cover ./...
go build -v ./...

# 3. Create the release from the existing tag (add --prerelease for -rc tags)
gh release create v1.0.0 --repo dmabry/gomonitor --verify-tag --generate-notes
```

### Versioning Guidelines

- **Patch** (e.g., v1.0.0 -> v1.0.1): Bug fixes only
- **Minor** (e.g., v1.0.0 -> v1.1.0): New features, backward compatible
- **Major** (e.g., v1.0.0 -> v2.0.0): Breaking changes require import path update (`github.com/dmabry/gomonitor/v2`)