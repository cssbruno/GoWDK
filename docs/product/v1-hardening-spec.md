# V1 Contract Hardening

## Problem

GOWDK has mature compiler and runtime slices, but several pre-v1 boundaries are
still implicit or weakly bounded. Configuration can silently accept unsupported
AST forms, project env loading mutates process state, generated packaging can
modify module files, LSP input and retained documents are unbounded, command
metadata is duplicated, and request-time errors are not connected to the
existing locale catalogs.

## Goals

- Make configuration, environment, packaging, CLI, LSP, and static-analysis
  behavior deterministic and bounded.
- Make source execution lanes and Go hook bindings explicit and inspectable.
- Reuse `runtime/i18n` for stable user-facing runtime error codes.
- Preserve build-time pages as the default and generated Go as adapter glue.

## Non-Goals

- Translating compiler diagnostics.
- Adding a second catalog or expression language.
- Guaranteeing identical binaries across different Go toolchains, targets, CGO
  toolchains, tags, or semantic inputs.
- Supporting unbounded LSP messages or documents.

## Requirements

### Configuration and environment

- AST-only config loading fails closed for unknown fields, unkeyed literals,
  duplicate fields, and unsupported expressions; supported dynamic Go values
  delegate to the executable loader.
- Project env files produce immutable overlays. Project loading never calls
  `os.Setenv`; config validation and child processes receive explicit lookups
  and environment slices.
- Root config types remain import-compatible but are split by concern. Invalid
  CSRF policy, target topology, identifiers, schedules, and provider references
  fail validation before generation.

### CLI and CI

- One recursive command schema owns command/subcommand names, flag groups,
  usage, documentation records, and shell completion records.
- CI runs broad `go vet` over every repository Go module and reviewed
  Staticcheck checks beyond `U1000`, with actionable per-module output.

### Language and bindings

- The v1 language budget classifies core, experimental, planned, and migration
  syntax. New syntax proposals use one complete compiler/tooling checklist.
- `g:for` and `g:if` carry an explicit `g:lane="server|client"`; the compiler
  rejects missing, mixed, or ownership-mismatched lane declarations. Tooling
  reports the declared/resolved lane.
- SSR loads, custom guards, and auth providers use explicit Go symbol
  references. The former magic names remain migration-only diagnostics.

### Packaging and LSP

- Native and WASM packaging use read-only module resolution, `-trimpath`,
  `-buildvcs=false`, temporary sibling outputs, atomic publication, and safe
  reproducibility metadata including SHA-256.
- LSP header lines, aggregate headers, header count, message bodies, open
  document count, individual documents, and aggregate retained text are bounded
  by per-server limits. Framing failures have stable codes and fatality policy.

### Localized runtime errors

- A runtime error value carries a stable code, safe default message, variables,
  and optional cause/status.
- `runtime/i18n` resolves that value through existing string-key catalogs and
  falls back to the default message.
- Generated validation, action/API, guard/auth, fragment, and SSR error
  responses expose the stable code and localized safe message.

## Acceptance Criteria

- [x] Issues #771, #758, #726, #724, #722, #721, #720, #717, #716, #714,
  #697, and #695 have focused tests covering their published acceptance items.
- [x] Generated examples and public docs use only current contracts.
- [x] Root tests, nested-module tests, CLI build, docs gates, static analysis,
  and representative generated-app builds pass.

## Edge Cases

- Conflicting duplicate `Content-Length` values terminate the LSP session;
  identical duplicates are accepted.
- An over-limit document update preserves the last valid snapshot.
- A failed package build preserves the previous valid artifact.
- Missing translations and unknown locales use the safe default message.
- Process environment values override env-file values without modifying either
  source map.

## Dependencies

- Internal: compiler IR, project loader, generated app plans, runtime response,
  runtime i18n, LSP, and command metadata.
- External: Go toolchain and the already-pinned Staticcheck tool only.
