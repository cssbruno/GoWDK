# 0.x Improvement Checklist

This document is the 0.x hardening backlog for GOWDK after the v0.1 release
line. It is intentionally not a finish-line roadmap. It records what to improve
while preserving the product rules already defined in
`docs/product/roadmap.md`, `docs/product/requirements.md`, and
`docs/engineering/architecture.md`.

Minor versions such as `v0.2.0`, `v0.3.0`, `v0.4.0`, or later `0.x` tags are
release vehicles. They may group work from the waves below, but they must not
promise production readiness or imply that the project has reached a final
framework shape.

## Product Rules To Protect

- [ ] Keep GOWDK framed as an experimental 0.x Go-first compiler/runtime.
- [ ] Do not add production-ready claims.
- [ ] Do not add a finish-line version promise.
- [ ] Do not add migration-guide docs.
- [ ] Do not add framework comparison docs as core docs.
- [ ] Keep `.gwdk` as the declaration layer.
- [ ] Keep normal Go as the behavior layer.
- [ ] Keep generated Go as adapter glue.
- [ ] Keep static/build-time pages as the default.
- [ ] Keep request-time behavior explicit.
- [ ] Keep SSR opt-in.
- [ ] Keep hybrid behavior explicit.
- [ ] Keep generated JavaScript as enhancement only.
- [ ] Do not let generated JavaScript own auth, routing truth, validation truth,
  business logic, server state, or cache policy.
- [ ] Keep `net/http` as the runtime boundary.
- [ ] Keep Gin, Echo, and Fiber as optional adapters.
- [ ] Keep Redis, NATS, SSE, and WebSocket adapters optional.
- [ ] Keep Tailwind optional.
- [ ] Keep npm optional.
- [ ] Do not download optional tools during normal builds.
- [ ] Make unsupported behavior fail loudly with diagnostics.
- [ ] Make partial behavior visible in docs, examples, CLI output, and release
  notes.

## Release Wave Index

These waves are ordered by dependency, not by promised version number.

| Wave | Theme | Outcome |
| --- | --- | --- |
| Public Truth | Release metadata, README status, known gaps, and repo issue hygiene. | Users can tell what works, what is partial, and what is unsafe to rely on. |
| Release Trust | Release workflow, notes, checksums, attestations, docs checks, and smoke tests. | Releases are reproducible and clearly experimental. |
| Compiler Spine | AST, analyzer, IR, diagnostics, generated output, and deterministic generation. | Compiler phases are explicit and boring. |
| Go Interop | Go binding inspection, stubs, typed params, build/load contracts, and package resolution. | Go code is easy to connect and debug. |
| Endpoint Adapters | Unified endpoint metadata, adapter IR, strict binding mode, and reports. | Generated backend glue is strict, readable, and framework-neutral. |
| Secure Runtime | Actions, APIs, fragments, CSRF, redirects, guards, timeouts, limits, errors, and logs. | Runtime behavior is safer without claiming production readiness. |
| SSR And Hybrid | Request-time page contracts, load, guards, route params, errors, cache, and hybrid behavior. | Request-time lanes are explicit and documented. |
| Components And Islands | Component contracts, client language, reactivity, SPA navigation, and WASM islands. | Browser behavior stays bounded to enhancement and local UI. |
| CSS And Assets | Optional CSS processors, Tailwind, scoped CSS, asset manifests, and packaging. | Assets are deterministic and optional tools stay optional. |
| DX And Examples | CLI inspection, doctor/explain, dev server, LSP, native docs, and examples. | Users can learn GOWDK through native examples, not migration positioning. |
| Ops And CI | Dependency policy, security policy, production-safety gates, CI, performance, and operations docs. | Hardening is measurable and visible. |

## Version Buckets

These buckets give each minor release a planning center. They are not promises:
if a release ships with partial scope, move the unfinished items to the next
bucket and record the deferral in release notes and requirements.

### v0.2 Public Truth And Release Trust

Focus: make the existing release line honest, installable, and easy to verify.

Must include:

- Release metadata audit for `v0.1.5` and future releases.
- Release note template with experimental, not-production-ready, known gaps,
  checksum, and attestation sections.
- README status/support matrix and links to known gaps.
- GitHub release install path, checksum verification, and attestation
  verification in getting started docs.
- Root `SECURITY.md` wording synced with `docs/engineering/security.md`.
- Public `0.x Hardening` issue/project structure or documented reason it is
  deferred.

Release gate:

- A user can install a release artifact, verify it, run `gowdk version`, and
  understand exactly what is safe to try.

### v0.3 Compiler Spine

Focus: make compiler phases and generated output dependable.

Must include:

- Downstream migration plan and progress for `internal/gwdkir`.
- IR, AST, generated Go, generated HTML, manifest, route report, and endpoint
  report golden tests where the surface exists.
- Source spans for the highest-impact parser/analyzer/validator paths.
- Deterministic generated output checks.
- Documented temporary exceptions for any string-generated Go.
- Contributor guidance for adding syntax.

Release gate:

- Build output, app generation, and CLI reports use typed IR for the release's
  supported paths, or remaining compatibility paths are explicitly listed.

### v0.4 Diagnostics And Language Contract

Focus: make unsupported syntax and partial behavior loud and toolable.

Must include:

- Stable diagnostic code inventory for supported language surfaces.
- `gowdk check --json` stabilization plan or first stable schema.
- Exact spans for package, route, render mode, `build`, `load`, `view`, action,
  API, fragment, component, and Go block errors where supported.
- Formatter idempotence and malformed syntax tests.
- Formal `.gwdk` language spec draft for current syntax.
- `gowdk explain <diagnostic-code>` design or first implementation.

Release gate:

- A syntax or binding failure gives a diagnostic code, useful span, and
  documented next step for common errors.

### v0.5 Go Interop

Focus: make normal Go package behavior easy to connect and debug.

Must include:

