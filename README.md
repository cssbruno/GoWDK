<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

[![CI](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8)

![Build](https://img.shields.io/badge/build-succeeds-2ea44f)
![Quality](https://img.shields.io/badge/quality-gated-2ea44f)

GOWDK is an experimental new way to build Go web applications. It is a compiler
that takes portable `.gwdk` pages and components, then turns them into
build-time web output, backend routes, and deployable Go binaries.

Current status: GOWDK is pre-release and still being shaped. The compiler can
already produce early build output, embedded app builds, first-slice
actions/fragments, generated JavaScript islands, and simple SSR pages, but the
runtime model and language surface are still evolving. If you are interested in
where this is going, feedback, ideas, experiments, and improvements are welcome.

Live demo: [gowdk.com](https://gowdk.com/)
Demo source: [cssbruno/gowdk-page](https://github.com/cssbruno/gowdk-page)


## Getting Started

During source development, run the CLI from this repository:

```sh
go run ./cmd/gowdk <command>
```

Build the CLI:

```sh
go build ./cmd/gowdk
./gowdk version
```

Create, build, and serve an app:

```sh
go run ./cmd/gowdk init my-app
cd my-app
../gowdk build
../gowdk serve --dir dist/site
```

Build an example:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk

go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

Project compiler commands require `gowdk.config.go` in the current directory,
or `--config <file>`, even when explicit `.gwdk` files are passed. This source
repository includes a root `gowdk.config.go` for example smoke commands.

Use `dev` for polling rebuilds, local serving, and browser reload:

```sh
go run ./cmd/gowdk dev --out /tmp/gowdk-build \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk
```

## Site Example

```gwdk
package blog

@page blog.post
@route "/blog/{slug}"
@layout root, blog
@render spa

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

act Refresh POST "/blog/{slug}"

view {
  <main class="page">
    <header class="hero">
      <p>Compile-first Go UI</p>
      <h1>{title}</h1>
      <p data-slug="{slug}">{description}</p>
    </header>

    <form g:post={Refresh} g:target="#article-list" g:swap="innerHTML">
      <input name="query" placeholder="Filter articles" />
      <button>Refresh</button>
    </form>

    <section id="article-list">
      <article>
        <h2>{slug}</h2>
        <p>Generated as spa HTML at build time.</p>
      </article>
    </section>
  </main>
}
```

`Refresh` is a normal exported Go function in package `blog`; generated code
wires the form POST and writes the returned `runtime/response.Response`.

## CLI

```sh
gowdk init [--force] [dir]
gowdk check [--config <file>] [--module <name>] [--json] [--ssr] [files...]
gowdk build [--config <file>] [--debug] [--ssr] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [files...]
gowdk dev [--addr 127.0.0.1:8080] [--interval 1s] [build flags...]
gowdk preview [--addr 127.0.0.1:8080] [--hot] [build flags...]
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

`gowdk dev --app <dir>` generates the app, compiles a dev binary, and restarts
that generated process after successful rebuilds. `gowdk preview` builds once
into a temporary deploy-preview output and serves it; `gowdk preview --hot`
uses the dev loop for local hot preview.

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
go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk
```

CI also smoke-builds spa, dynamic, CSS, and embedded-binary examples.

## Docs

- [Getting started](docs/getting-started.md)
- [Product vision](docs/product/vision.md)
- [Requirements](docs/product/requirements.md)
- [Roadmap](docs/product/roadmap.md)
- [Documentation checklist](docs/product/documentation-checklist.md)
- [Architecture](docs/engineering/architecture.md)
- [Release readiness](docs/engineering/release.md)
- [CLI reference](docs/reference/cli.md)
- [Config reference](docs/reference/config.md)
- [Routing reference](docs/reference/routing.md)
- [Deployment reference](docs/reference/deployment.md)
- [Language notes](docs/language/README.md)
- [Browser compiler](docs/compiler/browser-compiler.md)
- [Examples](examples/README.md)
- [VS Code extension](https://marketplace.visualstudio.com/items?itemName=GoWDK.gowdk-vscode)
