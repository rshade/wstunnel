# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands
- `make` - Build for local OS/arch
- `make build` - Build for linux/amd64, windows/amd64
- `make clean` - Remove build artifacts

## Test Commands 
- `make test` - Run all tests with coverage
- `ginkgo -focus="<test description>" ./tunnel` - Run a single test
- `ginkgo -r -noColor` - Run tests without colors

## Lint Commands
- `make lint` - Run gofmt check and go vet
- `golangci-lint run` - Run comprehensive linting checks

## Code Style Guidelines
- **Formatting**: Use gofmt, tabs for indentation
- **Imports**: Standard library first, third-party after, group related imports
- **Naming**: PascalCase for exported, camelCase for unexported, snake_case for files
- **Error Handling**: Check errors immediately, return to caller or log with context
- **Logging**: Use log15 with structured key-value pairs
- **Tests**: Use Ginkgo BDD-style tests with Gomega assertions
- **Blank Identifiers**: Don't use unused blank identifiers like `var _ fmt.Formatter`
- **Go Version**: Use `go 1.24` format in go.mod (not `go 1.24.0`)

## Version Control
- Create version.go with `make version`
- Update CHANGELOG.md with `make changelog`

## Common Issues & Fixes
- If you encounter linting errors with `golangci-lint run`, check for unused declarations
- Go version in go.mod should be `go 1.24` (not `go 1.24.0`)
- The application has no `-version` flag, check version in the generated `version.go` file