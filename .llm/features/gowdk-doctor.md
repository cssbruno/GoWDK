# Feature Spec: gowdk doctor

## Problem

Users need one command that explains whether their local GOWDK environment and
current project are healthy before they reach for build output or generated
source.

## Goals

- Check local Go/GOWDK tooling and current project health without writing files.
- Report missing config, missing sources, language diagnostics, route metadata
  readiness, and relevant optional tools.
- Provide human text output and JSON output for CI/editor integrations.

## Non-Goals

- Do not run `gowdk build`, compile generated binaries, install tools, mutate
  config, or create generated output.
- Do not make optional tools mandatory.

## Users And Permissions

- Primary users: GOWDK app authors and contributors debugging local setup.
- Roles or permissions: local repository read access and normal command
  execution permissions.
- Data visibility rules: do not print secret values; reuse existing redacted
  diagnostics.

## User Flow

1. User runs `gowdk doctor` from a project root.
2. The command prints a summary and ordered checks.
3. User fixes errors or warnings using the next-step messages.
4. Tooling can run `gowdk doctor --json` and inspect status/check records.

## Requirements

### Functional

- Support `gowdk doctor [--config <file>] [--module <name>] [--ssr] [--json] [files...]`.
- Exit non-zero only when at least one check has status `error`.
- Missing config is reported as a `config` check error.
- Missing optional tools are warnings only when relevant to config/files.

### Non-Functional

- Performance: no build or code generation.
- Reliability: checks should continue after recoverable failures and mark
  dependent checks as skipped.
- Accessibility: text output must be readable in plain terminals.
- Security/privacy: diagnostics remain redacted.
- Observability: JSON report includes version, status, summary, environment,
  and checks.

## Acceptance Criteria

- [x] Valid minimal projects report `ok`.
- [x] Missing config reports a config error and exits non-zero.
- [x] Invalid `.gwdk` source reports a language check error and exits non-zero.
- [x] Relevant missing optional tools warn without failing.
- [x] JSON output is valid and versioned.

## Edge Cases

- Config load failure skips source, language, route, and optional-tool checks.
- Missing sources skip language and route checks.
- Language errors skip route metadata construction.

## Dependencies

- Internal: project config loader, source discovery, language checks, compiler
  IR, route metadata.
- External: `go` executable; optional `tailwindcss` and `node`.

## Open Questions

- None for this v1 slice.
