# Addons Reference

Addons register feature IDs with the compiler. Core framework capabilities
such as SSR, actions, APIs, auth, DB, contracts, and rate limiting are fixed
GOWDK-owned features; enabling their feature IDs selects compiler and generator
logic in GOWDK itself. External addon behavior is limited to documented public
interfaces: `gowdk.CSSProcessor` for build-time CSS output,
`gowdk.SEOProvider` for build-time SEO files, and `gowdk.GoBlockConsumer` for
targeted `go addon.<name> {}` blocks.

The config loader parses built-in addon constructors from `gowdk.config.go`
through the Go AST when possible. If an addon constructor comes from another
importable Go module, the loader uses an executable config bridge so
GitHub-hosted addons can return real `gowdk.Addon` values and preserve supported
extension interfaces.

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
- `realtime`
- `auth`
- `db`
- `seo`

Current packages:

- `addons/static`
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
- `addons/realtime`
- `addons/auth`
- `addons/db`
- `addons/seo`

`addons/static` is the build-time static page output boundary. `addons/spa`
remains available for existing configs and static-first SPA navigation; both
enable the existing `spa` feature ID.

Use `gowdk add --list` to print the built-in names the CLI can wire into
`gowdk.config.go`:

```sh
gowdk add --list
gowdk add ssr actions partial realtime
```

`gowdk add <name>` inserts the canonical addon import and appends
`<name>.Addon()` to a literal `Config.Addons` list. It skips constructors that
are already present, including aliased imports. It does not install external Go
modules or discover third-party addons.

## Discovery Policy

Current addon discovery is intentionally narrow:

- `gowdk add --list` prints only built-in addons that the CLI can wire safely.
- Repository docs are the source of truth for documented external addons.
- External addons are resolved by normal Go module tooling after the app imports
  and configures them explicitly.
- Website or registry metadata may list addons, but it must not install,
  execute, or trust addon code.

Do not add remote CLI discovery until registry metadata can describe the addon
name, Go module path, package path, owner, source repository, license,
experimental/deprecated status, supported GOWDK versions or feature contracts,
implemented public interfaces, required external tools, network or process
behavior, and security notes.

