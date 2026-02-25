# AGENTS.md

## Cursor Cloud specific instructions

This is a pure Go project (Go 1.22+) with no external service dependencies for development. SQLite is embedded as a pure-Go library.

### Key commands

| Action | Command |
|--------|---------|
| Build all | `go build ./...` |
| Build binary | `go build -o claude-flow ./cmd/claude-flow` |
| Run tests | `go test ./...` |
| Run tests with race detector | `go test -race ./...` |
| Lint / vet | `go vet ./...` |
| System diagnostics | `./claude-flow doctor` |

### Notes

- `go vet` reports 2 pre-existing warnings (self-assignment in `internal/domain/claims/rules.go:363` and lock copy in `internal/infrastructure/federation/configuration_guards_test.go:341`). These are not regressions.
- `claude-flow neural train` panics with "strings: negative Repeat count" due to a progress bar rendering bug — this is a known pre-existing issue.
- The hive mind state is in-memory only; `hive-mind init` state does not persist across process invocations.
- No external services (PostgreSQL, IPFS, GCS) are needed for core development. The `ruvector` commands require PostgreSQL+pgvector but are optional.
- The binary entrypoint is `cmd/claude-flow/main.go`. CLI commands are in `cmd/claude-flow/commands/`.
