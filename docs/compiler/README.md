# Compiler

This directory documents GOWDK compiler behavior, internal handoffs, and
generated-output contracts.

## Pipeline

```text
.gwdk source
  -> shared tokenizer and parser
  -> typed source AST (`internal/gwdkast`)
  -> analyzed program records (`internal/gwdkanalysis`)
  -> compiler IR (`internal/gwdkir`)
  -> validation, discovery, and Go binding (`internal/compiler`)
  -> build and application generation (`internal/buildgen`, `internal/appgen`)
  -> go/format
  -> optional go build
```

`internal/gwdkir.Program` is the compiler handoff consumed by generated-output
passes. The full package ownership and compatibility boundaries live in
[Architecture](../engineering/architecture.md#compatibility-records).

## Contract Boundaries

- The machine-checked language contract lives in
  [Language Conformance](../language/conformance.md).
- Source discovery and project layout live in
  [Project Structure](project-structure.md).
- Compiler phase ownership lives in [Pipeline](pipeline.md).
- Generated files and directories live in
  [Generated Output](generated-output.md).
- Current product maturity lives in
  [Product Requirements](../product/requirements.md).

Compiler docs should describe observable contracts and stable handoffs. Avoid
copying an exhaustive capability list from requirements or architecture; link to
the owning document instead.

## Documents

| Document | Purpose |
| --- | --- |
| [Project Structure](project-structure.md) | Source discovery, config requirements, file kinds, modules, and build targets |
| [Pipeline](pipeline.md) | Compiler phases, package ownership, and current-to-target flow |
| [Generated Output](generated-output.md) | Generated directories, files, binaries, and ownership rules |
| [Browser Compiler](browser-compiler.md) | Generated browser runtime, JavaScript islands, and component WASM islands |
| [Build Report](build-report.md) | `gowdk-build-report.json` schema and debug output |
| [Manifest](manifest.md) | Manifest and site-map JSON contracts |
| [Incremental Cache Keys](incremental-cache-keys.md) | Cache-key inputs and invalidation boundaries |
| [Endpoint Binding Inspection](endpoint-binding-inspection.md) | Go package binding inspection and status output |
| [Syntax Contributor Checklist](syntax-contributors.md) | Required parser, diagnostics, IR, generation, docs, and fixture work for syntax changes |

## Maintenance Rules

- A syntax change updates language docs, conformance coverage, diagnostics, IR,
  generation, and examples together.
- A generated artifact change updates the generated-output, manifest, or build
  report contract that owns it.
- A compiler boundary change updates architecture and usually requires an ADR.
- A capability status change updates product requirements instead of adding a
  second status list here.
