# Implementation Plan: Deep Go Package Integration

## Context

Spec: `.llm/features/deep-go-package-integration.md`

Related plans and decisions:

- `.llm/plans/gowdk-world-roadmap.md`
- `.llm/plans/go-native-adapter-boundary.md`
- `docs/engineering/decisions/0005-generated-go-emission-boundary.md`
- `docs/engineering/decisions/0006-gowdk-compiler-and-kit-boundary.md`

The current worktree already contains first-slice backend binding, generated
action/API calls, backend binding metadata, split frontend/backend options, and
formatted generated app source. This plan covers the missing work to move from
that first slice to the package-integrated model where `.gwdk` files are Go
package peer files and generated Go is only adapter glue.

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

## Assumptions

- Breaking old action/API block syntax is acceptable for this slice if migration
  diagnostics and docs are clear.
- `.gwdk` files are package peers, not standalone route scripts.
- Routes remain declared in `.gwdk` files.
- Handler symbols are exact exported Go names.
- User validation, redirects, fragments, JSON, HTML, auth, and storage live in
  normal Go handlers returning `runtime/response.Response`.
- One-binary generation remains the preferred product path.
- Split frontend/backend generation remains optional and uses the same metadata.

## Current Baseline Checklist

- [x] Project compiler commands require `gowdk.config.go` or `--config`.
- [x] Existing parser records page annotations, imports, stores, view/build/load
      blocks, old action blocks, and old API blocks.
- [x] Existing compiler validates routes, render modes, missing SSR addon,
      dynamic SPA paths, duplicate routes, and method conflicts.
- [x] Existing backend binding discovers same-directory Go functions for old
      action/API declarations.
- [x] Existing binding metadata supports `bound`, `missing`, and
      `unsupported_signature`.
- [x] Existing generated apps can call same-package action handlers using
      `func(context.Context, form.Values) (response.Response, error)`.
- [x] Existing generated apps can call same-package API handlers using
      `func(context.Context, *http.Request) (response.Response, error)`.
- [x] Existing generated apps return `501` for missing or unsupported first-slice
      action/API handlers.
- [x] Existing generated app source is passed through `go/format` before write.
- [x] Existing build config and CLI include split backend app/binary fields.
- [x] `.gwdk` files require a `package <name>` declaration.
- [x] `.gwdk` package names are validated against sibling `.go` files.
- [x] Go `import` and GOWDK `use` are separate source lanes.
- [x] Page-level `use alias "package"` supports qualified component calls such
      as `<ui.Hero />`.
- [x] Action/API syntax uses exact exported endpoint declarations instead of old
      block bodies.
- [x] Handler binding uses exact exported names instead of name transformation.
- [ ] Handler signature discovery uses `go/types` instead of AST-only checks.
- [x] Sibling Go package parse and type-check failures are reported as
      `go_package_error`.
- [ ] Typed action structs are decoded from form tags and field types.
- [ ] Runtime has a shared backend router/adapter API.
- [ ] Generated route adapters are emitted through Go AST instead of broad raw
      string route bodies.
- [ ] `.gwdk` inputs flow through GOWDK parser, GOWDK AST, and GOWDK analyzer
      before generated Go.
- [ ] `.go` inputs flow through standard `go/parser`, `go/ast`, and `go/types`
      before handler/type validation.
- [ ] CSRF is wired into generated action adapters.
- [ ] Docs and examples use package-integrated syntax.

## Proposed Changes

- Add package declarations to the `.gwdk` language model.
- Replace action/API block parsing with exported endpoint declarations.
- Promote backend binding from first-slice name mapping to exact Go symbol
  resolution.
- Add typed form decoding for user-owned Go structs.
- Add `runtime/app` backend routing primitives used by generated adapters.
- Rework generated backend source around typed route metadata and Go AST
  emission.
- Update examples, docs, diagnostics, route output, manifest JSON, and build
  reports.

## Missing Work Checklist

### 1. Language And Parser

- [ ] Define the GOWDK AST nodes used by package declarations, annotations,
      routes, imports, stores, blocks, component contracts, and source spans.
- [x] Parse GOWDK source uses with `use alias "package"`.
- [ ] Add a package declaration parser for `package <identifier>`.
- [ ] Treat the package declaration as required for pages, components, and
      layouts.
