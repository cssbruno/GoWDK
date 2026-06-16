# CLI Reference

The current CLI includes language tooling, an initial build-output command,
generated embedded app output, and a local output-serving command for
development.

## Commands

```sh
gowdk version
gowdk init [--force] [--tests] [--template <site|minimal>] [dir]
gowdk add <addon> [--config <file>] [--base-url <url>]
gowdk add --list [--registry] [--json]
gowdk tokens <file.gwdk>
gowdk fmt [--write] <files>
gowdk check [--config <file>] [--module <name>] [--json] [--warnings-as-errors] [--ssr] [files...]
gowdk fix [--dry-run] [--code <diagnostic-code>] [--config <file>] [--module <name>] [--ssr] [files...]
gowdk manifest [--config <file>] [--module <name>] [--ssr] [files...]
gowdk sitemap [--config <file>] [--module <name>] [--ssr] [files...]
gowdk routes [--config <file>] [--module <name>] [--ssr] [files...]
gowdk endpoints [--config <file>] [--module <name>] [--ssr] [files...]
gowdk inspect ir|tree|endpoint-graph|go-bindings [--config <file>] [--module <name>] [--json] [--ssr] [files...]
gowdk generate stubs [--config <file>] [--module <name>] [--ssr] [files...]
gowdk explain [--json] <diagnostic-code>
gowdk doctor [--config <file>] [--module <name>] [--ssr] [--json] [files...]
gowdk audit [--config <file>] [--module <name>] [--ssr] [--json] [--emit-tests[=<file>]] [--run] [files...]
gowdk contracts [--json] [dir]
gowdk graph [--json] [dir]
gowdk trace <contract> [--json] [dir]
gowdk list commands|queries|events|jobs [--json] [dir]
gowdk build [--config <file>] [--debug] [--timings[=<file>]] [--ssr] [--allow-missing-backend] [--allow-insecure] [--obfuscate-assets] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--docker] [--docker-base <distroless|scratch>] [--deploy-recipe <caddy|nginx|split|static|systemd>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...]
gowdk clean [--config <file>] [--target <name>] [--out <dir>] [--dry-run] [--json]
gowdk dev [--addr <addr>] [--interval <duration>] [build flags...]
gowdk preview [--addr <addr>] [--hot] [build flags...]
gowdk serve --dir <dir> [--addr <addr>]
gowdk playground policy [--json]
gowdk playground export --dir <project> --out <project.zip> [--json]
gowdk playground run --dir <project> --out <dir> --allow-hosted-execution (--module-cache <dir> | --allow-shared-module-cache)
gowdk lsp [--ssr]
```

## Flags

- `--ssr`: enables SSR validation by adding the SSR addon to the in-memory config.
- `--force`: supported by `init`; overwrites starter files that already exist.
- `--tests`: supported by `init`; adds `tests/gowdk_smoke_test.go`, an optional generated app smoke test that runs only when `GOWDK_BIN` points at a built `gowdk` CLI.
- `--template`: supported by `init`; selects `site` or `minimal`. Defaults to `site`.
- `--list`: supported by `add`; prints addable built-in addon names the command can wire.
- `--registry`: supported by `add --list`; prints checked-in addon registry
  metadata, including built-in/documented-external kind, lifecycle,
  compatibility, and whether `gowdk add` can wire it.
- `--base-url`: supported by `add seo`; writes the required
  `seo.Options.BaseURL` value. The value must be an absolute `http` or
  `https` URL.
- `--json`: supported by `check`, `doctor`, `audit`, `explain`, `inspect`, `contracts`, `graph`, `trace`, `list`, `clean`, `playground policy`, and `playground export`; prints
  editor/tooling-friendly JSON. Contract JSON includes same-file handler
  signature diagnostics when available. `gowdk check --json` uses diagnostic
  schema version `1`. `gowdk inspect` emits JSON by default; `--json` is
  accepted for scripts that pass a format flag consistently.
- `--warnings-as-errors`: supported by `check`; exits non-zero when warning
  diagnostics are present.
- `gowdk doctor --json`: prints a versioned health report with overall status,
  summary counts, environment metadata, and check records.
