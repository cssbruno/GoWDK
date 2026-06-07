# ADR 0004: Production WASM Island ABI

Date: 2026-06-05

Status: Accepted

## Context

ADR 0003 keeps generated JavaScript as the default island runtime and makes
WASM explicit through component-level `@wasm` declarations, with
`g:island="wasm"` retained as a call-site override. The compiler needs a stable
ABI for bootstrapping state, passing props, dispatching events, lifecycle calls,
and DOM updates.

The ABI must preserve the compile-first model:

- App-shell HTML is still the initial rendered output.
- JavaScript remains the host that discovers island roots and loads WASM.
- WASM is opt-in per component, with call-site override support.
- Components must not require full-page hydration.

## Decision

Use a JS-hosted WASM island ABI. The generated loader owns DOM discovery,
bootstrap decoding, event listener attachment, lifecycle scheduling, and DOM
patch application. The Go WASM module owns component-local state transitions and
returns compiler-defined patch commands to the host.

Entrypoint naming:

- The generated loader looks for exported functions named
  `GOWDKMount<Component>`, `GOWDKHandle<Component>`, and
  `GOWDKDestroy<Component>`.
- Exported names are component-scoped to avoid a registry in the first slice.
- Missing required exports are compile or load diagnostics, not silent no-ops.

Bootstrap ABI:

- The loader passes one JSON object to `GOWDKMount<Component>`.
- The object contains `component`, `state`, `props`, `emits`, `refs`, and
  `bindings`.
- `state` is the same JSON object used by JS islands.
- `props` contains initial prop values and reactive prop expression names.
- `bindings` is the compiler-owned table of text, attribute, class, style,
  conditional, list, and event binding IDs.

Event ABI:

- DOM events are captured by the JS host.
- The host calls `GOWDKHandle<Component>` with `{ event, binding, detail }`.
- `event` is the DOM event name or component event name.
- `binding` is the compiler-assigned binding ID.
- `detail` contains scalar event payload fields.
- Component emits are returned as patch commands of type `emit`, and the host
  dispatches `CustomEvent` with the typed payload.

DOM update ABI:

- The WASM module does not directly mutate the DOM.
- WASM returns a JSON patch list. The JS host validates and applies patches.
- Initial patch operations are `setText`, `setAttr`, `removeAttr`,
  `toggleClass`, `setStyle`, `setHidden`, `replaceList`, and `emit`.
- Patch targets are compiler-owned binding IDs, not CSS selectors.

Lifecycle ABI:

- The host calls `GOWDKMount<Component>` once per island root.
- The host calls `GOWDKDestroy<Component>` when the island root is removed or on
  pagehide before unload.
- Future effect cleanup uses explicit patch/lifecycle return values rather than
  ambient goroutines.

Asset strategy:

- Component WASM stays at `assets/gowdk/islands/<Component>.wasm`.
- The loader stays at `assets/gowdk/islands/<Component>.wasm.js`.
- Multiple component instances share the same WASM module asset but receive
  separate bootstrap objects.
- JS and WASM islands may coexist on the same page.

## Consequences

### Positive

- The JS host keeps DOM mutation small, inspectable, and consistent with the
  generated JavaScript island runtime.
- WASM components can use real Go logic without taking over the whole page.
- Binding IDs give future partial swaps and remounts a common target model.
- Event dispatch stays consistent across JS and WASM islands.

### Negative

- The host must validate patch lists to avoid corrupting DOM state.
- The generated loader remains necessary even when the state logic lives in
  WASM.
- Exported function naming is simple but may need a registry if components are
  bundled together later.

### Neutral

- This ADR defines the ABI only. It does not require immediate implementation
  of user-authored browser-side Go packages.
- The first implementation can support a subset of patch operations as long as
  unsupported operations fail clearly.

## Alternatives Considered

- Let WASM directly mutate the DOM through `syscall/js`. Rejected because it
  duplicates host logic, makes partial remounting harder, and weakens
  compiler-owned binding guarantees.
- Use one global registry export. Rejected for the first production slice
  because component-scoped exports are simpler to validate and debug.
- Serialize HTML fragments from WASM. Rejected because it would bypass stable
  binding IDs and make fine-grained updates harder to reason about.

## Implementation

- GOWDK builds declared `@wasm` packages with `GOOS=js GOARCH=wasm`.
- Built WASM artifacts are rejected unless they export
  `GOWDKMount<Component>`, `GOWDKHandle<Component>`, and
  `GOWDKDestroy<Component>`.
- The generated loader passes the bootstrap object, applies the defined patch
  operations, rejects unknown patch operations through a console error, and
  supports JS/WASM island coexistence on the same page.

## Follow-Up

- Add browser tests for mount, event handling, visible state update, destroy,
  and JS/WASM coexistence against a real browser runtime.
