# Getting Started

GOWDK is currently used from source. The fastest path is clone, build the CLI,
scaffold a small app, build build output, and serve the generated directory.

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
`styles/**/*.css`, writes build output to `dist/site`, and ignores the default
dev output directory `gowdk_cache` in the scaffolded `.gitignore`.

## Build

From the app directory:

```sh
/path/to/GOWDK/gowdk build
```

The build writes app-shell HTML and manifests under `dist/site`:

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

`serve` serves generated build output files only. It does not run generated
actions, API handlers, partial fragments, or SSR routes. Use `gowdk build
--app --bin` for a generated binary that runs request-time handlers.

## Development Loop

Use `dev` for polling rebuilds, local serving, and browser reload:

```sh
/path/to/GOWDK/gowdk dev
```

`dev` builds into `gowdk_cache` by default, serves that directory, polls source
inputs for content changes, rebuilds on changes, and injects browser live reload
into served HTML. It keeps serving the last successful build after a failed
rebuild. Pass `--out <dir>` to use a different dev output directory.

When you pass `--app <dir>`, `dev` builds the generated app, compiles a local
dev binary, runs it on `GOWDK_ADDR`, and restarts that process after successful
rebuilds. Use this path for local backend, action, API, partial, and SSR flows.

Use `preview` for a one-shot local deploy preview:

```sh
/path/to/GOWDK/gowdk preview
```

Add `--hot` to run the same preview output through the dev rebuild loop.

## Build Repository Examples

From the GOWDK repository root:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk

go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

The repository root includes `gowdk.config.go` so these example commands have
the same required project config shape as a scaffolded app. Outside this
repository, run `gowdk init` first or pass `--config <file>`.

Dynamic SPA routes work when literal `paths {}` entries are present:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build \
  examples/pages/blog-post.page.gwdk
```

This writes `/tmp/gowdk-dynamic-build/blog/hello-gowdk/index.html` and
`/tmp/gowdk-dynamic-build/blog/compile-first/index.html`.

## Current Reality

Implemented today:

- Build output for simple `.gwdk` pages and components.
- Literal `paths {}` expansion for dynamic SPA routes.
- Literal `build {}` data and imported no-argument Go build data functions.
- Config-based discovery, module selection, and named build targets.
- Generated embedded app source, local binaries, and Go `js/wasm` deploy
  artifacts.
- Explicit browser-side Go/WASM island packages with ABI export validation.
- Feature-bound action/API handlers, action redirects, partial action
  fragments, standalone fragment routes, CSRF-wired actions, guards, and
  concrete or dynamic SSR pages with declared `load {}` identifier or dotted
  paths in generated binaries.
- CLI tooling for tokens, formatting, validation, manifest, sitemap, routes,
  dev, serve, and LSP.

Planned or partial:

- Broader generated validation coverage.
- Scoped component CSS/asset emission from existing metadata.
- Broader hybrid request-time behavior beyond explicit `load {}` branches.
- Richer generated-client reactivity and load/action invalidation.
