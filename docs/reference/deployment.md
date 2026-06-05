# Deployment

GOWDK currently supports three practical output shapes:

- Static files from `gowdk build --out`.
- A generated Go app from `gowdk build --out --app`.
- A local-platform binary or Go `js/wasm` artifact from the generated app.

Deployment orchestration is user-owned. GOWDK does not generate containers,
Kubernetes manifests, platform adapters, or CDN configuration.

## Static Files

Build static output:

```sh
gowdk build --out dist/site
```

Deploy the contents of `dist/site` with any static host that can serve
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

`gowdk serve` is a static file server. It does not run generated request-time
features.

## Single Binary

Build static output, generated app source, and a local binary:

```sh
gowdk build --out dist/site --app .gowdk/app --bin bin/site
```

Run the binary:

```sh
./bin/site
```

The generated app embeds the selected static output and serves it through
`runtime/app`. It also exposes:

- `/_gowdk/health`
- `X-GOWDK-*` identity response headers

Runtime identity environment variables:

- `GOWDK_APP_ID`: application identity metadata.
- `GOWDK_MODULE_NAME`: module identity metadata.
- `GOWDK_INSTANCE_ID`: stable runtime instance ID. If omitted, one is generated
  at process start.

The selected module set is fixed at build time. `GOWDK_MODULE_NAME` does not
change which files were embedded.

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

- Embedded static file serving.
- First-slice same-page POST action redirects.
- First-slice required-field validation for directly declared form controls.
- First-slice partial action fragment responses.
- Simple concrete `@render ssr` pages without `load {}`.

Generated binaries do not yet support:

- Real user Go action execution.
- CSRF-wired generated handlers.
- Generated API handlers.
- Request-time `load {}` execution.
- Guard enforcement.
- Dynamic SSR routes.
- General fragment routes.
- Hybrid request-time behavior.

## Local Redeploy

`watch --restart` rebuilds and restarts one generated binary:

```sh
gowdk watch --restart --out dist/site --app .gowdk/app --bin bin/site
```

With configured targets:

```sh
gowdk watch --restart --target admin
```

Failed rebuilds leave the current process running.
