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

`gowdk.config.go` is required for CLI commands that compile, validate, inspect,
or serve a development loop for `.gwdk` code: `check`, `manifest`, `sitemap`,
`routes`, `build`, and `dev`. Those commands load `gowdk.config.go` from the
current directory by default, or the file passed with `--config <file>`.

`SourceConfig` has include and exclude patterns. Discovery support exists in
`internal/discover`, and `gowdk build` reads literal `Source.Include` and
`Source.Exclude` fields from the loaded config when no explicit files are
supplied. Explicit file paths still require a loaded config.

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
		Mode: gowdk.Production,
		Head: gowdk.HeadConfig{
			SiteName: "Example",
			Favicon: "/favicon.ico",
			Image: "https://example.com/social.png",
			TwitterCard: "summary",
		},
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

The CLI parses this file as a literal config subset and does not execute user Go code. Non-literal
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

`RenderConfig.Default` controls the default render mode. When omitted, default mode is `spa`.

## Build

`BuildConfig.Output`, `BuildConfig.Mode`, `BuildConfig.Assets`,
`BuildConfig.Head`, `BuildConfig.CSRF`, `BuildConfig.AllowMissingBackend`, `BuildConfig.Stylesheets`, and
`BuildConfig.Targets` are target build settings. Current `gowdk build` reads
literal `Build.Output`, `Build.Mode`, `Build.Head`, `Build.CSRF`,
`Build.AllowMissingBackend`, `Build.Stylesheets`, and `Build.Targets` from
`gowdk.config.go`; `--out` overrides `Build.Output` for ad hoc builds.
`BuildConfig.Assets` remains planned.

`Build.Targets` declares repeatable module-to-output packaging:

```go
type BuildConfig struct {
	Output              string
	Mode                gowdk.BuildMode
	Assets              gowdk.AssetMode
	Head                gowdk.HeadConfig
	CSRF                gowdk.CSRFConfig
	AllowMissingBackend bool
	Stylesheets         []gowdk.Stylesheet
	Targets             []gowdk.BuildTargetConfig
}

type HeadConfig struct {
	SiteName    string
	Favicon     string
	Image       string
	TwitterCard string
}

type CSRFConfig struct {
	Enabled    bool
	SecretEnv  string
	CookieName string
	FieldName  string
	HeaderName string
	Insecure   bool
}

type BuildTargetConfig struct {
	Name          string
	Modules       []string
	Output        string
	App           string
	Binary        string
	WASM          string
	BackendApp    string
	BackendBinary string
}
```

`Mode` controls development metadata in generated frontend artifacts. The
default omitted mode behaves like `gowdk.Development` and emits JavaScript
island source maps. Set `Mode: gowdk.Production` to omit `.js.map` artifacts and
`sourceMappingURL` comments and to compact generated island JavaScript by
trimming formatting-only whitespace.

Production mode also requires explicitly declared `act` and `api` endpoints to
bind to supported same-package Go handlers. Missing or unsupported handlers fail
the build by default. Set `AllowMissingBackend: true` or pass
`--allow-missing-backend` when intentionally generating HTTP 501 stubs during a
migration.

`Head` controls app-level document head tags. `Favicon` emits
`<link rel="icon">`. `SiteName`, `Image`, and `TwitterCard` enable generated
Open Graph and Twitter metadata. A page-level `@image` overrides `Head.Image`
for that page.

`CSRF` controls generated action CSRF wiring. When `Enabled` is true, generated
apps require a signing secret from `SecretEnv` or `GOWDK_CSRF_SECRET`, inject a
hidden token field into served HTML POST forms, and validate action POSTs before
generated decoding or user handlers run. Invalid or missing tokens return HTTP
403 with `invalid csrf token` and `Cache-Control: no-store`. `CookieName`,
`FieldName`, and `HeaderName` override the generated token transport names.
`Insecure` disables the Secure cookie flag for local HTTP development only.

`Name` and `Output` are required. `Modules` selects configured modules; omit it
to use the default configured discovery set. `App` is optional and writes a
generated Go app that embeds the target output. `Binary` is optional, requires
`App`, and compiles that generated app for the local platform. `WASM` is
optional, requires `App`, and compiles the generated app with
`GOOS=js GOARCH=wasm`.

`BackendApp` is optional and writes a generated backend-only Go app for
feature-bound action/API endpoints. `BackendBinary` is optional, requires
`BackendApp`, and compiles that backend app. When a target has both frontend
`App`/`Binary` and `BackendApp`/`BackendBinary`, the frontend binary proxies
generated backend routes to `GOWDK_BACKEND_ORIGIN`.

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

`Addons` registers optional features such as spa, actions, partial, SSR, API,
embed, CSS, and rate limiting. Current validation uses SSR feature registration
for render-mode checks, and SPA builds invoke addons that implement
`gowdk.CSSProcessor`.

The literal config loader recognizes the known literal Tailwind addon subset:

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

Arbitrary addon constructors remain outside the current config subset.
Rate limiting can still be configured directly in user-owned server code through
`addons/ratelimit` middleware with either the in-memory store or a Redis store
adapter.
