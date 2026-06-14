# Generated Code Policy

Generated output exists for app-shell HTML, browser runtime assets, generated app
source, local binaries, Go `js/wasm` artifacts, supported action/API/fragment
handlers, guards, rate-limit hooks, concrete or dynamic SSR pages with declared
`load {}` fields, and concrete or dynamic hybrid request-time pages with or
without declared `load {}` data.
Hybrid streaming/data refresh, richer generated-client reactivity, and
production operations policy remain planned. This policy records constraints for
generated files as that surface grows.

## Ownership

Generated application output belongs to the user application unless a generated
file explicitly states otherwise. Repository licensing details live in
`../../LICENSE`.

## Safety Rules

Generated code must:

- Escape untrusted HTML by default.
- Keep raw HTML escape hatches explicit.
- Avoid logging secrets and sensitive form values.
- Enforce action/API body limits before decoding.
- Include conservative HTTP server timeouts and header limits.
- Exclude local env files, credentials, private source files, and temporary artifacts from embedded output.
- Keep generated files deterministic enough for tests and review.

## Compatibility

Public generated-runtime contracts should live under `runtime/`. Compiler internals should stay under `internal/` and must not be imported by generated user applications unless explicitly promoted.

## Generated Go Emission

Generated Go belongs to the compiler spine. New generated Go must be built from
Go AST nodes, printed with `go/printer`, and normalized with `go/format` before
being written or compiled. A formatting error is a generator bug and must stop
the build before `go build` sees broken generated files.

Current AST-backed Go emission surfaces:

- `internal/appgen`: generated app packages, backend route registrations,
  action/API/fragment/contract/guard/rate-limit/SSR adapter functions, generated
  imports, server main source, and split backend app source.
- `internal/goblockgen`: captured `go {}` blocks parsed as Go files and emitted
  through AST/printer/format into generated app package source.
- `internal/buildgen`: build-data helper programs and Go client/WASM helper
  source are parsed or formatted before execution or artifact emission.

Tests in `internal/appgen` ban `strings.Builder` and hardcoded line-writing in
the main generated Go emitters. Generated Go goldens under
`internal/appgen/testdata/generated_go_golden/` pin the inspectable output.

Allowed string payloads are limited to non-Go artifacts or user-owned Go source
that is parsed/formatted before use:

- HTML, CSS, JavaScript, JSON, markdown, route manifests, asset manifests, and
  build reports.
- User-authored inline `go {}` bodies, which are parsed into Go AST files before
  generated package emission.
- Literal source snippets used only as parse input for small Go helper programs,
  when the result is immediately parsed or formatted and the source is not
  appended through line-writing.

Any future temporary generated-Go string exception must name the migration step
in a code comment, be covered by a deterministic test or golden, and be removed
before the surrounding generated Go surface is considered stable.

Framework-owned browser JavaScript is not a broad string-payload exception.
Runtime assets such as `gowdk.js`, store persistence, JS islands, and WASM
loaders live as `.js` files under `internal/clientrt/assets/` and are embedded
through `go:embed`; Go code may only perform narrow placeholder substitution
for generated names and paths.
