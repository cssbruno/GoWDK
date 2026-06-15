# Native Cookbook

Status: current 0.x examples and recipes. This cookbook links to existing
examples and reference pages instead of repeating the full learning path.

Run repository commands from the repository root unless a recipe says to `cd`
into an example directory.

## Pages And Routes

Build a static page and component:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk
```

Build a dynamic SPA route declared with `paths {}`:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build \
  examples/pages/blog-post.page.gwdk
```

Use [routing](../reference/routing.md), [language blocks](../language/blocks.md),
and [examples/pages](../../examples/pages/) for the source contract.

## Components, Client State, And WASM Islands

Build source-level base components:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-base-components \
  examples/components/base/*.gwdk
```

Build the component WASM island ABI example:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-wasm-island \
  examples/components/wasm/*.gwdk
```

Use [components](../language/components.md), [markup](../language/markup.md),
and [examples/components](../../examples/components/) for supported component
and client behavior.

## Actions, APIs, And Fragments

Build the endpoint cookbook:

```sh
cd examples/endpoints
make check
make routes
make build
```

Use [actions](../language/actions.md), [APIs](../language/api.md),
[partials](../language/partials.md), and
[examples/endpoints](../../examples/endpoints/) for action redirects,
validation fragments, JSON APIs, webhooks, and standalone fragments.

## SSR, Hybrid Pages, And Guards

Build a simple generated SSR page:

```sh
go run ./cmd/gowdk build --ssr \
  --out /tmp/gowdk-ssr-build \
  --app /tmp/gowdk-ssr-app \
  --bin /tmp/gowdk-ssr-site \
  examples/ssr/simple-ssr.page.gwdk
```

Build the auth guard example:

```sh
cd examples/auth-guard
make check
make routes
make build
```

Use [SSR](../language/ssr.md), [hybrid pages](../language/hybrid.md),
[guards](../language/guards.md), and
[examples/auth-guard](../../examples/auth-guard/) for request-time page and
route-gate behavior. Backend resource authorization remains app-owned Go code.

## Go Interop And Contracts

Build a page that calls imported Go from `build {}`:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-go-interop \
  examples/go-interop/imported-build.page.gwdk
```

Build the command/query contract example:

```sh
go run ./cmd/gowdk build \
  --config examples/contracts/gowdk.config.go \
  --out /tmp/gowdk-contracts-build \
  --app /tmp/gowdk-contracts-app \
  --bin /tmp/gowdk-contracts-site \
  examples/contracts/patients.page.gwdk
```

Use [Go interop](../reference/go-interop.md),
[contracts](../reference/contracts.md), [realtime](../reference/realtime.md),
and [examples/contracts](../../examples/contracts/) for handler and contract
registration patterns.

## CSS, Assets, SEO, And Images

Build a configured stylesheet example:

```sh
go run ./cmd/gowdk build \
  --config examples/css/gowdk.config.go \
  --out /tmp/gowdk-css-build \
  examples/css/styled.page.gwdk
```

Build the SEO addon example:

```sh
go run ./cmd/gowdk build \
  --config examples/seo/gowdk.config.go \
  --out /tmp/gowdk-seo-build \
  examples/seo/*.gwdk
```

Use [CSS](../reference/css.md), [images](../reference/images.md),
[SEO](../reference/seo.md), and [manifest](../reference/manifest.md) for asset
and metadata behavior.

## Deployment, Testing, And Operations

Build a one-binary generated app:

```sh
go run ./cmd/gowdk build \
  --out /tmp/gowdk-embed-build \
  --app /tmp/gowdk-embed-app \
  --bin /tmp/gowdk-embed-site \
  examples/embed/site.page.gwdk
```

Emit optional deployment recipe starters:

```sh
go run ./cmd/gowdk build \
  --out /tmp/gowdk-build \
  --app /tmp/gowdk-app \
  --bin /tmp/gowdk-site \
  --deploy-recipe systemd,caddy \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk
```

Use [deployment](../reference/deployment.md), [testing](../reference/testing.md),
[dev server](../reference/dev.md), and
[security](../engineering/security.md) for current operations guidance.

## Playground Export

Export an ordinary source project archive:

```sh
gowdk playground export --dir . --out /tmp/gowdk-project.zip
```

Use [playground onboarding and sandboxing](../product/playground.md) for
hosted execution constraints and export rules.

## Coverage And Gaps

- Current cookbook coverage lives here, in
  [examples/README.md](../../examples/README.md), and in the focused example
  directories.
- Current reference coverage lives in [reference](../reference/README.md).
- Language syntax and semantics live in [language](../language/README.md);
  compiler output contracts live in [compiler](../compiler/README.md).
- Production auth/session policy, database schemas, storage, backups, incident
  response, and hosted playground infrastructure are app-owned or platform
  owned. See [requirements](../product/requirements.md) and
  [release-plan](../engineering/release-plan.md) for tracked partial areas.
