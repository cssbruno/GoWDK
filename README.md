# GOWDK

GOWDK is a portable Go web compiler.

The product direction is compile-first: movable `.gwdk` files should compile into static pages, components, typed actions, optional partial updates, and optional one-binary deploys. SSR is an addon, not the identity of the framework.

## Implemented Today

The repository currently contains the initial compile-first architecture scaffold and language tooling:

- Core config, render mode, and addon types.
- Optional addon packages for static, actions, partial fragments, SSR, API, and embed.
- Runtime package boundaries for components, rendering, HTML helpers, forms, validation, responses, and assets.
- Recursive `.gwdk` discovery for include/exclude patterns.
- Minimal page metadata parser for annotations and top-level blocks.
- Dynamic static routes validate with `paths {}` and can prerender the current
  literal string path subset.
- Literal `build {}` data for static `view {}` interpolation.
- Minimal static `view {}` markup parser for lowercase HTML elements, escaped
  text/attribute interpolation, and explicit component calls.
- Minimal `.cmp.gwdk` component parsing for capitalized components with string props.
- Language tooling for token inspection, formatting, checking, and manifest output.
- Dependency-free Language Server Protocol entrypoint for editor diagnostics, formatting, and completions.
- Manifest JSON output that includes page route, render mode, layouts, paths presence, and guards.
- Route-binding planning for static pages, actions, SSR pages, and APIs.
- First action body subset for `input := form Type`, `valid(input)?`, and local
  redirect declarations.
- Runtime form helpers and generated first-slice action input decoders that
  preserve repeated values and reject unexpected fields.
- Generated first-slice required-field validation for direct static form
  controls when actions declare `valid(input)?`.
- Initial `gowdk build [--config <file>] [--out <dir>] [files...]` support for simple static/action pages, explicit or discovered component files, static `gowdk.config.go` source/output settings, a generated static route manifest, and a generated asset manifest.
- Generated embedded static app output through `gowdk build --app <dir>` and
  optional one-binary compilation through `--bin <file>`, including static POST
  redirect handlers for the first supported action subset.
- Local `gowdk serve --dir <dir>` support for trying generated static output
  during development.
- Named config modules for organizing source groups such as `frontend`,
  `frontend2`, `backend`, and service modules during build discovery, with
  `gowdk build --module <name>` for selecting modules on demand.
- Initial CSS extension point for configured stylesheet links and compile-time CSS processors.
- Manifest validation for the core render rules.

The current build command emits static HTML for simple build-time pages only. It loads the literal subset of `gowdk.config.go` for `Source.Include`, `Source.Exclude`, `Modules`, `Build.Output`, and `Build.Stylesheets` when present. When no files are passed, it discovers configured root/module sources or `**/*.gwdk` from the current directory; `--module <name>` limits discovery to selected configured modules for user-owned deployment workflows. Discovery excludes `.git`, `vendor`, `node_modules`, configured excludes, and the selected output directory. It supports self-closing component calls with static string props when component files are passed explicitly or discovered. It can expand dynamic routes from literal `paths {}` declarations such as `=> { slug: "hello-gowdk" }`, render those route params and literal `build {}` data in the current static `view {}` interpolation subset, inject stylesheet links, invoke compile-time CSS processors that emit CSS assets, and record emitted CSS assets in `gowdk-assets.json`. It parses the first action body subset and generated apps can handle POST redirects for concrete static/action page routes. Generated action handlers now create named first-slice input wrappers, preserve repeated submitted values, reject unexpected fields inferred from direct static controls in same-page `g:post` forms, and return HTTP 422 for missing required fields when actions declare `valid(input)?`. `gowdk build --app` can generate a dependency-free Go app that embeds those static files, exposes `/_gowdk/health`, and identifies app/module instances through `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`, and `GOWDK_INSTANCE_ID`; when no instance ID is provided, the app generates one at process start. `--bin` can compile that app into one local binary. `gowdk serve` can serve this generated output locally for development. GOWDK does not yet support component children, non-string props, general expressions, arbitrary `build {}` execution, real user Go type resolution for typed form decoders, CSRF, generated user action logic, generated API/fragment handlers, partial fragment execution, SSR output, request-time routes in the generated binary, or built-in Kubernetes manifest generation.

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
- Node.js only when checking the VS Code extension with `node --check`.
- VS Code 1.85 or newer for the bundled extension.

