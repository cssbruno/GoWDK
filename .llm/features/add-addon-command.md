# Feature Spec: Add Addon Command

## Problem

Users need to wire common GOWDK addons into `gowdk.config.go` without hand
editing imports and `Config.Addons` for every project. The config loader already
recognizes built-in addon constructors, but the CLI does not provide a small
native workflow for adding them.

## Goals

- Add `gowdk add <addon> [--config <file>]`.
- Add `gowdk add --list` for built-in addon names.
- Modify `gowdk.config.go` through the Go AST and `go/format`.
- Avoid duplicate imports or addon constructor calls.

## Non-Goals

- Discover third-party addons.
- Rewrite arbitrary non-literal config expressions.
- Install Go modules or edit `go.mod`.

## Users And Permissions

- Primary users: local GOWDK app developers.
- Roles or permissions: write access to the selected config file.
- Data visibility rules: only local `gowdk.config.go` is read and rewritten.

## User Flow

1. Run `gowdk add --list`.
2. Run `gowdk add ssr` or `gowdk add actions partial`.
3. The CLI imports the matching addon packages and appends constructor calls to
   `Config.Addons`.

## Requirements

### Functional

- Known addon names map to canonical `github.com/cssbruno/gowdk/addons/...`
  packages.
- Existing canonical imports and constructor calls are detected, including
  aliased imports.
- Missing config files produce an actionable `gowdk init` hint.
- Existing non-literal `Config.Addons` fields fail instead of generating a
  duplicate key.

### Non-Functional

- Performance: single-file AST parse and format.
- Reliability: preserve valid Go formatting.
- Accessibility: not applicable.
- Security/privacy: no network or module installation.
- Observability: stdout reports added or already-present addons.

## Acceptance Criteria

- [ ] `gowdk add --list` prints stable known addon names.
- [ ] `gowdk add ssr --config <file>` inserts the import and `ssr.Addon()`.
- [ ] Re-adding an aliased existing addon does not duplicate it.
- [ ] Non-literal `Config.Addons` fails clearly.

## Edge Cases

- Config with no import block gets one inserted.
- Config with no `Addons` field gets a `[]gowdk.Addon` literal field.
- Multiple addon names are processed in argument order.

## Dependencies

- Internal: `internal/project.DefaultConfigFile`.
- External: standard Go parser/AST/printer/format packages.

## Open Questions

- Should third-party addon discovery remain docs-only until addon versioning and
  trust rules are defined?
