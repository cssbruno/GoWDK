# ADR 0006: GOWDK Compiler And Runtime Boundary

Date: 2026-06-05

Status: Accepted

## Context

GOWDK needs to grow from an early `.gwdk` page/component compiler into a full
Go-first web app system. The product naming should make the split as clear as
Svelte and SvelteKit, but adapted for Go:

```text
GOWDK Compiler
component/page compiler
        +
GOWDK Runtime
app/runtime layer
        =
Go-first full web app
```

The risk is creating competing models: one where `.gwdk` files own backend
behavior through custom action bodies, another where generated Go owns too much
application logic, and another where Go itself is forked or replaced. That
would weaken the product and make future implementation plans conflict.

## Decision

GOWDK is split into two named product layers:

```text
GOWDK Compiler:
  Parses and validates .gwdk package-peer files.
  Compiles pages, layouts, components, build data, CSS, islands, manifests,
  static output, and generated adapter source.

GOWDK Runtime:
  Provides routing, form decoding, response envelopes, action/API adapters,
  partial fragments, CSRF, SSR addon contracts, embedded assets, and one-binary
  app serving.
```

The repository and Go module can continue to ship both layers together for now.
This ADR is a product-language boundary, not an import-path rename.

The naming contract is:

| Name | Meaning |
| --- | --- |
| GOWDK | Product name and repository wordmark. |
| GOWDK Compiler | The `.gwdk` language/compiler layer: parser, AST, analyzer, IR, diagnostics, generated adapter source, build output, manifests, route metadata, asset metadata, formatter, and LSP. |
| GOWDK Runtime | The app/runtime layer: `runtime/`, `addons/`, generated `net/http` app serving, routing, request context, form decoding, response envelopes, actions, APIs, fragments, SSR hooks, embedded assets, contract runtime, and one-binary or split-binary wiring. |
| `gowdk` | The CLI binary, Go package name, module path segment, generated prefixes, and config filename prefix. |
| GOWDK app | A user application built by GOWDK Compiler and served through GOWDK Runtime. |
| addon | Optional feature-registration or integration package. Addons extend GOWDK Runtime or compiler behavior; they are not a third product layer. |

Do not use `GOWDK Kit` for the app/runtime layer. It is redundant because the
`K` in `GOWDK` already carries the kit idea, so `GOWDK Kit` reads as "kit kit."
Use `GOWDK Runtime` when the app/runtime layer must be named.

Avoid bare `core` in product docs because it hides the layer boundary. Use
`compiler core`, `runtime core`, or `repository core` when the distinction
matters.
Avoid creating public names such as `GOWDK World`, `GOWDK Core`, or `GOWDK
Framework` unless a later ADR accepts that rename.

The compiler has two input lanes:

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

The GOWDK AST is for `.gwdk` language structure. Standard Go AST is for normal
Go source and generated Go source. The two models meet through analyzer output:
normalized route, component, package, type, and handler binding metadata.

GOWDK will not fork the Go compiler for the current roadmap. User application
logic stays in normal Go code. ADR 0009 amends the authoring boundary: separate
`.go` files remain supported, and future optional inline Go in `.gwdk` must
extract to normal importable, testable package Go. `.gwdk` is the custom
compiler surface that connects markup, routes, components, build-time data, and
runtime bindings to normal Go code.

The package-integrated direction is:

```gwdk
package auth

route "/"
guard public

act Login POST "/"
api Session GET "/api/session"

view {
  <form g:post={Login}>
    <input name="email" />
    <button>Sign in</button>
  </form>
}
```

The matching behavior is normal Go:

```go
package auth

func Login(ctx context.Context, input LoginInput) (response.Response, error) {
	return response.RedirectTo("/dashboard"), nil
}
```

Generated Go remains adapter glue. It may decode requests, call user handlers,
wire runtime contracts, and package assets, but it must not generate user
domain logic, handlers, stores, auth, validation policy, or storage code.

## Consequences

### Positive

- GOWDK can become a full app framework without losing Go toolchain
  compatibility.
- `go test`, `go build`, Go modules, editors, and deployment stay standard.
- `.gwdk` can be more ergonomic than raw Go where web UI needs a compiler.
- Backend behavior stays inspectable and user-owned.
- Planning docs have a clear boundary for compiler work versus runtime
  work.

### Negative

- `.gwdk` is a real language surface and must carry migration diagnostics.
- Package-aware `.gwdk` parsing touches parser, compiler, docs, examples, LSP,
  and test fixtures together.
- Runtime contracts must be designed carefully so generated adapters stay small.

### Neutral

- Go can still be improved around GOWDK through generated adapters, package
  conventions, typed helpers, and compiler diagnostics.
- A future custom Go compiler or source preprocessor is not ruled out forever,
  but it is not part of the current roadmap.

## Alternatives Considered

- Fork or customize the Go compiler now. Rejected because it would break too
  much tooling before the product model is stable.
- Keep backend behavior inside non-Go `.gwdk act {}` bodies. Rejected because
  it creates a second backend language and conflicts with normal Go ownership.
  Optional inline Go authoring is allowed by ADR 0009 only when extracted code
  remains ordinary Go.
- Generate user backend code. Rejected because it makes the framework own
  application logic and creates trust/debugging problems.

## Follow-Up

- Treat `.llm/features/deep-go-package-integration.md` as the language-facing
  package integration source of truth.
- Treat `.llm/plans/go-native-adapter-boundary.md` as the generated adapter and
  runtime implementation source of truth.
- Keep `.llm/plans/gowdk-world-roadmap.md` as the active planning index.
- Keep server fragments in runtime responses, not old action body syntax.
