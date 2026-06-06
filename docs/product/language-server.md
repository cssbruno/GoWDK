# Feature Spec: GOWDK Language Server

## Problem

Developers editing `.gwdk` files need live feedback from the same language tooling used by the CLI. The current VS Code extension shells out to separate CLI commands for diagnostics and formatting, which works for one editor but does not give other editors a standard integration surface.

## Goals

- Provide a Language Server Protocol entrypoint through `gowdk lsp`.
- Reuse existing lexer, parser, formatter, and compiler validation behavior.
- Support unsaved editor buffers without writing temporary files.
- Keep the first version dependency-free and small enough to evolve with the language grammar.

## Non-Goals

- Implement semantic analysis beyond the current manifest validation rules.
- Replace the site-map visualizer or file-moving VS Code commands.
- Add request-time SSR behavior or compile/codegen features.
- Add editor-specific packages to the Go compiler core.

## Users And Permissions

- Primary users: developers authoring `.gwdk` pages and components.
- Roles or permissions: local editor process only.
- Data visibility rules: diagnostics and edits are derived from local file contents and should not leave the machine.

## User Flow

1. User starts an LSP-capable editor for a workspace containing `.gwdk` files.
2. The editor launches `gowdk lsp` over stdio.
3. The language server validates opened buffers, publishes diagnostics, returns formatting edits, offers completions, and returns hover text.

## Requirements

### Functional

- Start a JSON-RPC/LSP server with `gowdk lsp`.
- Handle `initialize`, `initialized`, `shutdown`, and `exit`.
- Accept full-document `textDocument/didOpen`, `textDocument/didChange`, `textDocument/didSave`, and `textDocument/didClose` notifications.
- Publish diagnostics using the current GOWDK parser and validation rules.
- Return whole-document formatting edits using `gowdk fmt` behavior.
- Return keyword completions for annotations, render modes, blocks, and `g:` directives.
- Return hover text for known annotations, directives, blocks, routes, stores,
  props, components, layouts, guards, and handler symbols from open documents.

### Non-Functional

- Performance: validate one open buffer quickly enough for interactive editing.
- Reliability: malformed protocol messages should return JSON-RPC errors instead of crashing.
- Accessibility: editor clients should receive standard diagnostics and completion metadata.
- Security/privacy: no network access and no external process execution inside the language server.
- Observability: protocol errors should be written to stderr.

## Acceptance Criteria

- [x] `gowdk lsp` starts and answers an LSP `initialize` request.
- [x] Opening an invalid `.gwdk` buffer publishes diagnostics without requiring the buffer to be saved.
- [x] `textDocument/formatting` returns a replacement edit matching `gowdk fmt`.
- [x] `textDocument/completion` returns the same language keywords exposed by editor tooling.
- [x] `textDocument/hover` returns concise markdown help for language tokens and open-project symbols.
- [x] `go test ./...` and `go build ./cmd/gowdk` pass.

## Edge Cases

- Missing `@page` or `@route` should publish a diagnostic at the beginning of the document.
- Closing a document should clear diagnostics for that URI.
- Unknown LSP requests should return a method-not-found error.
- Notifications without params should be ignored when safe.

## Dependencies

- Internal: `internal/lang`, `internal/parser`, `internal/compiler`, `internal/manifest`.
- External: none.

## Open Questions

- Workspace-wide manifest validation can reuse the existing duplicate page and
  route checks once the LSP maintains a project-wide manifest.
- Incremental sync can replace full-document sync if large `.gwdk` files become common.
