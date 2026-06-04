# GOWDK

GOWDK is a portable Go web compiler.

WDK does not have a canonical expansion; no one knows what it stands for. The
project promise is simple: GOWDK ships apps.

The product direction is compile-first: movable `.gwdk` files should compile into static pages, components, typed actions, optional partial updates, and optional one-binary deploys. SSR is an addon, not the identity of the framework.

## Implemented Today

The repository currently contains the initial compile-first architecture scaffold and language tooling:

- Core config, render mode, and addon types.
- Optional addon packages for static, actions, partial fragments, SSR, API, and embed.
- Runtime package boundaries for components, rendering, HTML helpers, forms, validation, responses, and assets.
- Recursive `.gwdk` discovery for include/exclude patterns.
- Minimal page metadata parser for annotations and top-level blocks.
- Parser validation for unknown or malformed annotations and unsupported
  top-level block declarations.
- Dynamic static routes validate with `paths {}` and can prerender the current
  literal string path subset.
- Literal `build {}` data for static `view {}` interpolation.
- Minimal static `view {}` markup parser for lowercase HTML elements, escaped
  text/attribute interpolation, expression attributes, shorthand class/id
  normalization, and explicit component calls.
- Minimal `.cmp.gwdk` component parsing for capitalized components with string
  props and default `<slot />` children.
- Language tooling for token inspection, formatting, checking, and manifest output.
- Dependency-free Language Server Protocol entrypoint for editor diagnostics, formatting, and completions.
- Manifest JSON output that includes page route, render mode, layouts, paths presence, and guards.
- Route-binding planning for static pages, actions, SSR pages, and APIs.
- First action body subset for `input := form Type`, `valid(input)?`, and local
  redirect declarations.
- First API route metadata subset for named method/route declarations.
- Runtime form helpers and generated first-slice action input decoders that
  preserve repeated values and reject unexpected fields.
- Signed double-submit CSRF validator in `addons/actions` with secure cookie
  defaults.
- Generated first-slice required-field validation for direct static form
  controls when actions declare `valid(input)?`.
- Direct generated action file inputs are rejected until upload security rules
  are defined.
- Generated action handlers cap request bodies before form parsing.
- Initial `gowdk build [--config <file>] [--out <dir>] [files...]` support for simple static/action pages, explicit or discovered component files, static `gowdk.config.go` source/output settings, a generated static route manifest, and a generated asset manifest.
- `gowdk init [dir]` project scaffolding for a buildable starter app.
- Generated embedded static app output through `gowdk build --app <dir>` and
  optional one-binary compilation through `--bin <file>`, including static POST
  redirect handlers for the first supported action subset.
- Local `gowdk serve --dir <dir>` support for trying generated static output
  during development.
- `gowdk watch` polling rebuild support for generated static output during
  development.
- Named config modules for organizing source groups such as `frontend`,
  `frontend2`, `backend`, and service modules during build discovery, with
  `gowdk build --module <name>` for selecting modules on demand.
- Initial CSS extension point for configured stylesheet links, compile-time CSS
  processors, discovered page CSS inputs, extracted static classes for
  processors, `@css` page selection, generated page CSS files, and an
  experimental no-npm Tailwind v4 standalone CLI wrapper.
- Rate limiting addon contracts for HTTP middleware, fixed-window decisions, an
  in-memory store, and Redis-backed storage through a dependency-free client
  adapter.
- SSR addon contracts for request-aware load context, ordered guard execution,
  route registration, layout stacks, and default error handling.
- Manifest validation for the core render and route rules, including malformed
  routes, duplicate route params, duplicate page route patterns, and
  route-method conflicts, and missing page `view {}` blocks.

The current build command emits static HTML for simple build-time pages only. It loads the literal subset of `gowdk.config.go` for `Source.Include`, `Source.Exclude`, `Modules`, `Build.Output`, `Build.Stylesheets`, and `CSS` when present. When no files are passed, it discovers configured root/module sources or `**/*.gwdk` from the current directory; `--module <name>` limits discovery to selected configured modules for user-owned deployment workflows. Discovery excludes `.git`, `vendor`, `node_modules`, configured excludes, and the selected output directory. It supports self-closing and wrapper component calls with static string props and default `<slot />` children when component files are passed explicitly or discovered. It can expand dynamic routes from literal `paths {}` declarations such as `=> { slug: "hello-gowdk" }`, bind those route params into literal `build {}` string values, render route params and literal `build {}` data in the current static `view {}` interpolation subset, inject stylesheet links, invoke compile-time CSS processors that emit CSS assets, discover CSS files by exported filename, emit generated page CSS for implicit `default page` or explicit `@css` selections, and record emitted CSS assets in `gowdk-assets.json`. It parses the first action body subset and first API method/route metadata subset; generated apps can handle POST redirects for concrete static/action page routes. Generated action handlers now create named first-slice input wrappers, cap request bodies before parsing, preserve repeated submitted values, reject unexpected fields inferred from direct static controls in same-page `g:post` forms, return HTTP 422 for missing required fields when actions declare `valid(input)?`, and reject direct file upload controls until upload security rules exist. `gowdk build --app` can generate a dependency-free Go app that embeds those static files, exposes `/_gowdk/health`, and identifies app/module instances through `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`, and `GOWDK_INSTANCE_ID`; when no instance ID is provided, the app generates one at process start. `--bin` can compile that app into one local binary. `gowdk serve` can serve this generated output locally for development. GOWDK does not yet support named/scoped slots, non-string props, arbitrary Go expressions, arbitrary `build {}` execution, real user Go type resolution for typed form decoders, wiring CSRF into generated action handlers, generated user action logic, generated API/fragment handlers, partial fragment execution, SSR output, request-time routes in the generated binary, page-aware CSS processors as selectable `@css` inputs, or built-in Kubernetes manifest generation.

