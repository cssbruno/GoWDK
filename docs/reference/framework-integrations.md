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

## Chi

```go
gowdkHandler, err := gowdkapp.Handler()
if err != nil {
	log.Fatal(err)
}

router := chi.NewRouter()
router.Mount("/", gowdkHandler)
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
gowdkecho.Mount(app, "/*", gowdkHandler)
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
gowdkgin.Mount(engine, "/*path", gowdkHandler)
```

Code reached by the generated handler can read the active Gin context when the
app is mounted through this adapter:

```go
ginContext, ok := gowdkgin.Context(ctx)
```

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
