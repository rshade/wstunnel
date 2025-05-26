# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
WStunnel is a WebSocket-based reverse HTTP/HTTPS tunneling solution that enables access to services behind firewalls. It consists of:
- **wstunsrv**: Server component that runs on public internet, receives HTTP requests
- **wstuncli**: Client component that runs behind firewall, connects to server via WebSocket
- Token-based routing allows multiple clients to connect to a single server
- Supports authentication, proxies, SSL/TLS, concurrent requests, and health monitoring

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

## Architecture Patterns
- **Goroutine per request**: Each HTTP request gets its own goroutine for concurrent handling
- **Request IDs**: 16-bit IDs enable multiplexing multiple requests over single WebSocket
- **WebSocket protocol**: Uses gorilla/websocket for reliable WebSocket implementation
- **Reverse proxy pattern**: Server forwards requests through persistent tunnel to client

## Key Files
- `main.go` - Entry point, determines server vs client mode
- `tunnel/wstunsrv.go` - Server implementation (accepts HTTP, forwards via WebSocket)
- `tunnel/wstuncli.go` - Client implementation (connects WebSocket, forwards to local HTTP)
- `tunnel/ws.go` - WebSocket handling, message types, connection management
- `tunnel/log.go` - Logging configuration and setup

## Testing Approach
- Integration tests use actual HTTP servers and tunnel components
- Tests cover authentication, proxies, failures, timeouts, concurrent requests
- Use `testutil.TestLogLevel()` to control log verbosity in tests
- Port allocation uses `:0` to get random available ports

## Security Considerations
- Tokens must be at least 16 characters
- Optional password authentication per token
- Certificate validation for SSL/TLS connections
- X-Host header validation with regex whitelist
- Never log sensitive information like passwords or tokens

## Version Control
- Create version.go with `make version`
- Update CHANGELOG.md with `make changelog`

## Common Issues & Fixes
- If you encounter linting errors with `golangci-lint run`, check for unused declarations
- Go version in go.mod should be `go 1.24` (not `go 1.24.0`)
- The application has no `-version` flag, check version in the generated `version.go` file
- WebSocket ping/pong failures often indicate network issues or proxy interference
- Request timeouts can be tuned with `-timeout` flag (default 30s)