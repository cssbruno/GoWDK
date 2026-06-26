# Getting Started

Install the GOWDK CLI, scaffold a small app, build its generated binary, and run
it locally.

## Prerequisites

- Go `1.26.4` on `PATH`.
- `curl` or `wget` for release installs.

## Install The CLI

Install through the Go checksum database:

```sh
go install github.com/cssbruno/gowdk/cmd/gowdk@latest
gowdk version
```

Use an exact published tag instead of `latest` when you need a reproducible
install.

Convenience install for supported Linux and macOS targets:

```sh
curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh | sh
gowdk version
```

The convenience installer is fetched from the mutable `main` branch before it
runs. It verifies the downloaded release binary against `checksums.txt`, but the
bootstrap script itself is not authenticated before execution.

## Add `gowdk` To Your Shell

If you install into `$HOME/.local/bin`, make sure that directory is on `PATH`:

```sh
mkdir -p "$HOME/.local/bin"
printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$HOME/.profile"
. "$HOME/.profile"
gowdk version
```

PowerShell user-path setup:

```powershell
$installDir = "$HOME\.local\bin"
New-Item -ItemType Directory -Force -Path $installDir
[Environment]::SetEnvironmentVariable("Path", "$installDir;$([Environment]::GetEnvironmentVariable('Path', 'User'))", "User")
$env:Path = "$installDir;$env:Path"
gowdk version
```

## Manual Artifact Verification

Use the artifact names from the GitHub release, then verify the checksum:

```sh
version=<version>
asset=gowdk-linux-amd64
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/${version}/${asset}"
curl -fsSLO "https://github.com/cssbruno/GoWDK/releases/download/${version}/checksums.txt"
grep " ${asset}$" checksums.txt | sha256sum -c -
install -m 0755 "$asset" "$HOME/.local/bin/gowdk"
gowdk version
```

GitHub attestation verification:

```sh
gh attestation verify ./gowdk-linux-amd64 -R cssbruno/GOWDK
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

## Create An App

```sh
gowdk init --tests --template site /tmp/gowdk-my-app
cd /tmp/gowdk-my-app
gowdk build
gowdk test
./bin/site
```

Open `http://127.0.0.1:8080/`.

The `site` template writes `gowdk.config.go`, page and component source under
`src/`, CSS under `styles/`, generated output under `.gowdk/`, and a binary under
`bin/site`. Existing files are not overwritten unless `--force` is passed.

## Development Loop

```sh
gowdk dev
```

`dev` rebuilds changed inputs, serves the last successful build after failures,
and injects browser live reload. Use generated-app mode for local backend,
action, API, partial, SSR, or hybrid flows:

```sh
gowdk dev --app .gowdk/dev-app
```

Use `preview` for a one-shot local deploy preview:

```sh
gowdk preview
```

## Build Repository Examples

From the GOWDK repository root:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-build \
  examples/pages/home.page.gwdk \
  examples/pages/hero.cmp.gwdk

go run ./cmd/gowdk serve --dir /tmp/gowdk-build
```

For the full lesson path, use [Native Learning Path](learning/native.md). For
recipes, use [Native Cookbook](cookbook/README.md). For commands and contracts,
use [Reference](reference/README.md).

## Troubleshooting

| Problem | Fix |
| --- | --- |
| Missing `gowdk.config.go` | Run from an initialized GOWDK app, run `gowdk init`, or pass `--config <file>`. |
| Missing Tailwind binary | Install Tailwind through your own toolchain and configure `tailwind.Options.Command`, or remove the Tailwind addon. |
| Unsupported Go handler signature | Check [actions](language/actions.md), [APIs](language/api.md), and [Go interop](reference/go-interop.md). |
| Missing SSR feature | Add the SSR addon in config or remove request-time behavior such as `server {}`. |
| Generated binary build failure | Rerun `gowdk build` from the app root and inspect generated app errors under `.gowdk/`. |

## Current Reality

GOWDK is experimental pre-1.0 software. Use
[Product Requirements](product/requirements.md) for the current implemented,
partial, experimental, planned, and out-of-scope capability matrix.
