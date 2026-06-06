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

```go
gowdkHandler, err := gowdkapp.Handler()
if err != nil {
	log.Fatal(err)
}

app := echo.New()
app.Any("/*", echo.WrapHandler(gowdkHandler))
```

## Gin

```go
gowdkHandler, err := gowdkapp.Handler()
if err != nil {
	log.Fatal(err)
}

engine := gin.Default()
engine.Any("/*path", gin.WrapH(gowdkHandler))
```

## Fiber

Fiber is not built on `net/http`, so mounting a generated GOWDK app requires
Fiber's adaptor package, for example `adaptor.HTTPHandler(gowdkHandler)`.
That bridge adds adapter overhead and Fiber-specific semantics around request
and response objects, middleware ordering, context cancellation, streaming,
and protocol features. Keep security, auth, validation, and persistence in
normal Go handlers behind GOWDK's `net/http` contract, and test behavior through
the final Fiber stack before deploying.

GOWDK core does not import Fiber and generated apps do not emit Fiber-specific
code by default. A future optional Fiber adapter should wrap the same generated
`http.Handler` contract instead of changing generated app output.

## Contract

GOWDK's generated app package is `net/http`-first. Framework compatibility comes
from the standard handler contract, so applications can choose Gin, Chi, Echo,
Fiber through an adaptor, or plain `net/http` without changing GOWDK core.
