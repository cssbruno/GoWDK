# Deployment

GOWDK currently supports three practical output shapes:

- Build output files from `gowdk build --out`.
- A generated Go app from `gowdk build --out --app`.
- A local-platform binary or Go `js/wasm` artifact from the generated app.

Deployment orchestration is user-owned. GOWDK can emit a minimal Docker context
for one-binary deploys and optional starter recipes for common static, process,
reverse-proxy, and split frontend/backend shapes, but it does not generate
Kubernetes manifests, platform adapters, secrets, TLS policy, storage,
backups, incident response, rollout logic, or CDN configuration.

| Shape | Use When | Current Command Surface |
| --- | --- | --- |
| Static output | The app has no generated request-time handlers. | `gowdk build --out <dir>` |
| Single binary | Static output and generated request-time handlers ship together. | `gowdk build --out <dir> --app <dir> --bin <file>` |
| Split frontend/backend | Static frontend and generated backend routes deploy separately. | `gowdk build --out <dir> --app <dir> --bin <file> --backend-app <dir> --backend-bin <file>` |
| Backend-only | A generated backend route app is deployed behind another frontend. | `gowdk build --backend-app <dir> --backend-bin <file>` |
| Go WASM artifact | A host can execute a Go `js/wasm` generated app artifact. | `gowdk build --out <dir> --app <dir> --wasm <file>` |

## Optional Recipes

`gowdk build --deploy-recipe <name>` emits starter deployment files after the
selected build artifacts are written. The flag may be repeated or
comma-separated:

```sh
gowdk build --out dist/site --deploy-recipe static
gowdk build --out dist/site --app .gowdk/app --bin bin/site --deploy-recipe systemd,caddy
gowdk build --out dist/site --backend-app .gowdk/backend --backend-bin bin/backend --deploy-recipe split
```

Supported recipes:

| Recipe | Requires | Output |
| --- | --- | --- |
| `static` | `--out` | `<out>/deploy/static-host.md` |
| `systemd` | `--bin` or `--backend-bin` | `<binary-dir>/gowdk-<binary>.service` |
| `caddy` | `--bin` or `--backend-bin` | `<binary-dir>/Caddyfile` |
| `nginx` | `--bin` or `--backend-bin` | `<binary-dir>/nginx.gowdk.conf` |
| `split` | `--out` and `--backend-bin` | `<out>/deploy/split-frontend-backend.md` |

Recipes are starting points, not production guarantees. Review every
environment-specific setting before using them. Keep domains, TLS, CDN policy,
secrets, storage, backups, health checks beyond `/_gowdk/health`, and rollout
strategy in app-owned infrastructure.

## Build Output Files

Build build output:

```sh
gowdk build --out dist/site
```

Deploy the contents of `dist/site` with any asset host that can serve
directory indexes:

```text
dist/site/
  index.html
  routes...
  assets...
  gowdk-routes.json
  gowdk-assets.json
  openapi.json
  asyncapi.json
  gowdk-build-report.json
```

Local smoke test:

```sh
gowdk serve --dir dist/site --addr 127.0.0.1:8080
```

`gowdk serve` serves generated build output from disk. It does not run generated
request-time features.

## Single Binary

Build build output, generated app source, and a local binary:

```sh
gowdk build --out dist/site --app .gowdk/app --bin bin/site
```

Run the binary:

```sh
./bin/site
```

Smoke-test a known generated route from the repository root:

```sh
GOWDK_SMOKE_ADDR=127.0.0.1:18085 scripts/smoke-generated-binary.sh bin/site /
```

The generated app embeds the selected build output and serves it through
`runtime/app`. It also exposes:

- `/_gowdk/health`
- `X-GOWDK-*` identity response headers

Generated apps may attach `runtime/app.Metrics` to the runtime handler. When
present, `/_gowdk/health` includes a snapshot of request, static, backend,
action, API, SSR, not-found, method-not-allowed, and CSRF-unavailable counters.