- Go interop docs page.
- Go symbol discovery/binding report.
- `gowdk inspect go-bindings` design or first implementation.
- `gowdk generate stubs` design or first implementation.
- Better diagnostics for unsupported signatures, build tags, missing exports,
  wrong packages, ambiguous imports, unsupported returns/params, and build-data
  encoding failures.
- Tests for package path resolution, aliased imports, build tags,
  same-package handlers, and generated `gowdk_go/` packages.

Release gate:

- A user can see why a Go function did or did not bind without reading
  generated source.

### v0.6 Endpoint Adapters

Focus: make generated backend glue strict, readable, and framework-neutral.

Must include:

- Unified endpoint/contract metadata fields for existing actions, APIs,
  fragments, SSR loads, commands, and queries where appropriate.
- Endpoint conflict diagnostics.
- Endpoint report command or route report expansion.
- Strict binding mode for production-shaped builds.
- 501 stubs only behind an explicit flag.
- Adapter generation from typed IR for supported paths.
- Deterministic imports, route registration, decoding, and response writing.

Release gate:

- Generated one-binary and split-backend behavior agree on endpoint metadata and
  unsupported handlers fail loudly unless explicit stub mode is selected.

### v0.7 Secure Runtime

Focus: harden request-time generated behavior without claiming production
readiness.

Must include:

- Action and API body limit policy.
- Request-time panic recovery coverage for generated user-Go calls.
- Explicit 405 behavior.
- Safe redirect docs/tests.
- CSRF secret docs and invalid CSRF response docs.
- Production-safe error response docs.
- Log/diagnostic secret-redaction policy.
- Initial `http.Server` timeout and header-limit plan or implementation.

Release gate:

- Actions, APIs, fragments, and generated request-time handlers have tested
  failure paths for invalid input, missing/unsupported handlers, CSRF failure,
  guard failure, and panic recovery.

### v0.8 SSR And Hybrid

Focus: stabilize explicit request-time page lanes.

Must include:

- SSR lifecycle docs.
- `load {}` and `go ssr {}` contract docs.
- Typed route params plan or first implementation for `load`.
- Guard-before-load ordering docs/tests.
- Route-local and endpoint-local error page docs/tests.
- Hybrid lifecycle docs and route/build report visibility.
- Cache/no-store behavior tests for SSR and hybrid.

Release gate:

- Concrete and dynamic request-time pages can be built into generated binaries
  with tested load, guard, error, cache, and direct refresh behavior.

### v0.9 Components And Client Language

Focus: improve local UI behavior without making browser JavaScript the app
contract.

Must include:

- Component contract docs.
- Prop, slot, event, state, and lifecycle support matrix.
- Real `g:if` mount/unmount plan or implementation.
- Client language docs for state, computed values, handlers, expressions,
  update order, cleanup, and rejected behavior.
- Dependency graph/cycle diagnostics plan or implementation.
- Browser tests for local state, events, class toggles, cleanup, and remounts.

Release gate:

- Component/client behavior remains clearly local and cannot own routing, auth,
  server validation, business logic, server state, or cache policy.

### v0.10 SPA Navigation And Fragments

Focus: make progressive enhancement and partial updates dependable.

Must include:

- Static-first SPA navigation docs.
- Link interception rules and no-JS behavior.
- Scroll/focus restoration behavior.
- Fragment syntax, headers, target, swap, fallback, error, no-store, and remount
  docs.
- Browser tests for navigation, back/forward, partial swaps, focus, no-JS
  fallback, and island remounts.
- Generated JavaScript size report.

Release gate:

- Static routes still work on direct refresh, while generated JavaScript remains
  optional enhancement.

### v0.11 WASM, CSS, Assets, And Packaging

Focus: make optional browser Go and asset output explicit.

Must include:

- Versioned WASM island ABI docs.
- Required export and unsafe import diagnostics.
- WASM size and asset manifest reporting.
- CSS processor docs and page-aware processor behavior.
- Tailwind optional-tool docs with no-download tests.
- Scoped component CSS, component assets, content hashing, and immutable cache
  docs/tests.
- Generated app, binary, embedded output, module selection, target selection,
  split build, backend-only build, and deploy-WASM docs.

Release gate:

- Optional browser WASM, Tailwind, and asset behavior are testable without
  making npm, Tailwind, or WASM mandatory.

### v0.12 Runtime Server And Operations Baseline

Focus: make generated server behavior understandable in real deployments.

Must include:

- `net/http` first runtime docs.
- Middleware registration, graceful shutdown, health/readiness, metrics,
  request IDs, structured logging hooks, panic logging, static serving, 404/500,
  cache-control, and reverse proxy docs.
- Caddy, Nginx, Docker, systemd, environment, secrets, and rollback examples.
- One-binary, split binary, backend-only, and WASM artifact smoke tests.

Release gate:

- A deployment-shaped example can run a generated binary behind a documented
  reverse proxy with health checks and rollback guidance.

### v0.13 Contracts, Workers, And Realtime

Focus: stabilize contract runtime roles and optional realtime adapters.

Must include:

- Contract model docs for commands, queries, events, jobs, roles, idempotency,
  retries, dead letters, replay, and observation names.
- Worker and cron binary generation plan or first implementation.
- Runtime role filtering docs/tests.
- File outbox, in-memory broker, Redis Streams, NATS, SSE, and WebSocket
  examples where supported.
- Contract CLI output docs/tests.

Release gate:

- Web command/query behavior and worker/replay behavior are explicit, optional,
  and do not replace action/API declarations prematurely.

### v0.14 CLI, Dev Server, LSP, And Native Docs

Focus: improve developer experience around current GOWDK-native workflows.

Must include:

- `gowdk doctor`.
- `gowdk explain`.
- `gowdk inspect` subcommands for IR, endpoints, assets, Go bindings,
  generated output, and dependencies where ready.
- `gowdk clean`, `gowdk env`, `gowdk version --json`, and `gowdk benchmark`
  where ready.
- Browser dev error overlay.
- Dev logs for rebuild reason, changed files, generated files, and restarts.
- LSP go-to-definition, hover, completions, and quick fixes for the highest
  value supported paths.