- [ ] Enforce that package is the first non-comment declaration.
- [ ] Add source spans for package declarations.
- [ ] Add `PackageName` and `PackageSpan` to `manifest.Page`,
      `manifest.Component`, and `manifest.Layout`.
- [ ] Update `internal/parser.ParseSyntax` to expose package declarations.
- [ ] Replace `act <name> { ... }` parsing with
      `act <ExportedGoFunc> POST "<route>"`.
- [ ] Replace `api <name> { METHOD "route" }` parsing with
      `api <ExportedGoFunc> <METHOD> "<route>"`.
- [ ] Reject non-exported handler names in `act`, `api`, and `g:post`
      references.
- [ ] Reject non-POST action methods.
- [ ] Preserve API endpoint validation for `GET`, `POST`, `PUT`, `PATCH`, and
      `DELETE`.
- [ ] Remove action body fields from the current route model after migration:
      input variable, pseudo validation, redirects, and declarative fragments.
- [ ] Emit migration diagnostics for old `act name {}` and `api name {}` forms.
- [ ] Update formatter, token tests, grammar golden files, diagnostics golden
      files, manifest golden files, and LSP completions.
- [x] Add diagnostics for duplicate/unknown GOWDK source uses and unknown
      qualified component references.

### 2. Manifest, Route Metadata, And Diagnostics

- [ ] Add package metadata to public manifest JSON where useful.
- [x] Add handler symbol, method, route, package, binding status, and message to
      route/build metadata consistently.
- [x] Update `gowdk routes` to report exact handler symbols and route methods.
- [ ] Make duplicate method/path checks use declared action/API endpoint literals,
      not implicit page route behavior.
- [ ] Add diagnostics:
  - [x] `missing_package_declaration`
  - [x] `package_must_be_first`
  - [x] `package_mismatch`
  - [x] `invalid_backend_handler_name`
  - [x] `unsupported_action_method`
  - [x] `old_action_block_syntax`
  - [x] `old_api_block_syntax`
  - [x] `go_package_error`
  - [ ] `unsupported_action_input_type`
- [ ] Update `docs/reference/diagnostics.md`.

### 3. Go Package Ownership And Binding

- [ ] Resolve each `.gwdk` file's Go package by source directory.
- [ ] Use `go list` to discover import path, package name, and files.
- [ ] Use standard `go/parser`, `go/ast`, and `go/types` to validate exported
      handlers and input types.
- [x] Surface type-check errors as focused GOWDK diagnostics.
- [ ] Validate `.gwdk` package name against sibling Go package name.
- [x] Fail on Go package type-check errors with `go_package_error`.
- [ ] Cache inspected packages by absolute directory or import path.
- [ ] Resolve exact exported handler symbols. Remove lowercase-to-exported name
      mapping.
- [ ] Keep missing handler symbols non-fatal and generate `501`.
- [ ] Keep unsupported handler signatures non-fatal and generate `501`.
- [ ] Support action signatures:
  - [ ] `func Name(context.Context) (response.Response, error)`
  - [ ] `func Name(context.Context, Input) (response.Response, error)`
  - [ ] `func Name(context.Context, *Input) (response.Response, error)`
  - [ ] `func Name(context.Context, form.Values) (response.Response, error)`
- [ ] Support API signature:
  - [ ] `func Name(context.Context, *http.Request) (response.Response, error)`
- [ ] Record binding signature kind, input type, input pointer mode, and import
      requirements in manifest/compiler metadata.
- [ ] Handle import alias collisions deterministically.
- [ ] Document that user feature packages must not import generated app output.

### 4. Runtime Adapter API

- [ ] Add `runtime/app.BackendHandler`.
- [ ] Add `runtime/app.BackendRouter`.
- [ ] Add `runtime/app.NewBackendRouter`.
- [ ] Add `BackendRouter.Action(method, route string, h BackendHandler)`.
- [ ] Add `BackendRouter.API(method, route string, h BackendHandler)`.
- [ ] Add `BackendRouter.ServeHTTP(http.ResponseWriter, *http.Request) bool`.
- [ ] Add action adapter helpers:
  - [ ] `Action0(func(context.Context) (response.Response, error))`
  - [ ] `ActionForm[T any](func(context.Context, T) (response.Response, error))`
  - [ ] `ActionFormPtr[T any](func(context.Context, *T) (response.Response, error))`
  - [ ] `ActionValues(func(context.Context, form.Values) (response.Response, error))`
