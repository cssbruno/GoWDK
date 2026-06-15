# Security Threat Model

This baseline covers the current GOWDK Compiler plus GOWDK Runtime repository
surfaces called out by the M5 secure endpoint runtime work. It is not a
production-readiness claim. It records the main trust boundaries, abuse paths,
current controls, and open follow-up areas for review.

## Assumptions

- Generated apps may be exposed to untrusted HTTP clients when users deploy the
  generated binary.
- User application Go code owns authentication, authorization, persistence,
  domain validation, audit policy, and incident response.
- GOWDK-generated code owns adapter glue: request decoding, route dispatch,
  generated CSRF checks, response envelopes, panic boundaries, embedded asset
  serving, and contract web adapters.
- During 0.x, unsupported or incomplete behavior must remain explicit in docs,
  diagnostics, or tracked issues.

## Assets

- User source code, generated Go source, build artifacts, embedded assets, and
  release artifacts.
- Secrets and credentials in config, environment variables, cookies, headers,
  form fields, logs, diagnostics, and source-adjacent files.
- Integrity of generated route, endpoint, contract, and asset metadata.
- Availability of generated HTTP handlers, contract workers, and local dev
  tooling.

## Boundaries

| Boundary | Entry Points | Current Controls | Open Work |
| --- | --- | --- | --- |
| `.gwdk` source to compiler diagnostics | Parser, analyzer, `gowdk check`, LSP diagnostics | Stable diagnostic registry, source spans where available, redaction policy for secret-like values | Broader exact spans and diagnostic expansion remain tracked outside M5. |
| Generated logs and panic sinks | `runtime/app` panic boundaries, contract worker logs | Panic responses avoid stack traces; recovered panic logs pass through secret redaction | Broader app-owned logging guidance and redaction coverage remain planned. |
| Browser/client to action endpoints | POST forms, enhanced partial forms, command form adapters | Expected-field decoding, direct literal validation, configurable action body cap defaulting to 1 MiB, default CSRF, 405 on wrong methods, no-store error responses | Broader upload policy, per-route limits, and full production CSRF rotation remain planned. |
| Browser/client to API endpoints | Generated API routes, contract query routes | Method dispatch, configurable API body cap defaulting to 1 MiB, generated CSRF for state-changing API methods, rate-limit hook when addon is enabled | Public API hardening, typed helper expansion, and per-route policy are tracked in #24. |
| Browser/client to fragments | Standalone fragments, action fragment responses | Fragment routing through generated handlers, escaped render core, no-store request-time responses; standalone fragments are GET-only and action fragments share the action cap | Broader auth/session policy remains planned. |
| Browser/client to SSR `load {}` | Request-time SSR routes, route-local error pages | SSR feature gate, guard execution, safe local redirect helpers, panic boundaries, no-store failures | Full guard contract, route-local auth/session policy, and richer request-time error policy remain planned. |
| Guard metadata to user authorization | `guard` declarations, `GOWDKGuardRegistry`, `GOWDKAuthProvider` | Guards run before generated request-time user logic; native RBAC helpers are defense-in-depth only | Backend resource authorization remains app-owned; full guard response contract is planned. |
| Embedded build output to generated server | Embedded SPA assets, generated error pages, health endpoint | Generated server uses HTTP timeouts and `MaxHeaderBytes`; embedded output skips known secret/private/temp artifacts | Broader asset policy remains planned. |
| VS Code extension to workspace | LSP/editor commands and workspace file access | Dependency-light local tooling; no production runtime authority | Extension command/file threat model needs focused review before broader editor automation. |
| WASM islands to browser runtime | `go client {}`, component WASM assets, host loader | WASM is explicit and separate from backend handlers; browser-unsafe import validation exists | Production ABI hardening and user-code runtime validation remain planned. |
| Contracts and realtime adapters | Command/query web adapters, workers, outbox, broker, fanout | Web-role validation, CSRF before command decoding, guard/rate-limit preflight, local default dispatch | Split worker/cron wiring, retry policy, managed deployment recipes, and realtime security policy remain planned. |

## Abuse Paths

| Abuse Path | Impact | Current Mitigation | Priority |
| --- | --- | --- | --- |
| Submit unexpected action fields to overwrite handler input. | Integrity of action input. | Generated decoders reject unexpected fields and skip runtime fields such as CSRF. | Medium until broader typed helper contracts stabilize. |
| Send large request bodies to exhaust memory or handler time. | Availability of generated servers. | Generated action/API adapters cap bodies with configurable app-level limits; generated server entrypoints set HTTP timeouts and max-header defaults. | Medium because per-route limits remain planned. |
| Reuse or forge generated CSRF tokens. | Cross-site action, command, or state-changing API execution. | Generated CSRF is enabled by default for generated action POSTs, command POSTs, and state-changing APIs, and validates before decoding or user handlers run. | Medium while secret rotation and deployment guidance continue to harden. |
| Trigger handler panics and read stack traces or secret values. | Secret exposure and debugging data leakage. | Runtime panic boundaries avoid stack traces in responses and redact recovered-panic logs. | Medium because app-owned logs are outside generated control. |
| Use unsafe redirects to move users off-site. | Phishing or token leakage through redirects. | First slices require safe local redirects for generated action/SSR redirect paths. | Medium until full redirect allowlists and diagnostics are complete. |
| Embed local secrets into generated output. | Secret exposure in release artifacts. | Docs require generated output to avoid local env/private files. | High until exclusion tests cover `.env`, source maps, private files, and temp artifacts. |
| Expose public APIs without auth, rate limits, or validation. | Data exposure, mutation, or availability loss. | Generated adapters stay framework-neutral and can use guard/rate-limit hooks. | High until public API helper and policy work is complete. |
| Treat guards as full backend authorization. | Authorization bypass in user domain code. | Docs state guards are route gates and backend authorization stays in app Go. | High if users rely on guards alone for protected resources. |
| Replay contract or realtime events incorrectly. | Duplicate state changes or stale UI state. | Local default dispatch and worker ack/nack helpers exist. | Medium until retry/dead-letter/realtime deployment policies are complete. |
| Let editor automation operate on untrusted workspaces without review. | Local file or command abuse. | Current editor surface is limited. | Medium before adding broader workspace automation. |

## Review Triggers

Apply the `security review` GitHub label to issues or pull requests that touch:

- authentication, authorization, sessions, CSRF, redirects, cookies, or headers;
- generated action/API/fragment/SSR request handling;
- request body/header limits, file uploads, parsing, or decoding;
- generated logs, diagnostics, error pages, or panic boundaries;
- embedded assets, source maps, release artifacts, or secret exclusion;
- VS Code commands, workspace file access, WASM islands, contracts, workers,
  queues, brokers, SSE, WebSocket, or realtime behavior.

## Follow-Up Areas

- Add per-route request body/header limit policy.
- Finish public API helper hardening in #24.