Runtime identity environment variables:

- `GOWDK_APP_ID`: application identity metadata.
- `GOWDK_MODULE_NAME`: module identity metadata.
- `GOWDK_INSTANCE_ID`: stable runtime instance ID. If omitted, one is generated
  at process start.

The selected module set is fixed at build time. `GOWDK_MODULE_NAME` does not
change which files were embedded.

Single-binary deploy is the primary GOWDK differentiator. Prefer this path when
the app needs generated actions, APIs, partial fragments, guards, CSRF, SSR, or
embedded assets in one artifact.

## Split Frontend And Backend

Use split frontend/backend output when static pages and backend routes have
different scaling, network, or deployment requirements. A split build creates:

- frontend build output and, when requested, a frontend binary that serves that
  output;
- a backend-only generated app and binary for generated action, API, fragment,
  SSR, and contract routes;
- frontend proxy metadata for generated backend routes.

The frontend process forwards generated backend routes to
`GOWDK_BACKEND_ORIGIN`. Set that variable to the internal backend origin, such
as `http://127.0.0.1:8081` on one host or a private service URL in a platform
network. Keep CSRF secrets and backend-only service credentials on the backend
process. Keep TLS, public host routing, compression, and request-ID generation
at the edge or reverse proxy.

Deploy frontend and backend artifacts together when route manifests, endpoint
metadata, CSRF policy, or generated asset paths changed. Roll them back
together for the same reason.

## Backend-Only

`gowdk build --backend-app <dir> --backend-bin <file>` writes a generated
backend route app without embedding frontend output. Use this for API/action
services behind an app-owned frontend or split deployment. A backend-only app
still exposes `/_gowdk/health`, generated headers, registered middleware,
guards, rate limits, CSRF checks where applicable, and request-time route
dispatch.

Backend-only output does not serve static pages. Pair it with static output, a
frontend binary, or a non-GOWDK frontend only when route ownership is explicit
and the frontend knows where generated endpoints live.

## Process Lifecycle And Logs

The generated `cmd/server` entrypoint is intentionally small: it constructs the
generated application with `gowdkapp.App()`, reads `GOWDK_ADDR`, installs the
documented `http.Server` timeout and header limits, logs startup, and calls
`runtime/app.Run`. The runtime supervisor mounts configured lifecycle services,
starts the HTTP server, cancels on service/server error or SIGINT/SIGTERM, and
uses a 10 second graceful shutdown timeout.

Apps that need a different drain policy can use the generated package from
app-owned startup code:

```go
application, err := gowdkapp.App()
if err != nil {
	return err
}
server := &http.Server{Addr: ":8080", Handler: application.Handler}
return gowdkruntime.Run(ctx, server, application, gowdkruntime.RunOptions{ShutdownTimeout: 30 * time.Second})
```

Request logging, structured logs, route logging, OpenTelemetry instrumentation,
compression, optional ETags, and protocol-specific background services are
app-owned middleware, lifecycle services, or reverse-proxy concerns. GOWDK keeps
generated panic responses generic and redacts secret-like text from generated
panic logs.

## Docker

`gowdk build --docker` emits a `Dockerfile` and `.dockerignore` beside the
compiled `--bin` artifact:

```sh
GOOS=linux CGO_ENABLED=0 gowdk build --out dist/site --app .gowdk/app --bin bin/site --docker
cd bin
docker build -t my-gowdk-site .
docker run --rm -p 8080:8080 my-gowdk-site
```

The default Dockerfile uses a distroless base:

```dockerfile
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY ["site", "/app/site"]
ENV GOWDK_ADDR=0.0.0.0:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/site"]
```

Use `--docker-base scratch` only with a statically linked Linux binary:

```sh
GOOS=linux CGO_ENABLED=0 gowdk build --out dist/site --app .gowdk/app --bin bin/site --docker --docker-base scratch
```

