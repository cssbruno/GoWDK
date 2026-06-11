---
name: gowdk-bug
description: Diagnose and fix a GOWDK bug report, failing command, broken example, compiler diagnostic regression, generated output mismatch, or CI failure tied to this repository.
---

# GOWDK Bug

Reproduce before editing.

## Baselines

- Compiler pipeline order: parse (`internal/parser`, `ParsePage`/`ParseComponent`/`ParseLayout` → `gwdkast`) → IR build (`gwdkanalysis.BuildProgram` → `gwdkir.Program`) → `compiler.DiscoverGoEndpoints` → `compiler.ValidateProgramReport` → `compiler.BindBackendHandlers` → generators (`buildgen`, `appgen`).
- Diagnostics are snake_case codes (e.g. `missing_ssr_addon`, `duplicate_page_id`) registered in `internal/diagnostics/registry.go`; `go test ./internal/diagnostics` fails if an emitted code is missing from the registry.
- Canonical smoke fixture: `examples/pages/home.page.gwdk` via `go run ./cmd/gowdk check examples/pages/home.page.gwdk` or `build --out /tmp/gowdk-build examples/pages/*.gwdk`.
- Goldens are static files compared by string equality (no `-update` flag): `internal/parser/testdata/golden/`, `internal/lang/testdata/format_golden/`, `internal/buildgen/testdata/`, `internal/appgen/testdata/generated_go_golden/app.go.golden`. A golden diff is a contract change — update it deliberately, never regenerate blindly.
- CI (`.github/workflows/ci.yml`, job `verify`): `scripts/test-go-modules.sh`, vulncheck, `go build ./cmd/gowdk`, `node --test editors/vscode/*.test.js`, then `gowdk check/manifest/sitemap/routes --ssr` plus build smokes over `examples/`.

## Core Workflow

1. Capture the exact command, input files, branch, expected behavior, and actual
   output.
2. Reproduce with the smallest local command — prefer `gowdk check`, `gowdk
   tokens`, or `gowdk build` on a single `.gwdk` file from `examples/` or a
   `t.TempDir()` fixture. If reproduction is impossible, document exactly what
   is missing before continuing.
3. Classify the failure lane along the pipeline above: parser/lang, IR build,
   compiler validation, buildgen, appgen/runtime, editor/LSP, docs/example,
   CI/release.
4. Trace the failing path before editing. Prefer the local cause over broad
   rewrites.
5. Add a regression test in the failing package unless the fix is
   documentation-only or depends on unavailable external state.
6. Fix narrowly and run the focused package test before broader gates.

## Lane Handoffs

- Fix requires changing public syntax or semantics: `gowdk-language-change`.
- Fix changes generated artifacts or runtime contracts: `gowdk-generated-output`.
- Fix reshapes IR, analyzer passes, or the diagnostics registry without public
  impact: `gowdk-compiler-internal`.

## Useful Gates

```bash
go test ./internal/parser ./internal/lang ./internal/compiler
go test ./internal/buildgen ./internal/appgen
go test ./cmd/gowdk
go test ./...                  # root module only
scripts/test-go-modules.sh     # all 7 Go modules, what CI runs
```

## Report

Include reproduction, root cause, files changed, verification, and any remaining
gap. If a bug is real but out of scope, create or link the issue instead of
burying it in chat.