- Native docs and cookbook pages, with no migration guides and no framework
  comparison docs.

Release gate:

- A new user can learn GOWDK through native docs and debug common failures with
  CLI/LSP help.

### v0.15 Examples And Learning Path

Focus: prove GOWDK through native examples.

Must include:

- Capability examples kept current.
- Larger native examples for static sites, build data, layouts, actions,
  sessions/guards, dashboards, APIs, fragments, database usage, components,
  WASM islands, Tailwind, one-binary deploy, Docker, systemd, Caddy, contracts,
  SSE, and WebSocket where supported.
- One flagship full-stack native GOWDK example.
- Every example documents purpose, commands, expected output, feature status,
  what GOWDK owns, what Go owns, tests/smoke checks, generated artifact paths,
  and known limitations.
- Native learning path lessons 1-20.

Release gate:

- Examples are runnable by documented commands and aligned with current
  requirement statuses.

### v0.16 Testing, CI, And Performance

Focus: make quality gates broader and measurable.

Must include:

- Parser, route, form decoder, and URL escaping fuzzing where practical.
- Generated Go/HTML/CSS/schema tests.
- Action, API, fragment, SSR, hybrid, guard, CSRF, generated binary, browser
  runtime, SPA navigation, and WASM integration tests where supported.
- Split CI jobs and OS matrix.
- Docs link checks, Markdown lint, generated output determinism, release dry
  run, release artifact smoke workflow, and nightly extended workflows where
  practical.
- Build timing, latency, memory, binary size, generated JS/CSS size, WASM size,
  and compiler phase timing baselines.

Release gate:

- Release notes include reproducible quality and performance evidence, not just
  feature summaries.

### v0.17 Website, Playground, And Addon SDK

Focus: improve public onboarding while keeping execution sandboxed and optional.

Must include:

- Website install docs synced to release assets.
- Website current release badge, experimental warning, what works today, known
  gaps, cookbook, examples index, generated output preview, route manifest
  preview, and build report preview.
- Playground architecture with sandboxed execution and export/download.
- Playground examples for static page, build data, components, actions, APIs,
  fragments, and SSR where supported.
- Addon SDK docs for lifecycle, registration, config, hooks, generated file
  ownership, compatibility, security, and version handshake.
- Example addon and addon test harness.

Release gate:

- Public onboarding does not imply production readiness or require hosted
  execution to trust user code.

### v0.18 Compatibility And Dependency Cleanup

Focus: reduce surprises before broader adoption.

Must include:

- Public syntax compatibility audit.
- Config field compatibility audit.
- Runtime package compatibility audit.
- Generated-output shape audit.
- Manifest/build-report/CLI JSON audit.
- Dependency directness review for optional adapters and brokers.
- Deprecation notes for unstable behavior.
- Removal plan for stale compatibility records.
- Security, dependency, generated-output, and example audits.

Release gate:

- Every accepted public contract is tested or has explicit manual verification,
  and every unstable surface is marked experimental.

### v0.19 Hardening Candidate

Focus: stop adding broad surfaces and close known gaps.

Must include:

- No large new core features unless an ADR explains why they block the 0.x
  hardening goal.
- Release checklist gap burn-down.
- Requirements, architecture, docs, examples, release notes, and security policy
  consistency audit.
- Generated app smoke suite covering static pages, actions, APIs, fragments,
  SSR, guards, CSRF, cache policy, and deployment health where supported.
- Artifact, checksum, attestation, dependency, vulnerability, license, and docs
  evidence.

Release gate:

- There are no known stale statuses in the repository docs for supported
  behavior.

### v0.20 Stabilization Window

Focus: stabilize the 0.x surface based on real generated apps. This is not a
production-ready finish line.

Must include:

- Compatibility fixes from v0.19 feedback.
- Documentation corrections from real usage.
- Bug fixes and regression tests.
- Release process cleanup.
- Issue backlog triage for remaining partial/planned surfaces.
- Clear statement of what is still experimental, partial, planned, and out of
  scope.

Release gate:

- The release is trustworthy as an experimental 0.x release, with no hidden
  production claim and no undocumented major caveats for supported behavior.

## Standard Release Gates

Every 0.x minor release must have:

- [ ] Release notes that begin with "Experimental 0.x release" and "Not
  production-ready."
- [ ] Release notes split into implemented, partial, planned, intentionally out
  of scope, required verification, and known gaps.
- [ ] Current requirement statuses in `docs/product/requirements.md`.
- [ ] Current architecture notes in `docs/engineering/architecture.md`.
- [ ] Current CLI, config, generated-output, routing, deployment, and examples
  docs for changed behavior.
- [ ] Passing CI and current manual release gates from
  `docs/engineering/release.md`.
- [ ] Checksums and artifact attestation instructions in release notes.
- [ ] A release checklist link in release notes.
- [ ] A "no production claim" check before publishing.
- [ ] Draft and pre-release GitHub release settings unless the release policy is
  deliberately changed.

## Public Truth

- [ ] Verify `v0.1.5` is marked as a pre-release on GitHub.
- [ ] If GitHub displays it only as "Latest," update release settings or release
  wording.
- [ ] Add a release note template in `.github/`.
- [ ] Add "known limitations" to every release.
- [ ] Add "breaking/unstable generated output" to every release.
- [ ] Add CLI artifact verification instructions to release notes.
- [ ] Add checksum verification instructions to release notes.
- [ ] Add attestation verification instructions to release notes.
- [ ] Add VS Code `.vsix` install instructions to release notes.
- [ ] Add exact Go version requirement to release notes.
- [ ] Add exact Node version requirement for extension build/test to release
  notes.
- [ ] Create a public project board named `0.x Hardening`.
- [ ] Add waves to the board: Public truth, Release trust, Compiler spine, Go
  interop, Endpoint adapters, Secure runtime, SSR/hybrid, Components/islands,
  CSS/assets, DX/examples, and Ops/docs.
