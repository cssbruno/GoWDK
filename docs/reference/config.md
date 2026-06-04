# Config Reference

The root package exposes config types used by compiler and future generated app behavior.

```go
type Config struct {
	AppName string
	Source  SourceConfig
	Modules []ModuleConfig
	Render  RenderConfig
	Build   BuildConfig
	CSS     CSSConfig
	Addons  []Addon
}
```

## Source

`SourceConfig` has include and exclude patterns. Discovery support exists in
`internal/discover`, and `gowdk build` reads literal `Source.Include` and
`Source.Exclude` fields from `gowdk.config.go` when no explicit files are
supplied.

`Modules` declares named source groups. Build discovery treats modules as
source selectors. Generated app and binary composition is controlled by the
modules selected for a specific build command.

Supported initial config subset:

```go
package app

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
		Exclude: []string{"src/**/draft.page.gwdk"},
	},
	Modules: []gowdk.ModuleConfig{
		{Name: "frontend", Type: "frontend"},
		{
			Name: "frontend2",
			Type: "marketing-ui",
			Source: gowdk.SourceConfig{
				Include: []string{"ui2/**/*.gwdk"},
			},
		},
		{
			Name: "backendmicroservice",
			Type: "backendmicroservice",
			Source: gowdk.SourceConfig{
				Include: []string{"services/backend/**/*.gwdk"},
				Exclude: []string{"services/backend/**/draft.page.gwdk"},
			},
		},
	},
	Build: gowdk.BuildConfig{
		Output: "dist/site",
		Stylesheets: []gowdk.Stylesheet{
			{Href: "/assets/app.css"},
		},
		Targets: []gowdk.BuildTargetConfig{
			{
				Name: "admin",
				Modules: []string{"admin"},
				Output: "dist/admin",
				App: ".gowdk/admin",
				Binary: "bin/admin",
				WASM: "bin/admin.wasm",
			},
			{
				Name: "public-admin",
				Modules: []string{"public", "admin"},
				Output: "dist/public-admin",
				App: ".gowdk/public-admin",
				Binary: "bin/public-admin",
			},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
		Default: []string{"global", "tokens"},
	},
}
```

The CLI parses this file statically and does not execute user Go code. Non-literal
values are ignored in the current subset.

## Modules

`ModuleConfig` names a logical source group:

```go
type ModuleConfig struct {
	Name   string
	Type   string
	Source SourceConfig
}
```

`Type` is user-defined metadata in the current build slice. GOWDK does not
validate or reserve type values, so projects can use values such as `frontend`,
`frontend2`, `backend`, `backendmicroservice`, `worker`, or any local module
role. Deployment code remains user-owned; GOWDK does not infer Kubernetes or
deployment settings from module type.

When `gowdk build` discovers files and a module has a name but no
`Source.Include`, it uses `<module-name>/**/*.gwdk`. For example,
`{Name: "frontend"}` discovers `frontend/**/*.gwdk`. Explicit module include
patterns override that default for the module. Root `Source.Exclude` and module
`Source.Exclude` patterns are both honored. `gowdk build --module <name>` limits
discovery to selected configured modules.

The selected modules define what gets compiled into the build output and, when
`--app` or `--bin` is used, what is copied into the generated app and embedded
in the generated binary. Ad hoc CLI flags can still package modules directly:

```sh
# Single-module binary.
gowdk build --module admin --out dist/admin --app .gowdk/admin --bin bin/admin

# Multi-module binary.
gowdk build --module public --module admin --out dist/app --app .gowdk/app --bin bin/app

# Separate binaries with different module sets.
gowdk build --module public --out dist/public --app .gowdk/public --bin bin/public
gowdk build --module admin,api --out dist/admin-api --app .gowdk/admin-api --bin bin/admin-api
```

Use distinct `--out` and `--app` directories for separate binaries so stale
artifacts from another module selection cannot be copied into the next binary.

## Render

`RenderConfig.Default` controls the default render mode. When omitted, default mode is `static`.

## Build

`BuildConfig.Output`, `BuildConfig.Assets`, `BuildConfig.Stylesheets`, and
`BuildConfig.Targets` are target build settings. Current `gowdk build` reads
literal `Build.Output`, `Build.Stylesheets`, and `Build.Targets` from
`gowdk.config.go`; `--out` overrides `Build.Output` for ad hoc builds.
`BuildConfig.Assets` remains planned.

`Build.Targets` declares repeatable module-to-output packaging:

```go
type BuildConfig struct {
	Output      string
	Assets      gowdk.AssetMode
	Stylesheets []gowdk.Stylesheet
	Targets     []gowdk.BuildTargetConfig
}

type BuildTargetConfig struct {
	Name    string
	Modules []string
	Output  string
	App     string
	Binary  string
	WASM    string
}
```

`Name` and `Output` are required. `Modules` selects configured modules; omit it
to use the default configured discovery set. `App` is optional and writes a
generated Go app that embeds the target output. `Binary` is optional, requires
`App`, and compiles that generated app for the local platform. `WASM` is
optional, requires `App`, and compiles the generated app with
`GOOS=js GOARCH=wasm`.

When `Build.Targets` is present, `gowdk build` runs every configured target
unless ad hoc build flags or explicit files are passed. Use `gowdk build
--target <name>` to run one or more named targets; `--target` may be repeated or
comma-separated.

## CSS

`CSSConfig` controls discovered CSS inputs and generated page CSS output:

```go
type CSSConfig struct {
	Include []string
	Exclude []string
	Default []string
	Output  CSSOutputConfig
}

type CSSOutputConfig struct {
	Dir        string
	HrefPrefix string
}
```

When omitted, CSS discovery scans `**/*.css`, excludes `.git`, `vendor`,
`node_modules`, and the selected build output directory, and uses `global.css`
as the default CSS input when present.

`CSS.Default` names discovered CSS inputs used by the `default` built-in in
`@css`. Generated page CSS defaults to `assets/gowdk/<page-id>.css` and hrefs
under `/assets/gowdk/`.

## Addons

`Addons` registers optional features such as static, actions, partial, SSR, API,
embed, CSS, and rate limiting. Current validation uses SSR feature registration
for render-mode checks, and static builds invoke addons that implement
`gowdk.CSSProcessor`.

The static config loader recognizes the known literal Tailwind addon subset:

```go
import "github.com/cssbruno/gowdk/addons/tailwind"

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		tailwind.Addon(tailwind.Options{
			Input:   "styles/app.css",
			Command: ".gowdk/bin/tailwindcss",
			Minify:  true,
		}),
	},
}
```

Arbitrary addon constructors remain outside the current static config subset.
Rate limiting can still be configured directly in user-owned server code through
`addons/ratelimit` middleware with either the in-memory store or a Redis store
adapter.