- `gowdk audit`: derives the security posture from validated IR, evaluates the
  built-in security baseline against it, and reports findings. It exits non-zero
  when any error-severity finding exists, so it can gate CI.
  `--json` prints the posture manifest plus findings and a summary. Every finding
  carries a diagnostic code; run `gowdk explain <code>` for details.
  `--emit-tests` writes a readable standalone `gowdk_audit_test.go` file (or the
  path from `--emit-tests=<file>`) that drives a `runtime/app` posture harness
  through `runtime/testkit`. `--run` builds a temporary generated app from the
  same validated IR and runs the generated app's audit test with
  `go test ./gowdkapp`; a failed expectation is reported as
  `audit_test_failed`.
  `gowdk build` runs the same baseline gate, blocks production builds on
  error-severity findings unless `--allow-insecure` is set, and writes the
  posture alone to a non-served `.gowdk/reports/<output-name>/gowdk-security.json`
  path outside the selected output directory.
- `--write`: supported by `fmt`; overwrites formatted files.
- `--dry-run`: supported by `fix` and `clean`; for `fix` it prints files with
  available registered fixes without writing changes, and for `clean` it lists
  the build outputs that would be removed without deleting anything.
- `--code`: supported by `fix`; limits rewrites to one diagnostic code that has
  a registered fix.
- `--config`: supported by `add`, `check`, `doctor`, `audit`, `manifest`, `sitemap`,
  `routes`, `endpoints`, `inspect`, `generate stubs`, and `build`; selects the
  config file. Compile commands load a literal config subset from the given
  path instead of the required default `gowdk.config.go`.
- `--debug`: supported by `build` and forwarded by `dev`; prints the structured SPA build report to stderr while generated paths remain on stdout.
- `--timings[=<file>]`: supported by `build`; writes a separate versioned JSON
  timing report. Without an explicit file, the report is written to
  `gowdk-build-timings.json` in the output root. Generated paths remain the
  only stdout payload.
- `gowdk build` writes `contract_reference` build-report events for
  `g:command` forms and `g:query` elements with `unknown`, `bound`, `missing`,
  or `invalid` status.
- `gowdk check` and CLI `gowdk build` fail on linked `missing` or `invalid`
  contract references, invalid contract handler signatures, and duplicate
  command owners.
- `--allow-missing-backend`: supported by `build` and forwarded by `dev`; in production mode, allows missing or unsupported action/API handlers to generate HTTP 501 stubs instead of failing the build.
- Security audit gate: `gowdk build` runs the same baseline as `gowdk audit` (see [security.md](../engineering/security.md)). A production build (`Build.Mode = gowdk.Production`) fails on any error-severity finding (missing CSRF, guardless or public-by-omission endpoints, roleless contracts, bundled secrets, raw-HTML sinks); non-production builds print a prominent summary without blocking. `--allow-insecure` downgrades the production gate to a warning for 0.x experimentation. Run `gowdk audit` for the full report and to gate CI explicitly.
- `--obfuscate-assets`: supported by `build` and forwarded by `dev`; enables
  deterministic production obfuscation/minification for compiler-owned
  generated browser JavaScript, forces `Build.Mode` to `gowdk.Production` for
  that build, records transformed assets in `gowdk-assets.json`, and writes
  `asset_obfuscation` / `asset_obfuscated` build-report events. This is an
  optimization/hardening option, not a security boundary.
- `--target`: supported by `build` and `clean`; may be repeated or comma-separated. For `build` it runs the selected `Build.Targets` entries; for `clean` it restricts removal to the selected targets' outputs.
- `--module`: supported by `check`, `doctor`, `audit`, `manifest`, `sitemap`, `routes`,
  `endpoints`, `inspect`, `generate stubs`, and `build`; may be repeated or
  comma-separated, and limits discovery to selected configured modules when no
  explicit file list is passed.
