# Implementation Plan: GOWDK Compiler Plus Runtime

## Context

Architecture decision:

- `docs/engineering/decisions/0006-gowdk-compiler-and-runtime-boundary.md`

Primary active planning sources:

- `.llm/features/deep-go-package-integration.md`
- `.llm/plans/deep-go-package-integration.md`
- `.llm/features/go-native-adapter-boundary.md`
- `.llm/plans/go-native-adapter-boundary.md`
- `.llm/features/contract-driven-runtime.md`
- `.llm/plans/contract-driven-runtime.md`

This roadmap reorganizes existing `.llm` plans around the product split:

```text
GOWDK Compiler
component/page compiler
        +
GOWDK Runtime
app/runtime layer
        =
Go-first full web app
```

Naming rules:

- `GOWDK` means the product and repository wordmark.
- `GOWDK Compiler` means the `.gwdk` language/compiler layer.
- `GOWDK Runtime` means the app/runtime layer.
- `gowdk` means CLI, package/module spelling, config prefixes, and generated
  asset/runtime prefixes.
- `GOWDK app` means user-owned app output produced by the compiler and served
  through GOWDK Runtime.
- `addon` means optional feature-registration or integration package, not a
  third product layer.
- Avoid bare `core`; write `compiler core`, `runtime core`, or `repository core`.

Compiler lanes:

```text
.gwdk file
  -> GOWDK parser
  -> GOWDK AST
  -> GOWDK analyzer
  -> generated normal Go code
  -> go/format
  -> go build
```

```text
.go files
  -> standard go/parser
  -> standard go/ast
  -> standard go/types
  -> validate exported handlers/types
```

## Product Layers

### GOWDK Compiler

Owns:

- `.gwdk` parsing, formatting, diagnostics, LSP, and syntax migration.
- Package-peer `.gwdk` files with `package <name>`.
- Pages, layouts, components, `view {}`, `paths {}`, `build {}`, and `load {}`
  validation.
- Component contracts, props, state, client blocks, CSS, islands, and static
  output.
- Manifest, route metadata, build reports, generated adapter source, and
  generated app source layout.

Does not own:

- User domain handlers.
- Auth/storage/business validation logic.
- Long-lived runtime state outside generated app/runtime contracts.

### GOWDK Runtime

Owns:

- `runtime/app` serving, backend route dispatch, embedded assets, health, and
  one-binary app contracts.
- Typed query, command, event, and job registration once the contract runtime
  slice lands.
- `runtime/form` typed and raw form decoding.
- `runtime/response` HTML, redirect, fragment, JSON, cookie, and error
  envelopes.
- Action and API adapter helpers.
- CSRF, body limits, no-store backend responses, and runtime security defaults.
- Partial fragment response handling.
- SSR addon request-time contracts.
- Optional split frontend/backend runtime wiring.

Does not own:

- Full-page SSR as the default identity.
- Generated user application logic.
- Mandatory JavaScript framework or npm runtime.
- Mandatory external queue, broker, scheduler, or ORM.

## Canonical Roadmap Order

1. Package-integrated `.gwdk` language.
2. Exact exported action/API endpoint declarations.
3. GOWDK AST and analyzer metadata.
4. Go package ownership and binding through standard `go/parser`, `go/ast`, and
   `go/types`.
5. Runtime-kit backend router and typed form decoder.
6. Full Go AST generated adapter emission.
7. One-binary and split-binary route unification.
8. CSRF-wired action adapters.
9. Server fragments through `runtime/response`.
10. Request-time SSR `load {}` and guards through the SSR addon.
11. Contract-driven runtime for typed queries, commands, backend-owned domain
    and integration events, presentation events, jobs, and optional
    web/worker/cron roles.
12. Hybrid render policy, cache policy, and revalidation.
13. GOWDK client language for generated JS islands.
14. Explicit WASM island ABI.

## Plan Alignment

| Plan | Status | Required Alignment |
| --- | --- | --- |
| `deep-go-package-integration.md` | Active source of truth | Owns package-first `.gwdk`, exact symbols, typed action inputs, and migration diagnostics. |
| `go-native-adapter-boundary.md` | Active supporting plan | Owns generated adapter shape and runtime glue; must not define competing syntax. |
| `contract-driven-runtime.md` | Planned after endpoint/adapter IR stability | Owns typed query/command/domain-event/integration-event/presentation-event/job registry, local runtime dispatch, optional worker/cron roles, and contract graph tooling; frontend UI events trigger commands or queries and must not become backend facts. |
| `golangish-reactive-islands.md` | Active compiler-side UI plan | Client language is a GOWDK subset, not forked Go or arbitrary JavaScript. |

## Absorbed Or Removed Planning Files

The previous fine-grained first-slice files were removed to avoid competing
sources of truth. Their useful direction is now folded into this roadmap:

- `feature-bound-backend-integration.md`: superseded by package-integrated
  exact exported declarations.
- `interactive-runtime.md`: folded into runtime fragment and island phases.
- `auto-route-detection.md`: folded into normalized route metadata and adapter
  generation phases.
- `gwdk-go-build-import.md`: folded into package-aware build data follow-up.
- `fast-dev-redeploy.md`: implemented static/dev slice; future app dev belongs
  to runtime phases.
- `module-binary-packaging.md`: implemented packaging slice; keep module
  selection as artifact packaging, not runtime module orchestration.
- `wasm-deploy-artifact.md`: implemented deploy artifact slice; explicit
  browser WASM islands remain separate.
- `vscode-extension-publish-workflow.md`: removed from product roadmap planning.

## Migration Rules For Existing Plans

- Replace old action blocks:

```gwdk
act login {
  input := form LoginInput
}
```

with endpoint declarations:

```gwdk
act Login POST "/"
```

- Replace old API blocks:

```gwdk
api session {
  GET "/api/session"
}
```

with endpoint declarations:

```gwdk
api Session GET "/api/session"
```

- Move redirects, fragments, validation, JSON, HTML, auth, and storage into
  normal Go handlers returning `runtime/response.Response`.
- Treat `.gwdk` package declarations as required once the package integration
  slice lands.
- Keep generated code as adapter glue only.

## Checklist

### Reorganization

- [x] Add ADR for compiler/runtime boundary.
- [x] Define stable product-layer names for GOWDK, GOWDK Compiler,
      GOWDK Runtime, `gowdk`, GOWDK app, and addons.
- [x] Add this roadmap as the planning index.
- [x] Scope the adapter-boundary plan under package integration.
- [x] Remove superseded and absorbed first-slice `.llm` feature/plan files.
- [x] Fold interactive runtime, route detection, build imports, dev reload,
      module packaging, and WASM deploy artifact direction into this roadmap.

### Implementation Still Missing

- [ ] Required `.gwdk package <name>` parser and diagnostics.
- [ ] GOWDK AST and analyzer metadata.
- [ ] Exact exported `act`/`api` declaration parser.
- [ ] Old action/API syntax rejection diagnostics.
- [ ] Package mismatch validation with sibling Go files.
- [ ] `go/types` handler binding.
- [ ] Runtime backend router.
- [ ] Typed `runtime/form.DecodeStruct`.
- [ ] Full Go AST generated adapter emission.
- [ ] CSRF-wired generated action adapters.
- [ ] Contract-driven runtime registry, backend-owned events, presentation-event
      metadata, and CLI contract graph.
- [ ] Login example migration.
- [ ] Full docs migration.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./...
go build ./cmd/gowdk
cd examples/login && make check
cd examples/login && make build
cd examples/login && make split-build
```
