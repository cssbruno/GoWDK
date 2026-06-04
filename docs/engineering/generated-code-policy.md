# Generated Code Policy

Generated output is planned. This policy records constraints before generation exists.

## Ownership

Generated application output belongs to the user application unless a generated file explicitly states otherwise. Repository licensing details live in `LICENSE.md`.

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
