# Feature Spec: Compiler And Extension Hardening

## Problem

GOWDK can interpret source after lowering through both typed IR and raw block
bodies, exposes validation state through APIs that still accept ordinary IR,
publishes multi-file generations incrementally, duplicates project compilation
orchestration between CLI and tooling, and uses an unversioned one-shot process
bridge for executable addons. Those gaps can make commands disagree, expose
partial output, or let extension execution hang indefinitely.

This specification covers GitHub issues #664, #667, #669, #670, #671, and
#672 as one dependency-ordered compiler hardening program.

## Goals

- Make typed lowered records the semantic source of truth after analysis.
- Make analyzed, validated, planned, and emitted phases distinct in Go APIs.
- Give project commands one reusable compilation snapshot service.
- Publish static and generated-app directories as committed generations.
- Separate built-in feature configuration from executable extensions.
- Version, bound, reuse, and diagnose the executable extension host.

## Non-Goals

- Change accepted `.gwdk` syntax or the compile-first rendering model.
- Make extensions a general runtime plugin system.
- Download extensions or execute untrusted remote code automatically.
- Move app-owned lifecycle services into the compiler extension host.

## Users And Permissions

- Primary users: GOWDK application authors, extension authors, editor users,
  and GOWDK contributors.
- Roles or permissions: extension processes run with the invoking developer's
  local permissions; GOWDK does not elevate privileges.
- Data visibility rules: protocol errors and stderr are bounded and redacted;
  configuration secret values are never serialized into diagnostics.

## User Flow

1. GOWDK loads project configuration and discovers the selected source set.
2. A shared workspace service parses, analyzes, enriches, links, validates, and
   returns one immutable compiled snapshot with structured diagnostics.
3. Output planners accept only the validated snapshot and produce complete
   static/app artifact generations.
4. Publishers stage and audit the generation, then replace the committed
   destination or roll back without exposing mixed files.
5. Built-in feature sections select compiler-owned behavior. Executable
   extensions negotiate a supported protocol and explicit capabilities with
   one reusable helper host per configuration digest.

## Requirements

### Functional

- `gwdkir.CheckInvariants` rejects missing typed records for supported blocks.
- Compiler, build, app, LSP, manifest, and dev dependency analysis do not
  reparse raw semantic block bodies after lowering.
- Layout composition and fragment rendering consume typed view nodes.
- Compiler validation is the only constructor of `ValidatedProgram`.
- Static and generated-app emitters reject zero or analyzed-only phase values.
- Route defaults, localization, schemas, bindings, guards, fragments, and
  request-time metadata are finalized before emission.
- A workspace snapshot owns analysis, Go binding, contract scanning/linking,
  and feature validation once per source set. Language tooling layers its
  editor-only accessibility diagnostics onto that snapshot.
- Static output and generated app files are staged and committed as complete
  directory generations; failed commits restore the prior generation.
- `Config.Features` selects built-in capabilities independently from executable
  extensions; legacy `Config.Addons` remains a documented 0.x adapter.
- Executable extensions declare a protocol version, required/optional
  capabilities, and supported compiler phases.
- The extension bridge performs a handshake, reuses one built helper/process,
  honors cancellation and deadlines, bounds payloads/stderr, validates emitted
  relative paths, and returns structured errors.

### Non-Functional

- Performance: compile snapshots and extension hosts are reusable within one
  command; unchanged output remains deterministic.
- Reliability: interrupted staging is recoverable and stale staging/backup
  directories are cleaned deterministically.
- Accessibility: language tooling retains the existing accessibility pass.
- Security/privacy: extension output paths cannot escape the generated app;
  errors are bounded and redacted.
- Observability: snapshot stages and extension failures expose stable structured
  codes without writing directly to command output streams.

## Acceptance Criteria

- [x] Issues #664, #667, #669, #670, #671, and #672 acceptance tests pass.
- [x] `gowdk check` and `gowdk build` agree on invalid project sources.
- [x] A publication failure leaves the prior static/app generation unchanged.
- [x] Repeated extension requests use one helper host and version handshake.
- [x] Existing 0.x addon configs continue through a documented compatibility
  path while new configs can use typed built-in feature selection.
- [ ] Compiler, generator, project, and CLI focused tests plus the repository
  quality gates pass.

## Edge Cases

- Empty supported blocks, parse recovery, and warning-only validation.
- Destination absent, destination already present, stale backup/staging data,
  rename failure, and interruption between backup and publish rename.
- Unknown optional versus required extension capabilities.
- Protocol mismatch, helper crash, malformed/oversized JSON, timeout,
  cancellation, unsafe generated paths, and bounded stderr.
- Legacy addons that combine feature markers with executable behavior.

## Dependencies

- Internal: parser/analyzer IR, compiler phases, buildgen, appgen, project
  config loader, contract scanner, accessibility diagnostics, dev loop.
- External: Go toolchain processes only; no new production dependency.

## Open Questions

- None for this slice. Remote extension discovery and independently versioned
  distribution remain outside this specification.
