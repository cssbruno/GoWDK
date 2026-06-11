# Implementation Plan: Plain Keyword Metadata

## Context

Relevant spec: `.llm/features/plain-keyword-metadata.md`.

## Assumptions

- The latest user direction supersedes the earlier migration idea: old `@`
  metadata should not be accepted.
- Endpoint-local error pages should also drop `@` to keep the language surface
  consistent.

## Proposed Changes

- Parse known metadata keywords directly and reject `@` metadata.
- Change endpoint error suffix parsing from `error` to `error`.
- Update validation diagnostics and classification helpers to use bare
  metadata.
- Rewrite examples, docs, snippets, and tests from `@keyword` to `keyword`.

## Files Expected To Change

- `internal/parser/`
- `internal/lang/`
- `internal/compiler/`
- `cmd/gowdk/`
- `docs/`
- `examples/`
- `editors/vscode/`
- relevant test fixtures and goldens

## Data And API Impact

- `.gwdk` source syntax changes.
- Internal IR and generated runtime contracts are unchanged.

## Tests

- Unit: parser, lang, compiler, CLI tests covering `.gwdk` parsing.
- Integration: existing build/app generation tests in the root module.
- End-to-end: `go test ./...`.
- Manual: inspect remaining `route`, `guard`, `component`, `css`, `asset`,
  `wasm`, `layout`, `cache`, `revalidate`, and `error` references.

## Verification Commands

```sh
go test ./internal/parser ./internal/lang ./internal/compiler ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Restore the parser metadata handling and docs/tests from the previous commit.

## Risks

- Broad fixture churn can miss a generated snippet or doc example.
- Some historical changelog references may intentionally mention old syntax.