- [ ] Convert every `Partial` requirement into an issue.
- [ ] Convert every `Planned` roadmap item into an issue unless the item is
  intentionally tracked only in docs.
- [ ] Add labels for compiler, parser, IR, diagnostics, generated Go, runtime,
  actions, API, fragments, SSR, hybrid, components, client, WASM, CSS, assets,
  security, ops, docs, examples, LSP, dev server, release blocker, breaking
  change, good first issue, safe to try today, blocked by compiler IR, blocked
  by security hardening, and blocked by generated app runtime.
- [ ] Add issue templates for compiler bugs, generated output bugs, runtime
  bugs, security concerns, docs gaps, example requests, language proposals, and
  addon proposals.
- [ ] Add README links to known gaps, the release checklist, and the public
  hardening board.

## README And Getting Started

- [ ] Keep the pre-release warning near the top.
- [ ] Add "experimental 0.x" wording near the top.
- [ ] Add "public contracts may change" near the install section.
- [ ] Move "What works today," "What is partial," and "What is planned" higher.
- [ ] Add a compact support matrix for static build output, dynamic SPA paths,
  build-time Go data, actions, APIs, fragments, SSR, hybrid, components, WASM
  islands, CSS/assets, one-binary output, contracts, dev server, and LSP.
- [ ] Include matrix columns for stable enough to demo, not production
  security, docs available, example available, and tests available.
- [ ] Add direct links from matrix rows to docs, examples, or issues.
- [ ] Add project laws: generated Go is glue, normal Go owns app behavior, no
  mandatory npm, no mandatory framework adapter, no mandatory broker, and
  unsupported features produce diagnostics.
- [ ] Replace vague "full-stack" wording with concrete supported lanes.
- [ ] Keep install from GitHub release asset as the first install path.
- [ ] Keep build from source as the contributor/development path.
- [ ] Add Linux, macOS Intel, macOS ARM, and Windows install examples.
- [ ] Add checksum and attestation verification examples.
- [ ] Add `gowdk version` verification.
- [ ] Add `gowdk doctor` verification once implemented.
- [ ] Add troubleshooting for missing `gowdk.config.go`, missing Tailwind
  binary, unsupported Go handler signatures, missing SSR feature, and generated
  binary build failures.
- [ ] Warn that `gowdk serve` is static-output only and generated binaries are
  needed for actions, APIs, fragments, SSR, and hybrid runtime behavior.

## Release Trust

- [ ] Keep `go test ./...` in the release gate.
- [ ] Keep `govulncheck ./...` in the release gate.
- [ ] Keep `go build ./cmd/gowdk` in the release gate.
- [ ] Keep Node syntax checks and VS Code Node tests.
- [ ] Keep example `check`, `manifest`, `sitemap`, and `routes` gates.
- [ ] Add `gowdk version --json` check after building artifacts.
- [ ] Add smoke execution for each OS/arch artifact where possible.
- [ ] Add generated CLI artifact checksum verification after checksum file
  generation.
- [ ] Add generated binary HTTP smoke tests for static, SSR, action POST, API
  GET, fragment, hybrid, and WASM build paths.
- [ ] Add VS Code `.vsix` package existence check.
- [ ] Add release body validation for experimental warning, not-production-ready
  warning, known gaps, and checksum instructions.
- [ ] Add docs link checker.
- [ ] Add Markdown lint.
- [ ] Add generated docs sync check.
- [ ] Add generated output determinism check.
- [ ] Add `gofmt` check.
- [ ] Add `go vet ./...`.
- [ ] Add dependency, license, module graph, and dependency-size reports.
- [ ] Add security policy consistency check.
- [ ] Add examples README command consistency check.
- [ ] Add a "no migration docs" check if this becomes a hard policy.

## Toolchain And Dependency Policy

- [ ] Add `toolchain go1.26.4` if stronger local toolchain behavior is desired.
- [ ] Add `gowdk doctor` checks for Go version and required local tools.
- [ ] Add CI and release checks that print `go version` and `go env GOVERSION`.
- [ ] Document exact Go version requirements and future patch compatibility.
- [ ] Explain `govulncheck` in release docs.
- [ ] Keep `docs/engineering/dependency-policy.md` current.
- [ ] Classify dependencies as compiler core, runtime core, optional HTTP
  adapters, optional broker adapters, optional realtime adapters, optional
  CSS/tool adapters, or test/dev only.
- [ ] Explain why Gin, Echo, Fiber, Redis, NATS, and WebSocket packages are
  direct dependencies, or move them to optional submodules.
- [ ] Add CI checks for new direct dependencies.
- [ ] Add dependency diff, license report, vulnerability report, and module
  graph report to releases.
- [ ] Enforce no mandatory npm and no build-time downloads.
- [ ] Test that generated code does not import Gin, Echo, Fiber, Redis, or NATS
  by default.

## Security And Production-Safety Gates

- [ ] Update root `SECURITY.md` to match `docs/engineering/security.md`.
- [ ] Keep the production warning.
- [ ] Replace outdated "planned but not complete" wording with precise
  "first slice exists, not production enforcement" wording.
- [ ] List implemented first slices: generated action decoding, unexpected
  field rejection, direct literal request-shape validation, opt-in CSRF, action
  body cap, safe local redirect slice, guard execution slice, SSR panic
  boundaries, and no-store request-time responses.
- [ ] List incomplete production areas: auth/session policy, full guard
  contract, CSRF secret rotation, full redirect policy, log redaction, request
  timeout defaults, broad body/header limits, file upload policy, public API
  hardening, realtime security policy, and admin tooling policy.
- [ ] Enable GitHub private vulnerability reporting if available.
- [ ] Add a vulnerability report contact path.
- [ ] Add threat models for compiler diagnostics, generated logs, actions,
  APIs, fragments, SSR load, guards, generated assets, VS Code extension, WASM
  islands, and contracts/realtime.
- [ ] Add security checklist items to the PR template.
- [ ] Add security review trigger labels.
- [ ] Add generated `http.Server` timeout configuration: read, write, idle, and
  read-header timeouts.
