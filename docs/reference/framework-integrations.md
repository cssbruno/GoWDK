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

## Contract

GOWDK's generated app package is `net/http`-first. Framework compatibility comes
from the standard handler contract, so applications can choose Gin, Chi, Echo,
or plain `net/http` without changing GOWDK core.

