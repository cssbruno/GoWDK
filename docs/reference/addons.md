# Addons Reference

Addons currently register feature IDs with the compiler. The config loader
parses built-in addon constructors from `gowdk.config.go` through the Go AST
when possible. If an addon constructor comes from another importable Go module,
the loader uses an executable config bridge so GitHub-hosted addons can return
real `gowdk.Addon` values. CSS processors can run during SPA builds, and addon
go block consumers can validate targeted `.gwdk` go blocks and emit generated
app files.

Current feature IDs:

- `spa`
- `actions`
- `partial`
- `ssr`
- `api`
- `embed`
- `css`
- `ratelimit`
- `contracts`

Current packages:

- `addons/spa`
- `addons/actions`
- `addons/partial`
- `addons/ssr`
- `addons/api`
- `addons/embed`
- `addons/css`
- `addons/tailwind`
- `addons/ratelimit`
- `addons/contracts`

The current compiler validator checks whether SSR is enabled when a page uses
`load {}` or `go ssr {}`. SPA builds invoke addons that implement
`gowdk.CSSProcessor`. Generated app builds invoke configured addons that
implement `gowdk.GoBlockConsumer` for `go addon.<name> {}` blocks.

The literal config loader recognizes no-argument constructors for these
built-ins:

```go
Addons: []gowdk.Addon{
	spa.Addon(),
	actions.Addon(),
	partial.Addon(),
	ssr.Addon(),
	api.Addon(),
	embed.Addon(),
	css.Addon(),
	ratelimit.Addon(),
	contracts.Addon(),
}
```

`addons/contracts` registers the contract-driven runtime feature. The current
runtime registry lives in `runtime/contracts`; generated adapters can use local
in-process dispatch, file outbox, Redis Streams, NATS, SSE, or WebSocket
adapters. The addon enables compiler integration and generated route plumbing;
apps still choose their sink in Go with `RegisterContractEventSink`. See
`docs/reference/contracts.md` for Redis, NATS, SSE, WebSocket, outbox, and
composite sink examples. Split runtime binaries and retry backoff policy remain
planned.

External addons use normal Go imports:

```go
import brand "github.com/example/gowdk-brand"

Addons: []gowdk.Addon{
	brand.Addon(),
}
```

External addons are regular importable Go packages. They can live in GitHub
modules, private modules, or local modules referenced with `replace`, and they
can import their own Go dependencies. The project module must already be able
to resolve the addon and its dependency graph with the Go toolchain through
`go.mod`, `go.sum`, `replace`, `GOPRIVATE`, or the user's configured module
proxy settings. GOWDK does not vendor, sandbox, or rewrite addon imports.

## Targeted Go Blocks

External addons can opt into targeted inline Go through
`gowdk.GoBlockConsumer`:

```go
type GoBlockConsumer interface {
	GoBlockTargets() []string
	ValidateGoBlock(gowdk.GoBlockTarget, gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic
	GeneratedGo(gowdk.GoBlockTarget, gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error)
}
```

For a `.gwdk` block like `go addon.contracts {}`, the addon named
`contracts` receives target `addon.contracts`. `GoBlockTargets` controls which
targets the addon accepts. `ValidateGoBlock` can return addon-owned diagnostics.
`GeneratedGo` can return files relative to the generated app directory; `.go`
files are formatted before writing. File paths must stay relative to the
generated app directory.

`addons/tailwind` is an experimental Tailwind v4 CSS processor wrapper around
the standalone CLI. When `Options.Command` is omitted it uses `tailwindcss` from
`PATH`. If the executable is missing, builds fail with an install-required
error. GOWDK does not download Tailwind, use npm, add Tailwind to the compiler
core or runtime core, or generate Tailwind v3 content configuration. The literal
config loader recognizes `tailwind.Addon` with a literal `tailwind.Options`
value.

`addons/ratelimit` provides request-time HTTP middleware with fixed-window
decisions, rate-limit response headers, a process-local in-memory store, and a
Redis-backed store adapter. It does not add a Redis client dependency or choose
an application policy automatically. When `ratelimit.Addon()` is enabled and a
generated app has action, API, fragment, SSR, or split-backend proxy routes, the
generated package exposes `RegisterRateLimiter(*ratelimit.Limiter)`.

```go
store := ratelimit.NewInMemoryStore(ratelimit.InMemoryOptions{})
limiter, err := ratelimit.New(ratelimit.Options{
	Limit:  60,
	Window: time.Minute,
	Store:  store,
})
if err != nil {
	return err
}

handler := limiter.Middleware(next)
```

Generated apps can register the same limiter from user-owned Go in the
generated package:

```go
package gowdkapp

import (
	"time"

	"github.com/cssbruno/gowdk/addons/ratelimit"
)

func init() {
	store := ratelimit.NewInMemoryStore(ratelimit.InMemoryOptions{})
	limiter, err := ratelimit.New(ratelimit.Options{
		Limit:  60,
		Window: time.Minute,
		Store:  store,
	})
	if err != nil {
		panic(err)
	}
	RegisterRateLimiter(limiter)
}
```

Generated action, API, fragment, SSR, and split-backend proxy handlers call the
registered limiter before guards and user handler logic. If no limiter is
registered, the generated handlers continue normally.

Distributed deployments can use `ratelimit.NewRedisStore` with a small
`RedisClient` adapter:

```go
redisStore, err := ratelimit.NewRedisStore(ratelimit.RedisOptions{
	Client: redisClientAdapter,
})
if err != nil {
	return err
}

limiter, err := ratelimit.New(ratelimit.Options{
	Limit:  300,
	Window: time.Minute,
	Store:  redisStore,
})
```

Example adapter for `github.com/redis/go-redis/v9`:

```go
import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type GoRedisRateLimitClient struct {
	Client *redis.Client
}

func (client GoRedisRateLimitClient) EvalInt64s(ctx context.Context, script string, keys []string, args ...string) ([]int64, error) {
	values, err := client.Client.Eval(ctx, script, keys, args).Slice()
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case int64:
			out = append(out, typed)
		case int:
			out = append(out, int64(typed))
		default:
			return nil, fmt.Errorf("unexpected redis rate-limit value %T", value)
		}
	}
	return out, nil
}
```
