# Feature Spec: Deep Go Package Integration

## Scope

This is the language-facing source of truth for package-peer `.gwdk` files. It
implements the compiler side of ADR 0006: GOWDK extends Go web authoring through
`.gwdk` compilation and generated adapters, not by forking the Go compiler.

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

## Problem

GOWDK already has first-slice feature-bound backend integration, but `.gwdk`
files still behave like standalone route scripts in several important places:
actions and APIs are block bodies, handler names are derived from lowercase
block names, generated adapters still understand old action body semantics, and
typed user Go input structs are not resolved.

The target model is stricter and more Go-native: a `.gwdk` file is a peer file
inside a Go package, declares `package <name>` first, and binds route
declarations to exact exported same-package Go symbols. User behavior stays in
normal Go; generated code remains adapter glue.

## Goals

- Require every page, layout, and component `.gwdk` file to declare a Go
  package as the first non-comment declaration.
- Validate that a `.gwdk` package matches sibling `.go` files in the same
  directory.
- Keep Go `import` limited to normal Go packages and use explicit GOWDK
  `use alias "package"` declarations for cross-package source reuse.
- Replace old `act name { ... }` and `api name { ... }` blocks with top-level
  declarations that name exact exported Go symbols.
- Keep routes in `.gwdk` files while moving redirects, fragments, validation,
  JSON, HTML, auth, storage, and domain logic into user Go handlers.
- Resolve same-package Go handlers with `go list`, `go/parser`, `go/ast`, and
  `go/types`.
- Generate deterministic adapter Go through full Go AST construction and
  standard Go formatting.
- Support typed action input structs without generating user structs.
- Preserve one-binary deployment and optional split frontend/backend deployment
  from the same route metadata.

## Non-Goals

- Implementing full SSR `load {}` execution.
- Making SSR the default product identity.
- Generating user domain handlers, services, stores, or validation logic.
- Supporting every Go handler signature in the first deep integration slice.
- Supporting file uploads before upload security rules are defined.
- Introducing a third-party router or form decoder dependency.
- Forking or replacing the Go compiler in the current roadmap.

## Users And Permissions

- Primary users: Go developers writing GOWDK product applications.
- Roles or permissions: no new framework-level roles.
- Data visibility rules: generated diagnostics and logs must not include
  submitted form values, secrets, credentials, or tokens.

## User Flow

1. A developer writes `package auth` at the top of `login.page.gwdk`.
2. The same directory contains normal Go files using `package auth`.
3. The page declares endpoint ownership with exact exported symbols:
   `act Login POST "/"` and `api Session GET "/api/session"`.
4. The page view posts with `<form g:post={Login}>`.
5. The compiler validates the package declaration, endpoint declarations, and
   same-package Go handler signatures.
6. GOWDK emits adapter code that decodes the request, calls `auth.Login` or
   `auth.Session`, and writes `runtime/response.Response`.
7. Missing or unsupported handlers remain non-fatal bindings and return clear
   `501` responses at runtime.

## Requirements

### Functional

- `.gwdk` files must reject missing package declarations.
- A package declaration must appear before annotations, imports, stores, route
  declarations, and blocks, ignoring blank lines and `//` comments.
- Package mismatch with sibling `.go` files must be a compiler diagnostic.
- Go `import` inside `.gwdk` files must import normal Go packages only.
- Page-level cross-package component calls must use `use alias "package"` and
  qualified tags such as `<ui.Hero />`.
- Imported components must resolve sibling components in their own package
  without making those names page-global.
- Component-scoped cross-package `use` declarations remain unsupported until
  their renderer and build-asset behavior is designed.
