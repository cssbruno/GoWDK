# Feature Spec: Inspect Compiler IR Command

## Problem

M2 compiler-spine work needs a direct way to inspect the typed compiler IR that
sits between parsing/analysis and generated output. Existing `manifest`,
`sitemap`, and `routes` commands expose compatibility or report shapes, but not
the normalized `internal/gwdkir.Program` handoff.

## Goals

- Add a first `gowdk inspect` subcommand for compiler IR.
- Reuse the same config, discovery, validation, backend binding, and contract
  validation path as current compile commands.
- Print deterministic JSON suitable for local debugging and future golden tests.

## Non-Goals

- Stabilize the inspect JSON as a public compatibility contract.
- Add every planned inspect target in this slice.
- Replace `manifest`, `sitemap`, or `routes`.

## Users And Permissions

- Primary users: GOWDK maintainers and advanced users debugging compiler output.
- Roles or permissions: local project access only.
- Data visibility rules: output includes source paths and source bodies already
  present in local `.gwdk` files.

## User Flow

1. Run `gowdk inspect ir [--config <file>] [--module <name>] [--ssr] [files...]`.
2. The CLI loads project config, discovers files when needed, validates sources,
   lowers to `internal/gwdkir.Program`, and prints JSON.
3. The user inspects routes, endpoints, templates, assets, packages, spans, and
   generated-output planning fields from one IR object.

## Requirements

### Functional

- `gowdk inspect ir` accepts the same project input flags as `routes`.
- The command fails before printing IR when parsing or validation fails.
- The command links contract references when present so IR output shows binding
  status and metadata.
- Unknown inspect targets fail clearly.

### Non-Functional

- Performance: use the existing single-pass project load and analyzer path.
- Reliability: no generated files are written.
- Accessibility: not applicable.
- Security/privacy: no secrets or external services are required.
- Observability: IR JSON is emitted to stdout; diagnostics go to stderr.

## Acceptance Criteria

- [ ] Explicit files with `--config` print IR JSON with pages, routes,
  endpoints, packages, and templates.
- [ ] Module discovery with `--module` limits the inspected file set.
- [ ] Unknown inspect targets fail with a clear error.
- [ ] CLI reference documents the new command.

## Edge Cases

- Missing config follows existing project command behavior.
- Missing `.gwdk` files follows existing discovery errors.
- Contract scan diagnostics fail the command instead of hiding bad Go metadata.

## Dependencies

- Internal: `internal/lang.CheckFiles`, `internal/gwdkanalysis.BuildIR`,
  contract reference linker.
- External: none.

## Open Questions

- When should inspect output gain a stable lower-camel JSON schema separate
  from the raw internal IR struct?
