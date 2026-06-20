# gowdk-page

Documentation site for GOWDK, written entirely in GOWDK source files. The pages
are `.gwdk`, styling is `app.css` (Tailwind v4), and client behavior lives in
GOWDK `js {}` blocks. No HTML, CSS, or JavaScript is generated from Go code.

It lives inside the GOWDK monorepo at `docs-site/` and is its own Go module. A
`replace github.com/cssbruno/gowdk => ../` directive in `go.mod` builds the site
against the **in-tree framework HEAD** rather than a published release, so the
site always reflects the sources it documents.

The documentation pages under `src/pages/docs/` and the sidebar
(`src/components/docs-sidebar.cmp.gwdk`) are **generated from the structured
markdown in the repo's `docs/` tree** (`../docs`) by `cmd/syncdocs`. Edit the
docs at the repo root, then regenerate (see "Sync docs" below) — do not hand-edit
the generated `docs/*.page.gwdk` files. The docs chrome (3-column layout,
sidebar, "on this page" TOC, breadcrumbs, prev/next, ⌘K search, copy buttons,
callouts) lives in the reusable `DocsPage`/`DocsSidebar`/`Callout` components, so
every generated page is modular and consistent.

## Prerequisites

- Go 1.26.4+.
- The framework is resolved from the parent directory via the `replace` in
  `go.mod`, so `go run github.com/cssbruno/gowdk/cmd/gowdk ...` runs the in-tree
  CLI. (The `require ... v0.7.0` line is the minimum version selector; the
  `replace` overrides it with the working-tree sources.)
- The Tailwind CSS v4 standalone CLI at `tools/tailwindcss`. The GOWDK tailwind
  addon (see `gowdk.config.go`) runs it during the build; it is not downloaded
  automatically. The Render deploy pins `tailwindcss-linux-x64` to v4.3.1 and
  verifies its SHA-256 before executing it. Pick the binary for your platform
  (on Apple Silicon use `tailwindcss-macos-arm64`) and verify it against the
  release `sha256sums.txt`:

  ```sh
  TAILWIND_VERSION=v4.3.1
  mkdir -p tools
  curl -fsSL -o tools/tailwindcss \
    "https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-linux-x64"
  echo "2526d063ba03b71f9a3ea7d5cee14f0aec147f117f222d5adc97b1d736d45999  tools/tailwindcss" | sha256sum -c -
  chmod +x tools/tailwindcss
  ```

## Sync docs

Regenerate the documentation pages and sidebar from the repo's `docs/` tree:

```sh
go run ./cmd/syncdocs
```

`GOWDK_SOURCE_ROOT` defaults to `..` (the monorepo root), so the generator reads
`../docs`. It walks the configured sections (Start, Language, Reference,
Compiler, Engineering, Decisions, Product — see the `sections` list in
`cmd/syncdocs/main.go`), renders each markdown file to a `DocsPage` page with
heading IDs, escaped braces, and rewritten `.md` cross-links, and writes the
grouped sidebar. Run it before the GOWDK build whenever the repo docs change.

Set `GOWDK_SOURCE_ROOT` only to render a docs tree from somewhere other than the
parent repo.

## Develop

```sh
go run github.com/cssbruno/gowdk/cmd/gowdk dev --addr 127.0.0.1:8091
```

Watches the `.gwdk` sources and `app.css` and rebuilds on change. Open
`http://127.0.0.1:8091/`.

## Build Site Output

```sh
go run ./cmd/syncdocs
rm -rf dist/site
go run github.com/cssbruno/gowdk/cmd/gowdk build
mkdir -p dist/site/assets
cp -R assets/. dist/site/assets/
cp assets/favicon.ico dist/site/favicon.ico
```

Always run `cmd/syncdocs` before the GOWDK build so the published docs match the
selected GOWDK source. `rm -rf dist/site` is required because the generated tree
mirrors the repo structure and stale routes must not linger.