`--docker` requires `--bin`; it packages the generated app binary and does not
build or push an image. `--docker-base scratch` rejects dynamically linked ELF
binaries, and all Dockerfile generation rejects non-ELF binaries with guidance
to build with `GOOS=linux`.

Pass app secrets, CSRF secrets, database URLs, and service credentials as
runtime environment variables owned by your deployment platform. For local
generated-binary runs, `GOWDK_ENV_FILE=/path/to/.env` can point the binary at a
dotenv file; host environment values still take precedence over file values.

## CSRF Secret Rotation

Generated CSRF currently validates tokens with one active signing secret from
`Build.CSRF.SecretEnv` or `GOWDK_CSRF_SECRET`. CSRF is enabled by default for
generated action and web-command POSTs; generated apps fail closed at startup if
those endpoints are present and the secret is absent. There is no multi-key
grace period yet.

Rotate CSRF secrets as a coordinated deploy:

1. Build and smoke-test the new binary.
2. Set the new secret in the deployment platform.
3. Restart or replace every generated app instance that serves action POSTs.
4. Confirm `/_gowdk/health` is reachable on every instance.
5. Expect forms rendered before the rotation to fail with HTTP 403
   `invalid csrf token`; users should reload the page and resubmit.

Do not run mixed old/new CSRF secrets behind the same load balancer for longer
than the deploy window. If a rollback is needed, restore both the previous
binary and the previous CSRF secret.

## systemd

Generate a starter unit beside a compiled frontend or backend binary:

```sh
gowdk build --out dist/site --app .gowdk/app --bin bin/site --deploy-recipe systemd
```

The generated `<binary-dir>/gowdk-<binary>.service` is a starting point for a
Linux VM. A typical unit shape is:

```ini
[Unit]
Description=GOWDK site
After=network.target

[Service]
WorkingDirectory=/opt/gowdk-site
ExecStart=/opt/gowdk-site/bin/site
Environment=GOWDK_ADDR=127.0.0.1:8080
Environment=GOWDK_APP_ID=site
Restart=on-failure
RestartSec=2s
User=gowdk
Group=gowdk

[Install]
WantedBy=multi-user.target
```

Keep secrets in systemd drop-ins, an environment file with correct filesystem
permissions, or the host secret manager. Do not commit them to the repository.

## Reverse Proxies

Generated binaries speak plain HTTP. Put TLS, HTTP/2, compression, and public
host routing in a normal reverse proxy.

Generate starter proxy snippets beside a compiled frontend or backend binary:

```sh
gowdk build --out dist/site --app .gowdk/app --bin bin/site --deploy-recipe caddy --deploy-recipe nginx
```

Caddy:

```caddyfile
example.com {
	reverse_proxy 127.0.0.1:8080
}
```

nginx:

