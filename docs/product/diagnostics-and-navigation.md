# Feature Spec: Diagnostics And Navigation

## Problem

Authors need diagnostic codes and editor navigation to be stable enough for CI,
editors, bug reports, and docs before GOWDK adds broad parser recovery and
deeper workspace navigation.

## Current Commands

```sh
gowdk check --json --warnings-as-errors --ssr --config gowdk.config.go
gowdk explain missing_ssr_addon
gowdk explain --json spa_dynamic_route_missing_paths
gowdk fix --dry-run --code old_action_block_syntax --ssr --config gowdk.config.go
gowdk routes --ssr --config gowdk.config.go
gowdk sitemap --ssr --config gowdk.config.go
gowdk lsp
```

## Goals

- Keep diagnostic codes as stable handles for CLI output, editor integrations,
  CI policy, docs, and bug reports.
- Make every emitted diagnostic code discoverable through `gowdk explain` and
  the registry-backed reference docs.
- Share safe fix metadata between `gowdk fix` and LSP code actions.
- Let navigation tooling use compiler-owned metadata for pages, routes,
  components, layouts, guards, stores, open Go handler symbols, and source
  ranges.
- Preserve GOWDK product rules: `.gwdk` declarations own web metadata, normal
  Go owns app behavior, and generated Go remains adapter glue.

## Non-Goals

- Broad parser recovery or replacing the current `parse_error` carrier with
  specific parser codes in this spec.
- Generated JavaScript ownership of routes, auth, business rules, validation,
  server state, loading policy, or cache policy.
- Replacing Go editor tooling for normal `.go` files.
- Implicit route discovery from folders, frameworks, or generated app output.

## Contract

Diagnostic output has three public surfaces:

- `gowdk check` emits human text or JSON diagnostics.
- `gowdk explain` describes a diagnostic code, severity, stability, next steps,
  and optional safe fix metadata.
- `gowdk fix` applies only registry-backed safe rewrites.

Navigation tooling has three public surfaces:

- `gowdk lsp` provides diagnostics, formatting, completions, hover,
  definitions, references, quick fixes, and semantic tokens for open editor
  documents.
- `gowdk routes` reports route and endpoint metadata from compiler IR.
- `gowdk sitemap` reports page source paths, dynamic params, and block
  presence for editor and tool integrations.

The diagnostic registry source of truth is
`internal/diagnostics/registry.go`. The public registry contract, current code
areas, stability policy, JSON shape, and safe-fix rules live in
[`docs/reference/diagnostic-codes.md`](../reference/diagnostic-codes.md).
CLI diagnostic output shape and source-span behavior live in
[`docs/language/diagnostics.md`](../language/diagnostics.md). The implemented
LSP slice lives in
[`docs/product/language-server.md`](language-server.md).

## Implemented Today

- Stable and experimental diagnostic-code registry entries with severity,
  area, summary, explanation, and optional safe fix metadata.
- `gowdk explain` plain text and JSON output.
- `gowdk check --json` diagnostics with 1-based positions and exclusive range
  end columns.
- Warning policy through `gowdk check --warnings-as-errors`.
- Registry-backed `gowdk fix` rewrites and matching LSP quick fixes.
- LSP diagnostics, formatting, completions, hover, open-document definitions,
  references, code actions, and full-document semantic tokens.
- CLI route and sitemap reports derived from compiler IR.

## Unsupported Or Planned

- Parser diagnostics still use broad `parse_error` until recovery has stable,
  specific codes.
- Not every compiler diagnostic has a precise range yet; missing spans should
  move toward parser-recorded source spans as each compiler lane hardens.
- Markup contract families currently surface through `view_parse_error` in
  compiler output even when `unsupported_markup_syntax` and
  `unsupported_markup_directive` document the stable families.
- LSP navigation is limited to open editor documents and supported compiler
  metadata. Workspace-wide route/type navigation remains planned.
- Route/type navigation must use `.gwdk` declarations, compiler IR, and normal
  Go symbol data. It must not infer behavior from framework route registration
  or generated adapter source.

## Acceptance Criteria

- [x] The diagnostic catalogue and navigation contract is specified before
      broad parser recovery or wider route/type navigation.
- [x] Current diagnostic code ownership, stability, JSON output, safe fixes,
      and LSP surfaces link to their source-of-truth docs.
- [x] Unsupported and planned behavior is explicit.
- [x] Product boundaries are preserved: build-time pages remain default,
      request-time rendering stays opt-in, app behavior stays in Go, and
      generated Go remains adapter glue.

## Verification

```sh
go run ./cmd/gowdk explain missing_ssr_addon
go run ./cmd/gowdk explain --json spa_dynamic_route_missing_paths
go test ./internal/diagnostics ./internal/lang ./internal/lsp
```
