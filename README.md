<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

[![CI](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8)

![Build](https://img.shields.io/badge/build-succeeds-2ea44f)
![Quality](https://img.shields.io/badge/quality-gated-2ea44f)

GOWDK is a portable Go web compiler. Write movable `.gwdk` files, compile first,
and ship static output or a single Go binary. SSR is optional, not the default.

Live demo: [gowdk.com](https://gowdk.com/)
Demo source: [cssbruno/gowdk-page](https://github.com/cssbruno/gowdk-page)

## Status

- Static render is the default.
- Pages declare routes inside files.
- Dynamic static routes use `paths {}`.
- `build {}` runs at build time.
- `act {}` handles backend actions without full-page SSR.
- Partial updates use server fragments.
- `load {}` and full-page request rendering require SSR.
- One-binary static serving is available.
- Early project: useful compiler/runtime foundation, not a production framework.

## Install Loop

During source development, run the CLI from this repository:

```sh
go run ./cmd/gowdk <command>
```

Create and serve an app:

```sh
go run ./cmd/gowdk init my-app
cd my-app
go run ../cmd/gowdk build
go run ../cmd/gowdk serve --dir dist/site
```

Build an example:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build \
  examples/basic/home.page.gwdk \
  examples/basic/hero.cmp.gwdk

go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

Use `dev` for rebuild, serve, watch, and browser reload:

```sh
go run ./cmd/gowdk dev --out /tmp/gowdk-build examples/basic/*.gwdk
```

## Site Example

```gwdk
@page blog.post
@route "/blog/{slug}"
@layout root, blog
@render static

paths {
  => { slug: "hello-gowdk" }
  => { slug: "compile-first" }
}

build {
  => {
    title: "GOWDK ships apps",
    description: "Portable pages, build-time data, actions, and fragments."
  }
}

act refresh {
  input := form ArticleFilter
  fragment "#article-list" {
    <section>
      <h2>Updated articles</h2>
      <p>Server-rendered fragment returned by an action.</p>
    </section>
  }
  -> "/blog/{slug}"
}

view {
  <main class="page">
    <header class="hero">
      <p>Compile-first Go UI</p>
      <h1>{title}</h1>
      <p data-slug="{slug}">{description}</p>
    </header>

    <form g:post={refresh} g:target="#article-list" g:swap="innerHTML">
      <input name="query" placeholder="Filter articles" />
      <button>Refresh</button>
    </form>

    <section id="article-list">
      <article>
        <h2>{slug}</h2>
        <p>Generated as static HTML at build time.</p>
      </article>
    </section>
  </main>
}
```

## CLI

```sh
gowdk init [--force] [dir]
gowdk check [--config <file>] [--module <name>] [--json] [--ssr] [files...]
gowdk build [--config <file>] [--debug] [--ssr] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [files...]
gowdk dev [--addr 127.0.0.1:8080] [--interval 1s] [build flags...]
gowdk watch [--once] [--restart] [--interval 1s] [build flags...]
gowdk serve --dir <dir> [--addr 127.0.0.1:8080]
gowdk fmt [--write] <file.gwdk>
gowdk tokens <file.gwdk>
gowdk manifest [--config <file>] [--module <name>] [--ssr] [files...]
gowdk sitemap [--config <file>] [--module <name>] [--ssr] [files...]
gowdk routes [--config <file>] [--module <name>] [--ssr] [files...]
gowdk lsp [--ssr]
```

Every successful disk build writes `gowdk-build-report.json`. Pass `--debug` to
mirror the structured report to stderr.

## Build Targets

`gowdk.config.go` can declare named outputs:

```go
Build: gowdk.BuildConfig{
	Targets: []gowdk.BuildTargetConfig{
		{Name: "admin", Modules: []string{"admin"}, Output: "dist/admin", App: ".gowdk/admin", Binary: "bin/admin"},
		{Name: "site", Modules: []string{"public"}, Output: "dist/site", App: ".gowdk/site", Binary: "bin/site"},
	},
}
```

Run all targets:

```sh
gowdk build
```

Run one target:

```sh
gowdk build --target admin
```

## Quality Gates

```sh
gofmt -w <changed-go-files>
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
```

CI also smoke-builds static, dynamic, CSS, and embedded-binary examples.

## Docs

- [Product vision](docs/product/vision.md)
- [Requirements](docs/product/requirements.md)
- [Roadmap](docs/product/roadmap.md)
- [Architecture](docs/engineering/architecture.md)
- [CLI reference](docs/reference/cli.md)
- [Language notes](docs/language/README.md)
- [Examples](examples/README.md)
- [VS Code extension](https://marketplace.visualstudio.com/items?itemName=GoWDK.gowdk-vscode)