```nginx
server {
    listen 80;
    server_name example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Configure trusted proxy behavior in app-owned middleware when handlers depend
on forwarded IP, host, or scheme values.

## Security Headers

Generated binaries can emit configured response headers through
`Build.SecurityHeaders`. Edge or app-owned middleware can still add deployment
headers that belong at the proxy/TLS boundary. Before making request-time routes
public, configure the generated app, reverse proxy, or app middleware for:

- `X-Content-Type-Options: nosniff`.
- `Referrer-Policy: strict-origin-when-cross-origin` for most sites, or
  `no-referrer` for apps with sensitive URLs.
- `Content-Security-Policy` scoped to the app's real asset needs. Start from
  `default-src 'self'`; add only the script, style, image, font, connect, WASM,
  and API origins the generated app and user code actually use.
- Frame policy through `Content-Security-Policy: frame-ancestors 'none'` or
  `'self'`. Use `X-Frame-Options: DENY` or `SAMEORIGIN` only as a compatibility
  header for older clients.
- `Strict-Transport-Security` only at the HTTPS edge, after TLS, redirects, and
  rollback behavior are verified. Do not send HSTS from local HTTP dev servers.

Preserve generated `Cache-Control: no-store` responses for actions, APIs,
fragments, SSR failures, generated errors, and CSRF-mutated HTML.

## Cookie Policy

Generated CSRF cookies use `HttpOnly`, `Secure`, and `SameSite=Lax` by default.
`Build.CSRF.Insecure` is only for local HTTP development and disables the
`Secure` flag because browsers reject secure-prefixed cookies over plain HTTP.

Application-owned cookies remain application-owned. When handlers use
`runtime/response.WithCookie`, `http.SetCookie`, or addon helpers, set:

- `HttpOnly` for cookies that JavaScript should not read.
- `Secure` for every non-local deployment.
- `SameSite=Lax` or `SameSite=Strict` unless a cross-site flow explicitly needs
  `SameSite=None; Secure`.
- `Path` to the narrowest route prefix that needs the cookie.
- `Domain` only when sharing across subdomains is intentional; omit it for
  host-only cookies.
- Short, explicit `MaxAge`/`Expires` values for session and action-flow cookies.

Do not store bearer tokens, API keys, private keys, passwords, CSRF secrets, or
database credentials in client cookies.

## Operations Security

Terminate TLS at the reverse proxy, load balancer, or platform edge. Generated
binaries should usually bind to `127.0.0.1:<port>` behind that edge on VMs, or
to `0.0.0.0:<port>` only inside a container or platform network that provides
the public TLS boundary.

Generate or sanitize request IDs at the trusted edge and pass them through a
single header such as `X-Request-ID`. Do not trust arbitrary client-supplied
request IDs for audit or correlation without validation. App-owned middleware
can attach the trusted request ID to logs and handler context.

Use `/_gowdk/health` for process and artifact identity checks. Treat it as an
operational endpoint: keep it internal when possible, or expose it only when
the identity fields and counters are acceptable for public visibility. Do not
put secrets, tenant data, user data, or database connectivity details in health
responses.

Generated runtime metrics are process-local counters exposed through
`runtime/app.Metrics` when the generated handler is configured with a collector.
Snapshots include request counts, active requests, errors, latency, and generated
backend route metrics keyed by route templates and endpoint IDs. Export, scrape,
or aggregate them through app-owned telemetry code. Keep metrics labels
low-cardinality and avoid user identifiers, tokens, submitted values, or full
URLs with sensitive query strings.

## Logging, Readiness, And Shutdown

Generated server entrypoints log startup and fatal listen errors with the Go
standard logger. App-owned middleware and handlers own request logs, structured
logs, sampling, and log sinks. Use `runtime/trace.SlogArgs(ctx)` or
`SlogAttrs(ctx)` to attach active trace/span IDs to app-owned `slog` records.
Do not log secrets, raw submitted form values, bearer tokens, CSRF tokens,
private keys, database URLs, or full query strings that may contain user data.

Use `/_gowdk/health` as a process/readiness check after the binary starts and
after each deployment step. If the app depends on a database, queue, cache, or
third-party service, expose those checks in app-owned endpoints or middleware;
GOWDK health responses must not include secret or tenant-specific details.

Generated entrypoints use the standard HTTP server shape. Platform shutdown,
drain time, signal handling, and connection draining remain deployment-owned:
configure systemd, containers, load balancers, or app-owned wrappers to stop
sending traffic before replacing the process.

## Artifact Layout

Keep deploy artifacts immutable and grouped by build:

```text
release-YYYY-MM-DD/
  bin/site
  dist/site/
  .gowdk/app/        # optional generated source for debugging/rebuilds
  checksums.txt
  gowdk-build-report.json
  gowdk-routes.json
  gowdk-assets.json
  gowdk-security.json