- `--out`: supported by `build` and `clean`; for `build` it selects the output directory and overrides `Build.Output`, and for `clean` it adds an extra output directory to remove alongside the configured outputs. `playground export` uses `--out` as the archive path. `playground run` uses `--out` as the generated output directory and never writes build output into the source project.
- `--app`: supported by `build`; writes generated Go app source that embeds the selected output directory.
- `--bin`: supported by `build`; requires `--app` and compiles the generated app with `go build -o <file>`.
- `--docker`: supported by `build`; requires `--bin` and emits `Dockerfile`
  plus `.dockerignore` beside the compiled binary. The generated Dockerfile
  runs `/app/site` as a non-root user on port `8080`.
- `--docker-base`: supported by `build` with `--docker`; selects
  `distroless` or `scratch`. The default is `distroless`. `scratch` requires a
  statically linked Linux ELF binary.
- `--deploy-recipe`: supported by `build`; may be repeated or comma-separated
  and emits optional starter deployment recipes. Supported values are `static`,
  `systemd`, `caddy`, `nginx`, and `split`. Recipes are starting points and do
  not own secrets, domains, TLS, storage, backups, CDN policy, or rollout logic.
- `--wasm`: supported by `build`; requires `--app` and compiles the generated app with `GOOS=js GOARCH=wasm go build -o <file>`.
- `--backend-app`: supported by `build`; writes generated backend-only Go app source for feature-bound action/API endpoints.
- `--backend-bin`: supported by `build`; requires `--backend-app` and compiles the generated backend app with `go build -o <file>`.
- `--addr`: supported by `dev`, `preview`, and `serve`; selects the listen address and defaults to `127.0.0.1:8080`.
- `--interval`: supported by `dev`; sets the polling interval, such as `500ms`, `1s`, or `2s`.
- `--hot`: supported by `preview`; runs the dev loop against the preview output instead of serving a one-shot build.
- `--dir`: supported by `serve`; selects the generated build output directory. `playground export` and `playground run` use `--dir` as the source project directory and require a `gowdk.config.go` file there.
- `--allow-hosted-execution`: supported by `playground run`; explicitly opts into the local sandbox build bridge. Without it, playground execution fails closed because hosted execution is disabled by default.
- `--module-cache`: supported by `playground run`; mounts a caller-supplied per-session Go module cache as the (readable) lower layer inside the sandbox. Required on shared/multi-tenant runners so one session cannot read another's cached modules.
- `--allow-shared-module-cache`: supported by `playground run`; deliberately exposes the host `GOMODCACHE` to the sandbox instead of a per-session cache. Intended for local single-user use; `playground run` fails closed if neither this nor `--module-cache` is given.

## Examples

