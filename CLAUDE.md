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
- `go test -run="<test name>" ./tunnel` - Run a single test
- `go test ./...` - Run all tests in the project

## Format Commands
- `make format` - Format all Go files with gofmt
- `markdownlint --fix README.md docs/*.md` - Auto-fix markdown formatting issues

## Lint Commands
- `make lint` - Run gofmt check, go vet, golangci-lint, and yamllint
- `golangci-lint run` - Run comprehensive linting checks
- `yamllint .github/workflows/` - Check YAML formatting in GitHub workflows
- `markdownlint README.md docs/*.md` - Check markdown formatting

**IMPORTANT** Always run `make format`, `make lint`, and `make test` after making code changes

**TIP**: Use `markdownlint --fix` to automatically fix markdown formatting issues instead of manually checking/fixing markdown

## CodeRabbit CLI Integration
- **IMPORTANT**: CodeRabbit.ai does not provide a standalone CLI tool - it operates through GitHub/GitLab integrations
- Available CodeRabbit commands are used within pull request comments: @coderabbitai full review, @coderabbitai resolve, @coderabbitai pause, @coderabbitai resume
- For automated fix application, use the local `~/bin/coderabbit-fix` tool which parses CodeRabbit comments
- **Markdown Issues**: Always attempt to fix markdown problems with `markdownlint --fix` before manual editing

## Code Style Guidelines
- **Formatting**: Use gofmt, tabs for indentation
- **Imports**: Standard library first, third-party after, group related imports
- **Naming**: PascalCase for exported, camelCase for unexported, snake_case for files
- **Error Handling**: Check errors immediately, return to caller or log with context
- **Logging**: Use log15 with structured key-value pairs
- **Tests**: Use standard Go testing with table-driven tests
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
- `tunnel/admin_service.go` - Admin API endpoints for monitoring and auditing
- `tunnel/admin_ui.go` - Web-based admin dashboard interface

## Testing Approach
- Integration tests use actual HTTP servers and tunnel components
- Tests cover authentication, proxies, failures, timeouts, concurrent requests
- Use `testutil.TestLogLevel()` to control log verbosity in tests
- Port allocation uses `:0` to get random available ports
- Use standard Go testing package with table-driven tests
- Test files should be named `*_test.go` and placed alongside the code they test
- **IMPORTANT**: Do NOT use Ginkgo/Gomega testing frameworks - use standard Go testing only
- **NEVER** create or convert tests to use Ginkgo - always use the standard testing package

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

