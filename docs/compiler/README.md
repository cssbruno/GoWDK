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

| Topic | Source |
| --- | --- |
| Accepted language syntax | [Language Conformance](../language/conformance.md) |
| Source discovery and project layout | [Project Structure](project-structure.md) |
| Compiler phases | [Pipeline](pipeline.md) |
| Generated files and directories | [Generated Output](generated-output.md) |
| Browser runtime and islands | [Browser Compiler](browser-compiler.md) |
| Build-report schema | [Build Report](build-report.md) |
| Public manifest output | [Reference Manifest](../reference/manifest.md) |
| Cache-key model | [Incremental Cache Keys](incremental-cache-keys.md) |
| Syntax-change checklist | [Syntax Contributor Checklist](syntax-contributors.md) |
| Current product maturity | [Product Requirements](../product/requirements.md) |

Compiler docs should describe observable contracts and stable handoffs. Avoid
copying an exhaustive capability list from requirements or architecture; link to
the owning document instead.

## Maintenance Rules

- A syntax change updates language docs, conformance coverage, diagnostics, IR,
  generation, and examples together.
- A generated artifact change updates the generated-output, manifest, or build
  report contract that owns it.
- A compiler boundary change updates architecture and usually requires an ADR.
- A capability status change updates product requirements instead of adding a
  second status list here.