```sh
go run ./cmd/gowdk init --template site my-site
go run ./cmd/gowdk init --tests --template site my-tested-site
go run ./cmd/gowdk init --template minimal my-minimal-site
go run ./cmd/gowdk add --list
go run ./cmd/gowdk add --list --registry
go run ./cmd/gowdk add --list --registry --json
go run ./cmd/gowdk add ssr actions partial
go run ./cmd/gowdk add seo --base-url https://example.com
go run ./cmd/gowdk check examples/pages/home.page.gwdk
go run ./cmd/gowdk check --config gowdk.config.go
go run ./cmd/gowdk check --warnings-as-errors --config gowdk.config.go
go run ./cmd/gowdk fix --dry-run --code old_action_block_syntax --config gowdk.config.go
go run ./cmd/gowdk check --ssr examples/ssr/dashboard.page.gwdk
go run ./cmd/gowdk explain missing_ssr_addon
go run ./cmd/gowdk explain --json spa_dynamic_route_missing_paths
go run ./cmd/gowdk audit --config gowdk.config.go
go run ./cmd/gowdk audit --json --ssr --config gowdk.config.go
go run ./cmd/gowdk audit --emit-tests --run --config gowdk.config.go
go run ./cmd/gowdk doctor
go run ./cmd/gowdk doctor --json
go run ./cmd/gowdk doctor --module frontend --ssr
go run ./cmd/gowdk manifest --module frontend --ssr
go run ./cmd/gowdk sitemap --module frontend --ssr
go run ./cmd/gowdk trace patients.CreatePatient
go run ./cmd/gowdk routes --module frontend --ssr
go run ./cmd/gowdk endpoints --module frontend --ssr
go run ./cmd/gowdk inspect ir --module frontend --ssr
go run ./cmd/gowdk inspect tree --json --module frontend --ssr
go run ./cmd/gowdk inspect endpoint-graph --json --module frontend --ssr
go run ./cmd/gowdk contracts --json .
go run ./cmd/gowdk graph .
go run ./cmd/gowdk list commands .
go run ./cmd/gowdk list events --json .
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk build --debug --out /tmp/gowdk-build examples/pages/home.page.gwdk
go run ./cmd/gowdk build --allow-missing-backend --out /tmp/gowdk-build examples/actions/signup.page.gwdk
go run ./cmd/gowdk build --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app --bin /tmp/gowdk-ssr-site examples/ssr/simple-ssr.page.gwdk
go run ./cmd/gowdk preview --out /tmp/gowdk-preview examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk dev --ssr --out /tmp/gowdk-ssr-build --app /tmp/gowdk-ssr-app examples/ssr/simple-ssr.page.gwdk
go run ./cmd/gowdk build --module frontend --module backend --out /tmp/gowdk-build
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
GOOS=linux CGO_ENABLED=0 gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site --docker examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-build --deploy-recipe static examples/pages/home.page.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site --deploy-recipe systemd,caddy examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --wasm /tmp/gowdk-site.wasm examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk build --module admin --out dist/admin --app .gowdk/admin --bin bin/admin
go run ./cmd/gowdk build --module admin --out dist/admin --app .gowdk/admin --wasm bin/admin.wasm
go run ./cmd/gowdk build --module public,admin --out dist/app --app .gowdk/app --bin bin/app
go run ./cmd/gowdk build --target admin
go run ./cmd/gowdk dev --out /tmp/gowdk-build examples/pages/home.page.gwdk examples/pages/hero.cmp.gwdk
go run ./cmd/gowdk dev --target admin --addr 127.0.0.1:8090
go run ./cmd/gowdk serve --dir /tmp/gowdk-build
go run ./cmd/gowdk playground policy --json
go run ./cmd/gowdk playground export --dir my-site --out /tmp/my-site.zip
go run ./cmd/gowdk playground run --dir my-site --out /tmp/my-site-dist --allow-hosted-execution
```

`init` creates a buildable starter project in the selected directory, or the
current directory when omitted. The default `site` template writes:

```text
gowdk.config.go
src/pages/home.page.gwdk
src/components/hero.cmp.gwdk
styles/global.css
```

Passing `--tests` also writes:

```text
tests/gowdk_smoke_test.go
```

The smoke test skips by default. Set `GOWDK_BIN=/path/to/gowdk` to make it run
`gowdk build` from the scaffolded project root and assert that `index.html` and
`bin/site` were generated.

`add` rewrites `gowdk.config.go` through the Go AST and `go/format`. It knows
the built-in addon packages listed in [addons.md](addons.md), inserts missing
imports, appends `<addon>.Addon()` to `Config.Addons`, and skips constructors
that are already present. It does not install third-party modules or rewrite
non-literal config expressions.

The generated config discovers `src/**/*.gwdk`, discovers CSS under
`styles/**/*.css`, declares a `site` build target, generates app source in
`.gowdk/site`, compiles `bin/site`, and writes `.gitignore` entries for
generated outputs. The target's intermediate build output is inferred as
`.gowdk/output/site`. Existing starter files are not overwritten unless
`--force` is passed. The `minimal` template skips the starter component and
writes only the config, `.gitignore`, one page, and one CSS file.

`check`, `audit`, `manifest`, `sitemap`, `routes`, `build`, and `dev` require a config
file before they compile or validate `.gwdk` code. By default they load
`gowdk.config.go` from the current directory; `--config <file>` can point at a
different config for project examples or one-off checks.

