# Config Reference

The root package exposes config types used by compiler and future generated app behavior.

```go
type Config struct {
	AppName   string
	Source    SourceConfig
	Modules   []ModuleConfig
	Render    RenderConfig
	Env       EnvConfig
	Lifecycle LifecycleConfig
	Build     BuildConfig
	CSS       CSSConfig
	I18N      I18NConfig
	Addons    []Addon
}
```

## Source

`gowdk.config.go` is required for CLI commands that compile, validate, inspect,
or serve a development loop for `.gwdk` code: `check`, `manifest`, `sitemap`,
`routes`, `inspect ir`, `build`, and `dev`. Those commands load
`gowdk.config.go` from the current directory by default, or the file passed
with `--config <file>`.

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
		CORS: gowdk.CORSConfig{
			Enabled: true,
			AllowedOrigins: []string{"https://app.example"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type", "X-CSRF"},
			ExposedHeaders: []string{"X-Total-Count"},
			AllowCredentials: true,
			MaxAgeSeconds: 600,
		},
		Stylesheets: []gowdk.Stylesheet{
			{Href: "/assets/app.css"},
		},
		Scripts: []gowdk.Script{
			{Src: "/assets/app.js", Type: "module"},
		},
		Targets: []gowdk.BuildTargetConfig{
			{
				Name: "admin",
				Modules: []string{"admin"},
				Output: "dist/admin",
				App: ".gowdk/admin",
				Binary: "bin/admin",
				WASM: "bin/admin.wasm",
				DeployRecipes: []string{"systemd"},
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
	Env: gowdk.EnvConfig{
		Vars: []gowdk.EnvVar{
			{Name: "GOWDK_BACKEND_ORIGIN", Required: true},
			{Name: "GOWDK_ADDR", Default: "127.0.0.1:8080"},
		},
		Secrets: []gowdk.SecretEnv{
			{Name: "DATABASE_URL", Required: true},
			{Name: "GOWDK_CSRF_SECRET", Required: true},
		},
	},
	Lifecycle: gowdk.LifecycleConfig{
		Services: []gowdk.ServiceRef{
			{ImportPath: "example.com/site/services", Function: "Services"},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
		Default: []string{"global", "tokens"},
	},
	I18N: gowdk.I18NConfig{
		DefaultLocale: "en",
		Locales: []gowdk.LocaleConfig{
			{Code: "en", Name: "English"},
			{Code: "pt-BR", PathPrefix: "/br", Name: "Brazilian Portuguese"},
		},
	},
}
```

The CLI first parses this file as a literal config subset. Unknown top-level
`gowdk.Config` fields are rejected instead of silently ignored. When a supported
field contains non-literal Go that the subset cannot reduce safely, the loader
falls back to the executable config bridge described below.

Addon constructors outside the built-in AST subset are loaded through a small
Go helper that imports the config package. That means addon packages are normal
Go modules:

```go
import brand "github.com/example/gowdk-brand"

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		brand.Addon(),
	},
}
```

The addon module may import other GitHub/private/local modules. The project
`go.mod` remains the source of truth for resolving those imports, including
`require`, `replace`, `GOPRIVATE`, and module proxy configuration.

## SEO Addon Options

`addons/seo` is opt-in. `BaseURL` is required because generated sitemap URLs
need deploy-owned origin policy. `ExtraURLs` adds build-time known URLs, and
`DynamicSitemap` lets generated apps serve request-time sitemap URLs from an
app-owned Go provider:

```go
package app

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/seo"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		seo.Addon(seo.Options{
			BaseURL: "https://example.com",
			ExtraURLs: []seo.URL{
				{Loc: "/rss.xml"},
			},
			DynamicSitemap: seo.DynamicSitemap{
				ImportPath:   "github.com/acme/site/sitemap",
				Function:     "DynamicURLs",
				MaxURLs:      500,
				CacheSeconds: 300,
			},
		}),
	},
}
```

See [SEO Addon](seo.md) for `jsonld` page metadata, static sitemap rules, and
the dynamic provider signature.

## Localization

`I18N` declares locale-prefixed page output. When `Locales` is empty, route
generation is unchanged. When locales are configured, each generated page route
is emitted once per locale:

```go
type I18NConfig struct {
	Locales           []LocaleConfig
	DefaultLocale     string
	OmitDefaultPrefix bool
}

