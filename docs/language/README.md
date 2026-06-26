# GOWDK Language

This directory documents the current `.gwdk` source contract. Application
business logic, persistence, authorization, and production policy remain normal
Go code.

## Minimal Page

```gwdk
package pages

route "/"
guard public

build {
  => { title: "GOWDK ships apps" }
}

view {
  <main>
    <h1>{title}</h1>
  </main>
}
```

## Contract Sources

- [Conformance Corpus](conformance.md) is the machine-checked source of truth for
  accepted and rejected syntax.
- [Grammar](grammar.md) describes the current parser grammar.
- [Syntax](syntax.md) documents lexical rules and accepted source forms.
- [Semantics](semantics.md) documents validation and render-mode rules.
- [Specification](spec.md) is the compact language overview.
- [Stability](stability.md) records construct stability and deprecation tiers.

When prose and parser behavior disagree, fix the prose and the conformance
coverage in the same change. Do not treat a planned form in a topic page as
accepted syntax.

## Topic Guide

| Area | Documents |
| --- | --- |
| Structure and data | [Blocks](blocks.md), [data](data.md), [guards](guards.md), and [layouts](layouts.md) |
| Markup and components | [Markup](markup.md), [components](components.md), and [partials](partials.md) |
| Backend endpoints | [Actions](actions.md), [APIs](api.md), and [forms](forms.md) |
| Request-time rendering | [SSR](ssr.md) and [hybrid](hybrid.md) |
| Security policy source | [Audit files](audit.md) |
| Tooling | [Formatting](formatting.md), [diagnostics](diagnostics.md), and [conformance](conformance.md) |

Route validation, generated route metadata, and output paths live in the
[Routing Reference](../reference/routing.md). Compiler file discovery and
project layout live in [Project Structure](../compiler/project-structure.md).

## File Kinds

Current discovery uses source declarations and suffixes:

- `*.cmp.gwdk`, or a file containing `component`, identifies a component.
- `*.layout.gwdk` identifies a layout file.
- `*.audit.gwdk` identifies an audit policy file consumed by `gowdk audit`.
- Other `.gwdk` files are treated as pages by default.
- `.asset.gwdk` is reserved for asset-adjacent classification; current support
  and limits are documented in
  [Project Structure](../compiler/project-structure.md#current-inputs).

File classification does not imply that every planned semantic or rendering
capability is complete. Check the relevant topic page and
[Product Requirements](../product/requirements.md) for current maturity.

## Change Requirements

A public language change must update:

1. The relevant topic page and grammar or syntax page.
2. The accept/reject conformance corpus or named integration coverage.
3. Stable diagnostics and the
   [Diagnostic Code Reference](../reference/diagnostic-codes.md), when applicable.
4. Compiler IR, generated-output contracts, and examples affected by the change.
5. Product requirements when capability status changes.

Use the [Syntax Contributor Checklist](../compiler/syntax-contributors.md) for
the complete implementation path.