## Commands

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js

go run ./cmd/gowdk version
go run ./cmd/gowdk tokens <file.gwdk>
go run ./cmd/gowdk fmt [--write] <file.gwdk>
go run ./cmd/gowdk check [--json] [--ssr] <file.gwdk>
go run ./cmd/gowdk manifest [--ssr] <file.gwdk>
go run ./cmd/gowdk sitemap [--ssr] <files>
go run ./cmd/gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]
go run ./cmd/gowdk serve --dir <dir> [--addr 127.0.0.1:8080]
go run ./cmd/gowdk lsp [--ssr]
```

Runnable examples and their current limitations are documented in `examples/README.md`.

## Editor Tools

The VS Code extension lives in `editors/vscode/`.

It provides syntax highlighting, bracket/comment configuration, diagnostics through `gowdk check --json`, document formatting through `gowdk fmt`, completions, token preview, manifest preview, a persistent Explorer site-map tree, and a larger site-map visualizer for movable `.gwdk` files.

Editors with Language Server Protocol support can launch `gowdk lsp` over stdio for live diagnostics, document formatting, and completions. Pass `--ssr` to validate buffers as if the SSR addon is enabled.

The site map shows page IDs, routes, render modes, layouts, blocks, and source files. It can open a page file or move it to a new folder while preserving the route declared inside the file.

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
- `docs/product/build-discovery-spec.md`: current build discovery and static route manifest slice.
- `docs/product/build-discovery-plan.md`: implementation plan for build discovery and static route manifest.
- `docs/product/module-config-spec.md`: current module config and source-group discovery slice.
- `docs/product/module-config-plan.md`: implementation plan for module config.
- `docs/product/dynamic-static-routes-spec.md`: current dynamic static route metadata slice.
- `docs/product/dynamic-static-routes-plan.md`: implementation plan for dynamic static route metadata.
- `docs/product/local-static-serve-spec.md`: current local static serving slice.
- `docs/product/local-static-serve-plan.md`: implementation plan for local static serving.
- `docs/product/embedded-static-app-spec.md`: current generated embedded static app slice.
- `docs/product/embedded-static-app-plan.md`: implementation plan for embedded static app output.
- `docs/product/typed-action-redirect-spec.md`: current executable action redirect slice.
- `docs/product/typed-action-redirect-plan.md`: implementation plan for action redirect output.
- `docs/product/action-form-directive-spec.md`: current `g:post` form directive slice.
- `docs/product/action-form-directive-plan.md`: implementation plan for `g:post` lowering.
- `docs/product/typed-form-decoder-spec.md`: current generated form decoder slice.
- `docs/product/typed-form-decoder-plan.md`: implementation plan for first-slice form decoders.
- `docs/product/action-required-validation-spec.md`: current required-field validation slice.
- `docs/product/action-required-validation-plan.md`: implementation plan for first-slice validation.
- `docs/product/missing-implementation-checklist.md`: current backlog and implementation gaps.
- `docs/language/`: current `.gwdk` language contract and planned grammar areas.
- `docs/compiler/`: current compiler pipeline and generated-output contracts.
- `docs/reference/`: CLI, manifest, config, addon, CSS, and diagnostics references.
- `docs/guides/`: user-facing guide index.
- `docs/engineering/architecture.md`: system design and boundaries.
- `docs/engineering/conventions.md`: coding and repository conventions.
- `docs/engineering/testing.md`: test strategy.
- `docs/engineering/security.md`: baseline security posture.
- `docs/engineering/operations.md`: runtime, deployment, observability, and maintenance notes.
- `docs/engineering/ci.md`: planned hosted CI gates and current local gates.
- `docs/engineering/release.md`: release readiness notes.
- `docs/engineering/generated-code-policy.md`: generated output safety and ownership policy.
- `docs/engineering/decisions/`: architecture decision records.

## Verification

The current scaffold is verified with:

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
test -f /tmp/gowdk-build/gowdk-routes.json
test -f /tmp/gowdk-build/gowdk-assets.json
```