- [ ] Add `MaxHeaderBytes`.
- [ ] Keep action request body caps and add API/fragment body caps where
  relevant.
- [ ] Add configurable body limits.
- [ ] Add explicit 405 responses.
- [ ] Ensure panic recovery wraps all generated request-time user Go.
- [ ] Ensure production-safe error pages and no stack traces in production mode.
- [ ] Prevent secret values in diagnostics and logs.
- [ ] Add log redaction for cookies, auth headers, CSRF tokens, passwords,
  secrets, sensitive form fields, and sensitive query params.
- [ ] Add secure headers middleware or docs for `X-Content-Type-Options`,
  `Referrer-Policy`, `Content-Security-Policy`, frame policy, and optional
  HSTS.
- [ ] Add cookie helper docs for `HttpOnly`, `Secure`, `SameSite`, path, and
  domain policy.
- [ ] Add safe redirect allowlists, open redirect tests, and unsafe external
  redirect diagnostics.
- [ ] Add embedded secret exclusion tests for `.env`, source maps with secrets,
  private files, and temporary artifacts.
- [ ] Add reverse proxy, TLS termination, request ID, health endpoint, and
  metrics endpoint security policy docs.

## Compiler Spine

- [ ] Keep lex -> parse -> AST -> analyze -> IR -> validate -> generate as
  strict phases.
- [ ] Finish downstream migration to `internal/gwdkir`.
- [ ] Make build output generation, app generation, CLI reports, and LSP
  metadata consume typed IR.
- [ ] Remove compatibility structs from long-term generation paths.
- [ ] Add IR, AST, generated Go, generated HTML, generated CSS, manifest, route
  report, endpoint report, component graph, and asset graph golden tests.
- [ ] Add source spans to every AST node and every IR node where possible.
- [ ] Add compiler invariant checks.
- [ ] Panic on invalid IR in tests.
- [ ] Add deterministic output, stale output cleanup, and unchanged-output
  preservation tests.
- [ ] Ban stringy generated Go except temporary documented exceptions.
- [ ] Move generated Go to `go/ast`, `go/printer`, and `go/format`.
- [ ] Add internal architecture docs for compiler passes.
- [ ] Add contributor guidance for new syntax requiring parser, formatter,
  diagnostic, IR, generation, docs, and example/fixture coverage.

## Parser, Formatter, Diagnostics, And Language Spec

- [ ] Add stable diagnostic codes.
- [ ] Add `gowdk explain <diagnostic-code>`.
- [ ] Make `gowdk check --json` a stable tooling contract.
- [ ] Add parser recovery so one syntax error does not hide the rest of the
  file.
- [ ] Add exact spans and suggestions for package declarations, imports, `use`,
  annotations, routes, layouts, render modes, `paths`, `build`, `load`, `view`,
  `style`, `client`, `go`, `go ssr`, `go client`, `go addon.*`, actions, APIs,
  fragments, component props, component state, and WASM declarations.
- [ ] Add suggestions for missing config, missing SSR feature, duplicate routes,
  unsupported handler signatures, missing exported Go symbols, invalid route
  params, unsupported build functions, unsupported component props, and missing
  Tailwind command.
- [ ] Add formatter idempotence and comment preservation tests.
- [ ] Add malformed syntax tests.
- [ ] Add parser, route matcher, view parser, and form decoder fuzz tests.
- [ ] Write a formal `.gwdk` language spec covering file kinds, package rules,
  Go imports, component `use`, layout references, asset references, addon
  references, annotations, blocks, expressions, view markup, component calls,
  slots, event bindings, class/style directives, `g:` directives, comments,
  reserved words, Go identifier mapping, route params, dynamic paths, raw HTML
  policy, unsupported syntax behavior, deprecation policy, and 0.x
  compatibility.
- [ ] Add grammar examples, invalid syntax examples, diagnostics examples, and a
  GOWDK-native mental model guide.

## Go Interop

- [ ] Make Go interop a first-class docs page.
- [ ] Add `gowdk inspect go-bindings`.
- [ ] Add `gowdk generate stubs`.
- [ ] Support build functions returning `(T, error)`.
- [ ] Support same-package build functions consistently.
- [ ] Support imported package aliases consistently.
- [ ] Support route params into build functions.
- [ ] Support `context.Context` for request-time functions.
- [ ] Support typed route params in `load`, APIs, actions, and fragments where
  relevant.
- [ ] Add Go symbol discovery reports.
- [ ] Add diagnostics for unsupported signatures, hidden-by-build-tags symbols,
  non-exported symbols, wrong packages, ambiguous imports, unsupported return
  types, unsupported parameter types, and JSON encoding failures for build data.
- [ ] Add examples using normal Go packages such as `database/sql`, `pgx`,
  `sqlc`, `slog`, session packages, validator packages, email packages,
  markdown packages, image processing packages, and queue packages.
- [ ] Keep serious app behavior in `.go` files.
- [ ] Keep inline `go {}` extractable and testable.
- [ ] Document that `.gwdk` calls supported Go contracts and is not arbitrary Go
  everywhere.
- [ ] Add tests for package path resolution, aliased imports, build tags,
  generated `gowdk_go/` packages, same-package handler discovery, and imported
  build-data errors.

## Routes, Layouts, View Engine, And HTML Safety

- [ ] Formalize route pattern grammar, route priority, trailing slash policy,
  encoded path handling, route params, typed params, and rest params if needed.
- [ ] Add route conflict diagnostics for page, API, action, and fragment
  combinations.
- [ ] Add route reports with route, render mode, params, guards, cache, layout
  stack, endpoints, source file, and generated output path.
- [ ] Add direct refresh, 404, path traversal, encoded param, static SPA,
  dynamic SPA `paths`, SSR, hybrid, API, action, and fragment route tests.
- [ ] Define layout composition, nested layout behavior, ordering, data rules,
  request-aware layouts, hybrid layouts, package-scoped layout imports, and
  qualified layout references or diagnostics.