These commands accept explicit file paths, but explicit paths do not remove the
config requirement. If no files are passed, commands discover configured root
`Source.Include` globs plus configured module sources, or `**/*.gwdk` when the
loaded config does not declare source includes. `--module` limits discovery to
selected configured modules and skips root `Source.Include`; explicit file
paths still bypass discovery. A module with a name and no explicit include uses
`<module-name>/**/*.gwdk`. Discovery excludes `.git`, `vendor`, `node_modules`,
`testdata`, root/module `Source.Exclude` globs, and the configured build output
directory when one exists. `build --out` overrides `Build.Output`; one of them
is required for `build`. Every successful disk build writes
`gowdk-build-report.json` to the output root. The report includes validation,
planning, write, manifest, cache-policy, cleanup, and completion events;
request-time SSR/hybrid pages that are intentionally skipped from static
prerender output appear as `request_time_page_skipped` events. Passing `--debug`
prints the same build report to stderr without changing stdout artifact-path
output. Passing
`--timings` writes `gowdk-build-timings.json` next to the build report, or
`--timings=<file>` writes the timing JSON to a custom path; timing data is kept
out of `gowdk-build-report.json` so normal build reports stay deterministic.
`gowdk dev`
uses `gowdk_cache` as its default output directory unless `--out <dir>` or one
selected build target supplies or infers an output directory.

For generated apps and binaries, the selected modules are the packaging set:
`--app` copies the selected build output; `--bin` and `--wasm` embed it. Prefer
`Build.Targets` in `gowdk.config.go` for repeatable packaging. Target `Output`
is optional; when omitted, it defaults to `.gowdk/output/<target-name>`. With
targets configured, `gowdk build` runs all targets when no ad hoc output,
module, app, binary, or explicit file arguments are passed; `gowdk build
--target <name>` runs selected targets. `--target` cannot be combined with
`--module`, `--out`, `--app`, `--bin`, `--wasm`, or explicit files. The ad hoc
flags remain useful for one-off builds.

`clean` removes the generated build outputs declared by the project config: the
top-level `Build.Output` directory and, when `Build.Targets` are configured,
each target's `Output`, `App`, `Binary`, `WASM`, `BackendApp`, and
`BackendBinary` paths. `--target` restricts removal to selected targets and
`--out` adds an extra directory. It only ever deletes configured output paths
inside the project root — it refuses to remove the project root itself or any
path that resolves outside it, and it never touches the source tree. Use
`--dry-run` to list what would be removed and `--json` for tooling. This is more
than `rm -rf`: it reads `gowdk.config.go` so the right paths are removed even
when outputs are customized or spread across targets.

`env` and `benchmark` were considered and intentionally **not** added. `env`
would duplicate `gowdk doctor` (which already reports the resolved Go toolchain,
CLI version, config, and tool availability, with `--json`) and Go's own
`go env`. `benchmark` would duplicate `gowdk build --timings`, which already
writes per-phase compiler timings, and Go's native `go test -bench`; broader
performance work is tracked in issue #414. Adding either would split one source
of truth across redundant commands.

`doctor` checks the local GOWDK environment and current project without writing
files. It verifies the Go toolchain, CLI version, config loading, source
discovery, language validation, route metadata construction, and relevant
optional tools such as Tailwind or Node. Missing config and language failures
are errors. Missing optional tools are warnings when the project appears to use
them. The command exits non-zero only when at least one check is an error.

`audit` reports the security posture, not the environment. It derives a
declarative posture from validated IR — every route, backend endpoint, and
contract with its guards, CSRF state, body limit, and source location — and
evaluates the built-in security baseline against it. The baseline encodes the
production-readiness gates from `docs/engineering/security.md` (for example:
actions, commands, and state-changing APIs must enforce CSRF, and APIs must not
be public by omission).
Findings carry a diagnostic code, a `file:line`, and remediation; run
`gowdk explain <code>` for details. `gowdk build` runs the same baseline gate
before writing output: production builds fail on error-severity findings unless
`--allow-insecure` is set, while non-production builds warn. The posture alone is
also emitted as `gowdk-security.json` by `gowdk build`, but outside the selected
output directory in a non-served `.gowdk/reports/<output-name>/` path.
Declared `*.audit.gwdk` policies are discovered with the rest of the source
set. `--emit-tests` writes a committable standalone `_test.go`; `--run` builds a
temporary generated app, executes `go test ./gowdkapp`, and folds failures back
into the audit report.

