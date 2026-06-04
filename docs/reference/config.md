# Config Reference

The root package exposes config types used by compiler and future generated app behavior.

```go
type Config struct {
	AppName string
	Source  SourceConfig
	Modules []ModuleConfig
	Render  RenderConfig
	Build   BuildConfig
	Addons  []Addon
}
```

## Source

`SourceConfig` has include and exclude patterns. Discovery support exists in
`internal/discover`, and `gowdk build` reads literal `Source.Include` and
`Source.Exclude` fields from `gowdk.config.go` when no explicit files are
supplied.

`Modules` declares named source groups. Current build discovery treats modules
as additive source groups, not separate deployment artifacts.

Supported initial config subset:

```go
package app

import "github.com/gowdk/gowdk"

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

## Render

`RenderConfig.Default` controls the default render mode. When omitted, default mode is `static`.

## Build

`BuildConfig.Output`, `BuildConfig.Assets`, and `BuildConfig.Stylesheets` are
target build settings. Current `gowdk build` reads literal `Build.Output` and
`Build.Stylesheets` from `gowdk.config.go`; `--out` overrides `Build.Output`.
`BuildConfig.Assets` remains planned.

## Addons

`Addons` registers optional features such as static, actions, partial, SSR, API,
embed, and CSS. Current validation uses SSR feature registration for render-mode
checks, and static builds invoke addons that implement `gowdk.CSSProcessor`.