type LocaleConfig struct {
	Code       string
	PathPrefix string
	Name       string
}
```

`Code` must be a BCP-47-like ASCII locale code such as `en`, `pt-BR`, or
`zh-Hant`. By default, the path prefix is `"/" + strings.ToLower(Code)`. Set
`PathPrefix` for explicit URL shape, for example
`{Code: "pt-BR", PathPrefix: "/br"}`. Prefixes must be absolute clean path
prefixes and cannot resolve to the root path.

`DefaultLocale` selects the fallback locale for helper APIs. If it is omitted,
the first declared locale is the default. Set `OmitDefaultPrefix` to keep the
default locale on the original page route while prefixing other locales.

Localized build output passes the active locale to Go build helpers through
`gowdk.BuildParams.Locale` and `BuildParams.LocaleCode()`. Generated
request-time SSR route handlers attach the same locale to
`runtime/app.Locale(ctx)`. Backend endpoints keep their declared paths; pass
locale explicitly through normal app-owned request data when an action, API,
fragment, command, or query needs endpoint-local locale policy.

Typed message catalogs live in `runtime/i18n`. The package provides:

- `Catalog` and `Bundle` for typed key lookup and fallback.
- `MessageReference`, `Bundle.Check`, and `Bundle.Template` for deterministic
  catalog completeness reports and starter templates in Go tests.
- `FormatPlural`, `FormatNumber`, `FormatDate`, and `FormatTime` for bounded,
  dependency-free formatting. These helpers are deterministic core helpers, not
  a CLDR or ICU MessageFormat replacement.

## Generated API CORS

`Build.CORS` is disabled by default. When enabled, generated embedded and
backend-only apps install CORS handling for API, command, and query routes:

```go
type CORSConfig struct {
	Enabled bool
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposedHeaders []string
	AllowCredentials bool
	MaxAgeSeconds int
}
```

`AllowedOrigins` accepts literal `http` or `https` origins, or `"*"` only when
`AllowCredentials` is false. Preflight requests must match a generated
API/command/query route and must request a method and headers allowed by the
policy. `AllowedMethods` is optional; when omitted, the matched route method is
returned. `AllowedHeaders` must include non-simple request headers such as
`Content-Type` for JSON APIs and any configured CSRF header.

`.gwdk` API declarations can attach endpoint-local CORS with a trailing `cors`
clause:

```gowdk
api Health GET "/api/health" cors origins "https://app.example" headers "Content-Type" credentials true
```

Endpoint-local policy options inherit omitted fields from `Build.CORS` when it
is enabled and override declared fields for that route only. Without
`Build.CORS`, the endpoint clause must include enough information to validate,
including at least one origin.

## Lifecycle Services

`Lifecycle.Services` declares app-owned service providers imported by the
generated app binary:

```go
type LifecycleConfig struct {
	Services []ServiceRef
}

type ServiceRef struct {
	ImportPath string
	Function   string
}
```

Each provider must be an exported no-argument function with this signature:

```go
func Services() ([]app.Service, error)
```

Provider packages import `github.com/cssbruno/gowdk/runtime/app` and return
`app.Service` values. The config loader validates that `ImportPath` and
`Function` are present; the generated app Go build validates that the symbol
exists and has the right signature.

Lifecycle services are generic process hooks. Use them for workers, metrics
listeners, protocol bridges, and app-owned servers. MCP adapters belong in app
code or an external package that returns lifecycle services; GOWDK does not
ship a core MCP addon or runtime package.

## Generated App Request Guards

When generated SSR, action, API, or fragment routes declare `guard`, the
generated app package can expose guard registration hooks. If `auth.Addon` is
configured, generated startup registers the default `auth.required` guard and a
session-backed provider for native `role:` / `permission:` guard IDs from the
addon options.

Custom guard IDs still require a generated app hook:

```go
package gowdkapp

