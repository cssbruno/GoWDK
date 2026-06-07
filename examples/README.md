# Examples

This directory contains small `.gwdk` files for the current compiler and runtime
scaffold. Examples are grouped by capability so build-time pages, generated
actions, partial fragments, SSR metadata, styling, embedding, and Go interop can
be tested independently.

Run these commands from the repository root. The root `gowdk.config.go` is part
of the example smoke setup and is required by project-level compiler commands,
even when explicit `.gwdk` files are passed.

## Current Examples

| File | Purpose | Command |
| --- | --- | --- |
| `pages/home.page.gwdk` | Buildable page using literal `build {}` data and `Hero`. | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk` |
| `pages/hero.cmp.gwdk` | Buildable component with string props. | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk` |
| `pages/blog-post.page.gwdk` | Buildable dynamic route example with literal `paths {}` source. | `go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build examples/pages/blog-post.page.gwdk` |
| `pages/layout-stack.page.gwdk` | Page that demonstrates ordered layout metadata. | `go run ./cmd/gowdk check examples/pages/layout-stack.page.gwdk` |
| `actions/signup.page.gwdk` | Buildable action page with the first POST redirect action subset. | `go run ./cmd/gowdk build --out /tmp/gowdk-action-build --app /tmp/gowdk-action-app --bin /tmp/gowdk-action-site examples/actions/signup.page.gwdk` |
| `actions/newsletter.page.gwdk` | Action-form syntax example with `g:post` and component markup. | `go run ./cmd/gowdk check examples/actions/newsletter.page.gwdk` |
| `login/` | Integrated auth-feature GWDK login with generated frontend binary plus feature-owned Go backend binary. | `cd examples/login && make serve` |
| `partials/patients-fragment.page.gwdk` | Partial/server fragment metadata example with `fragment`, `g:target`, and `g:swap`. | `go run ./cmd/gowdk manifest examples/partials/patients-fragment.page.gwdk` |
| `api/status.page.gwdk` | API route metadata example with a named `GET` endpoint. | `go run ./cmd/gowdk routes examples/api/status.page.gwdk` |
| `ssr/simple-ssr.page.gwdk` | Simple generated SSR page without `load {}`. | `go run ./cmd/gowdk build --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app --bin /tmp/gowdk-ssr-site examples/ssr/simple-ssr.page.gwdk` |
| `ssr/dynamic-ssr.page.gwdk` | Validation example for dynamic SSR route metadata. | `go run ./cmd/gowdk check --ssr examples/ssr/dynamic-ssr.page.gwdk` |
| `ssr/dashboard.page.gwdk` | SSR page with `load` and guard metadata. | `go run ./cmd/gowdk check --ssr examples/ssr/dashboard.page.gwdk` |
| `go-interop/imported-build.page.gwdk` | Buildable `.gwdk` import example that calls Go code from `build {}`. | `go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk` |
| `embed/site.page.gwdk` | Standalone one-binary generated app example. | `go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk` |
| `css/styled.page.gwdk` | Configured stylesheet-link example. | `go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk` |
| `tailwind/site.page.gwdk` | Tailwind v4 addon example using the standalone CLI. | `go run ./cmd/gowdk build --config examples/tailwind/gowdk.config.go --out /tmp/gowdk-tailwind-build examples/tailwind/site.page.gwdk` |
| `components/base/base-components.page.gwdk` | Source-level base component examples for `Button`, `TextField`, and `Card`. | `go run ./cmd/gowdk build --out /tmp/gowdk-base-components examples/components/base/*.gwdk` |
| `components/css/scoped-card.page.gwdk` | Component-local `@css` metadata example. | `go run ./cmd/gowdk check examples/components/css/*.gwdk` |

Check all current examples with SSR validation enabled:

```sh
go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk sitemap --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk routes --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
```

