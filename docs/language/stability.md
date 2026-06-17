# Language Construct Stability

This table is the per-construct stability and deprecation contract for the
experimental 0.x `.gwdk` language. The diagnostics registry already records a
stability tier per diagnostic code (`internal/diagnostics/registry.go`); this
page does the same for the language constructs themselves, so a user or tooling
author can tell which syntax is safe to depend on and which is still moving.

It complements, and is pinned by, the machine-checked
[Conformance Corpus](conformance.md): a `Stable` or `Partial` construct should
have an `accept/` case, and a `Planned`/`Deprecated` construct should have a
`reject/` case asserting the diagnostic code named below.

## Status Tiers

- **Stable**: accepted by the current compiler and not expected to change shape
  within 0.x without a deprecation step.
- **Partial**: accepted for a narrower slice than the final contract; the syntax
  is real but its capability will grow.
- **Planned**: not accepted as source behavior yet; using it is rejected with
  the listed diagnostic code so it cannot become accidental behavior.
- **Deprecated**: previously accepted spelling that is now rejected with a
  migration diagnostic.

The tiers below are the code-level registry `lang.ConstructStabilities()` (with
metadata keywords derived from `lang.MetadataKeywords` and directives checked
against `view.SupportedDirectiveNames()`). `TestStabilityRegistryCoversCodeConstructs`
asserts the registry covers every keyword and directive in code, and
`TestStabilityTableMatchesRegistry` asserts this page matches the registry, so
neither the table nor the registry can drift without failing a test.

## Top-Level Blocks

| Construct | Tier | Notes |
| --- | --- | --- |
| `package` | Stable | Required first declaration. |
| `import` | Stable | Go import for colocated blocks. |
| `use` | Stable | Package-scoped component import. |
| `paths {}` | Partial | Literal `=> { field: "value" }` records only. |
| `build {}` | Partial | Literal records and no-argument Go calls. |
| `server {}` | Partial | Request-time server-lane data; requires the SSR addon. |
| `view {}` | Stable | Markup; see directives below. |
| `style {}` | Stable | Scoped CSS body. |
| `client {}` | Partial | Bounded component client language. |
| `go {}` / `go build {}` / `go server {}` / `go client {}` / `go addon.* {}` | Partial | Colocated Go lanes. |
| `store` / `props` / `state` / `emits` | Partial | Component contracts. |
| Unknown top-level block | Planned | Rejected with `unsupported_top_level_block`. |

## Metadata Keywords

All metadata keywords are **Stable**. The canonical list is `lang.MetadataKeywords`.

| Keyword | Tier |
| --- | --- |
| `page` | Stable |
| `route` | Stable |
| `title` | Stable |
| `description` | Stable |
| `canonical` | Stable |
| `image` | Stable |
| `robots` | Stable |
| `noindex` | Stable |
| `preload` | Stable |
| `prefetch` | Stable |
| `layout` | Stable |
| `cache` | Stable |
| `revalidate` | Stable |
| `error` | Stable |
| `guard` | Stable |
| `css` | Stable |
| `component` | Stable |
| `wasm` | Stable |
| `asset` | Stable |

Legacy `@`-prefixed metadata is **Deprecated** and rejected with
`malformed_legacy_metadata`.

## View `g:` Directives

Supported exact-name directives (the closed set in
`view.SupportedDirectiveNames()`):

| Directive | Tier | Notes |
| --- | --- | --- |
| `g:if` | Stable | Conditional render. Server-side over a `server {}` field; a client island over state/store. `g:else-if`/`g:else` are client-only chains. |
| `g:for` / `g:key` | Stable | List render. Server-side over a `server {}` field; a client island over state/store. The lane is inferred from the operand. |
| `g:bind:value` / `g:bind:checked` | Partial | Two-way bindings. |
| `g:on:*` | Partial | Event handlers with `.prevent`/`.stop`/`.once`/`.capture`/`.debounce`/`.throttle`. |
| `g:post` / `g:target` / `g:swap` | Partial | Progressive form/fragment submission. |
| `g:message:*` | Partial | `required`, `minlength`, `maxlength`, `pattern`. |
| `g:island` | Partial | `js` or `wasm` island. |
| `g:command` / `g:query` | Partial | Contract web adapters. |
| `g:subscribe` | Partial | Realtime presentation-event subscription metadata on query-owned elements. |
| `g:event` | Partial | Parses to explain backend-owned domain events. |
| `g:unsafe-html` | Stable | Raw HTML escape hatch; `unsafe_raw_html` is reported. |
| `g:ref` | Partial | Client reference. |
| `g:slot` | Partial | Named/scoped slot. |

Component calls also accept `g:bind:<ExportedState>` for exported child state
fields. HTML elements remain limited to `g:bind:value` and `g:bind:checked`.

Planned directives are rejected. They currently surface as the generic
`parse_error` rather than the intended `unsupported_markup_directive` code; that
code lands when markup rejections carry their own code (see
[Conformance Corpus](conformance.md)).

| Directive family | Tier | Replacement |
| --- | --- | --- |
| `g:transition`, `g:animate` | Planned | CSS transitions or a future addon. |
| `g:window`, `g:document`, `g:body`, `g:head` | Planned | Page metadata or `g:on:*` on elements. |
| `g:await`, `g:async` | Planned | build/load data, actions, APIs, fragments. |
| `g:use`, `g:action`, `g:attach` | Planned | `client {}` with `g:ref`. |

Foreign template syntax (`{#if}`, `{@html}`, and similar) is **Planned/Unsupported**
and likewise currently surfaces as `parse_error` (intended:
`unsupported_markup_syntax`).

## Endpoint Declarations

| Construct | Tier | Notes |
| --- | --- | --- |
| `act` | Stable | `act <Name> POST "<path>"`; POST only today. |
| `api` | Stable | `api <Name> <METHOD> "<path>"`; GET/POST/PUT/PATCH/DELETE. |
| `fragment` | Partial | First-slice partial updates. |
| `act` block form | Deprecated | `act <name> { ... }`; rejected with `old_action_block_syntax`. |
| `api` block form | Deprecated | `api <name> { ... }`; rejected with `old_api_block_syntax`. |
