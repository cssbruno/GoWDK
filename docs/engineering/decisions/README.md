# Architecture Decision Records

Use this directory for decisions that are expensive to reverse or that future agents and maintainers must understand.

Create new records from `.llm/templates/adr.md`.

Recommended naming:

```text
0001-short-title.md
0002-short-title.md
```

## Records

- `0001-llm-ready-project-structure.md`: accepted project structure for LLM-ready docs, workflows, and templates.
- `0002-compile-first-render-model.md`: accepted compile-first render model with optional SSR.
- `0003-js-default-explicit-wasm-islands.md`: accepted default JS islands and explicit WASM island opt-in.
- `0004-production-wasm-island-abi.md`: accepted production ABI for explicit WASM islands.
- `0005-generated-go-emission-boundary.md`: accepted generated Go adapter boundary.
- `0006-gowdk-compiler-and-kit-boundary.md`: accepted split between the GOWDK compiler and GOWDK app/runtime kit.
- `0007-static-first-spa-navigation.md`: accepted static-first SPA navigation and generated JavaScript guardrails.
- `0008-bounded-client-language.md`: accepted bounded `client {}` language and page-scoped store boundaries.