`go run github.com/cssbruno/gowdk/cmd/gowdk build` compiles the `.gwdk` sources
to static HTML, emits each page's `<head>` from its `title`, `description`, and
`canonical` metadata plus
`BuildConfig.Head`, and runs the tailwind addon, which builds `app.css`
(Tailwind v4) into a hashed `assets/app.*.css` and links it into every page. The
only non-GOWDK step is copying static `assets/` (logo, favicon) into the output.

## Build Binary

```sh
mkdir -p bin
go build -o bin/gowdk-page .
```

The binary serves the embedded `dist/site` output.

## Run

```sh
GOWDK_ADDR=127.0.0.1:8091 ./bin/gowdk-page
```

The site binary serves the embedded site. If `GOWDK_ADDR` is omitted, it
defaults to `127.0.0.1:8080`.

## Preview & Deploy

The site advertises its own pre-release status: every page carries an
**Experimental 0.x — not production-ready** banner from `src/layouts/root.layout.gwdk`,
and the home page shows CI, release, license, and stability badges that link to
the authoritative GitHub sources. Keep both honest — GOWDK is `0.x` and the
language and generated output can change between releases.

To preview website changes locally before opening a PR:

```sh
go run github.com/cssbruno/gowdk/cmd/gowdk dev --addr 127.0.0.1:8091
# or a production-faithful preview that serves the exact built output through
# the site's own Go binary (the same one that ships to production):
go run ./cmd/syncdocs
rm -rf dist/site && go run github.com/cssbruno/gowdk/cmd/gowdk build
GOWDK_ADDR=127.0.0.1:8091 go run .
```

Deployment is a Go web service (see `render.yaml` at the repo root — Render only
reads a Blueprint from the repository root, not from a subdirectory). The service
is rooted at the **repo root**, not `docs-site/`: docs-site's `go.mod` replaces
the framework with `../` and `syncdocs` reads `../docs`, both outside
`docs-site/`, and Render makes files outside a service's root directory
unavailable at build time. So the build `cd`s into `docs-site/`, runs syncdocs
and the in-tree GOWDK build, copies `assets/`, then `go build`s `main.go` into
the `app` binary that embeds and serves `dist/site` (reading `$PORT`); the start
command is `cd docs-site && ./app`. The Blueprint build filter watches both `docs-site/**` and
`docs/**`, so source documentation changes deploy the generated site even though
`src/pages/docs/**` is gitignored. A static preview of any branch is just the
build output above served by any static file server, so contributors can review
a branch without a live runtime. None of this makes the site a product promise —
it is documentation for an experimental project.

## Structure

- `gowdk.config.go`: project config (Tailwind addon, `Build.Head` metadata).
- `app.css`: Tailwind v4 input and the site's visual system.
- `cmd/syncdocs/`: generator that builds the docs pages and sidebar from the
  main repo's `docs/` markdown (uses `goldmark`). See "Sync docs".
- `src/pages/index.page.gwdk`: the documentation home served at `/`.
- `src/pages/docs/**.page.gwdk`: the documentation pages — **generated**; do not
  hand-edit.
- `src/layouts/`: shared site chrome (`root`) and a passthrough docs layout.
- `src/components/docs-page.cmp.gwdk`: the `DocsPage` shell — 3-column layout plus
  the `js {}` block that builds the active nav, breadcrumb, "on this page" TOC
  (with scroll-spy), prev/next pager, copy-code buttons, and ⌘K command palette.
- `src/components/docs-sidebar.cmp.gwdk`: the `DocsSidebar` nav — **generated**.
- `src/components/callout.cmp.gwdk`: the `Callout` component (note/tip/warning/
  important) for "good to know" boxes.
- `src/components/cookie-notice.cmp.gwdk`: client-side cookie notice; the `js {}`
  block remembers dismissal in a cookie. Works in dev and the production build.
- `tools/tailwindcss`: standalone Tailwind CLI used by the build.
- `assets/`: static assets copied into `dist/site/assets`.
- `main.go` / `bin/gowdk-page`: single-binary site server. `main.go` serves the
  embedded files only; it constructs no markup.
- `gowdk`: local CLI build for the VS Code extension (`.vscode/settings.json`
  points `gowdk.cliPath` at it). Git-ignored.