- [ ] Add head and metadata support for title, route metadata, meta
  description, canonical URL, Open Graph, Twitter card, robots/noindex,
  sitemap metadata, and preload/prefetch declarations.
- [ ] Document supported HTML subset.
- [ ] Escape text and attributes by default.
- [ ] Define URL escaping, boolean attributes, class binding, style binding,
  event binding, form binding, and raw HTML policy.
- [ ] Add unsafe raw HTML diagnostics if an escape hatch is ever introduced.
- [ ] Add practical accessibility warnings for missing alt, missing labels,
  empty links, button type, and heading order.
- [ ] Add unsafe `href`, `src`, and `action` tests plus script and attribute
  injection tests.

## Endpoint Adapters, Actions, APIs, And Fragments

- [ ] Normalize actions, APIs, fragments, SSR loads, commands, and queries into
  one endpoint/contract metadata model where appropriate.
- [ ] Include source file, source span, kind, package path, package name, symbol,
  method, path, signature kind, input type, output type, guards, rate limit
  policy, CSRF policy, cache policy, and binding status.
- [ ] Add strict binding mode for production-shaped builds.
- [ ] Allow 501 stubs only behind an explicit flag.
- [ ] Add loud dev/migration compatibility mode only if needed.
- [ ] Add endpoint conflict diagnostics, endpoint report command, and endpoint
  graph output.
- [ ] Generate adapters from typed IR with deterministic imports, route
  registration, request decoding, and response writing.
- [ ] Test generated adapters for success, validation error, missing handler,
  unsupported handler, redirect, guard failure, CSRF failure, panic, no-store
  response, and method not allowed.
- [ ] Fully document action syntax, methods, form encoding, JSON support,
  direct file input rejection, multipart rejection, user-owned uploads, typed
  input decoding, scalar decoding, unknown field policy, missing/repeated field
  policy, checkbox/radio/select policy, submit intent, request-shape
  validation, domain validation handoff, validation error shape, partial
  validation fragments, redirects, reload outcomes, CSRF token placement,
  invalid CSRF response, and body limits.
- [ ] Add action examples for contact, newsletter, login, settings, validation
  fragments, redirects, and partial fragment responses.
- [ ] Document supported API signatures, context/request support, typed route
  params, typed query params, typed JSON bodies, typed responses,
  `response.Response`, optional `(T, error)` support, error-to-status mapping,
  content type, method not allowed, unsupported methods, and CORS.
- [ ] Add API examples for status, session, search, JSON CRUD, and webhooks with
  user-owned validation.
- [ ] Document fragments, standalone fragment routes, action-returned
  fragments, validation fragments, `g:target`, `g:swap`, swap modes, partial
  request headers, no-JS fallback, errors, focus restoration, island remounts,
  and no-store behavior.
- [ ] Add fragment examples for inline validation, table row update, list
  refresh, modal body update, and dashboard card refresh.

## SSR, Hybrid, Cache, Guards, And Auth Hooks

- [ ] Document SSR lifecycle, render mode, feature requirement, `load {}`
  grammar, declared load paths, typed route params, `(T, error)` load functions,
  `context.Context` load functions, redirects, not found, custom errors,
  route-local error pages, endpoint-local error pages, panic boundaries,
  guard-before-load ordering, layout-data merge, and cache policy.
- [ ] Add SSR examples for simple pages, dashboards, guarded account pages,
  dynamic detail pages, and route-local error pages.
- [ ] Document hybrid lifecycle, bare hybrid behavior, hybrid with and without
  `load`, SSR feature requirement, cache, revalidation, action invalidation,
  fragment refresh, and data refresh.
- [ ] Defer hybrid streaming until simpler behavior is stable.
- [ ] Add route/build report output that shows hybrid clearly.
- [ ] Document static asset, SPA HTML, SSR HTML, API, action, fragment, and
  hybrid cache policy.
- [ ] Document `@cache` and `@revalidate`.
- [ ] Add route report cache column and build report cache section.
- [ ] Test immutable asset cache, SPA `no-cache`, request-time `no-store`,
  `@cache`, `@revalidate`, and invalid `@revalidate`.
- [ ] Document guard syntax, signatures, registration, fail-closed missing guard
  behavior, and support matrix for SSR, actions, APIs, fragments, and hybrid.
- [ ] Document request context helpers for request, params, CSRF, session, and
  app context.
- [ ] Add user-owned session, cookie session, bearer token, admin role,
  guest-only page, JSON auth failure, redirect auth failure, and partial auth
  failure examples.

## Components, Client Language, SPA Navigation, And WASM

- [ ] Document component contracts, file structure, import/use rules, props,
  slots, events, state, lifecycle, CSS/assets, and unsupported behavior.
- [ ] Add required/default/boolean/numeric/string/object/array/imported Go
  struct prop support as contracts become stable.
- [ ] Add prop validation diagnostics.
- [ ] Add named slots and scoped slots only when syntax is stable.
- [ ] Add child-to-parent events, typed event payloads, bindable state, mount,
  update, cleanup, real `g:if`, `g:for`, keyed `g:for`, keyed DOM updates,
  recursion policy, and dynamic component policy.
- [ ] Add component snapshot and browser behavior tests.
- [ ] Add native component examples for buttons, text fields, cards, counters,
  tabs, modals, dropdowns, tables, pagination, toasts, form fields, and nav
  menus.
- [ ] Document `client {}` state, computed values, handlers, allowed/rejected
  expressions, dependency graph, cycles, batching, update order, cleanup, async
  policy, event policy, DOM patch policy, browser diagnostics, and production
  minification later.
- [ ] Test computed updates, class toggles, conditional DOM, event handlers,
  repeated state updates, cycles, cleanup, partial swap remounts, and SPA
  navigation remounts.
- [ ] Document static-first SPA navigation, link interception, external links,
  downloads, hash links, targets/new tabs, prefetch, route asset prefetch,
  scroll restoration, focus restoration, loading UI, error UI, and optional
  enhancement behavior.
