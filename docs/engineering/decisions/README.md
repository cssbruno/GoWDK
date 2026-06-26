# Architecture Decision Records

Architecture Decision Records capture choices that are expensive to reverse or
whose context and consequences future maintainers must understand.

Create new records from `.agents/templates/adr.md`. Use the next available
four-digit prefix and keep accepted records immutable; supersede a decision with
a new ADR instead of rewriting its history.

## Records

| Record | Decision |
| --- | --- |
| [0001](0001-llm-ready-project-structure.md) | LLM-ready project structure for documentation, workflows, and templates |
| [0002](0002-compile-first-render-model.md) | Compile-first render model with optional request-time SSR |
| [0003](0003-js-default-explicit-wasm-islands.md) | Generated JavaScript by default with explicit component WASM islands |
| [0004](0004-production-wasm-island-abi.md) | Production ABI for component-declared WASM islands |
| [0005](0005-generated-go-emission-boundary.md) | Generated Go adapter emission boundary |
| [0006](0006-gowdk-compiler-and-runtime-boundary.md) | GOWDK Compiler and GOWDK Runtime ownership boundary |
| [0007](0007-static-first-spa-navigation.md) | Static-first SPA navigation and generated JavaScript guardrails |
| [0008](0008-bounded-client-language.md) | Bounded `client {}` language and page-scoped store boundaries |
| [0009](0009-optional-inline-go-authoring.md) | Optional inline Go authoring extracted to normal package Go |
| [0010](0010-tokenizer-recursive-descent-parser.md) | Shared tokenizer and recursive-descent parser with recovery |
| [0011](0011-auth-addon-cryptography.md) | Auth addon cryptography and dependency stance |
| [0012](0012-realtime-subscribe-surface.md) | Explicit `g:subscribe` on query-owned elements |
| [0013](0013-built-in-tracing-observability.md) | Dependency-free tracing primitives before generated instrumentation |
| [0014](0014-addon-runtime-config-split.md) | Split between addon config and request-time runtime helpers |
| [0015](0015-generated-binary-lifecycle-services.md) | Generated binary lifecycle service contracts |
| [0016](0016-pure-go-helpers-from-bounded-client.md) | Pure Go helpers from bounded client code through WASM |
| [0017](0017-callback-props-and-scoped-cells.md) | Callback props and scoped cells for parent-child communication |

## Maintenance

- Link to this index instead of hard-coding the number of ADRs elsewhere.
- Add implementation status to the owning requirement or architecture document,
  not to the ADR title.
- When a decision is superseded, add reciprocal links between the old and new
  records.
