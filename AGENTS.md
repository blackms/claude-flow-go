# AGENTS.md

## Cursor Cloud specific instructions

This is a pure Go project (Go 1.22+) with no external service dependencies for development. SQLite is embedded as a pure-Go library.

### Key commands

See `Makefile` for all targets. Highlights:

| Action | Command |
|--------|---------|
| Build all | `make build` or `go build ./...` |
| Run tests | `make test` or `go test ./...` |
| Lint | `make lint` (requires golangci-lint) |
| Vet | `make vet` or `go vet ./...` |
| Build + test + lint | `make all` |
| System diagnostics | `./claude-flow doctor` |

### Notes

- No external services (PostgreSQL, IPFS, GCS) are needed for core development. The `ruvector` commands require PostgreSQL+pgvector but are optional.
- The binary entrypoint is `cmd/claude-flow/main.go`. CLI commands are in `cmd/claude-flow/commands/`.
- Hive-mind state persists to `~/.claude-flow/hivemind/state.json` and auto-restores on subsequent CLI invocations. Use `hive-mind shutdown` to clear persisted state.
- Many commands support `--format json` for structured output (hive-mind status, task, neural status, doctor, hooks, etc.).
- CI runs on GitHub Actions (`.github/workflows/ci.yml`) with Go 1.22/1.23 matrix and golangci-lint.
