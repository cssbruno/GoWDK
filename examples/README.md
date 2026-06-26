# Examples

This directory contains runnable `.gwdk` examples for the current compiler and
runtime surface. Use it as inventory; use the cookbook for task-oriented recipes.

Run commands from the repository root unless an example says to `cd` into its
own directory. The root `gowdk.config.go` is part of the example smoke setup and
is required by project-level compiler commands.

## Start Here

| Goal | Example | Command |
| --- | --- | --- |
| Static page and component | `examples/pages/` | `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk` |
| Dynamic SPA route | `examples/pages/blog-post.page.gwdk` | `go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build examples/pages/blog-post.page.gwdk` |
| Build-time Go data | `examples/go-interop/` | `go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk` |
| Actions, APIs, fragments | `examples/endpoints/` | `cd examples/endpoints && make check && make routes && make build` |
| SSR and guards | `examples/auth-guard/` | `cd examples/auth-guard && make check && make routes && make build` |
| One generated binary | `examples/embed/` | `go run ./cmd/gowdk build --out /tmp/gowdk-embed-build --app /tmp/gowdk-embed-app --bin /tmp/gowdk-embed-site examples/embed/site.page.gwdk` |
| Contracts and realtime | `examples/contracts/` | `go run ./cmd/gowdk build --config examples/contracts/gowdk.config.go --out /tmp/gowdk-contracts-build --app /tmp/gowdk-contracts-app --bin /tmp/gowdk-contracts-site examples/contracts/patients.page.gwdk` |
| CSS and Tailwind | `examples/css/`, `examples/tailwind/` | `go run ./cmd/gowdk build --config examples/css/gowdk.config.go --out /tmp/gowdk-css-build examples/css/styled.page.gwdk` |
| Component assets and WASM islands | `examples/components/` | `go run ./cmd/gowdk build --out /tmp/gowdk-wasm-island examples/components/wasm/*.gwdk` |
| Full-stack vertical slice | `examples/flagship/` | `cd examples/flagship && make check && make routes && make build` |

## Full Example Check

Validate the broad source set with SSR enabled:

```sh
go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/marketing/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk examples/go-interop/*.gwdk examples/components/base/*.gwdk examples/components/css/*.gwdk examples/components/assets/*.gwdk examples/components/wasm/*.gwdk examples/store-persist/*.gwdk examples/embed/*.gwdk examples/seo/*.gwdk examples/css/*.gwdk examples/tailwind/*.gwdk examples/contracts/*.gwdk examples/security/*.gwdk
```

Focused directories such as `examples/endpoints`, `examples/auth-guard`, and
`examples/flagship` include `Makefile` targets that run the local checks used by
CI.

## Documentation Map

| Need | Source |
| --- | --- |
| Recipes | [Native Cookbook](../docs/cookbook/README.md) |
| Learning path | [Native Learning Path](../docs/learning/native.md) |
| Commands and config | [Reference Index](../docs/reference/README.md) |
| Language syntax | [Language Index](../docs/language/README.md) |
| Generated output | [Compiler Index](../docs/compiler/README.md) |
| Current capability status | [Product Requirements](../docs/product/requirements.md) |

When an example changes, update its command here, the cookbook recipe if one
exists, and the contract doc that owns the behavior.