- `act <ExportedGoFunc> POST "<path>"` must declare an action endpoint.
- `api <ExportedGoFunc> <METHOD> "<path>"` must declare an API endpoint.
- Action route methods must be `POST` for the first slice.
- API endpoint methods must support `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.
- Handler names must be exact exported Go symbols. No lowercase-to-exported
  mapping is allowed.
- Old `act name { ... }` and `api name { ... }` forms must produce migration
  diagnostics instead of being silently accepted.
- `g:post={Name}` must reference an exported declared action symbol.
- Supported action signatures:
  - `func Name(context.Context) (response.Response, error)`
  - `func Name(context.Context, Input) (response.Response, error)`
  - `func Name(context.Context, *Input) (response.Response, error)`
  - `func Name(context.Context, form.Values) (response.Response, error)`
- Supported API signature:
  - `func Name(context.Context, *http.Request) (response.Response, error)`
- Typed action input structs must use exported struct types.
- Form field names must come from `form:"name"` tags first, then exported Go
  field names. `form:"-"` ignores a field.
- First-slice typed decoding must support `string`, `[]string`, `bool`, signed
  integers, and unsigned integers.
- Unknown submitted form fields must return HTTP 400.
- Missing handlers and unsupported signatures must generate `501` adapters
  without importing or referencing missing symbols.
- Go package type-check errors must fail check/build as `go_package_error`.

### Non-Functional

- Performance: package inspection must be cached by source directory or import
  path.
- Reliability: generated Go must be formatted and validated before write.
- Accessibility: generated partial and action behavior must preserve existing
  focus-restoration guarantees where partial forms are still supported.
- Security/privacy: generated action adapters must keep body limits, no-store
  responses, unknown-field rejection, and no form-value logging.
- Observability: manifest, route output, and build report metadata must expose
  package name, handler symbol, method, route, binding status, and binding
  message.

## Acceptance Criteria

- [ ] A `.gwdk` file without `package <name>` fails with a focused diagnostic.
- [ ] A `.gwdk` file whose package differs from sibling `.go` files fails with
      a focused diagnostic.
- [x] `use ui "components"` plus `<ui.Hero />` renders a component from a
      discovered package named `components`.
- [x] A page cannot call an imported package component by bare name unless that
      component is in the page's own package.
- [ ] `act Login POST "/"` binds only to exported `Login`, not to `login` or a
      transformed name.
- [ ] Old `act login {}` and `api session {}` syntax produces migration
      diagnostics.
- [ ] Bound action/API handlers are imported from normal user packages and
      called by generated adapters.
- [ ] Missing and unsupported handlers return `501` without broken imports.
- [ ] Typed action input structs decode from request form values and reject
      unknown fields.
- [x] Sibling Go package type-check errors fail validation with
      `go_package_error`.
- [ ] Generated one-binary and split-binary apps use the same backend route
      metadata.
- [ ] Generated Go route adapters are emitted through Go AST construction and
      pass `go/format` before write.
- [ ] Login example uses package-integrated `.gwdk` declarations and normal Go
      handlers.

## Edge Cases

- A directory contains `.gwdk` files but no sibling `.go` files.
- A directory contains malformed Go source.
- A directory contains multiple Go packages.
- A handler exists but is unexported.
- A handler exists with a supported name but unsupported signature.
- A typed input struct is unexported.
- A typed input struct has unsupported field types.
- Two imported feature packages need the same generated import alias.
- User packages import generated app output and create an import cycle.
- Split frontend and backend builds disagree on route metadata.

## Dependencies

- Internal: `internal/parser`, `internal/lang`, `internal/manifest`,
  `internal/compiler`, `internal/appgen`, `runtime/app`,
  `runtime/form`, `runtime/response`, examples and docs.
- External: Go toolchain only.

## Open Questions

- Should endpoint paths in `act <Name> POST "<path>"` default to the page route
  when the path literal is omitted, or should the path remain mandatory?
- Should server fragments stay exclusively in user Go `response.Response`, or
  should `.gwdk` regain declarative fragment templates later?
- What thin helper layer over `go/ast` keeps full-AST emission readable without
  becoming a custom source-template language?