## Configuration Options
- **Max Requests Per Tunnel**: Use `-max-requests-per-tunnel N` to limit queued requests per tunnel (default: 20)
- **Max Clients Per Token**: Use `-max-clients-per-token N` to limit concurrent clients per token (default: 0/unlimited)
- **Base Path**: Use `-base-path /path` to run behind reverse proxies with path-based routing (e.g., `-base-path /wstunnel`)
- When a tunnel reaches the max request limit, new requests return "too many requests in-flight, tunnel broken?"
- When a token reaches the max client limit, new connections return HTTP 429 "Maximum number of clients reached"
- Client counts are automatically decremented when clients disconnect
- Base path configuration automatically prefixes all endpoints (/_tunnel, /_health_check, /_stats, /_token/, /admin/*) with the specified path

## Admin Interface
- **Web UI**: Access at `http://localhost:8080/admin/ui` (or with base path: `/wstunnel/admin/ui`)
- **Quick Access**: Navigate to `/admin` to be redirected to the UI
- **Real-time Monitoring**: Auto-refreshing dashboard with tunnel status and metrics
- **Search & Filter**: Find tunnels by token, IP, or hostname
- **API Documentation**: Self-documenting API at `/admin/api-docs`
- **API Endpoints**: `/admin/monitoring` (metrics), `/admin/auditing` (detailed tunnel info)
- **No Configuration**: Admin interface works automatically when server starts

## CodeRabbit Review Settings
The project uses CodeRabbit for automated code reviews (see `.coderabbit.yaml`). When writing code, ensure compliance with:
- **Go conventions**: Use gofmt, organize imports (stdlib first), proper error handling
- **Security**: Never log passwords/tokens, validate certificates, prevent timing attacks
- **Testing**: Use standard Go testing with table-driven tests, cover edge cases
- **Path-specific rules**: WebSocket code must follow patterns in tunnel/ws.go, use goroutine-per-request
- **Excluded paths**: vendor/, build/, node_modules/, generated code, coverage.txt are not reviewed
- CodeRabbit auto-approves dependency updates from Renovate and documentation-only changes

## CodeRabbit Fix Tool
Use `~/bin/coderabbit-fix` to automatically apply CodeRabbit suggestions:
- `coderabbit-fix 153 --ai-format` - Generate AI-formatted prompts from PR 153
- `coderabbit-fix 153` - Apply all fixes from PR 153
- `coderabbit-fix 153 --dry-run` - Show what would be changed without applying
- The tool extracts detailed instructions from CodeRabbit comments including "Prompt for AI Agents" sections
- Always run `make format`, `make lint` and `make test` after applying fixes to ensure code quality

## CodeRabbit Best Practices (Learned from PR #158)
To minimize CodeRabbit issues and maintain high code quality, follow these critical practices:

### Database and Context Management
- **Always use context.Context**: Add `ctx context.Context` parameter to all database methods (Exec → ExecContext, Query → QueryContext)
- **Goroutine lifecycle management**: Use channels (done chan struct{}) and sync.WaitGroup for proper goroutine termination
- **Resource cleanup**: Ensure all goroutines can be gracefully stopped via Close() methods
- **Input validation**: Validate all input parameters for length, required fields, and format before database operations
- **Row validation**: Check `RowsAffected()` after UPDATE/DELETE operations to ensure they actually modified records

### Testing Excellence
- **Extract test helpers**: Create `setupTestX()` functions to reduce code duplication across test files
- **Specific assertions**: Test for specific expected values/tables instead of just counting (e.g., check table names exist vs count > 2)
- **Error scenarios**: Always include test cases for error conditions, not just happy paths
- **Table-driven tests**: Use standard Go testing patterns with subtests for comprehensive coverage

### Markdown and Documentation
- **Always run `markdownlint --fix`**: After editing ANY markdown file, immediately run this command
- **Trailing newlines**: All files must end with a single newline character (markdownlint enforces this)
- **Blank lines**: Ensure proper spacing around code blocks and lists in markdown
- **Auto-fix first**: Use automated tools before manual fixes to prevent formatting inconsistencies

### Go Code Quality
- **Method signatures**: Include context parameters in all I/O operations (database, HTTP, file)
- **Error handling**: Return meaningful errors with context, check all error return values
- **Concurrent safety**: Use proper mutex locking for shared resources
- **Resource management**: Always pair resource allocation with cleanup (defer Close(), cancel contexts)
- **Validation first**: Validate inputs at method entry points before processing

### Development Workflow
- **Pre-commit checks**: Always run `make format`, `make lint`, `make test` before committing
- **Incremental testing**: Run focused tests during development (`go test -run TestSpecificFunc`)
- **Coverage maintenance**: Monitor test coverage to ensure it doesn't decrease
- **Review automation**: Set CodeRabbit to "chill" mode to reduce aggressive nitpicking while maintaining quality

### Common Anti-Patterns to Avoid
- **Goroutine leaks**: Starting goroutines without termination mechanisms
- **Missing context**: Database operations without context support
- **Hardcoded sleeps**: Using time.Sleep instead of proper synchronization
- **Unchecked updates**: Database updates without verifying affected rows
- **Manual markdown**: Editing markdown without running markdownlint --fix afterward
- **Test duplication**: Copy-pasting test setup code instead of extracting helpers