---
name: gowdk-language-change
description: Change GOWDK language syntax or semantics. Use for parser, formatter, lexer, analyzer, diagnostics, `.gwdk` metadata, `view {}` markup, guards, routes, layouts, actions, APIs, `build`, `load`, `paths`, or docs/examples for language behavior.
---

# GOWDK Language Change

Treat language changes as compiler contract changes.

## Baselines

- Grammar source of truth: `docs/language/grammar.md`. Canonical declaration
  forms: metadata lines (`package`, `page`, `route "/"`, `guard`, `layout`,
  `css`, `title`, `import`, `use`), blocks (`paths`, `build`, `server`, `view`,
  `style`, `props`, `state`, `store`, `client`), Go blocks (`go {}`,
  `go server {}`, `go client {}`), endpoints (`act Name POST "/path"`,
  `api Name METHOD "/path"` with GET|POST|PUT|PATCH|DELETE).
- Parser entry points: `ParsePage` / `ParsePageWithDefaultID` /
  `ParseComponent` / `ParseLayout` / `ParseSyntax` in `internal/parser`,
  producing `gwdkast` types (`File`, `PageDecl`, `Block`, `Endpoint`, ...).
- Lexer/formatter/completions live in `internal/lang`: `Lex`, `Format`,
  `ParseSource`, `Completions` (the editor keyword list — update it when adding
  keywords).
- Goldens (static, string-compared, updated by hand): parser AST goldens in
  `internal/parser/testdata/golden/` (`*.gwdk` + `parse.golden.json`),
  formatter goldens in `internal/lang/testdata/format_golden/{input,expected}.gwdk`.
- New diagnostics must be added to `internal/diagnostics/registry.go`
  (snake_case `Code` with Area/Stability/Severity/Summary) and documented in
  `docs/reference/diagnostic-codes.md`; `go test ./internal/diagnostics`
  enforces registration.
- Reference example shape: `examples/pages/home.page.gwdk` (`package pages`,
  `page home`, `route "/"`, `guard public`, `layout root`, `build {}`,
  `view {}`).

## Core Workflow

1. Read the current contract in `docs/language/` (start at `grammar.md`) and
   the closest `examples/` file.
2. Update parser/lexer/formatter/analyzer code only for the requested syntax or
   semantics. Touch in dependency order: `internal/parser` → `internal/lang`
   (tokens, formatter, completions) → `internal/gwdkanalysis` (IR lowering) →
   `internal/compiler` (validation) → `internal/lsp` + `editors/vscode`
   (`syntaxes/gwdk.tmLanguage.json` for highlighting).
3. Add focused tests in each touched package and update the parser/formatter
   goldens deliberately.
4. Update `docs/language/`, `docs/reference/diagnostic-codes.md`, and
   `examples/` in the same change when public syntax changes.
5. Run the smallest relevant check first, then broaden:

```bash
go test ./internal/parser ./internal/lang ./internal/compiler ./internal/lsp
go build ./cmd/gowdk
go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/ssr/*.gwdk
```

## Lane Handoffs

- The change also alters generated artifacts or runtime contracts: apply
  `gowdk-generated-output` for that part.
- Internal-only IR/analyzer/diagnostics plumbing: `gowdk-compiler-internal`.

## Guardrails

- Do not add `@` metadata syntax back as canonical public style; metadata is
  plain keywords (`page home`, not `@page home`).
- Keep diagnostic codes stable; renames go through the registry and its docs.
- Dynamic SPA routes still require `paths {}` unless the page switches to
  request-time SSR via `server {}` / `go server {}`.

## Report

Say what syntax changed, which docs/examples changed, and which tests prove the
contract.
