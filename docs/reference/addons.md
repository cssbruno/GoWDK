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

## Addon Lifecycle

An addon participates in up to four ordered phases. Each phase corresponds to a
specific interface, and an addon implements only the interfaces for the phases
it needs. The base `gowdk.Addon` (`Name()`, `Features()`) is always required;
the rest are opt-in extension points.

1. **Config loading** — `gowdk.Addon`. The loader resolves the constructor from
   `gowdk.config.go` (Go AST for built-ins, executable bridge for external
   modules) and reads `Features()`. Feature IDs gate compiler and generator
   logic through `Config.HasFeature`.
2. **Compiler validation** — `gowdk.GoBlockConsumer.ValidateGoBlock` validates
   `go addon.<name> {}` blocks and may return addon-owned diagnostics. Built-in
   feature gates also run here (for example, a page using `server {}` or
   `go server {}` requires the `ssr` feature).
3. **Generated output** — build-time emitters run while writing output:
   `gowdk.CSSProcessor.ProcessCSS` (CSS), `gowdk.SEOProvider.SEOOptions`
   (`sitemap.xml`/`robots.txt`), and `gowdk.GoBlockConsumer.GeneratedGo` (files
   relative to the generated app directory, formatted before writing).
4. **Runtime hook registration** — generated apps register runtime hooks from
   user-owned Go in the generated package, for example
   `RegisterRateLimiter(*ratelimit.Limiter)`, custom `GOWDKGuardRegistry`
   entries, `GOWDKAuthProvider() auth.Provider`, or
   `RegisterContractEventSink(...)`. The built-in auth addon is the narrow
   exception: `auth.Addon(auth.Options{...})` wires its own session provider and
   `auth.required` guard. GOWDK never calls third-party runtime code implicitly;
   the app wires it. External addons do not become implicit runtime services;
   app-owned background work is declared separately through
   `Config.Lifecycle.Services`.

### Addon categories

The contract distinguishes addon categories by which interfaces they implement.
The registry records this in each entry's `publicInterfaces`:

| Category | Interface(s) | Phase |
| --- | --- | --- |
| Marker / feature addon | `gowdk.Addon` only | config loading (feature gate) |
| Compiler addon | `gowdk.GoBlockConsumer` | compiler validation + generated output |
| CSS processor | `gowdk.CSSProcessor` | generated output |
| Build-time provider | `gowdk.SEOProvider` | generated output |
| Runtime addon | generated-app registration hooks | runtime hook registration |

A single addon can span categories (for example `addons/css` implements both
`gowdk.Addon` and `gowdk.CSSProcessor`).

## Version and Feature Handshake

Addons declare what they target two ways:

- **Feature handshake.** `Features()` declares feature IDs (see below).
  `Config.HasFeature` gates the matching compiler and generator logic; a page
  that uses a capability without its feature addon is reported by a diagnostic
  (for example `missing_ssr_addon`).
- **Version handshake.** A registry entry declares the GOWDK line it supports
  with `minGOWDK` and optional `maxGOWDK`. `addonregistry.Entry.SupportsVersion`
  checks a concrete CLI version against those inclusive bounds and returns
  `VersionSupported`, `VersionUnsupported`, or `VersionUnknown` (when a bound or
  the queried version is unset or unparseable, so tooling warns rather than
  wrongly blocking). `Registry.UnsupportedFor(version)` lists entries a given
  CLI version excludes. The curated `compatibility` field
  (`compatible`/`incompatible`/`unknown`) is the human-reviewed signal that
  complements the computed bound check. Both are covered by
  `internal/addonregistry` tests.

## Failure Modes

- **Unsupported addon go block** — `go addon.<name> {}` targeting an enabled
  addon that does not implement `gowdk.GoBlockConsumer`, or whose
  `GoBlockTargets` omits the exact target, fails `gowdk check` and builds with
  `unsupported_addon_go_block_target`.
