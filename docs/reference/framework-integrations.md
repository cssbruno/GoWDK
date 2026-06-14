# Framework Integrations

Generated GOWDK apps expose standard `net/http` handlers. The generated server
uses the same handler that other Go routers can mount or wrap.

## Generated API

Generated apps include an importable package:

```go
import "gowdk-generated-app/gowdkapp"

handler, err := gowdkapp.Handler()
mux, err := gowdkapp.ServeMux()
```

`Handler()` returns `http.Handler`. `ServeMux()` returns the concrete
`*http.ServeMux`.

Route-aware framework adapters can also consume the generated `openapi.json`
report:

```go
openAPI, err := os.ReadFile("dist/site/openapi.json")
if err != nil {
	log.Fatal(err)
}
```

Use `WithPrefix("/app")` when the host framework serves the generated GOWDK app
below a prefix. The adapter registers host-framework routes under that prefix
and strips it before dispatching to the generated handler, so the generated
router still sees the original GOWDK paths. The adapter also keeps a generated
handler fallback mounted for page and asset routes that are not listed in
OpenAPI, and rewrites same-origin root-relative `Location` headers and generated
HTML URLs under the prefix.

When publishing the generated OpenAPI report for a prefixed mount, rewrite its
server URL with the dependency-free helper:

```go
import gowdkadapters "github.com/cssbruno/gowdk/runtime/adapters"

prefixedSpec, err := gowdkadapters.OpenAPIWithServerURL(openAPI, "/app")
```

## Chi

The Chi adapter is a nested optional module. Add it only when the application
uses Chi:

```sh
go get github.com/cssbruno/gowdk/runtime/adapters/chi
```

```go
import gowdkchi "github.com/cssbruno/gowdk/runtime/adapters/chi"
import "github.com/go-chi/chi/v5"
```

```go
gowdkHandler, err := gowdkapp.Handler()
if err != nil {
	log.Fatal(err)
}

router := chi.NewRouter()
if err := gowdkchi.MountOpenAPI(router, openAPI, gowdkHandler, gowdkchi.WithPrefix("/app")); err != nil {
	log.Fatal(err)
}
```

## Echo

The Echo adapter is a nested optional module. Add it only when the application
uses Echo:

```sh
go get github.com/cssbruno/gowdk/runtime/adapters/echo
```

```go
import gowdkecho "github.com/cssbruno/gowdk/runtime/adapters/echo"
import "github.com/labstack/echo/v5"

gowdkHandler, err := gowdkapp.Handler()
if err != nil {
	log.Fatal(err)
}

app := echo.New()
if err := gowdkecho.MountOpenAPI(app, openAPI, gowdkHandler, gowdkecho.WithPrefix("/app")); err != nil {
	log.Fatal(err)
}
```

Code reached by the generated handler can read the active Echo context when the
app is mounted through this adapter:

```go
echoContext, ok := gowdkecho.Context(ctx)
```

## Gin

The Gin adapter is a nested optional module. Add it only when the application
uses Gin:

```sh
go get github.com/cssbruno/gowdk/runtime/adapters/gin
```

```go
import gowdkgin "github.com/cssbruno/gowdk/runtime/adapters/gin"
import "github.com/gin-gonic/gin"

gowdkHandler, err := gowdkapp.Handler()
if err != nil {
	log.Fatal(err)
}

engine := gin.Default()
if err := gowdkgin.MountOpenAPI(engine, openAPI, gowdkHandler, gowdkgin.WithPrefix("/app")); err != nil {
	log.Fatal(err)
}
```

Code reached by the generated handler can read the active Gin context when the
app is mounted through this adapter:

```go
ginContext, ok := gowdkgin.Context(ctx)
```

Gin rejects ambiguous route patterns such as two same-method dynamic routes that
could match the same request. `MountOpenAPI` returns a mount-time error naming
the conflicting GOWDK routes instead of letting Gin panic during registration.

## Fiber

The Fiber adapter is a nested optional module. Add it only when the application
uses Fiber:

```sh
go get github.com/cssbruno/gowdk/runtime/adapters/fiber
```

```go
import gowdkfiber "github.com/cssbruno/gowdk/runtime/adapters/fiber"
import "github.com/gofiber/fiber/v2"

gowdkHandler, err := gowdkapp.Handler()
if err != nil {
	log.Fatal(err)
}

app := fiber.New()
gowdkfiber.Mount(app, "/*", gowdkHandler)
```

Code reached by the generated handler can read the active Fiber context when the
app is mounted through this adapter:

```go
fiberContext, ok := gowdkfiber.Context(ctx)
```

Fiber is not built on `net/http`, so `runtime/adapters/fiber` uses Fiber's
adaptor package internally. That bridge adds adapter overhead and
Fiber-specific semantics around request and response objects, middleware
ordering, context cancellation, streaming, and protocol features. Keep security,
auth, validation, and persistence in normal Go handlers behind GOWDK's
`net/http` contract, and test behavior through the final Fiber stack before
deploying.

## Contract

GOWDK's generated app package is `net/http`-first. Framework compatibility comes
from the standard handler contract, so applications can choose Gin, Chi, Echo,
Fiber through an adaptor, or plain `net/http` without changing GOWDK Runtime or the
generated app contract.
Generated apps do not emit framework-specific code by default; optional adapter
packages wrap the same generated `http.Handler`.
Framework context accessors are integration escape hatches for applications
that intentionally opt into a framework adapter. GOWDK route declarations,
handler binding, CSRF, fragments, APIs, and SSR behavior still use the generated
`net/http` request flow as the source of truth.

Adapters register host-framework routes from generated metadata only; they do
not move generated protections into framework middleware. Keep middleware order
simple: framework recovery, request logging, and app-owned auth can wrap the
mounted routes, but do not duplicate generated request body limits, CSRF checks,
panic boundaries, or response writing unless the app deliberately replaces that
policy and tests the final stack.