- [ ] Add no-JS, direct refresh, browser back/forward, route swap, island
  remount, fragment remount, and generated JS size tests/reports.
- [ ] Document and version the WASM island ABI, required exports, optional
  cleanup, mount/remount, multiple instances, event bridge, DOM patch bridge,
  browser-unsafe imports, diagnostics, size reporting, asset manifest reporting,
  and `wasm_exec.js` version.
- [ ] Add WASM tests for compile success, missing export, bad export signature,
  unsafe imports, mount, event, patch, emit, cleanup, remount after fragment,
  and remount after SPA navigation.

## CSS, Assets, Packaging, Runtime, And Contracts

- [ ] Keep Tailwind optional, outside compiler/runtime core, and never
  downloaded during builds.
- [ ] Add tests proving no Tailwind download and clear missing Tailwind
  diagnostics.
- [ ] Document Tailwind installation through user-owned toolchains and
  `tailwind.Options.Command`.
- [ ] Document CSS processor API, page-aware processors, scoped component CSS,
  component `@css`, component `style {}`, layout `style {}`, component
  `@asset`, non-CSS assets, image/font/icon assets, asset manifest helpers,
  content hashing, immutable cache, CSS ordering, duplicate CSS warnings,
  unused CSS warnings, missing asset diagnostics, asset graph command, and
  `gowdk inspect assets`.
- [ ] Document generated app directory layout, binary layout, embedded output,
  module selection, target selection, split frontend/backend builds,
  backend-only builds, and deploy WASM versus browser island WASM.
- [ ] Add generated output ownership, file cleanup, stale cleanup,
  deterministic output, unchanged file preservation, binary size, generated
  source size, asset size, selected module, and embedded asset reports.
- [ ] Add one-binary, split binary, backend-only, and WASM artifact smoke tests.
- [ ] Keep generated apps `net/http` first.
- [ ] Document middleware registration, graceful shutdown, health/readiness,
  `/_gowdk/health`, metrics collectors, request counters, request IDs,
  structured logging hooks, future OpenTelemetry hooks, route logging, panic
  logging, static asset serving, 404/500 handling, compression, optional ETags,
  cache-control helpers, reverse proxies, trusted proxy/header policy, Caddy,
  Nginx, Docker, systemd, environment variables, secrets, and binary rollback.
- [ ] Document contract model, command/query/event/job signatures, one command
  owner, backend-owned domain events, presentation events as untrusted UI
  notifications, idempotency, retry, backoff, dead-letter, replay, runtime role
  filtering, and contract CLI output.
- [ ] Add worker binary generation and cron binary generation when the runtime
  role contract is ready.
- [ ] Add examples for signup email jobs, checkout commands, domain events,
  admin notifications, realtime dashboard updates, and background sync.

## CLI, Dev Server, LSP, Docs, And Examples

- [ ] Add `gowdk doctor`.
- [ ] Add `gowdk explain <diagnostic-code>`.
- [ ] Add `gowdk inspect ir`, `gowdk inspect endpoints`, `gowdk inspect assets`,
  `gowdk inspect go-bindings`, `gowdk inspect generated`, and
  `gowdk inspect deps`.
- [ ] Add `gowdk generate stubs`.
- [ ] Add `gowdk clean`, `gowdk env`, `gowdk version --json`, and
  `gowdk benchmark`.
- [ ] Improve JSON output for `check`, `routes`, `manifest`, and `sitemap`.
- [ ] Add build timing, binary size, generated file, stale cleanup, strict mode,
  stub mode, debug mode, and machine-readable build report schema docs.
- [ ] Add browser error overlay to `gowdk dev`.
- [ ] Show compiler errors, generated Go build errors, and dev-only runtime
  panics in the browser.
- [ ] Keep last successful build clearly visible.
- [ ] Log restart reason, changed files, rebuild timing, generated files
  changed, and generated binary rebuilds.
- [ ] Document backend proxy mode, `--app`, `preview`, and `--hot`.
- [ ] Add dev tests for no-op rebuilds, component changes, layout changes, CSS
  changes, config changes, backend process restarts, failed rebuild recovery,
  and generated app dev flow.
- [ ] Add LSP exact source-range diagnostics once spans are complete.
- [ ] Add go-to-definition for components, layouts, Go handlers, Go build
  functions, CSS inputs, and assets.
- [ ] Add hover docs, completions, quick fixes, tree views, graph views, build
  report viewer, generated output viewer, workspace health view, and extension
  compatibility docs.
- [ ] Add native docs for building static sites, full GOWDK apps, Go package
  interop, forms/actions, typed APIs, fragments, SSR pages, hybrid pages,
  guarded routes, components, WASM islands, Tailwind, one binary, deployment,
  generated Go, security, known gaps, when not to use GOWDK, troubleshooting,
  cookbook, language reference, CLI reference, config reference, runtime
  reference, addon reference, dependency policy, release process, and testing
  strategy.
- [ ] Do not add migration guides.
- [ ] Do not add "versus framework X" docs as core positioning.
- [ ] Keep capability examples and add larger native examples for static sites,
  build data, layouts, actions, session guards, dashboards, APIs, fragments,
  database usage, components, WASM islands, Tailwind, one-binary deploys,
  Docker, systemd, Caddy, contracts workers, SSE, and WebSocket.
- [ ] Add one flagship full-stack native GOWDK example with home page, login,
  cookie session, protected dashboard, SSR load, action submit, API route,
  fragment update, CSRF, guard, database package in normal Go, one-binary
  deploy, tests, and README.
- [ ] Require every example to include purpose, commands, expected output,
  feature status, what GOWDK owns, what Go owns, tests or smoke checks,
  generated artifact paths, and known limitations.

## Testing, CI, Operations, Performance, Playground, And Addons

- [ ] Keep `go test ./...`, CLI build tests, and VS Code tests.
- [ ] Add parser, route, form decoder, and URL escaping fuzzing.
- [ ] Add generated Go, HTML, CSS, manifest, sitemap, route report, and build
  report schema tests.
