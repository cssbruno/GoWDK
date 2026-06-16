# Go Interop

GOWDK source declares web surface. Normal Go packages own behavior.

## Build Data

`build {}` can call one no-argument Go function:

```gwdk
import interop "github.com/acme/site/content"

build {
  => interop.HomePage()
}
```

Bare same-package calls are also supported when the page directory is a
buildable Go package:

```gwdk
build {
  => HomePage()
}
```

Supported return shapes:

```go
func HomePage() HomeCopy
func HomePage() (HomeCopy, error)
```

The returned value must JSON-encode to a non-empty object. Scalar object fields
become string interpolation data for `view {}`. Build-helper stderr is kept
separate from the JSON payload; successful logging does not corrupt build data,
and failed helpers include stderr in the error message.

Route params can be used in literal `build {}` expressions with
`param("name")`. Passing route params into Go build functions remains deferred.

## Actions And APIs

Actions and APIs bind exported same-package Go functions or default `go {}`
block functions with these signatures:

```go
func Submit(context.Context) (response.Response, error)
func Submit(context.Context, SignupInput) (response.Response, error)
func Submit(context.Context, *SignupInput) (response.Response, error)
func Submit(context.Context, form.Values) (response.Response, error)
func Health(context.Context, *http.Request) (response.Response, error)
```

`SignupInput` must be an exported same-package struct with supported scalar
form fields. Missing handlers are non-fatal in development builds and produce
generated HTTP 501 handlers. Production builds require bound handlers unless
`Build.AllowMissingBackend` or `--allow-missing-backend` is set.

Use:

```sh
gowdk inspect go-bindings --ssr
gowdk generate stubs
```

`inspect go-bindings` reports actions, APIs, fragments, SSR load functions,
build-time Go calls, and web command/query references with status, package,
symbol, signature, input metadata, reason, and next-step suggestions.

`generate stubs` starts conservatively with missing action/API handlers. It
writes `gowdk_stubs.go` next to the owning source package and refuses to
overwrite an existing stub file.

`gowdk check` and `gowdk build` also surface binding near-misses as non-fatal
warnings, so a wrong signature or a casing mistake is visible without reading
the JSON report or running a strict production build:

- `unsupported_backend_signature` — a same-named Go function exists but its
  signature is not a supported action/API/load/fragment shape.
- `unexported_backend_handler` — a same-named Go function exists but is not
  exported, so binding cannot see it (for example `func submit` when the block
  expects `Submit`).
- `ambiguous_backend_handler` — the same handler is declared in both
  same-package Go and an inline `go {}` block. (When both live in the same
  compiled package, Go's own redeclaration error surfaces first.)

A handler with no candidate function stays silent because the default workflow
generates 501 stubs for not-yet-implemented handlers; strict production builds
still fail closed through `backend_binding_required`.

When the sibling Go package fails to compile, binding does not fall back to an
inline `go {}` block and report a misleading bound handler: the load/action/API
binding stays "could not be inspected" and the package error itself is reported
by `go_package_error`.

## Load Functions

Request-time pages with `server {}` bind same-package functions named
`Load<PageID>`:

```go
func LoadDashboard(ssr.LoadContext) map[string]any
func LoadDashboard(ssr.LoadContext) (map[string]any, error)
```

`context.Context` is available through `ssr.LoadContext` /
`runtime/guard.Context`. Route, endpoint, params, typed params, CSRF, session,
and request metadata are available through `runtime/app` helpers where the
generated route attaches them.

## Route Params

Generated request-time route handlers attach raw params through
`app.Params(ctx)` and decoded typed params through `app.TypedParams(ctx)`.
The lower-level `runtime/route` helpers decode `string`, `int`, `int64`,
`uint`, `uint64`, `bool`, and `float64` from raw params without echoing raw
request values in errors.

Generated per-route param struct types are deferred. Typed load-result and
action-result accessors are also deferred until those result contracts are
stable.

## Middleware And Hooks

Generated apps expose ordinary `net/http` entry points:

```go
gowdkapp.RegisterMiddleware(middleware)
handler, err := gowdkapp.Handler()
mux, err := gowdkapp.ServeMux()
```

Register app-wide middleware before calling `Handler()` or `ServeMux()`, or
wrap the returned handler in startup code. Generated route rewriting, response
transformation hooks, and fetch/navigation interception hooks are not part of
the current contract.
