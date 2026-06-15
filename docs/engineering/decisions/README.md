# Architecture Decision Records

Use this directory for decisions that are expensive to reverse or that future agents and maintainers must understand.

Create new records from `.agents/templates/adr.md`.

Recommended naming:

```text
0001-short-title.md
0002-short-title.md
```

## Records

- `0001-llm-ready-project-structure.md`: accepted project structure for LLM-ready docs, workflows, and templates.
- `0002-compile-first-render-model.md`: accepted compile-first render model with optional SSR.
- `0003-js-default-explicit-wasm-islands.md`: accepted default JS islands and component-declared WASM island opt-in.
- `0004-production-wasm-island-abi.md`: accepted production ABI for component-declared WASM islands.
- `0005-generated-go-emission-boundary.md`: accepted generated Go adapter boundary.
- `0006-gowdk-compiler-and-runtime-boundary.md`: accepted split between
  GOWDK Compiler and GOWDK Runtime.
- `0007-static-first-spa-navigation.md`: accepted static-first SPA navigation and generated JavaScript guardrails.
- `0008-bounded-client-language.md`: accepted bounded `client {}` language and page-scoped store boundaries.
- `0009-optional-inline-go-authoring.md`: accepted optional inline Go authoring direction, with extraction to normal package Go.
- `0010-tokenizer-recursive-descent-parser.md`: accepted shared tokenizer and
  recursive-descent parser with error recovery, migrated behind the stable
  `gwdkast` AST seam.
- `0011-auth-addon-cryptography.md`: accepted auth addon cryptography and
  dependency stance for PBKDF2 defaults, custom hashers, and session secrets.
- `0012-realtime-subscribe-surface.md`: accepted explicit `g:subscribe` on
  query-owned elements as the first realtime reactivity source contract.
- `0013-built-in-tracing-observability.md`: accepted dependency-free
  `runtime/trace` primitives before generated app auto-instrumentation.
