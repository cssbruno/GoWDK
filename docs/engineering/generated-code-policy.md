# Generated Code Policy

Generated output exists for app-shell HTML, browser runtime assets, generated app
source, local binaries, Go `js/wasm` artifacts, first-slice action/fragment
handlers, and first-slice concrete or dynamic SSR pages with declared `load {}`
fields.
Broader generated action, API, fragment, guard, `load {}`, and hybrid behavior
remains planned. This policy records constraints for generated files as that
surface grows.

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