- [ ] Add `APIHandler(func(context.Context, *http.Request) (response.Response, error))`.
- [ ] Add `NotImplemented(message string) BackendHandler`.
- [ ] Update `runtime/app.Handler` to use one `Backend HandlerFunc` hook.
- [ ] Keep existing `Action` and `API` fields temporarily only if needed for
      internal compatibility.
- [x] Ensure backend adapters set no-store on request-time responses.
- [ ] Preserve request body limits for action forms.

### 5. Typed Form Decoding

- [ ] Add `runtime/form.DecodeStruct[T any](Values) (T, error)`.
- [ ] Read field names from `form:"name"` tags first, then exported Go field
      names.
- [ ] Ignore fields tagged `form:"-"`.
- [ ] Reject unknown submitted fields.
- [ ] Support `string`.
- [ ] Support `[]string`.
- [ ] Support `bool`.
- [ ] Support signed integers.
- [ ] Support unsigned integers.
- [ ] Return structured decode errors without exposing submitted values.
- [ ] Decide empty-value behavior for numeric and boolean fields.
- [ ] Add tests for tags, ignored fields, unknown fields, duplicate tags,
      repeated values, scalar conversion, unsupported fields, and pointer input.

### 6. Generated Adapter Emission

- [ ] Define a typed backend adapter IR for imports, routes, handlers, decoding,
      response writing, and fallback `501` behavior.
- [ ] Generate backend route registration from the IR through Go AST.
- [ ] Generate all generated Go with `go/ast` and `go/printer`, then always run
      `go/format`.
- [ ] Do not assemble generated Go with hardcoded line writing,
      `WriteString` chains, token concatenation, or source snippets unless a
      temporary exception is strictly necessary and documented at the call site.
- [ ] Assert generated route registration structurally through AST-aware tests
      or focused formatted-output tests, not by preserving hand-written source
      snippets as the implementation model.
- [ ] Preserve `//go:embed app` comments.
- [ ] Sort imports and routes deterministically.
- [ ] Do not generate user handler functions, input structs, validation
      functions, auth logic, storage logic, or service code.
- [ ] Do not import missing or unsupported handler packages solely for `501`
      routes.
- [ ] Replace broad action/API string builders with AST emission.
- [ ] Replace generated app shell Go templates with AST emission.
- [ ] Move split frontend proxy and backend-only app generation to the same
      backend route metadata.
- [ ] Keep generated shell files small: package, imports, embedded assets,
      `Handler`, and route hook wiring.

### 7. CSRF And Security

- [ ] Wire `addons/actions.NewCSRF` or a runtime equivalent into generated action
      adapters.
- [ ] Define how CSRF tokens are exposed to generated forms.
- [ ] Add failure status and response shape for invalid CSRF.
- [ ] Keep `NoopCSRF` test-only.
- [ ] Preserve local redirect safety through user `response.RedirectTo` helpers.
- [ ] Ensure request body size limits stay in generated adapters.
- [ ] Ensure form values are not logged in diagnostics, build reports, or
      runtime error responses.

### 8. Examples And Docs

- [ ] Update login `.gwdk` files to start with `package auth`.
- [ ] Update login syntax to `act Login POST "/"`,
      `act Logout POST "/logout"`, and `api Session GET "/api/session"`.
- [ ] Update login forms to use `g:post={Login}` and `g:post={Logout}`.
- [ ] Add typed `LoginInput` in `auth.go`.
- [ ] Update login handlers to use typed input where appropriate.
- [ ] Update `README.md` example syntax.
- [ ] Update `docs/language/actions.md`.
- [ ] Update `docs/language/api.md`.
- [ ] Update `docs/reference/routing.md`.
- [ ] Update `docs/reference/config.md` if generated outputs or target fields
      change.
- [ ] Update `docs/reference/deployment.md`.
- [ ] Update `docs/engineering/architecture.md`.
- [ ] Update `docs/product/requirements.md` statuses.
- [ ] Update `examples/README.md`.
- [ ] Add or update an ADR if the package-first syntax is accepted as a
      hard-to-reverse language decision.

