# ADR 0006: GOWDK And GOWDK Kit Boundary

Date: 2026-06-05

Status: Accepted

## Context

GOWDK needs to grow from an early `.gwdk` page/component compiler into a full
Go-first web app system. The product naming should make the split as clear as
Svelte and SvelteKit, but adapted for Go:

```text
GOWDK
component/page compiler
        +
GOWDK Kit
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
GOWDK:
  Parses and validates .gwdk package-peer files.
  Compiles pages, layouts, components, build data, CSS, islands, manifests,
  static output, and generated adapter source.

GOWDK Kit:
  Provides routing, form decoding, response envelopes, action/API adapters,
  partial fragments, CSRF, SSR addon contracts, embedded assets, and one-binary
  app serving.
```

The repository and Go module can continue to ship both layers together for now.
This ADR is a product-language boundary, not an import-path rename.

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
logic stays in normal Go packages. `.gwdk` is the custom compiler surface that
connects markup, routes, components, build-time data, and runtime-kit bindings
to normal Go code.

The package-integrated direction is:

```gwdk
package auth

@page login
@route "/"

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
wire runtime-kit contracts, and package assets, but it must not generate user
domain logic, handlers, stores, auth, validation policy, or storage code.

## Consequences

### Positive

- GOWDK can become a full app framework without losing Go toolchain
  compatibility.
- `go test`, `go build`, Go modules, editors, and deployment stay standard.
- `.gwdk` can be more ergonomic than raw Go where web UI needs a compiler.
- Backend behavior stays inspectable and user-owned.
- Planning docs have a clear boundary for compiler work versus runtime-kit
  work.

### Negative

- `.gwdk` is a real language surface and must carry migration diagnostics.
- Package-aware `.gwdk` parsing touches parser, compiler, docs, examples, LSP,
  and test fixtures together.
- Runtime-kit contracts must be designed carefully so generated adapters stay
  small.

### Neutral

- Go can still be improved around GOWDK through generated adapters, package
  conventions, typed helpers, and compiler diagnostics.
- A future custom Go compiler or source preprocessor is not ruled out forever,
  but it is not part of the current roadmap.

## Alternatives Considered

- Fork or customize the Go compiler now. Rejected because it would break too
  much tooling before the product model is stable.
- Keep backend behavior inside `.gwdk act {}` bodies. Rejected because it
  creates a second backend language and conflicts with normal Go ownership.
- Generate user backend code. Rejected because it makes the framework own
  application logic and creates trust/debugging problems.

## Follow-Up

- Treat `.llm/features/deep-go-package-integration.md` as the language-facing
  package integration source of truth.
- Treat `.llm/plans/go-native-adapter-boundary.md` as the generated adapter and
  runtime-kit implementation source of truth.
- Keep `.llm/plans/gowdk-world-roadmap.md` as the active planning index.
- Keep server fragments in runtime-kit responses, not old action body syntax.
