# Testing Strategy

## Current Status

Use Go's standard test runner.

## Baseline Expectations

- Unit tests cover domain logic and pure transformations.
- Integration tests cover persistence, API boundaries, queues, and external service adapters.
- End-to-end tests cover critical user workflows once a UI or API surface exists.
- Regression tests accompany meaningful bug fixes.

## Commands

```sh
scripts/test-go-modules.sh
scripts/check-root-deps.sh
go test ./...
scripts/vulncheck-go-modules.sh
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --test editors/vscode/*.test.js
scripts/test-parser-fuzz.sh
scripts/test-generated-app-integration.sh
scripts/test-generated-output-determinism.sh
scripts/test-runtime-race.sh
go build -o /tmp/gowdk-cli ./cmd/gowdk
rm -rf /tmp/gowdk-init && /tmp/gowdk-cli init /tmp/gowdk-init && (cd /tmp/gowdk-init && /tmp/gowdk-cli build)
rm -rf /tmp/gowdk-init-tested && /tmp/gowdk-cli init --tests /tmp/gowdk-init-tested && (cd /tmp/gowdk-init-tested && /tmp/gowdk-cli test --count=1 --timeout=2m)
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
test -f /tmp/gowdk-build/gowdk-routes.json
test -f /tmp/gowdk-build/gowdk-assets.json
test -f /tmp/gowdk-build/openapi.json
test -f /tmp/gowdk-build/asyncapi.json
go test ./cmd/gowdk
```

Use `node --check` and `node --test editors/vscode/*.test.js` when editor extension files change or before release-style verification.

Example commands run from the repository root use the root `gowdk.config.go`.
Project-level compiler commands must have that file in the working directory or
must pass `--config <file>`.

## Verification Matrix