### 9. Tests

- [x] Parser accepts `package auth` as the first non-comment declaration.
- [ ] Parser rejects missing package declarations.
- [x] Parser rejects package declarations after annotations/imports/blocks.
- [x] Parser accepts `act Login POST "/"`.
- [x] Parser accepts `api Session GET "/api/session"`.
- [x] Parser rejects lowercase or otherwise non-exported handler names.
- [x] Parser rejects old action/API block syntax with migration diagnostics.
- [x] Compiler rejects package mismatch with sibling Go files.
- [x] Compiler fails on Go package type-check errors.
- [x] Compiler reports missing handlers.
- [x] Compiler reports unsupported signatures.
- [x] Compiler resolves exact same-package exported handlers.
- [x] Compiler resolves typed action input structs and rejects unexported input
      types.
- [ ] Runtime `BackendRouter` dispatches by method and normalized path.
- [ ] Runtime action/API adapter helpers write `response.Response`.
- [ ] Runtime handler errors map through `response.HandlerStatus`.
- [ ] `DecodeStruct` covers tags, ignored fields, unknown fields, repeated
      values, scalar conversion, unsupported field types, and pointer inputs.
- [ ] Appgen formats generated source successfully.
- [ ] Appgen preserves `//go:embed app`.
- [ ] Appgen emits route registration calls, not custom user logic.
- [ ] Appgen omits imports for missing handlers.
- [ ] Appgen emits `ActionForm[pkg.Type](pkg.Func)` or
      `ActionFormPtr[pkg.Type](pkg.Func)` for typed handlers.
- [ ] One-binary login flow signs in, redirects, reads session API, and logs out.
- [ ] Split frontend/backend login flow exercises the same backend metadata.

## Files Expected To Change

- `.llm/features/deep-go-package-integration.md`
- `.llm/plans/deep-go-package-integration.md`
- `docs/engineering/decisions/`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`
- `docs/reference/diagnostics.md`
- `docs/reference/routing.md`
- `docs/reference/deployment.md`
- `docs/language/actions.md`
- `docs/language/api.md`
- `examples/login`
- `internal/parser`
- `internal/lang`
- `internal/manifest`
- `internal/compiler`
- `internal/appgen`
- `runtime/app`
- `runtime/form`
- `runtime/response`
- `cmd/gowdk`

## Data And API Impact

- `.gwdk` syntax changes:
  - add required `package <name>`.
  - replace action/API blocks with endpoint declarations.
  - require exported handler references in `g:post`.
- Manifest/build/route metadata gains package and exact symbol details.
- Generated app internals move toward `BackendRouter`.
- Existing generated output layout can remain compatible.
- Old `.gwdk` action/API syntax is a migration path, not a supported current
  syntax after this change lands.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./internal/parser ./internal/lang ./internal/compiler ./runtime/app ./runtime/form ./internal/appgen ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
cd examples/login && make check
cd examples/login && make build
cd examples/login && make split-build
```

## Rollback Plan

- Keep the current first-slice feature-bound backend integration on a separate
  branch point until package-first syntax is complete.
- If package-first parsing causes broad regressions, keep package parsing behind
  validation while preserving old action/API block execution temporarily.
- If typed input decoding is unstable, keep `form.Values` handlers as the only
  supported bound action signature and leave typed signatures planned.
- If AST emission causes churn, keep the formatter gate and revert only the new
  AST emitter package.
- If split generation regresses, preserve one-binary generation and disable
  split frontend/backend output until shared metadata is corrected.

## Risks

- This is a breaking syntax change and needs clear migration diagnostics.
- Requiring packages for every `.gwdk` file may affect examples, docs, tests,
  editor tooling, and generated fixtures at once.
- `go/types` package loading can slow validation without caching.
- Typed generic runtime helpers require generated imports and aliases to be
  deterministic.
- A full AST rewrite can add ceremony. Keep helper functions small and
  transparent; do not replace raw templates with a hidden custom source
  language.
- User packages can create import cycles by importing generated app output.
- CSRF wiring touches runtime behavior and examples, so it should land after the
  package/symbol migration is stable.
