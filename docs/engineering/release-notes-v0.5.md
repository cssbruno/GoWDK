# Experimental 0.x release: GOWDK v0.5

GOWDK v0.5 is the first normal GitHub release for the 0.x line.

Not production-ready. Public syntax, generated output, runtime packages, and
tooling contracts may change before 1.0.

## Changed

- `gowdk version` and the VS Code extension metadata report `0.5.0`.
- GitHub release automation publishes the selected 0.x tag as a normal visible
  release instead of marking it as a GitHub pre-release.

## Implemented

- Page stores can opt into browser persistence with `persist "local"` or
  `persist "session"`, including schema-hash invalidation, SPA navigation
  rehydration, cross-tab localStorage mirroring, and persistence diagnostics.
- M4 Go interop reports are available through `gowdk inspect go-bindings` for
  actions, APIs, fragments, SSR load functions, build-time Go calls, and web
  command/query references.
- `gowdk generate stubs` writes conservative missing action/API handler stubs.
- Build-time Go helpers may return `T` or `(T, error)`.
- Build-helper stdout/stderr handling no longer lets successful helper logging
  corrupt JSON build data.
- `docs/reference/go-interop.md` documents current Go binding, route-param,
  and middleware contracts.
- `gowdk check` and `gowdk build` surface same-name Go binding near-misses as
  non-fatal warnings.
- Backend handler binding reports ambiguous handlers, sibling package
  compilation failures, and component-script resolution failures instead of
  silently falling back.
- Generated `g:command` and `g:query` web adapters use one JSON success/error
  response contract.
- Contract event envelopes carry stable IDs for durable delivery, with
  deduplication support for event workers and safer file-backed record updates.

## Partial

- Store persistence is a JS-island/store runtime feature; WASM islands do not
  yet participate in page stores.
- Go interop diagnostics cover the current 0.x surface, but broader examples
  and unsupported-signature diagnostics remain in hardening.
- Contract web adapter JSON behavior is defined for the generated web surface;
  fragment/API-specific query execution and richer realtime patch shapes remain
  planned.

## Planned

- Passing route params into Go build functions.
- Generated per-route param structs and typed load/action result accessors.
- Broader Go binding diagnostics for unsupported signatures, build-tag-hidden
  symbols, and unsupported return/parameter types.
- Broader Go-package interop examples for `database/sql`, `pgx`, `sqlc`,
  `slog`, and similar packages.

## Intentionally Out Of Scope

- Production-readiness claims.
- Migration guides.
- Framework comparison docs as core positioning.
- Mandatory full-page SSR or full-page hydration.
- User-written JavaScript as the normal app contract.
- Mandatory Tailwind, npm, Chi, Gin, Echo, Fiber, Redis, NATS, or another
  optional framework/tool dependency.

## Known Gaps

- GOWDK remains not production-ready.
- Generated output remains pre-1.0 and unstable unless a reference doc marks a
  surface stable.
- Secure runtime, SSR/hybrid, component/client, operations, and production
  hardening milestones remain incomplete.

## Breaking Or Unstable Generated Output

Generated output is pre-1.0. Treat generated Go, generated JavaScript,
manifests, build reports, route reports, endpoint reports, inspect reports, and
runtime package contracts as unstable unless a reference doc explicitly marks a
surface stable.

## Required Release Verification

Run the full release checklist before publishing:

- `docs/engineering/release.md`
- `docs/engineering/release-plan.md`

Required local gates:

```sh
git diff --check
scripts/test-go-modules.sh
scripts/vulncheck-go-modules.sh
go build ./cmd/gowdk
node editors/vscode/scripts/sync-version.js --check
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
```

## Artifact Verification

Download the CLI artifact for your platform and `checksums.txt` from the GitHub
release.

```sh
grep ' <artifact>$' checksums.txt | sha256sum -c -
```

On macOS, use:

```sh
shasum -a 256 <artifact>
```

Verify GitHub artifact attestations:

```sh
gh attestation verify <artifact> -R cssbruno/GOWDK
```

## VS Code Extension

Install the packaged `.vsix` manually when the release includes one:

```sh
code --install-extension gowdk-vscode-0.5.0.vsix
```

## Tool Versions

- Go: `1.26.4`
- Node.js for extension checks: `24`
