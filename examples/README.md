# Examples

This directory contains small `.gwdk` files for the current language tooling scaffold.

## Current Examples

| File | Purpose | Command |
| --- | --- | --- |
| `basic/home.page.gwdk` | Buildable static page using literal `build {}` data and `Hero`. | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk` |
| `basic/hero.cmp.gwdk` | Buildable static component with string props. | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk` |
| `basic/blog-post.page.gwdk` | Buildable dynamic static route example with literal `paths {}` source. | `go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build examples/basic/blog-post.page.gwdk` |
| `basic/signup.page.gwdk` | Buildable static action page with the first POST redirect action subset. | `go run ./cmd/gowdk build --out /tmp/gowdk-action-build --app /tmp/gowdk-action-app --bin /tmp/gowdk-action-site examples/basic/signup.page.gwdk` |
| `basic/newsletter.page.gwdk` | Static action-form syntax example with `g:post` and component markup. | `go run ./cmd/gowdk check examples/basic/newsletter.page.gwdk` |
| `basic/dashboard.page.gwdk` | SSR page with `load` and guard metadata. | `go run ./cmd/gowdk check --ssr examples/basic/dashboard.page.gwdk` |

Check all current examples with SSR validation enabled:

```sh
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
go run ./cmd/gowdk sitemap --ssr examples/basic/*.gwdk
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

## Current Limitations

- `gowdk build` emits static HTML, `gowdk-routes.json`, and `gowdk-assets.json` for the simple home page, hero component, and literal dynamic paths today.
- Default build discovery exists, but running it from `examples/basic` currently includes an SSR example that needs future generated request-time output. Pass explicit files for build smoke commands.
- Static examples can be served locally with `gowdk serve` or compiled into an
  embedded static binary with `gowdk build --app --bin`; the current generated
  binary supports first-slice action redirects, form input decoder wrappers, and
  required-field validation, but not real typed action logic, APIs, partial
  fragments, or SSR handlers yet.
- `view {}` bodies are parsed only for a small static HTML subset; `act` bodies
  support the first form-input/redirect subset, while `api` and `load` bodies
  are still not parsed beyond top-level block detection.
- `@guard` is metadata only and does not enforce authentication.
- Route params from literal `paths {}` are available to the current static
  `view {}` interpolation subset, but not to `build {}` expressions yet.
- Literal `build {}` string data is available to the current static `view {}`
  interpolation subset.
- Component children, partial fragment, API route, CSS/plugin, `g:target`,
  `g:swap`, and broader generated app examples are planned.
