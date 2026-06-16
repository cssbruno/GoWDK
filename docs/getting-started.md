# Getting Started

GOWDK can be installed from the latest GitHub release or built from source. The
fastest app path is install the CLI, scaffold a small app, build the generated
app binary, and run it locally.

## Prerequisites

- Go `1.26.4` installed and available on `PATH`.
- `curl` or `wget` for release installs.

## Install The CLI

Install the latest visible GitHub release:

```sh
curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh | sh
gowdk version
```

Pin the current CLI release or install into a user-writable directory:

```sh
GOWDK_VERSION=v0.5.0 GOWDK_INSTALL_DIR="$HOME/.local/bin" \
  sh -c "$(curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh)"
```

The installer resolves `latest` from published GitHub releases, downloads the
matching binary for the current OS/architecture, verifies it against the
published `checksums.txt`, and fails before binary download if that release
does not publish the matching artifact.

## Add `gowdk` To Your Shell

If you install into `$HOME/.local/bin`, make sure that directory is on `PATH`.

For zsh:

```sh
mkdir -p "$HOME/.local/bin"
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$HOME/.zshrc"
exec zsh
gowdk version
```

For bash:

```sh
mkdir -p "$HOME/.local/bin"
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$HOME/.bashrc"
exec bash
gowdk version
```

For POSIX login shells:

```sh
mkdir -p "$HOME/.local/bin"
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$HOME/.profile"
. "$HOME/.profile"
gowdk version
```

For fish:

```fish
mkdir -p "$HOME/.local/bin"
fish_add_path "$HOME/.local/bin"
gowdk version
```

For PowerShell:

```powershell
$installDir = "$HOME\.local\bin"
New-Item -ItemType Directory -Force -Path $installDir
[Environment]::SetEnvironmentVariable(
  "Path",
  "$installDir;$([Environment]::GetEnvironmentVariable('Path', 'User'))",
  "User"
)
$env:Path = "$installDir;$env:Path"
gowdk version
```

Direct artifact names:

| Platform | Artifact |
| --- | --- |
| Linux amd64 | `gowdk-linux-amd64` |
| Linux arm64 | `gowdk-linux-arm64` |
| macOS Intel | `gowdk-darwin-amd64` |
| macOS ARM | `gowdk-darwin-arm64` |
| Windows amd64 | `gowdk-windows-amd64.exe` |

Manual Linux install:

```sh
version=v0.5.0
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/$version/gowdk-linux-amd64"
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/$version/checksums.txt"
grep ' gowdk-linux-amd64$' checksums.txt | sha256sum -c -
chmod 0755 gowdk-linux-amd64
mkdir -p "$HOME/.local/bin"
mv gowdk-linux-amd64 "$HOME/.local/bin/gowdk"
gowdk version
```

Manual macOS Intel install:

```sh
version=v0.5.0
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/$version/gowdk-darwin-amd64"
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/$version/checksums.txt"
expected="$(awk '$2 == "gowdk-darwin-amd64" { print $1 }' checksums.txt)"
actual="$(shasum -a 256 gowdk-darwin-amd64 | awk '{ print $1 }')"
test "$expected" = "$actual"
chmod 0755 gowdk-darwin-amd64
mkdir -p "$HOME/.local/bin"
mv gowdk-darwin-amd64 "$HOME/.local/bin/gowdk"
gowdk version
```

Manual macOS ARM install:

```sh
version=v0.5.0
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/$version/gowdk-darwin-arm64"
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/$version/checksums.txt"
expected="$(awk '$2 == "gowdk-darwin-arm64" { print $1 }' checksums.txt)"
actual="$(shasum -a 256 gowdk-darwin-arm64 | awk '{ print $1 }')"
test "$expected" = "$actual"
chmod 0755 gowdk-darwin-arm64
mkdir -p "$HOME/.local/bin"
mv gowdk-darwin-arm64 "$HOME/.local/bin/gowdk"
gowdk version
```

Manual Windows install from PowerShell:

```powershell
$version = "v0.5.0"
Invoke-WebRequest "https://github.com/cssbruno/GoWDK/releases/download/$version/gowdk-windows-amd64.exe" -OutFile "gowdk.exe"
Invoke-WebRequest "https://github.com/cssbruno/GoWDK/releases/download/$version/checksums.txt" -OutFile "checksums.txt"
$expected = (Select-String -Path checksums.txt -Pattern "gowdk-windows-amd64.exe").Line.Split(" ")[0]
$actual = (Get-FileHash .\gowdk.exe -Algorithm SHA256).Hash.ToLower()
if ($expected -ne $actual) { throw "checksum mismatch" }
.\gowdk.exe version
```

Verify a downloaded artifact with GitHub attestations:

```sh
gh attestation verify ./gowdk-linux-amd64 -R cssbruno/GOWDK
```

Install the VS Code extension package from a release when a `.vsix` is
published:

```sh
code --install-extension gowdk-vscode-0.5.0.vsix
```

## Build From Source

```sh
git clone https://github.com/cssbruno/GOWDK.git
cd GOWDK
go build ./cmd/gowdk
./gowdk version
```

During repository development, you can also run the CLI without installing it:

```sh
go run ./cmd/gowdk version
```

Use the built binary when running commands from outside this repository.