import gowdkguard "github.com/cssbruno/gowdk/runtime/guard"

func GOWDKGuardRegistry() gowdkguard.Registry {
	return gowdkguard.Registry{
		"auth.required": func(ctx gowdkguard.Context) error {
			return nil
		},
	}
}
```

Native RBAC guard IDs such as `role:admin` and `permission:patients.read` use
an application-owned principal source instead of a custom guard function:

```go
import (
	"net/http"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
)

func GOWDKAuthProvider() gowdkauth.Provider {
	return gowdkauth.ProviderFunc(func(request *http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{ID: "user-1", Roles: []string{"admin"}}, nil
	})
}
```

This file belongs with generated app startup code, not inside feature packages
that declare handlers. Missing required backing functions fail the generated app
Go build when no addon supplies them. Guard errors still return HTTP 403 before
SSR load functions, action decoding, API handlers, or user business logic run.

Native RBAC guards are a defense-in-depth redundancy layer for generated
route/page access. They must never replace backend authorization inside
handlers, services, repositories, or external systems.

See [hooks.md](hooks.md) for guard, rate-limit, and middleware ordering.

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

`RenderConfig.Default` controls the default render mode. When omitted, default
mode is `spa`. Supported render modes are `spa`, `hybrid`, and `ssr`; actions
are endpoint declarations, not render modes.

## Env

`EnvConfig` declares the runtime environment contract. Config owns the names,
required flags, and safe non-secret defaults. Deployment owns all values.

```go
type EnvConfig struct {
	Vars    []gowdk.EnvVar
	Secrets []gowdk.SecretEnv
}

type EnvVar struct {
	Name     string
	Required bool
	Default  string
}

type SecretEnv struct {
	Name     string
	Required bool
}
```

Use `Vars` for normal runtime settings:

```go
Env: gowdk.EnvConfig{
	Vars: []gowdk.EnvVar{
		{Name: "GOWDK_ADDR", Default: "127.0.0.1:8080"},
		{Name: "GOWDK_BACKEND_ORIGIN", Required: true},
	},
}
```

Use `Secrets` for secret names:

```go
Env: gowdk.EnvConfig{
	Secrets: []gowdk.SecretEnv{
		{Name: "DATABASE_URL", Required: true},
		{Name: "GOWDK_CSRF_SECRET", Required: true},
	},
}
```

Validation runs when `gowdk.config.go` is loaded. Required names that are unset
or blank in the host environment fail with a direct diagnostic such as
`DATABASE_URL is required but is not set`. Required vars with `Default` are
treated as satisfied by the default. Secrets have no `Default` or value field by
type; `Default` and `Value` are rejected in literal config parsing too.

Project-aware CLI commands can load local env files before validation:

```sh
gowdk check --env-file .env.dev
gowdk build --env-file .env.production
```

If `--env-file` is omitted, GOWDK auto-loads `.env.<GOWDK_ENV>` from the
project root when `GOWDK_ENV` is set and the file exists, otherwise `.env` when
present. Process environment values always win over file values. The file is
only a value source for the same validation contract; it does not bypass
`Required` or `MinBytes`.

The same name cannot appear in both `Vars` and `Secrets`. Secret-looking var
names ending in `_SECRET`, `_TOKEN`, `_PASSWORD`, or `_KEY` are rejected and
must move to `Secrets`. Diagnostics print names only and never print values.

Generated app binaries repeat the required env check before serving requests.
For direct binary runs, set `GOWDK_ENV_FILE=/path/to/.env` or place `.env` in
the process working directory. This is a startup redundancy layer only. It does
not replace backend
authorization, handler validation, database checks, deployment secrets, or
runtime-specific security controls.

## Build

`BuildConfig.Output`, `BuildConfig.Mode`, `BuildConfig.Assets`,
`BuildConfig.ObfuscateAssets`,
`BuildConfig.Head`, `BuildConfig.CSRF`, `BuildConfig.SecurityHeaders`, `BuildConfig.BodyLimits`,
`BuildConfig.AllowMissingBackend`, `BuildConfig.Stylesheets`,
`BuildConfig.Scripts`, and `BuildConfig.Targets` are target build settings.
Current `gowdk build` reads literal `Build.Output`, `Build.Mode`,
`Build.ObfuscateAssets`, `Build.Head`, `Build.CSRF`,
`Build.SecurityHeaders`, `Build.BodyLimits`, `Build.AllowMissingBackend`,
`Build.Stylesheets`, `Build.Scripts`, and `Build.Targets` from
`gowdk.config.go`; `--out` overrides `Build.Output` for ad hoc builds and
`--obfuscate-assets` overrides `Build.Mode` to production for that build.
`BuildConfig.Assets` remains planned.

`Build.Targets` declares repeatable module-to-output packaging:

```go
type BuildConfig struct {
	Output              string
	Mode                gowdk.BuildMode
	Assets              gowdk.AssetMode
	ObfuscateAssets     bool
	Head                gowdk.HeadConfig
	CSRF                gowdk.CSRFConfig
	SecurityHeaders     gowdk.SecurityHeadersConfig
	BodyLimits          gowdk.BodyLimitsConfig
	AllowMissingBackend bool
	Stylesheets         []gowdk.Stylesheet
	Scripts             []gowdk.Script
	Worker              gowdk.ContractWorkerConfig
	Cron                gowdk.ContractCronConfig
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
	Disabled   bool
	SecretEnv  string
	CookieName string
	FieldName  string
	HeaderName string
	Insecure   bool
}

