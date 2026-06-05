# Feature Spec: Go-Native Adapter Boundary

## Scope

This spec supports the package-integrated direction in
`.llm/features/deep-go-package-integration.md` and ADR 0006. It owns generated
adapter and app/runtime-kit boundaries; it does not define an independent
`.gwdk` syntax.

## Problem

GOWDK needs generated app code to package build-time output, backend actions,
APIs, partial responses, and optional SSR into Go binaries. But broad generated
Go does not feel native to Go developers when it owns too much behavior, hides
application logic, or grows as raw string templates.

The product boundary should be clear: users write normal Go packages and normal
`.gwdk` files; GOWDK compiles declarations into a small adapter layer that
imports user packages and wires runtime contracts.

## Goals

- Keep user backend logic in normal Go source files owned by the application.
- Limit generated Go to deterministic compiler-owned adapter packages.
- Make generated adapter code formatted, inspectable, and valid before write.
- Replace broad raw string generation with full Go AST emission over typed
  compiler data.
- Preserve one-binary and split-binary deployment from the same route metadata.
- Make missing or unsupported handlers visible through binding metadata and
  clear runtime responses.
- Document the boundary so future features do not expand generated user logic.

## Non-Goals

- Removing every generated Go file immediately.
- Using Go AST for non-Go artifacts such as HTML, CSS, JavaScript, JSON, or
  markdown.
- Generating user handler functions, domain services, stores, or auth logic.
- Making request-time full-page SSR the default framework identity.
- Introducing a router framework dependency only to avoid adapter generation.

## Users And Permissions

- Primary users: Go developers building GOWDK apps and binaries.
- Roles or permissions: no new permissions.
- Data visibility rules: generated adapter code must not expose secrets or read
  runtime environment beyond documented runtime settings.

## User Flow

1. A developer writes package-peer `.gwdk` route declarations next to normal Go
   feature code.
2. Action and API declarations name exact exported Go symbols.
3. The compiler validates declarations and discovers matching Go handlers.
4. GOWDK writes a small generated adapter package that imports the feature
   package and runtime packages.
5. `go build` compiles normal user Go plus the generated adapter into a binary.
6. If a handler is missing or has an unsupported signature, the adapter returns a
   clear `501` response and route metadata reports the binding problem.

## Requirements

### Functional

- User Go handler bodies must never be generated or rewritten.
- Generated action/API code may decode request data, call user handlers, and
  write `runtime/response.Response`.
- Generated adapter code must import user feature packages only when a binding is
  valid and needed.
- Missing and unsupported bindings must not reference nonexistent symbols.
- The compiler must expose binding metadata for `bound`, `missing`, and
  `unsupported_signature`.
- One-binary and split-binary outputs must use the same binding model.
- Generated Go must be emitted from `go/ast`, printed through standard Go AST
  tooling, and `gofmt`/`go/format` validated before it is written.
- New generated app behavior must be driven from typed route/action/API data,
  not ad hoc string searches over emitted code.
- Raw Go string templates are migration debt, not an accepted path for new
  generated Go. New generated Go must use AST construction.
- Adapter work consumes normalized compiler metadata from package-integrated
  `.gwdk` parsing instead of inventing a parallel route model.

### Non-Functional

- Performance: package discovery should cache per feature directory/import path.
- Reliability: invalid adapter source should fail at generation time with a
  focused compiler error.
- Accessibility: no UI impact.
- Security/privacy: generated adapters should keep request body limits and
  no-store response behavior for backend routes.
- Observability: route/build reports should identify binding status and source
  ownership.

## Acceptance Criteria

- [ ] Feature-bound action/API handlers are normal Go functions imported by the
      generated adapter.
- [ ] Generated adapter output is formatted and validated before write.
- [ ] Action/API/SSR adapter generation uses typed compiler route data and Go
      AST construction rather than file-sized raw string templates.
- [ ] Missing and unsupported backend handlers produce `501` responses without
      broken imports or references.
- [ ] Docs describe the generated adapter as compiler glue, not user logic.
- [ ] Golden or structural tests cover generated imports, route dispatch, and
      binding status.

## Edge Cases

- A feature directory has no Go package.
- A Go function exists but has the wrong signature.
- Two feature packages need the same import alias.
- A bound feature package imports generated app output and creates an import
  cycle.
- Split frontend and backend binaries disagree on route metadata.
- Generated adapter code is syntactically invalid before `go build`.

## Dependencies

- Internal: `internal/compiler`, `internal/manifest`, `internal/appgen`,
  `internal/codegen`, `runtime/app`, `runtime/form`, `runtime/response`.
- External: Go toolchain only.

## Open Questions

- How small should the local helper layer over `go/ast` be so AST construction
  stays readable without hiding generated source shape?
- Should generated adapter size be enforced by tests or only reviewed through
  focused package boundaries?
- Should `gowdk routes` become the primary human-facing explanation of generated
  adapter behavior?
