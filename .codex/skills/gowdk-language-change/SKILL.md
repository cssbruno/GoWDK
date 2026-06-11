---
name: gowdk-language-change
description: Change GOWDK language syntax or semantics. Use for parser, formatter, lexer, analyzer, diagnostics, `.gwdk` metadata, `view {}` markup, guards, routes, layouts, actions, APIs, `build`, `load`, `paths`, or docs/examples for language behavior.
---

# GOWDK Language Change

Treat language changes as compiler contract changes.

## Core Workflow

1. Read the current contract in `docs/language/` and the relevant examples.
2. Update parser/lexer/formatter/analyzer code only for the requested syntax or
   semantics.
3. Add focused tests in the touched package:
   - parser syntax: `internal/parser`
   - formatting/tokens/diagnostics helpers: `internal/lang`
   - validation/semantic diagnostics: `internal/compiler`
   - LSP behavior: `internal/lsp` and `editors/vscode`
4. Update docs and examples in the same change when public syntax changes.
5. Run the smallest relevant check first, then broaden if shared behavior moved:

```bash
go test ./internal/parser ./internal/lang ./internal/compiler ./internal/lsp
go build ./cmd/gowdk
```

## Guardrails

- Do not add `@` metadata syntax back as canonical public style.
- Keep diagnostics stable: update registry/docs when codes change.
- Dynamic SPA routes still require `paths {}` unless the page switches to
  request-time SSR.

## Report

Say what syntax changed, which docs/examples changed, and which tests prove the
contract.