- **Missing required external tool** — an addon that shells out to a tool (for
  example `addons/tailwind`) fails the build with an install-required error when
  the tool is absent; GOWDK does not download it.
- **Missing feature addon** — using a capability without enabling its addon is a
  compiler diagnostic, not a silent no-op.
- **Version-incompatible addon** — `SupportsVersion` returns `VersionUnsupported`
  for a CLI version outside an entry's `minGOWDK`/`maxGOWDK`. Tooling can surface
  this; build-time auto-enforcement of the version bound remains a deliberate
  follow-up (see Discovery Policy).
- **Deprecated or experimental lifecycle** — surfaced in `gowdk add --list
  --registry` output so users see stability before wiring an addon.
- **External addon resolution** — external addons resolve through normal Go
  module tooling; missing modules surface as ordinary Go build errors.

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
- `observability`
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
- `addons/observability`
- `addons/auth`
- `addons/db`
- `addons/seo`

Request-time helpers live under `runtime/` even when an addon enables the
feature. Config files still import `addons/<name>` and call `<name>.Addon()`.
Generated apps and request-time extension code should import the runtime package
for helpers:

| Config addon | Request-time helpers |
| --- | --- |
| `addons/actions` | `runtime/actions` |
| `addons/api` | `runtime/api` |
| `addons/partial` | `runtime/partial` |
| `addons/ratelimit` | `runtime/ratelimit` |
| `addons/realtime` | `runtime/realtime` |
| `addons/ssr` | `runtime/ssr` |

The addon packages re-export their runtime helpers for 0.x compatibility, but
new generated app code imports `runtime/<name>` so request-time binaries do not
pull in the root build-time config package through addon markers.

`addons/static` is the build-time static page output boundary. `addons/spa`
remains available for existing configs and static-first SPA navigation; both
enable the existing `spa` feature ID.

Use `gowdk add --list` to print the addable built-in names the CLI can wire into
`gowdk.config.go`:

```sh
gowdk add --list
gowdk add --list --registry
gowdk add --list --registry --json
gowdk add ssr actions partial realtime observability
gowdk add seo --base-url https://example.com
```

`gowdk add <name>` inserts the canonical addon import and appends
`<name>.Addon()` to a literal `Config.Addons` list. It skips constructors that
are already present, including aliased imports. It does not install external Go
modules or discover third-party addons. `gowdk add seo` also requires
`--base-url` so the generated config can pass SEO build validation.

## Discovery Policy

Current addon discovery is intentionally narrow and metadata-first:

- `gowdk add --list` prints only built-in addons that the CLI can wire safely.
- `gowdk add --list --registry` prints the checked-in addon registry metadata.
- `gowdk add --list --registry --json` prints the same metadata as JSON so
  docs and website tooling can render entries without importing or executing
  addon code.
- Repository docs are the source of truth for documented external addons.
- External addons are resolved by normal Go module tooling after the app imports
  and configures them explicitly.
- Registry metadata may list addons, but it must not install, execute, or trust
  addon code.

The machine-readable registry lives in `internal/addonregistry/registry.json`.
Each entry must describe:

- `name`, `summary`, and `description`;
- `kind`: `built-in` or `documented-external`;
- `lifecycle`: `stable`, `experimental`, or `deprecated`;
- `compatibility`: `compatible`, `incompatible`, or `unknown`;
- `minGOWDK` and optional `maxGOWDK`;
- `modulePath`, `packagePath`, and `importPath`;
- `owner`, `sourceRepository`, `license`, and `documentation`;
- enabled `features`;
- implemented `publicInterfaces`, such as `gowdk.Addon`,
  `gowdk.CSSProcessor`, `gowdk.SEOProvider`, or `gowdk.GoBlockConsumer`;
