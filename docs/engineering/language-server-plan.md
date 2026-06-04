# Implemented Plan: GOWDK Language Server

Status: Implemented for the first dependency-free LSP slice.

## Context

Relevant spec: `docs/product/language-server.md`

## Assumptions

- The first language server should prioritize correctness and reuse over protocol breadth.
- Full-document text synchronization is acceptable for v0.1 because `.gwdk` files are expected to be small.
- Diagnostics may validate one open document at a time until workspace-level manifest rules exist.

## Implemented Changes

- Added in-memory language APIs so editor buffers can be checked without temporary files.
- Added a small dependency-free `internal/lsp` package for stdio JSON-RPC framing and core LSP methods.
- Added `gowdk lsp` to the CLI.
- Shared completion definitions from `internal/lang`.
- Updated README and architecture docs with the new command.

## Files Changed

- `cmd/gowdk/main.go`
- `internal/lang/*`
- `internal/lsp/*`
- `README.md`
- `docs/engineering/architecture.md`
- `docs/product/requirements.md`
- `editors/vscode/README.md`

## Data And API Impact

- Adds a CLI command: `gowdk lsp`.
- Adds internal language helper APIs for source buffers and completions.
- No public Go package API changes.
- No persisted data changes.

## Tests

- Unit: in-memory source checking and completion helpers.
- Integration: LSP initialize, diagnostics, formatting, completion, shutdown, and exit over protocol framing.
- End-to-end: not added in this slice.
- Manual: run `go run ./cmd/gowdk lsp` from an LSP client or protocol harness.

## Verification Commands

```sh
gofmt -w cmd/gowdk/main.go internal/lang/*.go internal/lsp/*.go
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove `gowdk lsp`, delete `internal/lsp`, and keep the existing CLI-backed VS Code behavior.

## Risks

- LSP protocol coverage is intentionally small and may need expansion for more editors.
- Position conversion is based on current 1-based language diagnostics and is best for ASCII-heavy source until the parser tracks richer offsets.