- [ ] Add action, API, fragment, SSR, hybrid, guard, CSRF, generated binary,
  generated WASM, browser client runtime, fragment, SPA navigation, and WASM
  island integration tests.
- [ ] Add performance, memory, binary size, generated output determinism, docs
  command, examples command, release checklist, and regression tests.
- [ ] Keep baseline CI fast and split jobs for Go unit tests, compiler tests,
  runtime tests, appgen tests, CLI tests, examples smoke, docs checks, VS Code
  tests, security scan, and dependency/license scan.
- [ ] Add OS matrix for Linux, macOS, and Windows.
- [ ] Add architecture matrix where useful for amd64 and arm64.
- [ ] Cache Go and Node dependencies properly.
- [ ] Add docs link check, Markdown lint, generated output determinism check,
  release dry run, release artifact smoke workflow, nightly extended examples,
  nightly fuzz/benchmark where practical, and branch protection once stable.
- [ ] Expand operations docs for static-only deploy, one-binary deploy, split
  frontend/backend deploy, backend-only deploy, Docker, systemd, Caddy, Nginx,
  environment variables, secrets, CSRF secrets, logs, metrics, health,
  readiness, graceful shutdown, cache/CDN, binary rollback, artifact layout,
  backup/restore as user responsibility, incident response as user
  responsibility, dependency update policy, and observability TODOs.
- [ ] Add performance benchmarks for cold build, incremental build, dev rebuild,
  generated binary startup, static route latency, SSR latency, action latency,
  API latency, fragment latency, memory, binary size, generated JS size,
  generated CSS size, and WASM size.
- [ ] Add compiler phase timing for discovery, parse, analyze, IR, validate,
  generate, write, and `go build`.
- [ ] Add build timing to build reports and `gowdk benchmark`.
- [ ] Update website install docs to match release assets and sync website docs
  from the repo automatically.
- [ ] Add website current release badge, experimental warning, what works today,
  known gaps, cookbook, examples index, runnable snippets, generated output
  preview, route manifest preview, build report preview, and website link
  checker.
- [ ] Document addon lifecycle, registration, config, compiler hooks, runtime
  hooks, generated file ownership, version compatibility, security
  restrictions, CSS processor addon, Tailwind addon, rate-limit addon, embed
  addon, SSR addon, partial addon, contracts addon, addon test harness, example
  addon, incompatible/missing addon diagnostics, version handshake, and addon
  docs examples.

## Native Learning Path

- [ ] Lesson 1: install GOWDK.
- [ ] Lesson 2: create a page.
- [ ] Lesson 3: add build-time Go data.
- [ ] Lesson 4: add a component.
- [ ] Lesson 5: add CSS/assets.
- [ ] Lesson 6: add an action.
- [ ] Lesson 7: add validation.
- [ ] Lesson 8: add CSRF.
- [ ] Lesson 9: add an API.
- [ ] Lesson 10: add a fragment.
- [ ] Lesson 11: add SSR.
- [ ] Lesson 12: add a guard.
- [ ] Lesson 13: use a database from Go.
- [ ] Lesson 14: build one binary.
- [ ] Lesson 15: deploy behind Caddy.
- [ ] Lesson 16: inspect generated Go.
- [ ] Lesson 17: troubleshoot diagnostics.
- [ ] Lesson 18: add tests.
- [ ] Lesson 19: add optional Tailwind.
- [ ] Lesson 20: add optional WASM island.

## Priority Queue

Start with these in order:

- [ ] Verify release metadata shows experimental/pre-release correctly.
- [ ] Open public issue backlog.
- [ ] Add `0.x Hardening` project board.
- [ ] Update website install docs for release binaries.
- [ ] Sync root `SECURITY.md` with deeper security baseline.
- [ ] Keep dependency policy current and add missing enforcement.
- [ ] Add license/dependency scan to CI.
- [ ] Add release note template.
- [ ] Add `gowdk doctor`.
- [ ] Add `gowdk explain`.
- [ ] Add `gowdk inspect go-bindings`.
- [ ] Add `gowdk generate stubs`.
- [ ] Stabilize `gowdk check --json`.
- [ ] Add diagnostic codes.
- [ ] Add exact source spans where missing.
- [ ] Finish downstream `gwdkir` migration.
- [ ] Add generated Go golden tests.
- [ ] Add endpoint IR report.
- [ ] Add strict production-shaped binding mode.
- [ ] Add generated app HTTP smoke tests.
- [ ] Add CSRF secret docs.
- [ ] Add safe redirect tests.
- [ ] Add guard contract docs and tests.
- [ ] Add request timeout, header limit, and body limit support.
- [ ] Build flagship full-stack native GOWDK example.
- [ ] Build deployment-shaped example.
- [ ] Add native cookbook.
- [ ] Add browser dev error overlay.
- [ ] Add VS Code quick fix for creating a missing Go handler.
- [ ] Add performance/build timing report.

## Do Not Add For Now

- [ ] Do not add migration guides.
- [ ] Do not add "GOWDK vs X" docs as core positioning.
- [ ] Do not make SSR default.
- [ ] Do not make full-page hydration default.
- [ ] Do not make browser JavaScript the app contract.
- [ ] Do not generate user domain logic.
- [ ] Do not generate auth or business validation logic.
- [ ] Do not auto-discover endpoints by function name.
- [ ] Do not scan Gin/Echo/Fiber route registrations as route truth.
- [ ] Do not require npm.
- [ ] Do not require Tailwind.
- [ ] Do not require Redis.
- [ ] Do not require NATS.
- [ ] Do not require Gin, Echo, or Fiber.
- [ ] Do not download optional tools during builds.
- [ ] Do not hide partial features behind confident wording.
- [ ] Do not add more syntax without diagnostics, tests, docs, and examples.

## Direction

```text
Make the current 0.x surface trustworthy.
Make the compiler spine boring.
Make Go interop excellent.
Make generated adapters strict and readable.
Make security warnings precise.
Make examples native to GOWDK.
Do not expand into comparison or migration positioning.
```