Until that metadata, trust policy, and compatibility check exist, GOWDK must not
scan GitHub or module proxies for addons, execute unknown constructors to build
a list, download hidden dependencies, auto-add external modules, or enable an
external addon that is not already present in project Go code. The follow-up for
registry-backed website and CLI discovery is
[#422](https://github.com/cssbruno/GoWDK/issues/422).

`gowdk.NewAddon(name, features...)` creates a marker addon for feature checks.
It does not by itself make the compiler, app generator, or runtime call
third-party code; implement `CSSProcessor`, `SEOProvider`, or
`GoBlockConsumer` when the addon needs behavior.

The current compiler validator checks whether SSR is enabled when a page uses
`load {}` or `go ssr {}`. SPA builds invoke addons that implement
`gowdk.CSSProcessor` or `gowdk.SEOProvider`. Generated app builds invoke
configured addons that implement `gowdk.GoBlockConsumer` for
`go addon.<name> {}` blocks.

The literal config loader recognizes no-argument constructors for most
built-ins and the literal SEO options subset for `addons/seo`:

```go
Addons: []gowdk.Addon{
	static.Addon(),
	spa.Addon(),
	actions.Addon(),
	partial.Addon(),
	ssr.Addon(),
	api.Addon(),
	auth.Addon(),
	embed.Addon(),
	css.Addon(),
	db.Addon(),
	ratelimit.Addon(),
	contracts.Addon(),
	seo.Addon(seo.Options{}),
	realtime.Addon(),
}
```

`addons/contracts` registers the contract-driven runtime feature. The current
runtime registry lives in `runtime/contracts`; generated adapters can use local
in-process dispatch, file outbox, in-memory broker, SSE, or optional nested
Redis Streams, NATS, and WebSocket adapter modules. The addon enables compiler
integration and generated route plumbing; apps still choose their sink in Go
with `RegisterContractEventSink`. See `docs/reference/contracts.md` for Redis,
NATS, SSE, WebSocket, outbox, and composite sink examples. Split runtime
binaries and retry backoff policy remain planned.

`addons/realtime` registers the browser presentation-event fanout feature. It
does not import the optional WebSocket transport dependency or patch the DOM.
Use dependency-free `runtime/contracts/sse` through `realtime.NewSSE` for
one-way browser notifications, or opt into the nested
`runtime/contracts/websocketfanout` module when the app needs WebSocket
sessions. See `docs/reference/realtime.md`.

## Auth Addon

`addons/auth` is experimental 0.x authentication plumbing. It enables the
`auth` feature and provides:

- `PasswordHasher`, with `PBKDF2Hasher` as the default.
- `HashPassword`, `HashPasswordWithIterations`, and `VerifyPassword` helpers
  backed by Go standard-library PBKDF2-HMAC-SHA256.
- Signed-cookie `Sessions` that implement `runtime/auth.Provider` for native
  `role:` and `permission:` guards.

The cryptography and dependency stance is recorded in
[ADR 0011](../engineering/decisions/0011-auth-addon-cryptography.md).

Use the default hasher:

```go
encoded, err := auth.HashPassword(password)
if err != nil {
	return err
}
if !auth.VerifyPassword(password, encoded) {
	return errors.New("invalid credentials")
}
```

`HashPasswordWithIterations` and `PBKDF2Hasher{Iterations: ...}` reject values
below `MinIterations`; leave `Iterations` unset to use `DefaultIterations`.
Verification also rejects malformed PBKDF2 encodings that do not match the
canonical salt, key, and iteration policy emitted by `HashPassword`.

Or replace it behind the small interface:

```go
type PasswordStore struct {
	Hasher auth.PasswordHasher
}
```

Session secrets fail closed. Pass a direct `Secret` or read from a runtime
environment variable with `SecretEnv`; do not set both. Either value must be at
least 32 bytes. Environment secret values are used as exact bytes. Errors name
the setting, never the secret value.

```go
sessions, err := auth.New(auth.Options{
	SecretEnv:  auth.DefaultSessionSecretEnv,
	CookieName: "myapp_session",
	TTL:        12 * time.Hour,
})
if err != nil {
	return err
}
```

`CookieName` must be a valid HTTP cookie name. A zero `TTL` uses
`DefaultSessionTTL`; explicit positive values must be at least one second, and
negative values are rejected. Issued sessions require a non-empty
`Principal.ID`.

Register the session provider from generated app hook code when using native
RBAC guard IDs:

```go
func GOWDKAuthProvider() gowdkauth.Provider {
	return sessions.Provider()
}
```

GOWDK owns generated guard dispatch, CSRF validation, signed session cookie
helpers, and native RBAC checks. Application Go owns user lookup, credential
policy, MFA, OAuth, account recovery, durable storage, session lifetime,
custom guard decisions, and backend resource authorization.

For generated actions, ordering matters:

- A public login action has no guard, so generated CSRF validation runs before
  form decoding and before the login handler.
- A protected action, such as logout, runs rate limiting and guards first. A
  missing or invalid session fails at the guard step before CSRF validation.
- If the guard succeeds but the CSRF token is missing or invalid, generated
  code returns HTTP 403 `invalid csrf token` with `Cache-Control: no-store`.

See `examples/auth-guard` for a small public-login and protected-dashboard
flow.

## DB Addon

`addons/db` registers the database helper feature and provides thin
`database/sql` plumbing: `Open`, readiness checks, `WithTx`, and ordered
user-authored SQL migration application. It imports no SQL driver and owns no
schema, query generation, repository abstraction, or domain logic. See
`docs/reference/db.md` for the migration tracking contract and sqlc
walkthrough.

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

Custom addons are configured in Go, not declared by `.plugin.gwdk` source
files. Use addon constructor options for addon-level configuration, implement
`gowdk.CSSProcessor` for build-time CSS output, and implement
`gowdk.GoBlockConsumer` when the addon needs source-local input through
`go addon.<name> {}` blocks.

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
generated app directory. The AST config loader and executable config bridge both
preserve this interface; external addons are not downgraded to inert feature
markers when they implement `GoBlockConsumer`. If `go addon.<name> {}` targets
an enabled addon that does not implement `GoBlockConsumer`, or whose
`GoBlockTargets` does not include the exact target, `gowdk check` and builds
fail with `unsupported_addon_go_block_target`.

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

`addons/seo` emits `sitemap.xml` and `robots.txt` at build time. It requires
`seo.Options.BaseURL`, includes static and `paths {}`-expanded SPA routes, and
records request-time route exclusions in the build report. See [seo.md](seo.md).

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
