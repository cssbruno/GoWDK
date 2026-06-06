# Deployment

GOWDK currently supports three practical output shapes:

- Build output files from `gowdk build --out`.
- A generated Go app from `gowdk build --out --app`.
- A local-platform binary or Go `js/wasm` artifact from the generated app.

Deployment orchestration is user-owned. GOWDK does not generate containers,
Kubernetes manifests, platform adapters, or CDN configuration.

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

## Cache Defaults

Generated binaries use explicit cache headers:

- Embedded SPA HTML uses `Cache-Control: no-cache`, so browsers may store it
  but must revalidate before reuse.
- Generated CSS and generated browser runtime assets recorded in
  `gowdk-assets.json` use their recorded cache policy. The current generated
  policy is `Cache-Control: public, max-age=31536000, immutable` with SHA-256
  content hashes in the asset manifest. Generated CSS is minified and emitted
  with a content-hashed filename; the asset manifest maps the stable logical
  CSS path to the emitted hashed path.
- CSRF-personalized HTML, action responses, API responses, partial fragments,
  SSR HTML without an explicit `@cache`, SSR load redirects, generated handler
  errors, generated error pages, and invalid-CSRF responses use
  `Cache-Control: no-store`.
- Page-level `@cache` records route response cache intent in compiler, route,
  manifest, and generated SSR route metadata. Generated SSR binaries apply it
  to HTML responses for that page. It does not override the no-store safety
  policy for actions, APIs, partial responses, load redirects, generated
  errors, or CSRF-mutated HTML.

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

## WASM Deploy Artifact

`--wasm` compiles the generated app with `GOOS=js GOARCH=wasm`:

```sh
gowdk build --out dist/site --app .gowdk/app --wasm bin/site.wasm
```

This is a Go `js/wasm` deploy artifact for runtimes that can execute that
artifact. It is separate from browser island assets emitted by
`g:island="wasm"`.

## Request-Time Feature Limits

Generated binaries currently support:

- Embedded app file serving.
- Feature-bound same-package action handlers with no-input, typed value, typed
  pointer, or `form.Values` signatures.
- Feature-bound same-package API handlers.
- No-store panic boundaries for generated SSR, action, and API request-time
  lanes.
- First-slice same-page POST action redirects.
- CSRF-wired generated action handlers when `Build.CSRF.Enabled` is set and
  the configured secret environment variable is present.
- First-slice required-field validation for directly declared form controls.
- First-slice partial action fragment responses.
- First-slice concrete and dynamic `@render ssr` pages with declared `load {}`
  identifier or dotted paths.
- Bare `@render hybrid` pages as normal build-time SPA output.
- Optional split frontend/backend generation with `--backend-app` and
  `--backend-bin`; the frontend proxies backend routes to
  `GOWDK_BACKEND_ORIGIN`.

Generated binaries do not yet support:

- General fragment routes.
- Hybrid request-time behavior.

## Local Development

`dev` rebuilds generated build output, serves it locally, and live reloads the
browser after successful rebuilds:

```sh
gowdk dev --out dist/site
gowdk dev --target admin
```