type SecurityHeadersConfig struct {
	Enabled bool
	Headers map[string]string
}

type BodyLimitsConfig struct {
	ActionBytes int64
	APIBytes    int64
}

type Script struct {
	Src  string
	Type string
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
	WorkerApp     string
	WorkerBinary  string
	Worker        ContractWorkerConfig
	CronApp       string
	CronBinary    string
	Cron          ContractCronConfig
	DeployRecipes []string
}

type ContractWorkerConfig struct {
	EventSource ServiceRef
	SeenStore   ServiceRef
	Backoff     ServiceRef
}

type ContractCronConfig struct {
	Jobs []ContractCronJobConfig
}

type ContractCronJobConfig struct {
	Type            string
	Schedule        string
	OverlapPolicy   string
	MissedRunPolicy string
}
```

`Build.Worker` and `Build.Cron` provide defaults for ad hoc role builds.
`WorkerApp` / `WorkerBinary` and `CronApp` / `CronBinary` generate standalone
contract role apps and binaries from configured targets, with target-level
`Worker` and `Cron` overriding the build defaults. Worker targets require an
`EventSource` provider function returning `(contracts.EventSource, error)` and
may provide `SeenStore` and `Backoff` providers. Cron targets require explicit
job types and schedules; the first scheduler slice supports `@once` and
`@every <duration>` with `skip` overlap and missed-run policies.

`Mode` controls development metadata in generated frontend artifacts. The
default omitted mode behaves like `gowdk.Development` and emits JavaScript
island source maps. Set `Mode: gowdk.Production` to omit `.js.map` artifacts and
`sourceMappingURL` comments and to compact generated island JavaScript.

`ObfuscateAssets` is a production-only optimization/hardening switch for
compiler-owned generated browser JavaScript such as the SPA/partial runtime,
store runtime, island runtime/stubs, and WASM loader glue. It uses deterministic
minification/identifier shortening, disables generated source maps through
production mode, records transformed assets in `gowdk-assets.json`, and writes
`asset_obfuscation` / `asset_obfuscated` build-report events. It is not a
security boundary and does not replace server-side auth, guards, CSRF,
validation, or handler authorization. Configs that set `ObfuscateAssets: true`
must also set `Mode: gowdk.Production`; the CLI flag `--obfuscate-assets`
sets both for the current build.

Production mode also requires explicitly declared `act` and `api` endpoints to
bind to supported same-package Go handlers. Missing or unsupported handlers fail
the build by default. Set `AllowMissingBackend: true` or pass
`--allow-missing-backend` when intentionally generating HTTP 501 stubs during a
migration.

`Head` controls app-level document head tags. `Favicon` emits
`<link rel="icon">`. `SiteName`, `Image`, and `TwitterCard` enable generated
Open Graph and Twitter metadata. A page-level `image` overrides `Head.Image`
for that page.

`Scripts` declares global script tags emitted into every GOWDK-generated HTML
document. Use page or component `js "./file.js"`, `js "./file.ts"`, or inline
`js {}` declarations when a browser module should be emitted and linked only
where that page/component is used.

`CSRF` controls generated action and web-command CSRF wiring. CSRF is enabled by
default for generated state-changing form endpoints. Generated apps require a
signing secret from `SecretEnv` or `GOWDK_CSRF_SECRET`, inject a hidden token
field into served HTML POST forms, and validate POSTs before generated decoding
or user handlers run. Invalid or missing tokens return HTTP 403 with
`invalid csrf token` and `Cache-Control: no-store`. Set `Disabled: true` only
for an intentional non-production/test opt-out. `Enabled` is retained for older
configs but is no longer required. `CookieName`, `FieldName`, and `HeaderName`
override the generated token transport names.
`Insecure` is for local HTTP development only: it disables the Secure cookie
flag, uses the default cookie name `gowdk-csrf` instead of
`__Host-gowdk-csrf`, and rejects explicit `__Host-`/`__Secure-` cookie names
because browsers require those prefixes to be Secure.

`SecurityHeaders` controls additional headers written by generated app
handlers. When `Enabled` is true, each entry in `Headers` is passed to
`runtime/app` and emitted on every generated response path, including health
checks and generated errors. Use it for app-owned headers such as
`X-Content-Type-Options`, `Referrer-Policy`, `Content-Security-Policy`, and
`X-Frame-Options`. Keep TLS-boundary headers such as `Strict-Transport-Security`
at the HTTPS edge unless the generated app is directly responsible for TLS.

`BodyLimits` controls generated request body caps in bytes. Omitted or
non-positive values use the default 1 MiB cap. `ActionBytes` applies to
generated action POST handlers and web command form adapters before form
decoding, including multipart action forms. Per-file upload policy is declared
on file controls with `g:max-file-size`, `g:max-files`, and MIME `accept`.
`APIBytes` applies to generated API handlers before user code reads the request
body.

`Name` is required. `Output` is optional and defaults to
`.gowdk/output/<target-name>` when omitted. `Modules` selects configured
modules; omit it to use the default configured discovery set. `App` is optional
and writes a generated Go app that embeds the target output. `Binary` is
optional, requires `App`, and compiles that generated app for the local
platform. `WASM` is optional, requires `App`, and compiles the generated app
with `GOOS=js GOARCH=wasm`.

`BackendApp` is optional and writes a generated backend-only Go app for
feature-bound action/API endpoints. `BackendBinary` is optional, requires
`BackendApp`, and compiles that backend app. When a target has both frontend
`App`/`Binary` and `BackendApp`/`BackendBinary`, the frontend binary proxies
generated backend routes to `GOWDK_BACKEND_ORIGIN`.

`DeployRecipes` is optional and accepts `static`, `systemd`, `caddy`, `nginx`,
and `split`. The values map to `gowdk build --deploy-recipe` and emit starter
deployment files for the target shape. They do not add secrets, domains, TLS
policy, platform-specific rollout logic, storage, backups, or CDN settings.

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
`node_modules`, `.gowdk`, `dist`, and the selected build output directory, and
uses `global.css` as the default CSS input when present.

`CSS.Default` names discovered CSS inputs used by the `default` built-in in
`css`. Generated page CSS defaults to `assets/gowdk/<page-id>.css` and hrefs
under `/assets/gowdk/`.

## Addons

`Addons` registers optional features such as spa, actions, partial, SSR, API,
embed, CSS, contracts, realtime, auth, DB helpers, rate limiting, and SEO
output. Current validation uses feature registration for render-mode,
realtime, and other compiler checks; SPA builds invoke addons that implement
`gowdk.CSSProcessor` or `gowdk.SEOProvider`.

DB helper usage is ordinary Go code around `database/sql`; see [db.md](db.md)
for migrations, transactions, readiness, and sqlc usage.

Use `gowdk add --list` to print addable built-in addon names, and
`gowdk add --list --registry` to inspect the local discovery metadata. Use
`gowdk add <name>` to insert the canonical import and `<name>.Addon()`
constructor into `gowdk.config.go`. `gowdk add seo` requires
`--base-url <url>` because SEO build output requires `seo.Options.BaseURL`. The
command rewrites literal `Config.Addons` lists only; if `Addons` is computed by
Go code, edit the config manually.

The literal config loader recognizes built-in addon constructors when they are
imported from their canonical package paths. Most are no-argument constructors;
`addons/auth` accepts the generated-app-safe session options subset
(`SecretEnv`, `CookieName`, `TTL`, `Insecure`), and `addons/seo` accepts the
literal SEO options subset:

```go
import (
	"github.com/cssbruno/gowdk/addons/actions"
	"github.com/cssbruno/gowdk/addons/api"
	"github.com/cssbruno/gowdk/addons/auth"
	"github.com/cssbruno/gowdk/addons/contracts"
	"github.com/cssbruno/gowdk/addons/css"
	"github.com/cssbruno/gowdk/addons/db"
	"github.com/cssbruno/gowdk/addons/embed"
	"github.com/cssbruno/gowdk/addons/partial"
	"github.com/cssbruno/gowdk/addons/ratelimit"
	"github.com/cssbruno/gowdk/addons/realtime"
	"github.com/cssbruno/gowdk/addons/seo"
	"github.com/cssbruno/gowdk/addons/spa"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/addons/static"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		static.Addon(),
		spa.Addon(),
		actions.Addon(),
		partial.Addon(),
		ssr.Addon(),
		api.Addon(),
		auth.Addon(auth.Options{
			SecretEnv:  "GOWDK_AUTH_SESSION_SECRET",
			CookieName: "gowdk_session",
		}),
		contracts.Addon(),
		embed.Addon(),
		css.Addon(),
		db.Addon(),
		ratelimit.Addon(),
		realtime.Addon(),
		seo.Addon(seo.Options{
			BaseURL: "https://example.com",
		}),
	},
}
```

If `Addons` contains a constructor outside that AST-only subset, the loader
uses an executable config bridge: it creates a temporary helper inside the
project module, imports the config package as normal Go, and reads the resulting
`gowdk.Config`. That allows addons from other modules, including GitHub-hosted
addons, to participate through the regular `gowdk.Addon`,
`gowdk.CSSProcessor`, `gowdk.SEOProvider`, and `gowdk.GoBlockConsumer`
interfaces:

```go
import (
	"github.com/cssbruno/gowdk"
	"github.com/example/gowdk-brand"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		brand.Addon(),
	},
}
```

External addon dependencies must already be resolvable by the project module,
for example with `go get github.com/example/gowdk-brand`. Config packages must
be importable Go packages, not `package main`.

The literal loader also recognizes the known literal Tailwind addon options
subset without needing the executable bridge:

```go
import "github.com/cssbruno/gowdk/addons/tailwind"

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		tailwind.Addon(tailwind.Options{
			Input:  "styles/app.css",
			Minify: true,
		}),
	},
}
```

When `Command` is omitted, the Tailwind addon uses `tailwindcss` from `PATH`.
If the executable is missing, builds fail with an install-required error. The
executable bridge runs project config code only when the AST-only loader finds
addon constructors it cannot reduce safely.

When `ratelimit.Addon()` is enabled, generated apps with request-time action,
API, fragment, SSR, or split-backend proxy routes expose
`gowdkapp.RegisterRateLimiter(*ratelimit.Limiter)`. User-owned Go still creates
the limiter and chooses the in-memory store, Redis store adapter, key function,
limit, and window.