`--wasm` produces a Go `js/wasm` compile artifact from the generated app. This
is a deploy artifact for hosts that can run Go WebAssembly; it is separate from
component-level browser island assets emitted for `wasm` components.

`dev` is the one-command SPA development loop. It forwards non-dev flags to
`build`, resolves the output directory from `--out`, exactly one selected
`Build.Targets` entry, or the default `gowdk_cache` dev output, serves that
directory, polls explicit or discovered build inputs plus the loaded config
file, prints changed/added/removed input paths, rebuilds after content changes,
and injects a tiny server-sent-events live-reload script into served HTML pages.
Rebuild failures are printed and the last successful output keeps serving. See
[dev.md](dev.md) for HMR, polling, browser overlay, restart, and last-good-build
behavior.

Build output, route/asset manifests, generated `go.mod`, generated
`gowdkapp/app.go`, generated `cmd/server/main.go`, and embedded build output
files are only rewritten when their bytes change, which keeps local dev loops
from retriggering on no-op generation. For plain SPA `--out` builds, page-only
source edits use an incremental SPA renderer that validates the full compiler
IR, refreshes manifests, writes only changed page output, and removes stale route
output for changed pages. Component, layout, CSS, config, source-set, target,
app, binary, and WASM changes use the full build path.

Generated apps created with `--app` read `GOWDK_APP_ID`, `GOWDK_MODULE_NAME`,
and `GOWDK_INSTANCE_ID`, expose `/_gowdk/health`, and include identity in
`X-GOWDK-*` response headers. If `GOWDK_INSTANCE_ID` is omitted, the generated
app creates one at process start; set it explicitly when deployment code needs a
stable ID. `GOWDK_MODULE_NAME` is runtime identity metadata; it does not change
which modules were embedded. Embedded module composition is fixed at build time
by `Build.Targets` or the selected `--module` flags.

`gowdk routes` prints validated route and endpoint metadata as JSON. The current
schema is version `1`. The `routes` list is limited to page/file route kinds
such as `static`, `spa`, `ssr`, and `hybrid`; route records include package,
render/cache metadata, route params, layouts, guards, source file, source span,
and planned handler. Backend actions, APIs, fragments, and routable command or
query contracts appear in the separate `endpoints` list with source path,
source span, `.gwdk` package, method, path, page ID, route params when
declared, no-store backend cache policy, inherited guards, CSRF applicability,
planned adapter handler, and backend or contract binding metadata. Backend
binding metadata includes the Go
package name, import path when known, handler symbol, signature/input metadata
when bound, status, and binding message. Non-fatal route-mode notes, such as
request-time page rendering disabled on a SPA route or static SPA output
disabled on an SSR route, appear in `info` and are also mirrored to stderr as
`info:` console lines.

`gowdk endpoints` uses the same project loading, validation, binding, and
contract-reference scan as `gowdk routes`, but prints only the versioned
`endpoints` report. Use it when tooling needs backend surface metadata without
page route records or route-mode notes.

`gowdk inspect ir` prints the validated compiler IR. `gowdk inspect tree` prints
a versioned source-linked node tree with `program`, `package`, `page`,
`component`, `layout`, `route`, `endpoint`, `contract-reference`, `view`,
`element`, `component-call`, and `text` nodes. Each node has a stable `id`,
`kind`, optional `name`, source path, source span when known, node-specific
`props`, and ordered `children`.

`gowdk inspect endpoint-graph` prints a versioned graph with `page`, `route`,
`endpoint`, `handler`, `guard`, and `contract` nodes plus deterministic
`declares`, `owns_endpoint`, `handled_by`, `uses_guard`, and
`references_contract` edges. Endpoint nodes include method/path, source kind,
cache, guards, CSRF policy, binding status, signature, and input type when
available. The graph is additive on top of `gowdk routes`/`gowdk endpoints`;
keep those commands for stable route and endpoint report integrations.