For focused recipes after the first app, use the
[Native Cookbook](cookbook/README.md). For command and data contracts, use the
[Reference Index](reference/README.md).

## Create An App

With `gowdk` on `PATH`:

```sh
gowdk init --tests --template site /tmp/gowdk-my-app
cd /tmp/gowdk-my-app
```

`init --template site` writes a starter `gowdk.config.go`, one page, one
component, and one CSS file. `init --template minimal` writes a smaller
page/CSS starter. `init --tests` adds `tests/gowdk_smoke_test.go`, which skips
unless `GOWDK_BIN` points at a built `gowdk` CLI. Existing files are not
overwritten unless `--force` is passed.

The generated config discovers `src/**/*.gwdk`, discovers CSS from
`styles/**/*.css`, declares a `site` build target, generates app source in
`.gowdk/site`, compiles `bin/site`, and ignores generated outputs in the
scaffolded `.gitignore`. The target's intermediate build output is inferred as
`.gowdk/output/site`.

## Build

From the app directory:

```sh
gowdk build
```

Run the optional scaffolded smoke test:

```sh
GOWDK_BIN="$(command -v gowdk)" go test ./tests
```

The build writes app-shell HTML and manifests under `.gowdk/output/site`, then
embeds that output into `bin/site`:

```text
.gowdk/output/site/
  index.html
  gowdk-routes.json
  gowdk-assets.json
  gowdk-build-report.json
.gowdk/site/
bin/site
```

Every successful disk build writes `gowdk-build-report.json`.

## Run

```sh
./bin/site
```

Open `http://127.0.0.1:8080/`.

The generated binary serves embedded frontend output and supported request-time
handlers. For static-only inspection, `gowdk serve --dir .gowdk/output/site`
still serves the generated directory, but it does not run generated actions, API
handlers, partial fragments, or SSR routes.

## Development Loop

Use `dev` for polling rebuilds, local serving, and browser reload:

```sh
gowdk dev
```

`dev` builds into `gowdk_cache` by default, serves that directory, polls source
inputs for content changes, rebuilds on changes, and injects browser live reload
into served HTML. It keeps serving the last successful build after a failed
rebuild. Pass `--out <dir>` to use a different dev output directory.

When you pass `--app <dir>`, `dev` builds the generated app, compiles a local
dev binary, runs it on `GOWDK_ADDR`, and restarts that process after successful
rebuilds. Use this path for local backend, action, API, partial, and SSR flows.

Use `preview` for a one-shot local deploy preview:

```sh
gowdk preview
```

Add `--hot` to run the same preview output through the dev rebuild loop.

## Build Repository Examples

From the GOWDK repository root:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk

go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

The repository root includes `gowdk.config.go` so these example commands have
the same required project config shape as a scaffolded app. Outside this
repository, run `gowdk init` first or pass `--config <file>`.

For a lesson-by-lesson path through pages, build-time Go data, components,
CSS/assets, actions, validation, CSRF, APIs, fragments, SSR, guards,
database-owned Go code, one-binary deploys, Caddy, diagnostics, tests,
Tailwind, and WASM islands, use the [Native Learning Path](learning/native.md).
For website-style onboarding and hosted execution constraints, use the
[Playground Onboarding and Sandboxing](product/playground.md) contract. The
local bridge commands are:

```sh
gowdk playground policy
gowdk playground export --dir . --out /tmp/gowdk-project.zip
```

Dynamic SPA routes work when literal `paths {}` entries are present:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build \
  examples/pages/blog-post.page.gwdk
```

This writes `/tmp/gowdk-dynamic-build/blog/hello-gowdk/index.html` and
`/tmp/gowdk-dynamic-build/blog/compile-first/index.html`.

## Current Reality

Implemented today:

- Build output for simple `.gwdk` pages and components.
- Literal `paths {}` expansion for dynamic SPA routes.
- Literal `build {}` data and imported no-argument Go build data functions.
- Config-based discovery, module selection, and named build targets.
- Generated embedded app source, local binaries, and Go `js/wasm` deploy
  artifacts.
- Component-level browser-side Go/WASM island packages with ABI export validation.
- Feature-bound action/API handlers, action redirects, partial action
  fragments, standalone fragment routes, CSRF-wired actions, guards, and
  concrete or dynamic SSR pages with declared `server {}` identifier or dotted
  paths, plus concrete or dynamic hybrid request-time pages with or without
  declared `server {}` data, in generated binaries.
- CLI tooling for tokens, formatting, validation, manifest, sitemap, routes,
  compiler IR inspection, dev, serve, and LSP.

Planned or partial:

- User-defined domain validation helpers beyond generated request-shape checks.
- Hybrid streaming, data refresh, and non-HTTP revalidation.
- Richer generated-client reactivity beyond explicit reload/fragment outcomes.

Troubleshooting:

- Missing `gowdk.config.go`: run commands from an initialized GOWDK app, run
  `gowdk init`, or pass `--config <file>`.
- Missing Tailwind binary: install Tailwind through your own approved toolchain
  and configure `tailwind.Options.Command`, or remove the Tailwind addon.
- Unsupported Go handler signature: check the action/API docs and use a
  supported exported function signature.
- Missing SSR feature: add the SSR addon in config or remove request-time page
  behavior such as `server {}`.
- Generated binary build failure: rerun `gowdk build` from the app root and
  inspect generated app errors under `.gowdk/`.
