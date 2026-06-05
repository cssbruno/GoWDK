# Getting Started

GOWDK is currently used from source. The fastest path is clone, build the CLI,
scaffold a small app, build static output, and serve the generated directory.

## Prerequisites

- Go installed and available on `PATH`.
- A local checkout of this repository.

## Clone And Build

```sh
git clone https://github.com/cssbruno/GOWDK.git
cd GOWDK
go build ./cmd/gowdk
./gowdk version
```

During repository development, you can also run the CLI without installing it:

```sh
go run ./cmd/gowdk version
```

Use the built binary when running commands from outside this repository.

## Create An App

From the repository root:

```sh
/path/to/GOWDK/gowdk init /tmp/gowdk-my-app
cd /tmp/gowdk-my-app
```

`init` writes a starter `gowdk.config.go`, one page, one component, and one CSS
file. Existing files are not overwritten unless `--force` is passed.

The generated config discovers `src/**/*.gwdk`, discovers CSS from
`styles/**/*.css`, and writes static output to `dist/site`.

## Build

From the app directory:

```sh
/path/to/GOWDK/gowdk build
```

The build writes static HTML and manifests under `dist/site`:

```text
dist/site/
  index.html
  gowdk-routes.json
  gowdk-assets.json
  gowdk-build-report.json
```

Every successful disk build writes `gowdk-build-report.json`.

## Serve

```sh
/path/to/GOWDK/gowdk serve --dir dist/site
```

Open `http://127.0.0.1:8080/`.

`serve` serves generated static files only. It does not run generated actions,
API handlers, partial fragments, or SSR routes.

## Development Loop

Use `dev` when the project has `Build.Output` in `gowdk.config.go`:

```sh
/path/to/GOWDK/gowdk dev
```

`dev` builds, serves the output directory, polls source inputs for content
changes, rebuilds on changes, and injects browser live reload into served HTML.
It keeps serving the last successful build after a failed rebuild.

## Build Repository Examples

From the GOWDK repository root:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build \
  examples/basic/home.page.gwdk \
  examples/basic/hero.cmp.gwdk

go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

Dynamic static routes work when literal `paths {}` entries are present:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build \
  examples/basic/blog-post.page.gwdk
```

This writes `/tmp/gowdk-dynamic-build/blog/hello-gowdk/index.html` and
`/tmp/gowdk-dynamic-build/blog/compile-first/index.html`.

## Current Reality

Implemented today:

- Static output for simple `.gwdk` pages and components.
- Literal `paths {}` expansion for dynamic static routes.
- Literal `build {}` data and imported no-argument Go build data functions.
- Config-based discovery, module selection, and named build targets.
- Generated embedded app source, local binaries, and Go `js/wasm` deploy
  artifacts.
- First-slice action redirects, partial action fragments, and simple concrete
  SSR pages in generated binaries.
- CLI tooling for tokens, formatting, validation, manifest, sitemap, routes,
  watch, dev, serve, and LSP.

Planned or partial:

- Real user Go action execution and CSRF wiring in generated handlers.
- Generated API handlers.
- Request-time `load {}` execution and guard enforcement.
- Dynamic SSR routes in generated binaries.
- Full browser-side Go/WASM island ABI.
