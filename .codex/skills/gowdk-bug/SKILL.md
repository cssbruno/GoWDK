---
name: gowdk-bug
description: Diagnose and fix a GOWDK bug report, failing command, broken example, compiler diagnostic regression, generated output mismatch, or CI failure tied to this repository.
---

# GOWDK Bug

Reproduce before editing.

## Core Workflow

1. Capture the exact command, input files, branch, expected behavior, and actual
   output.
2. Reproduce with the smallest local command or fixture.
3. Classify the failure lane: parser/lang, compiler validation, buildgen,
   appgen/runtime, editor/LSP, docs/example, CI/release.
4. Add a regression test unless the fix is documentation-only or depends on
   unavailable external state.
5. Fix narrowly and run the focused package test before broader gates.

## Useful Gates

```bash
go test ./cmd/gowdk
go test ./internal/parser ./internal/lang ./internal/compiler
go test ./internal/buildgen ./internal/appgen
go test ./...
scripts/test-go-modules.sh
```

## Report

Include reproduction, root cause, files changed, verification, and any remaining
gap. If a bug is real but out of scope, create or link the issue instead of
burying it in chat.