`gowdk inspect go-bindings` prints a versioned report of Go interop bindings:
actions, APIs, fragments, SSR load functions, build-time Go function calls, and
web command/query contract references where those surfaces exist. Records
include source, package, expected symbol, package path when known, method/path
for request-time handlers, binding status, signature/input metadata, message,
and a suggested next step for missing or unsupported bindings.

`gowdk playground policy` prints the sandbox contract used by local and future
hosted playground execution. `gowdk playground export` creates a normal source
project archive and omits generated `.gowdk/`, `dist/`, `bin/`, `gowdk_cache/`,
dependency vendor folders, local env files, private keys, temp files, and
generated reports. `gowdk playground run` stages the same allowed files into a
disposable workspace and then re-executes the `gowdk` binary inside an OS-level
Linux namespace sandbox (no network, `pivot_root` away from the host filesystem,
resource limits, dropped capabilities, `no_new_privs`) with isolated Go caches
and `GOPROXY=off`, `GOSUMDB=off`, `GOWORK=off`. It requires
`--allow-hosted-execution` and a module-cache choice (`--module-cache <dir>` or
`--allow-shared-module-cache`), needs a fresh empty `--out` directory, and fails
closed where the sandbox is unavailable (non-Linux hosts or disabled
unprivileged user namespaces).

`gowdk generate stubs` writes missing action/API handler stubs to
`gowdk_stubs.go` beside the owning source package. It refuses to overwrite an
existing stub file and does not generate load, fragment, command, query, or
unsupported-signature replacements. The generated code is normal importable Go
that should be edited or moved into app-owned files before serious use.

`openapi.json` describes the routable web surface: actions, APIs, fragments,
and web-routable command/query contract references. It includes paths, methods,
path/query/form request schemas when input fields are known, and deterministic
named response schema references. Full response struct expansion is deferred to
[#316](https://github.com/cssbruno/GoWDK/issues/316).

`asyncapi.json` describes integration-event contract registrations. Local event
payload structs contribute JSON-field schemas; imported event payload schemas
fall back to deterministic named object schemas until
[#315](https://github.com/cssbruno/GoWDK/issues/315) resolves imported payload
fields. Domain and presentation events are excluded from the default report.

`gowdk inspect ir` prints the validated `internal/gwdkir.Program` compiler IR as
JSON. This is an M2 compiler-spine debugging and snapshot surface, not a stable
public schema yet. It uses the same project config, discovery, validation,
backend binding, and contract-reference checks as the other compile commands.

`gowdk explain <diagnostic-code>` prints the registry metadata, stability,
summary, next steps, and examples when available for a diagnostic code. It does
not read project config or source files. Unknown codes fail with close-code
suggestions. Use `--json` for editor and tooling integrations. See
[diagnostic-codes.md](diagnostic-codes.md) for the registry and stability
policy.

Current `build` limitations: it emits app-shell HTML files,
`gowdk-routes.json`, `gowdk-assets.json`, `openapi.json`, `asyncapi.json`,
`gowdk-build-report.json`, generated embedded app source, and an optional
generated binary for build-time pages with non-dynamic routes or literal
`paths {}` dynamic routes, literal `build {}` data, lowercase HTML markup,
imported or same-package no-argument Go build data functions returning `T` or
`(T, error)`,
component files with string props, supported action redirect handlers with form
decoder wrappers and required-field validation, and supported action fragment
responses for partial requests.

Current generated binary limitations: it serves embedded build output files for
the selected build output and local POST endpoints for the supported action
subset, including form input decoder wrappers, required-field validation, CSRF
validation by default for generated action POSTs, command POSTs, and
state-changing APIs, action fragment responses for
`X-GOWDK-Partial` requests, standalone fragment routes, feature-bound API
handlers, guards, and concrete or dynamic SSR pages rendered from `view {}`
and literal or imported `build {}` data. Hybrid pages use the same generated
request-time page path with or without declared `load {}` data and appear as
`hybrid` routes in `gowdk routes`. It does not stream hybrid responses, refresh
hybrid server data in place, or run non-HTTP revalidation today.

Current `serve` limitations: it serves generated build output files only. It does not
run generated actions, APIs, partial fragments, or SSR routes.