`addons/ratelimit` can wrap request-time handlers with fixed-window limits
today, using either process-local memory or Redis-backed distributed counters
through a dependency-free client adapter. Generated handler wiring is planned.

## Target Architecture

Core GOWDK renders at build time by default. The SSR addon renders full pages at request time only where enabled.

Planned compiler output includes static pages, components, typed actions, API handlers, server fragments, embedded assets, and one Go binary.

## Render Modes

- `static`: build-time HTML.
- `action`: static page with backend form/actions/API behavior.
- `hybrid`: static by default with selected request-time behavior.
- `ssr`: request-time full-page rendering through the SSR addon.

Default render mode is `static`.

## Prerequisites

- Go 1.26 or newer.
- Node.js only when checking and testing the VS Code extension.
- VS Code 1.85 or newer for the bundled extension.

## Commands

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --test editors/vscode/*.test.js

go run ./cmd/gowdk version
go run ./cmd/gowdk init [--force] [dir]
go run ./cmd/gowdk tokens <file.gwdk>
go run ./cmd/gowdk fmt [--write] <file.gwdk>
go run ./cmd/gowdk check [--config <file>] [--module <name>] [--json] [--ssr] [files...]
go run ./cmd/gowdk manifest [--config <file>] [--module <name>] [--ssr] [files...]
go run ./cmd/gowdk sitemap [--config <file>] [--module <name>] [--ssr] [files...]
go run ./cmd/gowdk routes [--config <file>] [--module <name>] [--ssr] [files...]
go run ./cmd/gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]
go run ./cmd/gowdk watch [--once] [--interval 1s] [build flags...]
go run ./cmd/gowdk serve --dir <dir> [--addr 127.0.0.1:8080]
go run ./cmd/gowdk lsp [--ssr]
```

Runnable examples and their current limitations are documented in `examples/README.md`.

## Editor Tools

The VS Code extension lives in `editors/vscode/`.

It provides syntax highlighting, bracket/comment configuration, diagnostics through `gowdk check --json`, document formatting through `gowdk fmt`, completions, token preview, manifest preview, a dedicated GOWDK Activity Bar page hierarchy, and a larger site-map visualizer for movable `.gwdk` files.

Editors with Language Server Protocol support can launch `gowdk lsp` over stdio for live diagnostics, document formatting, and completions. Pass `--ssr` to validate buffers as if the SSR addon is enabled.

The page hierarchy is built from declared routes returned by `gowdk sitemap`, not from source folders. It shows page IDs, routes, render modes, layouts, blocks, and source files, and it can open a page file or move it to a new folder while preserving the route declared inside the file.

During source development, the extension automatically runs `go run ./cmd/gowdk ...` when opened at the repository root. In normal use, configure `gowdk.cliPath` or keep `gowdk` available on `PATH`.

## LLM Files

- `AGENTS.md`: Codex entrypoint and repository instruction file.
- `.llm/workflows/`: reusable LLM workflows for features, bug fixes, reviews, and refactors.
- `.llm/templates/`: reusable LLM output templates for specs, plans, ADRs, tests, and PRs.

## Project Documents

- `LICENSE`: Apache-2.0 license text for GOWDK core source.
- `LICENSE.md`: mixed-license map and generated output ownership policy.
- `CONTRIBUTING.md`: contribution workflow and verification expectations.
- `SECURITY.md`: vulnerability reporting and security scope.
- `docs/product/vision.md`: product intent and target users.
- `docs/product/requirements.md`: functional and non-functional requirements.
- `docs/product/roadmap.md`: MVP and phase roadmap.
- `docs/language/`: current `.gwdk` language contract and planned grammar areas.
- `docs/compiler/`: current compiler pipeline and generated-output contracts.
- `docs/reference/`: CLI, manifest, config, addon, CSS, and diagnostics references.
- `docs/engineering/architecture.md`: system design and boundaries.
- `docs/engineering/conventions.md`: coding and repository conventions.
- `docs/engineering/testing.md`: test strategy.
- `docs/engineering/security.md`: baseline security posture.
- `docs/engineering/operations.md`: runtime, deployment, observability, and maintenance notes.
- `docs/engineering/ci.md`: hosted CI gates and current local gates.
- `docs/engineering/release.md`: release workflow and readiness notes.
- `docs/engineering/generated-code-policy.md`: generated output safety and ownership policy.
- `docs/engineering/decisions/`: architecture decision records.

## Verification

The current scaffold is verified with:

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --test editors/vscode/*.test.js
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
test -f /tmp/gowdk-build/gowdk-routes.json
test -f /tmp/gowdk-build/gowdk-assets.json
go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk
go run ./cmd/gowdk build --config examples/tailwind/gowdk.config.go --out /tmp/gowdk-tailwind-build examples/tailwind/site.page.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk
```