Build the current simple page:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
test -f /tmp/gowdk-build/gowdk-routes.json
test -f /tmp/gowdk-build/gowdk-assets.json
go run ./cmd/gowdk serve --dir /tmp/gowdk-build
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
```

Build the current dynamic page:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build examples/pages/blog-post.page.gwdk
test -f /tmp/gowdk-dynamic-build/blog/hello-gowdk/index.html
test -f /tmp/gowdk-dynamic-build/blog/compile-first/index.html
```

Build the current action redirect page:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-action-build --app /tmp/gowdk-action-app --bin /tmp/gowdk-action-site examples/actions/signup.page.gwdk
test -x /tmp/gowdk-action-site
```

Run the current login backend example:

```sh
cd examples/login
make serve
```

Build the current simple generated SSR page:

```sh
go run ./cmd/gowdk build --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app --bin /tmp/gowdk-ssr-site examples/ssr/simple-ssr.page.gwdk
test -x /tmp/gowdk-ssr-site
```

Build the current `.gwdk` Go import example:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk
test -f /tmp/gowdk-go-interop/go-imported/index.html
go test ./examples/go-interop
```

Build the one-binary generated app example:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk
test -x /tmp/gowdk-embed-site
```

Build the CSS stylesheet-link example:

```sh
go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk
grep -F '<link rel="stylesheet" href="/assets/site.css">' /tmp/gowdk-css-build/styled/index.html
go test ./examples/css
```

Build the Tailwind addon example. The addon uses `tailwindcss` from `PATH` or
downloads the official standalone executable into `.gowdk/bin`:

```sh
go run ./cmd/gowdk build --config examples/tailwind/gowdk.config.go --out /tmp/gowdk-tailwind-build examples/tailwind/site.page.gwdk
test -f /tmp/gowdk-tailwind-build/tailwind/index.html
grep -F 'assets/app.' /tmp/gowdk-tailwind-build/tailwind/index.html
```

Build the source-level base component examples:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-base-components examples/components/base/*.gwdk
test -f /tmp/gowdk-base-components/components/base/index.html
```

Check the component-local CSS metadata example:

```sh
go run ./cmd/gowdk check examples/components/css/*.gwdk
```

## Current Limitations

- `gowdk build` emits app-shell HTML, `gowdk-routes.json`, and
  `gowdk-assets.json` for simple app pages, components, literal dynamic
  paths, and imported Go build data today.
- Default build discovery exists, but the examples are intentionally split by
  capability because some files are validation-only for the current slice. Pass
  explicit files for build smoke commands.
- Build-output examples can be served locally with `gowdk serve` or compiled into
  an embedded binary with `gowdk build --app --bin`; the current generated
  binary supports action redirects, partial action fragments, form input
  decoder wrappers, and required-field validation, plus concrete and
  dynamic SSR pages with declared `load {}` fields. It runs declared guards for
  generated SSR/action/API routes and fails closed when guard functions are not
  registered.
- `view {}` bodies are parsed only for a small app-shell HTML subset; `act` bodies
  support the first form-input/redirect subset, `api` bodies support the first
  method/route metadata line, and `load` bodies are still not parsed beyond
  top-level block detection.
- `.gwdk` page imports currently support the first build-time data slice:
  `build { => alias.Func() }` for a no-argument Go function returning a JSON
  object. Generated action, API, partial, and `load {}` user handler wiring is
  implemented for the supported first request-time signatures.
- `@guard` is enforced by generated SSR/action/API handlers. Guarded routes
  fail closed unless the generated app registers an `ssr.GuardRegistry`.
- Route params from literal `paths {}` are available to the current
  `view {}` interpolation subset and to literal `build {}` string
  interpolation. Imported build functions do not receive route params yet.
- Literal `build {}` string data and scalar fields returned by imported
  no-argument Go build functions are available to the current `view {}`
  interpolation subset.
- Component children, generated API handlers, and rich local client-side
  reactivity are planned.
