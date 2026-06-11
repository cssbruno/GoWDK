---
name: gowdk-generated-output
description: Change GOWDK build output, generated Go app source, generated routes, manifests, CSS/assets, SSR handlers, action/API adapters, fragments, guards, or runtime/appgen/buildgen contracts.
---

# GOWDK Generated Output

Generated output is a public debugging surface. Keep it small, formatted,
inspectable, and tested.

## Baselines

- `internal/buildgen` (entry points `Build`, `BuildFromIR`,
  `BuildFromValidatedIR`) writes to the output dir (default `gowdk_cache/`):
  route-derived HTML (`index.html`, `blog/{slug}/index.html`), manifests
  `gowdk-routes.json`, `gowdk-assets.json`, `gowdk-build-report.json`, CSS
  under `assets/site.css` + `assets/gowdk/pages|components/...`, island JS/WASM
  under `assets/gowdk/islands/`.
- `internal/appgen` (entry points `Generate`, `GenerateWithOptions`,
  `GenerateBackendWithOptions`) emits `gowdkapp/app.go` (exports `Handler()`
  and `ServeMux()`), `cmd/server/main.go` (listens on `$GOWDK_ADDR`, default
  `127.0.0.1:8080`), and `go.mod`. Golden:
  `internal/appgen/testdata/generated_go_golden/app.go.golden` (hand-updated).
- Contract docs to keep in sync: `docs/compiler/generated-output.md`,
  `docs/compiler/manifest.md`, `docs/compiler/build-report.md`,
  `docs/reference/cli.md`, `docs/engineering/generated-code-policy.md`.
- Runtime packages consumed by generated apps live in `runtime/` (app, asset,
  form, guard, response, route, validation, ...); optional features in `addons/`
  (ssr, actions, api, auth, db, embed, partial, spa, tailwind, ...). Separate
  Go modules exist only for `runtime/adapters/{echo,fiber,gin}` and
  `runtime/contracts/{natsbroker,redisstream,websocketfanout}` — touch those
  and `scripts/test-go-modules.sh` is the gate.
- CLI smoke surface: `gowdk build --out <dir> examples/pages/*.gwdk`, plus
  `--ssr`, `--app <dir>`, `--bin <file>`, `--backend-app`, `--backend-bin`
  variants that CI exercises.

## Core Workflow

1. Identify the lane: SPA build output, generated Go app, SSR, action/API,
   partial fragment, CSS/asset, manifest/report JSON, or binary packaging.
2. Read the matching contract doc above before changing a generator.
3. Add/update tests close to the generator: `internal/buildgen` for static
   artifacts and reports, `internal/appgen` for generated app source,
   `runtime/*`/`addons/*` when runtime contracts change.
4. Prefer AST/printer-based Go generation over raw Go string assembly.
5. Verify with focused tests plus a CLI smoke when user-visible output changes:

```bash
go test ./internal/buildgen ./internal/appgen ./internal/compiler
go build ./cmd/gowdk
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/*.gwdk
ls /tmp/gowdk-build   # confirm manifests + asset layout
```

## Lane Handoffs

- Output change driven by new public syntax: start from
  `gowdk-language-change`, return here for the generator side.
- Change confined to IR/analyzer internals feeding the generators:
  `gowdk-compiler-internal`.

## Guardrails

- Do not make request-time full-page rendering the default (ADR 0002).
- Do not make generated code own user domain logic (ADR 0005).
- Keep single-binary deploy working with and without SSR.
- Manifest JSON shapes (`gowdk-routes.json`, `gowdk-assets.json`) are
  versioned contracts — bump and document, never mutate silently.

## Report

Name the generated files or JSON shapes affected and include the exact
verification command.
