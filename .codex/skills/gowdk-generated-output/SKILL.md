---
name: gowdk-generated-output
description: Change GOWDK build output, generated Go app source, generated routes, manifests, CSS/assets, SSR handlers, action/API adapters, fragments, guards, or runtime/appgen/buildgen contracts.
---

# GOWDK Generated Output

Generated output is a public debugging surface. Keep it small, formatted,
inspectable, and tested.

## Core Workflow

1. Identify the lane: SPA build output, generated Go app, SSR, action/API,
   partial fragment, CSS/asset, manifest, route report, or binary packaging.
2. Inspect both the generator and the generated artifact contract docs:
   `docs/compiler/generated-output.md`, `docs/compiler/manifest.md`,
   `docs/reference/cli.md`, and `docs/engineering/architecture.md`.
3. Add/update tests close to the generator:
   - `internal/buildgen` for static artifacts and reports.
   - `internal/appgen` for generated app source/routes.
   - `runtime/*` or `addons/*` when runtime contracts change.
4. Prefer AST/printer-based Go generation over raw Go string assembly.
5. Verify with focused tests plus at least one CLI smoke when user-visible
   output changes:

```bash
go test ./internal/buildgen ./internal/appgen ./internal/compiler
go build ./cmd/gowdk
```

## Guardrails

- Do not make request-time full-page rendering the default.
- Do not make generated code own user domain logic.
- Keep single-binary deploy working with and without SSR.

## Report

Name the generated files or JSON shapes affected and include the exact
verification command.
