# Product Vision

## Product Name

GOWDK

WDK does not have a canonical expansion. No one knows what it stands for; GOWDK
just ships apps.

## One-Line Description

GOWDK is a Go-first web app compiler paired with GOWDK Kit, its app/runtime
layer.

## Product Layer Names

- GOWDK: the `.gwdk` language, parser, analyzer, component/page compiler,
  diagnostics, LSP, and generated adapter source.
- GOWDK Kit: the app/runtime layer for serving, routing, request context, form
  decoding, response envelopes, actions, APIs, CSRF, partial fragments, SSR
  contracts, embedded assets, and one-binary or split-binary wiring.
- `gowdk`: the CLI that runs the compiler, produces build artifacts, and wires
  GOWDK Kit output.

This mirrors the Svelte/SvelteKit product split at the naming level:
compiler/language first, app/runtime kit second. It does not rename Go module
paths or public runtime packages yet.

Use the names this way:

| Name | Product Meaning |
| --- | --- |
| GOWDK | Language/compiler layer. It owns `.gwdk`, AST/analyzer/IR, generated Go adapter source, build output, manifests, route metadata, asset metadata, diagnostics, formatting, and LSP. |
| GOWDK Kit | Runtime/app layer. It owns `runtime/`, `addons/`, generated `net/http` serving, backend routing, form decoding, responses, actions, APIs, fragments, SSR hooks, embedded assets, contract runtime, and one-binary or split-binary wiring. |
| `gowdk` | CLI and Go package/module spelling. The CLI drives both layers. |

Avoid treating `GOWDK World`, `GOWDK Core`, or `GOWDK Framework` as separate
public names. In docs, use `compiler core`, `Kit core`, or `repository core`
when `core` is unavoidable.

## Product Shape

GOWDK grows as two coordinated parts:

```text
GOWDK
component/page compiler
        +
GOWDK Kit
app/runtime layer
        =
Go-first full web app
```

GOWDK owns package-peer `.gwdk` files, pages, layouts, components, build-time
output, CSS, islands, manifests, diagnostics, endpoint metadata, and generated
adapter source. GOWDK Kit owns serving, routing, request context helpers, form
decoding, response envelopes, actions, APIs, CSRF, partial fragments, SSR
contracts, embedded assets, contract runtime, and one-binary or split-binary
wiring.

User application behavior stays in normal Go packages. GOWDK should improve Go
web authoring through `.gwdk` compilation, GOWDK Kit contracts, and generated
adapters before considering any custom Go compiler work.

## Execution Lanes

- Build-time page lane: full pages default to static SPA/prerender output.
- Backend endpoint lane: actions, APIs, and fragments run at request time
  without making the page itself request-rendered.
- Request-time page lane: `@render ssr` pages are compiled into generated SSR
  handlers and run through GOWDK Kit.

SSR is integrated into the compiler/runtime code path and selected per page. The
current `addons/ssr` package and `--ssr` flag are feature gates for enabling
that lane in config and CLI flows; they are not a separate product layer.

## Target Users

- Go developers building product applications who want build-time page output,
  typed backend behavior, and one-binary deployment.
- Small teams that want Go-first UI authoring without a large JavaScript
  application stack.
- Builders who want request-time page rendering only where request context,
  guards, sessions, or per-request data actually matter.

## Problem

Modern web frameworks often force teams into one rendering ideology: full SSR,
full SPA, or deploy-only static output. GOWDK should let Go teams compile
portable `.gwdk` files into build-time pages, components, typed backend
endpoints, partial updates, request-time pages, and deployable Go binaries while
keeping the route, handler, and runtime contracts explicit.

## Differentiation

- Files are portable: routes and layouts are declared in files, not implied by
  folder nesting.
- Full pages default to build-time SPA output.
- Actions, APIs, and fragments are request-time endpoint behavior, not page
  route kinds.
- Partial updates use server fragments instead of full-page request rendering.
- SSR is an integrated non-default request-time page lane selected per page.
- User behavior stays in normal Go packages; generated Go is adapter glue.
- Production can ship as one Go binary with embedded frontend assets and
  generated request-time handlers.

## Success Metrics

- Developers can explain GOWDK in one sentence: GOWDK ships Go web apps through
  a component/page compiler plus GOWDK Kit.
- The compiler can produce real build-time page output, route metadata, endpoint
  metadata, CSS/assets, generated adapter Go, and deployable artifacts from
  package-peer `.gwdk` files.
- Actions and APIs bind to exact exported Go handlers with typed form decoding,
  explicit request context, safe response envelopes, CSRF, and production-safe
  error handling.
- `@render ssr` pages can execute `load {}`, guards, typed route params,
  request-aware layouts, redirects, and error boundaries through generated SSR
  handlers.
- One-binary and split-binary deployments use the same route and endpoint
  metadata.

## Constraints

- Language: Go-first compiler, runtime, and deployment.
- Behavior: domain logic, auth, validation, storage, and services stay in user
  Go packages.
- Generated code: generated Go is adapter glue, not generated application
  logic.
- Styling: CSS tooling is plugin-driven. Tailwind is optional, not core.
- JavaScript: generated JavaScript may enhance navigation, forms, fragments, and
  local UI state, but normal app contracts must not depend on user-written
  JavaScript.
- Rendering: full pages default to build-time SPA output; request-time page
  rendering is explicit with `@render ssr` or a future hybrid branch.
- Deployment: one-binary production deploy must work with and without
  request-time page rendering.
- Extensibility: actions, APIs, partials, SSR, embed, CSS plugins, framework
  adapters, and WASM islands should remain modular implementation boundaries
  around the same core metadata.