```

Do not serve `.gowdk/` or non-public reports directly from a static host. For a
single binary, the embedded output is already inside `bin/site`; keep the source
reports next to the artifact for audit and rollback, not as public web files.

## Cache Defaults

Generated binaries use explicit cache headers:

- Embedded SPA HTML uses `Cache-Control: no-cache` by default, so browsers may
  store it but must revalidate before reuse. A page-level `cache` overrides
  this default for successful static SPA HTML generated by that page.
- Generated CSS and generated browser runtime assets recorded in
  `gowdk-assets.json` use their recorded cache policy. The current generated
  policy is `Cache-Control: public, max-age=31536000, immutable` with SHA-256
  content hashes in the asset manifest. Generated CSS is minified and emitted
  with a content-hashed filename; the asset manifest maps the stable logical
  CSS path to the emitted hashed path.
- CSRF-personalized HTML, action responses, API responses, partial fragments,
  SSR HTML without an explicit `cache`, SSR load redirects, generated handler
  errors, generated error pages, and invalid-CSRF responses use
  `Cache-Control: no-store`.
- Page-level `cache` records route response cache intent in compiler, route,
  build-report, manifest, generated asset metadata, and generated SSR route
  metadata.
  Generated binaries apply it to successful static SPA HTML and SSR HTML
  responses for that page. It does not override the no-store safety policy for
  actions, APIs, partial responses, load redirects, generated errors, or
  CSRF-mutated HTML.
- Page-level `revalidate` requires `cache` and appends
  `stale-while-revalidate=<seconds>` to the generated Cache-Control header for
  successful static SPA HTML and SSR HTML responses. Accepted values are whole
  seconds or whole-second durations such as `60s`, `5m`, or `1h`.

## Static Hosts And CDN

For pure build-time output, deploy `gowdk build --out dist/site` to any static
host that supports directory indexes.

Recommended CDN policy:

- Respect generated `Cache-Control` headers when serving through a generated
  binary.
- For static-file hosting, cache content-hashed assets under `assets/` for a
  long time.
- Revalidate HTML unless the page has an explicit `cache` policy.
- Do not cache action/API/fragment/SSR error responses from a generated binary.

Cloudflare Pages, Vercel, and Netlify can serve static `dist/site` output when
the app does not need generated request-time handlers. Use a generated binary,
container, VM, or platform that can run Go when the app needs actions, APIs,
fragments, SSR, guards, CSRF, or server validation.

Cloudflare Workers compatibility is limited to generated static output or the
separate Go `js/wasm` deploy artifact. GOWDK does not currently emit a Workers
adapter.

Kubernetes guidance is intentionally not generated. Use normal container and
service manifests around the Docker/single-binary shape only when your
deployment environment already requires Kubernetes.

## Rollback

Keep each release artifact immutable:

- the generated binary;
- the generated build output used to create that binary;
- the config and environment variable set used at runtime;
- the checksum and attestation for the artifact.

Rollback means restoring the previous known-good artifact and its matching
runtime configuration. For single-binary deploys, keep the previous binary on
the host or in the image registry and switch the process manager, container tag,
or deployment descriptor back to that version. For static hosts, redeploy the
previous `dist/site` output directory. For split frontend/backend deploys,
rollback both sides together when route, endpoint, CSRF, or asset manifests
changed.

After rollback, verify:

```sh
curl -fsS http://127.0.0.1:8080/_gowdk/health
```

Then smoke-test one static page and one generated request-time route if the app
uses actions, APIs, fragments, SSR, guards, or CSRF.

## Backups, Incidents, And Dependencies

GOWDK does not own application data backups, restore testing, incident response,
dependency update policy, or platform patching. Treat generated artifacts as
replaceable build output. Back up app-owned databases, object storage, queues,
event logs, user uploads, secrets, and deployment descriptors according to the
platform that owns them.

Before a release, record the GOWDK version, Go version, module checksums,
enabled addons, build target, artifact checksum, and runtime environment names.
During an incident, use generated route, asset, build, and security reports to
identify which routes and endpoints are present, then debug user-owned handlers
and infrastructure through the app's normal observability stack.

## Module And Target Builds

Use modules for source selection:

```sh
gowdk build --module public --out dist/public --app .gowdk/public --bin bin/public
gowdk build --module admin,api --out dist/admin-api --app .gowdk/admin-api --bin bin/admin-api
```

Use `Build.Targets` for repeatable packaging:

```go
Build: gowdk.BuildConfig{
	Targets: []gowdk.BuildTargetConfig{
		{
			Name: "public",
			Modules: []string{"public"},
			Output: "dist/public",
			App: ".gowdk/public",
			Binary: "bin/public",
		},
		{
			Name: "admin",
			Modules: []string{"admin"},
			Output: "dist/admin",
			App: ".gowdk/admin",
			Binary: "bin/admin",
		},
	},
}
```

Run every target:

```sh
gowdk build
```

Run one target:

```sh
gowdk build --target admin
```

Use distinct `Output` and `App` directories for separate binaries.

Configured build targets can also request deployment recipes:

```go
Build: gowdk.BuildConfig{
	Targets: []gowdk.BuildTargetConfig{
		{
			Name: "site",
			Output: "dist/site",
			App: ".gowdk/site",
			Binary: "bin/site",
			DeployRecipes: []string{"systemd", "caddy"},
		},
	},
}
```

## WASM Deploy Artifact

`--wasm` compiles the generated app with `GOOS=js GOARCH=wasm`:

```sh
gowdk build --out dist/site --app .gowdk/app --wasm bin/site.wasm
```

Smoke-test the emitted module header:

```sh
scripts/smoke-generated-wasm.sh bin/site.wasm
```

This is a Go `js/wasm` deploy artifact for runtimes that can execute that
artifact. It is separate from browser island assets emitted for component-level
`wasm` declarations. GOWDK does not emit a generic host runtime or loader for
this deploy artifact; that integration belongs to the selected deploy platform.

## Addons

Addons are normal Go packages imported by `gowdk.config.go`:

```go
import (
	"github.com/cssbruno/gowdk/addons/actions"
	"github.com/cssbruno/gowdk/addons/partial"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		actions.Addon(),
		partial.Addon(),
	},
}
```

Third-party addons should ship as Go modules. Versioning follows Go module
versions, not hidden CLI discovery. GOWDK should not load production addons
from runtime filesystem scans, network registries, or hidden project metadata.

## Request-Time Feature Limits

Generated binaries currently support:

- Embedded app file serving.
- Feature-bound same-package action handlers with no-input, typed value, typed
  pointer, or `form.Values` signatures.
- Feature-bound same-package API handlers.
- Configurable action and API request body caps through
  `Build.BodyLimits`, defaulting to 1 MiB.
- No-store panic boundaries for generated SSR, action, and API request-time
  lanes.
- First-slice same-page POST action redirects.
- CSRF-wired generated action handlers when the configured secret environment
  variable is present. CSRF is enabled by default unless `Build.CSRF.Disabled`
  is set.
- First-slice required-field validation for directly declared form controls.
- First-slice partial action fragment responses.
- Standalone concrete and dynamic fragment routes with raw and typed route
  params exposed to fragment hooks.
- First-slice concrete and dynamic request-time SSR pages with declared
  `server {}` identifier or dotted paths.
- Optional split frontend/backend generation with `--backend-app` and
  `--backend-bin`; the frontend proxies backend routes to
  `GOWDK_BACKEND_ORIGIN`.

Generated binaries do not yet support:

- Hybrid streaming, data refresh, and non-HTTP revalidation.

## Local Development

`dev` rebuilds generated build output, serves it locally, and live reloads the
browser after successful rebuilds:

```sh
gowdk dev --out dist/site
gowdk dev --target admin
```
