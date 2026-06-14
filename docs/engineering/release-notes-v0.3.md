# Experimental 0.x release: GOWDK v0.3

GOWDK v0.3 is an experimental 0.x compiler/runtime release.

Not production-ready. Public syntax, generated output, runtime packages, and
tooling contracts may change before a stable release.

## Implemented

- M3 route, endpoint, and contract reportability for the current 0.x surface.
- `gowdk endpoints` prints a versioned endpoint-only report for actions, APIs,
  fragments, and routable command/query contract references.
- `gowdk inspect tree --json` prints a source-linked compiler node tree for
  editor and tooling navigation.
- `gowdk inspect endpoint-graph --json` prints a source-linked endpoint dispatch
  graph with pages, routes, endpoints, guards, handlers, bindings, and notes.
- `gowdk routes` exposes source spans, binding state, route params,
  render/cache metadata, guards, planned handlers, and non-fatal route-mode
  notes for the supported route and endpoint surface.
- `gowdk build` writes `openapi.json` for the routable web surface and
  `asyncapi.json` for contract integration-event metadata.
- `gowdk check --json` diagnostics can include related source locations, and
  the language server reports those locations as related information.
- A machine-checked `.gwdk` conformance corpus pins accept/reject cases against
  stable diagnostic codes.
- A per-construct stability and deprecation table documents current language
  support and is checked against the code registries.
- The shared tokenizer/parser cutover records byte offsets for source positions
  and keeps formatter indentation stable around braces inside strings,
  comments, and template literals.
- Guardless pages are default-denied in generated apps: omission produces a
  warning, public access still requires `guard public`, and pages with backend
  endpoints still hard-error without an explicit guard.
- `v0.3.0` release metadata: CLI/editor versions, optional module root-version
  requirements, root changelog, release-doc current-version examples, and M3
  release-note body.

## Partial

- Route, endpoint, and contract reports are versioned for the current 0.x
  surface, but broader exact source spans and future report fields may still be
  added.
- Generated OpenAPI and AsyncAPI reports cover the implemented routable web and
  integration-event metadata slices, not every future API or contract shape.
- Guard metadata is still generated access redundancy and does not replace
  authorization in normal Go backend handlers and services.

## Planned

- M4 Go interop: Go binding inspection, stubs, typed params, build/load
  contracts, and package resolution.
- M5 secure endpoint runtime: strict adapters, request limits, CSRF response
  contract, panic boundaries, and tested failure paths.
- M7 request-time SSR/hybrid hardening and later component/client-language
  milestones remain separate work.

## Intentionally Out Of Scope

- Production-readiness claims.
- Migration guides.
- Framework comparison docs as core positioning.
- Mandatory full-page SSR or full-page hydration.
- User-written JavaScript as the normal app contract.
- Mandatory Tailwind, npm, Chi, Gin, Echo, Fiber, Redis, NATS, or another optional
  framework/tool dependency.
- Replacing backend authorization with generated page guards.

## Known Gaps

- v0.3 remains experimental and focused on reportability and language/compiler
  hardening.
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
code --install-extension gowdk-vscode-0.3.0.vsix
```

## Tool Versions

- Go: `1.26.4`
- Node.js for extension checks: `24`
