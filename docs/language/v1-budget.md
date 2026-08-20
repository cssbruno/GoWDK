# V1 Language Budget

GOWDK v1 is a small compiler language around Go, HTML, and CSS. New syntax must
remove more app complexity than it adds to parser, analyzer, formatter, LSP,
generated-output, and migration contracts.

## Core

- Metadata, routes, layouts, imports, components, `view {}`, and `style {}`.
- Build-time pages by default; `paths {}` expands dynamic SPA routes.
- `act`, `api`, and fragments for request-time backend behavior.
- `server {}` / `go server {}` for the explicit non-default request-time page
  lane.
- `build {}` for bounded data assembly: literals, bounded collection
  expressions, and explicit no-argument calls into ordinary Go.
- `client {}` for bounded UI event/state orchestration only.

## Experimental

Every accepted construct marked `Experimental` in
[Language Construct Stability](stability.md) is inside the implementation but
outside the frozen v1 core. It must retain conformance or named integration
coverage and may change through an explicit migration diagnostic.

## Planned Or Out Of Budget

- General computation belongs in Go, not a growing `build {}` expression
  language. Add data-shaping primitives only when they stay deterministic,
  bounded, and materially clearer than a Go helper.
- TypeScript is transform-only. GOWDK may strip/compile supported TypeScript
  syntax for browser assets; it does not implement a second type checker or
  server-side TypeScript runtime.
- Browser client code does not own routing truth, authorization, validation,
  durable state, cache policy, or business workflows.
- Foreign template mini-languages and implicit execution-lane inference are
  outside the v1 budget.

## Proposal Gate

Before accepting a new language construct, answer all of these in the feature
spec or issue:

1. What user problem cannot be solved clearly with existing syntax plus Go?
2. Is the construct core or experimental, and what is its migration story?
3. Which execution lane owns it: build, server, or client?
4. What are its deterministic resource bounds and failure diagnostics?
5. Which AST, IR, formatter, inspect, LSP, and generated-output contracts change?
6. Which conformance accept/reject case or named integration test covers it?
7. What existing syntax can be removed or kept out because this is added?

`scripts/check-language-budget.sh` is the CI gate. It rejects a supported
keyword/directive missing from the stability registry, a registry construct
missing from the published classification, or a construct without corpus or
named integration coverage.