- `requiredExternalTools`;
- `networkBehavior`, `processBehavior`, and `securityNotes`;
- `trust.level` and `trust.notes`;
- `constructor.addable`, `constructor.package`, `constructor.function`, and
  optional constructor option metadata.

`constructor.addable` is intentionally separate from registry visibility. A
registry entry can be visible to docs and CLI discovery while still requiring
manual Go-module setup. Documented external addons must not be addable by
`gowdk add`; users import and configure them through normal Go module tooling.
The bundled registry currently contains built-in entries only, plus addable and
non-addable built-in distinctions such as `addons/tailwind`. The schema and CLI
table are ready for documented external, deprecated, and incompatible entries
when the project has real entries to publish.

The registry now provides a computed version handshake
(`addonregistry.Entry.SupportsVersion` / `Registry.UnsupportedFor`, see
[Version and Feature Handshake](#version-and-feature-handshake)) on top of the
curated `compatibility` field. Even so, GOWDK must not scan GitHub or module
proxies for addons, execute unknown constructors to build a list, download
hidden dependencies, auto-add external modules, or enable an external addon that
is not already present in project Go code. Remote registry sync and *build-time*
automatic compatibility enforcement (failing a build on an out-of-range
`minGOWDK`) remain out of scope for the local registry slice; the handshake is
available for tooling to warn.

`gowdk.NewAddon(name, features...)` creates a marker addon for feature checks.
It does not by itself make the compiler, app generator, or runtime call
third-party code; implement `CSSProcessor`, `SEOProvider`, or
`GoBlockConsumer` when the addon needs build-time behavior. Runtime background
services stay app-owned and are imported through `Config.Lifecycle.Services`.

The current compiler validator checks whether SSR is enabled when a page uses
`server {}` or `go server {}`. SPA builds invoke addons that implement
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
	observability.Addon(),
	seo.Addon(seo.Options{
		BaseURL: "https://example.com",
	}),
	realtime.Addon(),
}
```

`addons/contracts` registers the contract-driven runtime feature. The current
runtime registry lives in `runtime/contracts`; generated adapters can use local
in-process dispatch, file outbox, in-memory broker, SSE, or optional nested
Redis Streams, NATS, and WebSocket adapter modules. The addon enables compiler
integration and generated route plumbing; apps still choose their sink in Go
with `RegisterContractEventSink`. See `docs/reference/contracts.md` for Redis,
NATS, SSE, WebSocket, outbox, worker backoff, and composite sink examples.
Separate worker/cron binary generators remain planned deployment tooling.

`addons/realtime` registers the browser presentation-event fanout feature. It
does not import the optional WebSocket transport dependency or patch the DOM.
Use dependency-free `runtime/contracts/sse` through `realtime.NewSSE` for
one-way browser notifications, including server-owned audience scoping through
`WithSSEAudienceFromRequest`, or opt into the nested
`runtime/contracts/websocketfanout` module when the app needs WebSocket
sessions. See `docs/reference/realtime.md`.

`addons/observability` registers the generated trace instrumentation feature.
Debug builds wire route, endpoint, guard, browser navigation, and island spans
to the dependency-free `runtime/trace` collector and local viewer. The runtime
also exposes trace/span `slog` helpers, local health snapshots, and
process-local generated route metrics; optional OTLP export is isolated in the
nested `runtime/trace/otel` module. See `docs/reference/observability.md`.

## Auth Addon

`addons/auth` is experimental 0.x authentication plumbing. It enables the
`auth` feature and provides:

- `PasswordHasher`, with `PBKDF2Hasher` as the default.
- `HashPassword`, `HashPasswordWithIterations`, and `VerifyPassword` helpers
  backed by Go standard-library PBKDF2-HMAC-SHA256.
- Signed-cookie and revocable `Sessions` that implement
  `runtime/auth.Provider` for native `role:` and `permission:` guards.
- A minimal `SessionStore` boundary plus `InMemorySessionStore` for tests and
  single-process development.
- Generated app startup wiring for `auth.required`, `role:`, and
  `permission:` guards when `auth.Addon` is enabled. Generated startup uses the
  signed-cookie baseline because real revocable stores are application runtime
  objects, not build-time config values.

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
`Principal.ID`. The default mode is `SessionModeSignedCookie`; the cookie
carries the principal ID, roles, permissions, optional authorization version,
and expiry. It is dependency-free and useful for development or bounded simple
deployments, but it is not server-revocable.

Use `SessionModeRevocable` when the next protected request must observe logout,
session revocation, account disablement, role/permission changes, or
authorization-version changes:

```go
store := auth.NewInMemorySessionStore() // replace with app-owned durable store
sessions, err := auth.New(auth.Options{
	SecretEnv:  auth.DefaultSessionSecretEnv,
	Mode:       auth.SessionModeRevocable,
	Store:      store,
	TTL:        12 * time.Hour,
	IdleTTL:    30 * time.Minute,
	KeyID:      "2026-06",
	Insecure:   true, // local HTTP development only
})
if err != nil {
	return err
}
```

Revocable cookies carry a signed session pointer. `Principal` is resolved from
the store on every request, so applications can update the store record after
role removal or account disablement. `AuthorizationVersion` is compared with the
version issued into the cookie; a mismatch rejects the request and forces a new
session. `ClearRequest` revokes the current session before clearing the browser
cookie, `RevokeSession` invalidates one session, `RevokePrincipal` invalidates
all current sessions for one principal, and `Rotate` revokes the current
session before issuing a fresh one after authentication or sensitive changes.

Signing-key rotation is explicit. Set `KeyID` for the current key and put
bounded previous keys in `PreviousKeys`; a previous key stops verifying after
its `AcceptUntil` time. Keep previous-key windows short and remove retired keys.

In generated apps, configure the addon instead of writing guard hook files:

```go
auth.Addon(auth.Options{
	SecretEnv:  "GOWDK_AUTH_SESSION_SECRET",
	CookieName: "myapp_session",
	TTL:        12 * time.Hour,
	Insecure:   true, // local HTTP development only
})
```

Generated startup constructs the signed-cookie session manager, registers it as
the native RBAC provider, and adds the default `auth.required` guard.
Login/logout handlers can issue or clear the same cookie through the configured
manager:

```go
sessions, err := auth.DefaultSessions()
if err != nil {
	return response.Response{}, err
}
cookie, err := sessions.Cookie(auth.Principal{ID: userID, Roles: []string{"user"}})
```

Custom guard IDs still require `GOWDKGuardRegistry`. Native `role:` and
`permission:` guards require `GOWDKAuthProvider` only when the auth addon is not
configured.

GOWDK owns generated guard dispatch, CSRF validation, signed session cookie
helpers, the revocable session interface, and native RBAC checks. Application Go
owns user lookup, credential policy, MFA, OAuth, account recovery, durable
storage, concurrent-session policy, custom guard decisions, and backend resource
authorization.

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

`runtime/ratelimit` provides request-time HTTP middleware with fixed-window
decisions, rate-limit response headers, a process-local in-memory store, and a
Redis-backed store adapter. It does not add a Redis client dependency or choose
an application policy automatically. When `ratelimit.Addon()` is enabled and a
generated app has action, API, fragment, SSR, or split-backend proxy routes, the
generated package exposes `RegisterRateLimiter(*ratelimit.Limiter)`.

`addons/seo` emits `sitemap.xml` and `robots.txt` at build time, enables
supported `jsonld` structured-data metadata, and can configure a generated app
runtime `/sitemap.xml` provider. It requires `seo.Options.BaseURL`, includes
public static and `paths {}`-expanded SPA routes, and records request-time,
`noindex`, or default-denied route exclusions in the build report. See
[seo.md](seo.md).

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

	"github.com/cssbruno/gowdk/runtime/ratelimit"
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