| Area | Command | When |
| --- | --- | --- |
| Go packages | `scripts/test-go-modules.sh` | Every code change; includes the root module and nested optional adapter modules. |
| Root dependency surface | `scripts/check-root-deps.sh` | Dependency-policy changes, CI, and release-style verification. |
| Root Go packages | `go test ./...` | Core compiler/runtime changes when optional adapter modules are not relevant. |
| Go vulnerability scan | `scripts/vulncheck-go-modules.sh` | Release-style checks and dependency changes. |
| CLI build | `go build ./cmd/gowdk` | CLI, compiler, runtime, addon, or release changes. |
| Go formatting | `gofmt -w <files>` | Changed Go files before handoff. |
| Parser fuzz smoke | `scripts/test-parser-fuzz.sh` | Parser, tokenizer, syntax recovery, and diagnostic recovery changes. |
| Generated app integration | `scripts/test-generated-app-integration.sh` | Generated binary, SSR, action, fragment, CSRF, and contract adapter changes. |
| Generated output determinism | `scripts/test-generated-output-determinism.sh` | Generated HTML, manifest, report, inspect, or route/sitemap output changes. |
| Runtime race detector | `scripts/test-runtime-race.sh` | Shared-state runtime packages, lifecycle, contracts, SSE, rate limiting, trace, or testkit changes. |
| VS Code extension syntax | `node --check editors/vscode/extension.js` | Editor extension changes and broad verification. |
| VS Code extension behavior | `node --test editors/vscode/*.test.js` | Editor extension pure helper changes and broad verification. |
| SPA and action endpoint examples | `go run ./cmd/gowdk check examples/pages/home.page.gwdk examples/actions/newsletter.page.gwdk` | Language/tooling changes. |
| Init project smoke | `go build -o /tmp/gowdk-cli ./cmd/gowdk && rm -rf /tmp/gowdk-init && /tmp/gowdk-cli init /tmp/gowdk-init && (cd /tmp/gowdk-init && /tmp/gowdk-cli build)` | CLI scaffold changes. |
| Init project tests | `go build -o /tmp/gowdk-cli ./cmd/gowdk && rm -rf /tmp/gowdk-init-tested && /tmp/gowdk-cli init --tests /tmp/gowdk-init-tested && (cd /tmp/gowdk-init-tested && /tmp/gowdk-cli test --count=1 --timeout=2m)` | CLI scaffold or `gowdk test` changes. |
| SSR example | `go run ./cmd/gowdk check --ssr examples/ssr/dashboard.page.gwdk` | SSR validation or example changes. |
| Manifest smoke | `go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk` | Manifest, parser, or CLI output changes. |
| SPA build smoke | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk && test -f /tmp/gowdk-build/gowdk-routes.json && test -f /tmp/gowdk-build/gowdk-assets.json && test -f /tmp/gowdk-build/openapi.json && test -f /tmp/gowdk-build/asyncapi.json` | Parser, view, buildgen, component, or CLI build changes. |
| Dev loop tests | `go test ./cmd/gowdk` | Dev mode changes. |
| Action redirect smoke | `go run ./cmd/gowdk build --out /tmp/gowdk-action-build --app /tmp/gowdk-action-app --bin /tmp/gowdk-action-site examples/actions/signup.page.gwdk` | Action parsing or generated action endpoint changes. |
| Local serve tests | `go test ./cmd/gowdk` | CLI serving or option parsing changes. |
| Generated app tests | `go test ./cmd/gowdk ./internal/appgen` | Generated embedded app or binary-serving changes. |

## Fuzz, Integration, And Determinism

- `scripts/test-parser-fuzz.sh` is the explicit parser fuzz runner. It defaults
  to `GOWDK_FUZZTIME=1s` for CI smoke cost; use a longer local run such as
  `GOWDK_FUZZTIME=30s scripts/test-parser-fuzz.sh` before risky parser work.
- `scripts/test-generated-app-integration.sh` is the generated-app integration
  slice. It builds temporary binaries through `internal/appgen` tests and
  exercises representative request-time flows.
- `scripts/test-generated-output-determinism.sh` is the generated output and
  report determinism slice. It compares two clean builds plus manifest, sitemap,
  routes, and asset-graph report output.
- `scripts/test-runtime-race.sh` is the focused race-detector slice. It runs an
  explicit package list under `go test -race -count=1` and fails preflight when
  a selected package has no tests.

Keep these as separate commands so CI can show whether a failure is fuzz input,
runtime integration, race detection, or nondeterministic output.

## Coverage Priorities

1. Core domain behavior.
2. Compiler diagnostics for render modes and addon requirements.
3. Manifest generation and route normalization.
4. Parser behavior for `.gwdk` syntax.
5. SPA/prerender output and generated route handlers.
6. Typed action decoding, validation, CSRF, redirects, and fragments.
7. SSR addon routing, guards, `server {}`, layouts, and errors.

## Existing Coverage

- `internal/compiler` tests cover SSR addon enforcement, duplicate page/component/layout identities, layout reference resolution, dynamic SPA routes requiring `paths`, actions without SSR, and `server {}` rejection on SPA pages.
- `internal/discover` tests cover recursive `.gwdk` include/exclude matching.
- `internal/parser` tests cover page/component/layout metadata declarations, `paths`, `build`, `server`, `view`, `props`, `act`, captured `paths`/`build`/`server`/`view` bodies, the first action input/redirect body subset, and render mode rejection.
- `internal/viewrender` tests cover view markup rendering, escaping, expression attributes, shorthand class/id normalization, component expansion, and missing component/prop errors.
- `internal/buildgen` tests cover app-shell HTML emission, literal build data,
  imported Go build data functions, build-data route-param binding, literal
  dynamic paths, route and asset manifest output, component expansion, nested
  route output paths, and no partial output on unsupported pages.
- `internal/lang` tests cover lexical tokenization, diagnostics, formatting, file checks, and manifest JSON from parsed source files.
- `internal/lang` tests cover site-map JSON for movable page files.
- `internal/lang` golden tests cover the IR-derived manifest JSON render/path/guard/action output.
- `internal/compiler` tests cover route metadata for SPA/SSR routes, endpoint
  metadata for actions/APIs, and missing SSR addon rejection.
- `internal/clientrt` tests cover the embedded framework browser runtime source
  files, render the placeholder templates, require Node for the client
  expression conformance test against `internal/clientlang`, run `node --check`
  for those `.js` files when `node` is available, and run a dependency-free Node
  DOM harness for innerHTML and outerHTML swaps.
- `internal/lsp` tests cover initialize, diagnostics, formatting, completion,
  hover, component and open-Go go-to-definition, references, semantic tokens,
  code actions, shutdown, and exit protocol behavior.
- `editors/vscode` tests cover extension route hierarchy helpers.
- `cmd/gowdk` tests cover `build --out` writing `index.html`, expanding a component file, discovering build inputs when explicit paths are omitted, loading literal build config for source/output settings, configured build targets, selected target builds, and local generated-output serving behavior.
- `cmd/gowdk` tests cover dev option parsing, invalid dev intervals,
  content-hash input snapshots, snapshot diffs, no-op touch detection,
  incremental SPA page rebuild selection, and component-change fallback.
- `cmd/gowdk` and `internal/appgen` tests cover generated embedded app
  source, binary compilation, WASM artifact compilation, live binary HTTP
  serving, and first-slice action redirect routing.
- `cmd/gowdk` tests cover `gowdk test` building temporary generated artifacts,
  starting the generated binary on an ephemeral address, and running scaffolded
  generated app tests through ordinary `go test`.
- `runtime/testkit` tests cover scenario helpers, cookie-preserving direct and
  `httptest.Server` clients, JSON/form request helpers, response assertions,
  and redacted failure summaries.
- Update the generated app golden with
  `go test ./internal/appgen -run TestGeneratedGoMatchesGoldenFixture -update`
  when an intentional generated Go change lands.
- `internal/buildgen` and `internal/appgen` tests cover preserving unchanged
  generated file modification times so local dev loops do not retrigger on
  identical output.
- `internal/buildgen` tests cover incremental changed-page rendering, complete
  route manifest refreshes, and stale route output cleanup.
- `internal/project` tests cover literal `gowdk.config.go` parsing for source
  discovery, module source groups, build output, and build targets.
- Nested optional adapter modules cover Chi, Echo, Fiber, Gin, Redis Streams,
  NATS, and WebSocket integration packages without adding those third-party
  dependencies to the root module graph.
- `internal/parser/fuzz_test.go` defines `FuzzParseSyntax` with page,
  component, layout, endpoint, and broken-source seeds.
- `scripts/test-generated-output-determinism.sh` compares generated HTML,
  route/asset manifests, OpenAPI/AsyncAPI, build reports, and public report
  command output across two clean runs.
