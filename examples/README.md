# Examples

This directory contains small `.gwdk` files for the current language tooling scaffold.

## Current Examples

| File | Purpose | Command |
| --- | --- | --- |
| `basic/home.page.gwdk` | Buildable static page using literal `build {}` data and `Hero`. | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk` |
| `basic/hero.cmp.gwdk` | Buildable static component with string props. | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk` |
| `basic/blog-post.page.gwdk` | Buildable dynamic static route example with literal `paths {}` source. | `go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build examples/basic/blog-post.page.gwdk` |
| `basic/layout-stack.page.gwdk` | Static page that demonstrates ordered layout metadata. | `go run ./cmd/gowdk check examples/basic/layout-stack.page.gwdk` |
| `basic/signup.page.gwdk` | Buildable static action page with the first POST redirect action subset. | `go run ./cmd/gowdk build --out /tmp/gowdk-action-build --app /tmp/gowdk-action-app --bin /tmp/gowdk-action-site examples/basic/signup.page.gwdk` |
| `basic/newsletter.page.gwdk` | Static action-form syntax example with `g:post` and component markup. | `go run ./cmd/gowdk check examples/basic/newsletter.page.gwdk` |
| `basic/patients-fragment.page.gwdk` | Partial/server fragment metadata example with `fragment`, `g:target`, and `g:swap`. | `go run ./cmd/gowdk manifest examples/basic/patients-fragment.page.gwdk` |
| `basic/status.page.gwdk` | API route metadata example with a named `GET` endpoint. | `go run ./cmd/gowdk routes examples/basic/status.page.gwdk` |
| `basic/simple-ssr.page.gwdk` | Simple generated SSR page without `load {}`. | `go run ./cmd/gowdk build --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app --bin /tmp/gowdk-ssr-site examples/basic/simple-ssr.page.gwdk` |
| `basic/dynamic-ssr.page.gwdk` | Validation example for dynamic SSR route metadata. | `go run ./cmd/gowdk check --ssr examples/basic/dynamic-ssr.page.gwdk` |
| `basic/dashboard.page.gwdk` | SSR page with `load` and guard metadata. | `go run ./cmd/gowdk check --ssr examples/basic/dashboard.page.gwdk` |
| `go-interop/imported-build.page.gwdk` | Buildable `.gwdk` import example that calls Go code from `build {}`. | `go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk` |
| `embed/site.page.gwdk` | Standalone one-binary static serving example. | `go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk` |
| `css/styled.page.gwdk` | Configured stylesheet-link example. | `go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk` |
| `tailwind/site.page.gwdk` | Tailwind-style markup with a configured stylesheet link. | `go run ./cmd/gowdk build --config examples/tailwind/gowdk.config.go --out /tmp/gowdk-tailwind-build examples/tailwind/site.page.gwdk` |
| `components/base/base-components.page.gwdk` | Source-level base component examples for `Button`, `TextField`, and `Card`. | `go run ./cmd/gowdk build --out /tmp/gowdk-base-components examples/components/base/*.gwdk` |

Check all current examples with SSR validation enabled:

```sh
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk sitemap --ssr examples/basic/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
go run ./cmd/gowdk routes --ssr examples/basic/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/embed/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk
```

Build the current simple static page:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
test -f /tmp/gowdk-build/gowdk-routes.json
test -f /tmp/gowdk-build/gowdk-assets.json
go run ./cmd/gowdk serve --dir /tmp/gowdk-build
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
```

Build the current dynamic static page:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build examples/basic/blog-post.page.gwdk
test -f /tmp/gowdk-dynamic-build/blog/hello-gowdk/index.html
test -f /tmp/gowdk-dynamic-build/blog/static-first/index.html
```

Build the current static action redirect page:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-action-build --app /tmp/gowdk-action-app --bin /tmp/gowdk-action-site examples/basic/signup.page.gwdk
test -x /tmp/gowdk-action-site
```

Build the current simple generated SSR page:

```sh
go run ./cmd/gowdk build --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app --bin /tmp/gowdk-ssr-site examples/basic/simple-ssr.page.gwdk
test -x /tmp/gowdk-ssr-site
```

Build the current `.gwdk` Go import example:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk
test -f /tmp/gowdk-go-interop/go-imported/index.html
go test ./examples/go-interop
```

Build the one-binary static serving example:

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

Build the Tailwind-style stylesheet-link example:

```sh
go run ./cmd/gowdk build --config examples/tailwind/gowdk.config.go --out /tmp/gowdk-tailwind-build examples/tailwind/site.page.gwdk
test -f /tmp/gowdk-tailwind-build/tailwind/index.html
```

Build the source-level base component examples:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-base-components examples/components/base/*.gwdk
test -f /tmp/gowdk-base-components/components/base/index.html
```

## Current Limitations

- `gowdk build` emits static HTML, `gowdk-routes.json`, and
  `gowdk-assets.json` for simple static pages, components, literal dynamic
  paths, and imported Go build data today.
- Default build discovery exists, but running it from `examples/basic` currently includes an SSR example with `load {}` and guard metadata that is validation-only. Pass explicit files for build smoke commands.
- Static examples can be served locally with `gowdk serve` or compiled into an
  embedded static binary with `gowdk build --app --bin`; the current generated
  binary supports first-slice action redirects, form input decoder wrappers, and
  required-field validation, plus first-slice concrete SSR pages without
  `load {}`. It does not run real typed action logic, APIs, partial fragments,
  request-time `load {}` functions, guard enforcement, or dynamic SSR routes yet.
- `view {}` bodies are parsed only for a small static HTML subset; `act` bodies
  support the first form-input/redirect subset, `api` bodies support the first
  method/route metadata line, and `load` bodies are still not parsed beyond
  top-level block detection.
- `.gwdk` page imports currently support the first build-time data slice:
  `build { => alias.Func() }` for a no-argument Go function returning a JSON
  object. Generated action, API, partial, and `load {}` user handler wiring is
  still planned.
- `@guard` is metadata only and does not enforce authentication.
- Route params from literal `paths {}` are available to the current static
  `view {}` interpolation subset and to literal `build {}` string
  interpolation. Imported build functions do not receive route params yet.
- Literal `build {}` string data and scalar fields returned by imported
  no-argument Go build functions are available to the current static `view {}`
  interpolation subset.
- Component children, generated API handlers, generated partial fragment
  handlers, active partial-update client behavior, and full configured plugin
  instantiation are planned.
