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
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --test editors/vscode/*.test.js
go build -o /tmp/gowdk-cli ./cmd/gowdk
rm -rf /tmp/gowdk-init && /tmp/gowdk-cli init /tmp/gowdk-init && (cd /tmp/gowdk-init && /tmp/gowdk-cli build)
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
go run ./cmd/gowdk watch --once --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
test -f /tmp/gowdk-build/gowdk-routes.json
test -f /tmp/gowdk-build/gowdk-assets.json
go test ./cmd/gowdk
```

Use `node --check` and `node --test editors/vscode/*.test.js` when editor extension files change or before release-style verification.

## Verification Matrix

| Area | Command | When |
| --- | --- | --- |
| Go packages | `go test ./...` | Every code change. |
| CLI build | `go build ./cmd/gowdk` | CLI, compiler, runtime, addon, or release changes. |
| Go formatting | `gofmt -w <files>` | Changed Go files before handoff. |
| VS Code extension syntax | `node --check editors/vscode/extension.js` | Editor extension changes and broad verification. |
| VS Code extension behavior | `node --test editors/vscode/*.test.js` | Editor extension pure helper changes and broad verification. |
| Static/action examples | `go run ./cmd/gowdk check examples/basic/home.page.gwdk examples/basic/newsletter.page.gwdk` | Language/tooling changes. |
| Init project smoke | `go build -o /tmp/gowdk-cli ./cmd/gowdk && rm -rf /tmp/gowdk-init && /tmp/gowdk-cli init /tmp/gowdk-init && (cd /tmp/gowdk-init && /tmp/gowdk-cli build)` | CLI scaffold changes. |
| SSR example | `go run ./cmd/gowdk check --ssr examples/basic/dashboard.page.gwdk` | SSR validation or example changes. |
| Manifest smoke | `go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk` | Manifest, parser, or CLI output changes. |
| Static build smoke | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk && test -f /tmp/gowdk-build/gowdk-routes.json && test -f /tmp/gowdk-build/gowdk-assets.json` | Parser, view, staticgen, component, or CLI build changes. |
| Watch smoke | `go run ./cmd/gowdk watch --once --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk` | Watch/dev mode changes. |
| Action redirect smoke | `go run ./cmd/gowdk build --out /tmp/gowdk-action-build --app /tmp/gowdk-action-app --bin /tmp/gowdk-action-site examples/basic/signup.page.gwdk` | Action parsing or generated action route changes. |
| Local serve tests | `go test ./cmd/gowdk` | CLI serving or option parsing changes. |
| Generated static app tests | `go test ./cmd/gowdk ./internal/appgen` | Generated embedded app or binary-serving changes. |

## Coverage Priorities

1. Core domain behavior.
2. Compiler diagnostics for render modes and addon requirements.
3. Manifest generation and route normalization.
4. Parser behavior for `.gwdk` syntax.
5. Static/prerender output and generated route handlers.
6. Typed action decoding, validation, CSRF, redirects, and fragments.
7. SSR addon routing, guards, `load {}`, layouts, and errors.

## Existing Coverage

- `internal/compiler` tests cover SSR addon enforcement, duplicate page/component/layout identities, layout reference resolution, dynamic static routes requiring `paths`, static actions without SSR, and `load {}` rejection on static pages.
- `internal/discover` tests cover recursive `.gwdk` include/exclude matching.
- `internal/parser` tests cover page/component/layout annotations, `paths`, `build`, `load`, `view`, `props`, `act`, captured `paths`/`build`/`load`/`view` bodies, the first action input/redirect body subset, and render mode rejection.
- `internal/view` tests cover static markup rendering, escaping, expression attributes, shorthand class/id normalization, component expansion, and missing component/prop errors.
- `internal/staticgen` tests cover static HTML emission, literal build data, build-data route-param binding, literal dynamic paths, route and asset manifest output, component expansion, nested route output paths, and no partial output on unsupported pages.
- `internal/lang` tests cover lexical tokenization, diagnostics, formatting, file checks, and manifest JSON from parsed source files.
- `internal/lang` tests cover site-map JSON for movable page files.
- `internal/manifest` tests cover route manifest JSON render/path/guard/action output.
- `internal/codegen` tests cover route bindings for static pages, actions, SSR pages, APIs, and missing SSR addon rejection.
- `internal/clientrt` tests cover the emitted partial-update runtime source and
  run a dependency-free Node DOM harness for innerHTML and outerHTML swaps when
  `node` is available.
- `internal/lsp` tests cover initialize, diagnostics, formatting, completion, shutdown, and exit protocol behavior.
- `editors/vscode` tests cover extension route hierarchy helpers.
- `cmd/gowdk` tests cover `build --out` writing `index.html`, expanding a component file, discovering build inputs when explicit paths are omitted, loading literal build config for source/output settings, and local static serving behavior.
- `cmd/gowdk` tests cover `watch --once`, invalid watch intervals, and input
  snapshot change detection.
- `cmd/gowdk` and `internal/appgen` tests cover generated embedded static app
  source, binary compilation, live binary HTTP serving, and first-slice action
  redirect routing.
- `internal/project` tests cover static `gowdk.config.go` parsing for source
  discovery, module source groups, and build output.
