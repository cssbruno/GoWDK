# Implementation Plan: Golangish Reactive Islands

## Context

Relevant spec: `.llm/features/golangish-reactive-islands.md`

Relevant ADR: `docs/engineering/decisions/0003-js-default-explicit-wasm-islands.md`

The current island implementation supports only:

- `Field++`
- `Field--`
- `Field = scalar`
- `Field = OtherField`
- `Field = !OtherField`
- `{Field}` text binding

That is useful as a proof, but it is not enough. The next step is a Go-like
client language that compiles to generated JavaScript by default.

## Assumptions

- The language should feel familiar to Go developers but must not be full Go.
- State and props contracts continue to come from imported Go structs.
- `client {}` logic is component-local in the first production slice.
- Generated JavaScript remains the default local island runtime.
- WASM remains explicit per component instance with `g:island="wasm"`.
- Unsupported syntax is always a compile error.

## Proposed Syntax Baseline

```gwdk
@component SearchBox

import ui "github.com/acme/app/ui"

props ui.SearchProps
state ui.SearchState = ui.NewSearchState()

client {
  computed HasQuery bool {
    return Query != ""
  }
}

view {
  <input g:bind:value={Query} />

  <p g:if={HasQuery}>Filtering</p>

  <ul>
    <li
      g:for={item in Items}
      g:key={item.ID}
      g:if={Query == "" || contains(lower(item.Name), lower(Query))}
    >
      {item.Name}
    </li>
  </ul>
}
```

The example shows the currently supported filter pattern. Broader collection
helpers such as `filter(...)` remain future work and must land in a separate
small slice with tests.

## Feature Checklist

### 1. General Expressions

Goal: replace the current tiny event-expression parser with a typed expression
AST.

Syntax:

```gwdk
client {
  fn IncrementBy(step int) {
    Count = Count + step
  }

  computed Status string {
    return if Count > 0 { "active" } else { "empty" }
  }
}
```

Support:

- [x] literals: string, int, float, bool, nil
- [x] unary operators: `!`, `-`
- [x] binary operators: `+`, `-`, `*`, `/`, `%`
- [x] comparisons: `==`, `!=`, `<`, `<=`, `>`, `>=`
- [x] boolean logic: `&&`, `||`
- [x] parentheses
- [x] simple field reads: `Count`
- [x] nested field reads: `User.Name`
- [x] index reads: `Items[0]`
- [x] ternary or Go-ish alternative decision:
  - option A: `cond ? a : b`
  - option B: `if cond { a } else { b }` selected and implemented

Compiler work:

- [x] Add expression lexer/parser under `internal/clientlang`.
- [x] Build a typed AST.
- [x] Add expression source spans.
- [x] Type-check against props, state, and params.
- [x] Type-check against computed values, locals, and built-ins.
- [x] Lower expression behavior to generated JavaScript.
- [x] Add diagnostics for unsupported operators, unknown fields, and type
  mismatch.
- [x] Add diagnostics for invalid nil use beyond scalar comparison.

Tests:

- [x] Parse valid expression fixtures.
- [x] Reject unsupported expressions with spans.
- [x] Compile arithmetic, comparison, and boolean expressions to JS.
- [x] Compile nested field reads to JS.

### 2. Component Functions

Goal: replace inline event expressions with named component-local functions.

Syntax:

```gwdk
client {
  fn Increment() {
    Count++
  }

  fn Select(id string) {
    SelectedID = id
  }
}

view {
  <button g:on:click={Increment()}>+</button>
}
```

Support:

- [x] first slice: `fn Name() { ... }` with no params
- [x] typed params: `fn Select(id string)`
- [x] local variables: `let next int = Count + 1`
- [x] assignments to state fields only for the existing safe expression subset
- [x] return values only for internal helper functions, not event handlers
- [x] call graph validation
- [x] handler-to-handler calls are rejected in the first slice, which also
  prevents recursion

Compiler work:

- [x] Parse `client {}` no-argument function declarations.
- [x] Type-check supported param declarations and event-call arity.
- [x] Type-check return values and broader expression calls.
- [x] Type-check scalar locals in ordered client statement blocks.
- [x] Validate first-slice statements against component state fields.
- [x] Mark declared functions callable from `g:on:*`.
- [x] Generate handler dispatch data consumed by the generated JS island
  runtime.
- [x] Preserve current inline expression support as sugar.

Tests:

- [x] Event handler calls generated function.
- [x] Event handler passes scalar or state-field arguments into generated
  function params.
- [x] Unknown function call is a compile error.
- [x] Function mutating props is a compile error.
- [x] Function body referencing unknown state is a compile error.
- [x] Recursive function is rejected by disallowing function calls inside
  function bodies in the first slice.

### 3. Derived/Computed State

Goal: let views depend on derived values without storing duplicate state.

Syntax:

```gwdk
client {
  computed FullName string {
    return FirstName + " " + LastName
  }

  computed IsOpenClass string {
    return if Open { "open" } else { "closed" }
  }
}
```

Support:

- [x] `computed Name Type { return expr }`
- [x] dependencies on props, state, and computed values
- [x] automatic recompute after state mutation
- [x] cycle detection
- [x] no mutation inside computed blocks

Compiler work:

- [x] Parse computed declarations.
- [x] Build dependency graph.
- [x] Reject cycles and mutation.
- [x] Generate recompute function that updates bindings.

Tests:

- [x] Computed text binding updates after event.
- [x] Computed depending on computed updates correctly.
- [x] Cycle is rejected.
- [x] Mutation inside computed is rejected.

### 4. Conditional Rendering

Goal: support show/hide or mount/unmount UI without full hydration.

Syntax:

```gwdk
view {
  <p g:if={Open}>Visible</p>
  <p g:else>Hidden</p>
}
```

Support:

- [x] `g:if={boolExpr}`
- [x] optional `g:else`
- [x] optional `g:else-if={boolExpr}` after first slice
- [x] static fallback HTML from initial state
- [x] client updates after state mutation

Compiler work:

- [x] Parse conditional directives in `internal/view`.
- [x] Validate condition type is bool.
- [x] Emit stable comment/marker anchors.
- [x] Generate DOM update code for condition changes using `hidden`.

Tests:

- [x] Initial true renders expected static HTML.
- [x] Initial false omits or hides element according to chosen strategy.
- [x] Event toggles condition.
- [x] Invalid non-bool condition is rejected.

Decision needed:

- [x] Choose hide/show with `hidden` for first slice or real mount/unmount.
  Hide/show is simpler and preserves static HTML; mount/unmount is closer to
  framework behavior.

### 5. List Rendering

Goal: render arrays and update them predictably.

Syntax:

```gwdk
view {
  <li g:for={item in Items} g:key={item.ID}>{item.Name}</li>
}
```

Support:

- [x] `g:for={item in Items}`
- [x] required `g:key={item.ID}` for non-static arrays
- [x] item field interpolation
- [x] index variable after first slice: `item, i in Items`
- [x] append/remove/reorder updates

Compiler work:

- [x] Resolve list element type from Go slice/array fields.
- [x] Add loop scope to view binding validation.
- [x] Generate keyed DOM update code.
- [x] Reject missing keys for mutable lists.

First implementation note: the current JS runtime emits list markers and
refreshes rows from the latest state during normal render passes. It reuses and
reorders existing row elements by `g:key`, then removes stale rows. The client
language supports compiler-owned list mutation built-ins:
`append(Items, { Field: expr })`, `remove(Items, index)`, and
`move(Items, from, to)`.

Tests:

- [x] Initial list renders from state.
- [x] Event appends item and DOM updates.
- [x] Event removes item and DOM updates.
- [x] Missing key is rejected.
- [x] Unknown item field is rejected.

### 6. Attribute Bindings

Goal: update attributes from state and computed values.

Syntax:

```gwdk
view {
  <button disabled={Saving} aria-expanded={Open}>{Label}</button>
  <img src={AvatarURL} alt={Name} />
}
```

Support:

- [x] string attributes
- [x] boolean attributes
- [x] numeric attributes converted to strings
- [x] ARIA attributes
- [x] first-slice safe URL attribute checks by rejecting reactive URL attrs

Compiler work:

- [x] Distinguish static attrs from reactive attrs.
- [x] Type-check attr expression against expected attr kind.
- [x] Generate attr update code.
- [x] Preserve route-param taint protections by rejecting reactive URL attrs
  until sanitizer rules exist.

Tests:

- [x] Boolean attr toggles.
- [x] ARIA attr updates.
- [x] URL attr rejects unsafe route-param/client interpolation where needed.
- [x] Unknown field is rejected.

### 7. Class And Style Bindings

Goal: support common visual state without string-heavy code.

Syntax:

```gwdk
view {
  <button class:active={Open} class:error={HasError}>Save</button>
  <div style:height.px={PanelHeight}></div>
}
```

Support:

- [x] `class:name={boolExpr}`
- [x] multiple class toggles
- [x] `style:name={expr}`
- [x] style unit suffixes such as `.px`, `.rem`; percent uses `.%`

Compiler work:

- [x] Extend attr parser for `class:` directive names.
- [x] Extend attr parser for `style:` directive names.
- [x] Validate class toggles are bool.
- [x] Validate style values are scalar.
- [x] Generate classList update code.
- [x] Generate style update code.

Tests:

- [x] Class toggles on event.
- [x] Multiple toggles do not overwrite static classes.
- [x] Style updates with unit.
- [x] Non-bool class expression rejected.
- [x] Bool style expression rejected.

### 8. Two-Way Form Bindings

Goal: make form controls update component state.

Syntax:

```gwdk
view {
  <input g:bind:value={Query} />
  <input type="checkbox" g:bind:checked={Enabled} />
  <select g:bind:value={SelectedID}>...</select>
}
```

Support:

- [x] `g:bind:value` for text inputs
- [x] `g:bind:value` for textareas and selects
- [x] `g:bind:checked` for checkboxes
- [x] number parsing for numeric state fields on `<input type="number">`
- [x] radio-group binding with `input type="radio" g:bind:value`
- [x] validation diagnostics for unsupported controls
- [x] state-to-control and control-to-state synchronization

Compiler work:

- [x] Parse first-slice `g:bind:value`.
- [x] Validate target is a writable string state field.
- [x] Generate event listeners and update functions.
- [x] Avoid fighting normal `g:post` form behavior.

Tests:

- [x] Typing updates text state and dependent text binding.
- [x] Textarea/select value bindings render initial state.
- [x] Checkbox updates bool state.
- [x] Number input parses numeric state.
- [x] Radio group updates string state.
- [x] Binding to prop is rejected.

### 9. Event Modifiers

Goal: cover common event behavior declaratively.

Syntax:

```gwdk
view {
  <form g:on:submit.prevent={Save()}>
  <button g:on:click.stop.once={Close()}>Close</button>
  <input g:on:input.debounce(250ms)={Search(event.value)} />
}
```

Support:

- [x] `.prevent`
- [x] `.stop`
- [x] `.once`
- [x] `.capture`
- [x] `.debounce(duration)`
- [x] `.throttle(duration)`

Compiler work:

- [x] Extend directive parser for modifier chains.
- [x] Validate modifier compatibility.
- [x] Generate listener options/wrappers.
- [x] Parse duration literals.

Tests:

- [x] Prevent default is lowered.
- [x] Stop propagation is lowered.
- [x] Once handler is lowered.
- [x] Debounce duration is parsed and lowered.
- [x] Throttle duration is parsed and lowered.

### 10. Lifecycle Hooks And Effects

Goal: run controlled code when state changes or the island mounts/unmounts.

Syntax:

```gwdk
client {
  on mount {
    Focused = true
  }

  effect when Query {
    Dirty = true
  }
}
```

Support:

- [x] `on mount`
- [x] `on destroy`
- [x] `effect when Field`
- [x] cleanup return after first slice
- [x] no arbitrary DOM access initially

Compiler work:

- [x] Parse lifecycle/effect blocks.
- [x] Validate effects mutate only state.
- [x] Generate mount/destroy registration.
- [x] Re-run effects after dependency changes.

Tests:

- [x] Mount hook is lowered and registered once.
- [x] Effect runs after dependency change.
- [x] Effect cycle is guarded.

### 11. Async State

Goal: support browser-local async workflows without turning the language into
arbitrary JS.

Syntax:

```gwdk
client {
  async fn Search() {
    Loading = true
    result := await fetchJSON[[]ui.Item]("/api/search?q=" + Query)
    Items = result
    Loading = false
  }
}
```

Support:

- [x] `async fn`
- [x] `await` only inside async functions
- [x] compiler-owned `fetchJSON[T]`
- [x] loading/error conventions
- [x] cancellation or stale-response guard

Compiler work:

- [x] Type-check async functions separately.
- [x] Generate Promise-based JS.
- [x] Add optional AbortController support.
- [x] Define safe JSON decode expectations.

Tests:

- [x] Async function sets loading and result.
- [x] Error path sets error state.
- [x] Stale response does not overwrite newer state.

First implementation note: async fetches use a per-target stale token and an
optional `AbortController` to cancel superseded requests. `fetchJSON` sends an
`Accept: application/json` header, rejects non-2xx status, rejects non-JSON
content types, returns `null` for empty JSON responses, and reports invalid JSON
with a GOWDK-owned error. Runtime conventions clear `Error` before fetch and
set `Error` plus `Loading=false` on failures when those fields exist.

Decision needed:

- [ ] Whether async should wait until typed `api {}` handlers are mature.

### 12. Component Communication

Goal: let child islands communicate upward without global state.

Syntax:

```gwdk
@component Child

emits {
  select(id string)
}

view {
  <button g:on:click={emit select(ID)}>Select</button>
}

// caller
<Child g:on:select={SelectedID = event.id} />
```

Support:

- [x] `emits { event(args...) }`
- [x] `emit event(args...)`
- [x] parent event listeners on component calls
- [x] typed payloads

Compiler work:

- [x] Add event contract metadata to manifest.
- [x] Validate child emit calls.
- [x] Validate parent listeners and payload fields.
- [x] Generate CustomEvent dispatch/listen code.

Tests:

- [x] Child emits event and parent state updates.
- [x] Unknown event rejected.
- [x] Payload type mismatch rejected.

First implementation note: component event contracts now flow from parser to
manifest, validation, view lowering, and the generated JS island runtime.
Generated tests verify markup/runtime wiring and diagnostics. A Chromium
verification against a disposable `gowdk build` output confirmed a child
`emit select(id)` updates parent state through `g:on:select`.

### 13. Parent/Child State Passing

Goal: allow parent state to flow into child props without two-way mutation.

Syntax:

```gwdk
<UserCard name={SelectedName} active={Open} />
```

Support:

- [x] reactive prop expressions on component calls
- [x] child receives updated prop values
- [x] props remain read-only in child client logic
- [x] no implicit child mutation of parent state

Compiler work:

- [x] Track component call prop expressions.
- [x] Generate parent update code for child prop changes.
- [x] Decide whether child rerender is marker-based or function-based.

Tests:

- [x] Parent state update changes child displayed prop.
- [x] Child cannot assign to prop.
- [x] Missing child prop remains a compile error.

First implementation note: reactive component prop expressions lower to
`data-gowdk-props` on the child island wrapper. Parent islands dispatch
`gowdk:props` updates after rerender, child islands merge those fields into
local state and rerender, and parent runtimes now avoid binding nested child
island internals. A Chromium verification against a disposable `gowdk build`
output confirmed parent state changes update the child's displayed prop.

### 14. Stores And Shared State

Goal: support state shared by several islands without a global SPA.

Syntax option:

```gwdk
store cart ui.CartState = ui.NewCartState()

client {
  use cart
}
```

Support:

- [ ] page-scoped stores first
- [ ] module-scoped stores later
- [ ] explicit `use storeName`
- [ ] no hidden globals
- [ ] generated JS store asset per page/module

Compiler work:

- [ ] Add store declarations to parser/manifest.
- [ ] Type-check store fields.
- [ ] Generate store runtime.
- [ ] Wire island subscriptions.

Tests:

- [ ] Two components read same store.
- [ ] One component mutates store and other updates.
- [ ] Missing `use` is rejected.

Decision needed:

- [ ] Delay stores until component communication and parent props are stable.

### 15. DOM Refs

Goal: expose limited DOM access for focus and measurement without arbitrary JS.

Syntax:

```gwdk
client {
  ref searchInput HTMLInputElement

  fn FocusSearch() {
    searchInput.Focus()
  }
}

view {
  <input g:ref={searchInput} />
}
```

Support:

- [x] `ref name ElementKind`
- [x] `g:ref={name}`
- [x] safe methods only: `Focus`, `Blur`, `ScrollIntoView`
- [x] no arbitrary DOM traversal in first slice

Compiler work:

- [x] Parse ref declarations.
- [x] Validate one DOM binding per ref.
- [x] Generate ref assignment on mount.
- [x] Generate wrappers for allowed methods.

Tests:

- [x] Ref focus function compiles and calls focus.
- [x] Unknown ref rejected.
- [x] Ref used before binding rejected or nullable.

### 16. Hydration/Reconciliation

Goal: move from simple text replacement to a stable generated DOM update model.

Support:

- [x] compiler-owned island root markers
- [x] binding IDs for text, attrs, classes, styles, conditionals, and lists
- [x] keyed list updates
- [x] fragment replacement reattaches islands
- [x] no full-page hydration

Compiler work:

- [x] Assign stable binding IDs during view compilation.
- [x] Emit compact binding table into JS asset.
- [x] Generate per-binding update functions.
- [x] Add island mount registry.
- [x] Integrate with partial runtime after server fragment swaps.

Implementation note: the current slice emits compiler-owned `bN` attributes
and the generated JS builds an in-memory binding table from them on mount and
rerender.

Tests:

- [x] Text/attr/list updates share one scheduler.
- [x] Partial swap remounts new islands.
- [x] Removed island runs destroy hook.

Second implementation note: generated JS islands now enqueue post-mutation
updates through one microtask scheduler. Text, attribute/class/style/form, list,
conditional, and child-prop sync updates still run through the shared render
pipeline; only the initial mount render remains immediate.

Third implementation note: island roots now carry stable `data-gowdk-island`
markers. Generated JS island assets register idempotent component mount
functions in a browser-global registry, and the partial-update runtime calls the
global mount hook after `innerHTML` or `outerHTML` swaps so new fragment islands
can attach without full-page hydration. Destroy cleanup for removed islands
runs through the same registry before partial swaps replace existing island
roots.

Fourth implementation note: generated JS now routes the collected binding table
through per-binding update functions for text, values, checked state, classes,
styles, attributes, conditionals, and lists.

Fifth implementation note: keyed list rendering now evaluates each `g:key`
expression during rerender, reuses existing keyed DOM nodes through
`syncElement`, and removes keyed nodes that are no longer present in state.
Generated JavaScript mounts only matching `<gowdk-island>` roots; it does not
hydrate or replace `document.body` or `document.documentElement`.

Sixth implementation note: generated JS island assets now include a compact
`bindingTable` descriptor that centralizes direct binding selectors and
class/style/attribute binding prefixes. Runtime collection uses that table to
build per-root binding entries from compiler-owned binding IDs, preserving
dynamic list support while avoiding hardcoded per-kind scans.

### 17. Source Maps And Debug Output

Goal: make generated JS debuggable.

Support:

- [x] readable generated function names
- [x] optional unminified dev output
- [ ] source maps from `.gwdk` source spans to generated JS
- [x] CLI flag or config for dev/prod asset mode

Compiler work:

- [x] Preserve source spans in client AST.
- [x] Add generated JS source map writer.
- [x] Record source map assets in `gowdk-assets.json`.

Tests:

- [x] Source map JSON validates.
- [x] Generated JS includes sourceMappingURL in dev mode.

First implementation note: generated JavaScript island assets now emit
companion `<Component>.js.map` files with source map v3 JSON, component `.gwdk`
source names, and source content. The source map assets are written and recorded
in `gowdk-assets.json`, and generated JS includes a `sourceMappingURL` comment.
Span-accurate mappings remain open.

Second implementation note: `gowdk.BuildConfig.Mode` now controls debug asset
output. The default/development mode emits JavaScript island source maps and
`sourceMappingURL` comments; `gowdk.Production` omits `.js.map` artifacts,
manifest entries, and JS source-map comments. Span-accurate mappings remain
open.

Third implementation note: generated JavaScript island entry points now use
component-specific names such as `mountCounterIsland` and
`destroyCounterIsland`, so browser stack traces and devtools call frames point
at the component owning the generated runtime.

Fourth implementation note: development/default build mode keeps generated
JavaScript island output formatted and readable. Production mode applies a
conservative generated-source compaction pass that trims indentation and blank
lines without rewriting JavaScript tokens. Span-accurate mappings remain open.

Fifth implementation note: generated JavaScript source maps now include
first-slice mappings from generated component identity, mount, and render helper
lines back to the component declaration, `client {}`, and `view {}` source
spans when those spans are available. Finer per-expression and per-binding
generated mappings remain open.

Sixth implementation note: generated JavaScript source maps now anchor broader
runtime regions to source spans: statement execution, computed recomputation,
and island mounting map to `client {}`, while binding tables, binding
collection, conditional/list rendering, and binding updaters map to `view {}`.
Finer per-expression and per-binding generated mappings remain open.

### 18. Production WASM Island ABI

Goal: define what explicit WASM islands actually execute.

Support:

- [x] WASM island entrypoint naming convention
- [x] props/state bootstrap ABI
- [x] event dispatch ABI
- [x] DOM update ABI or host callback ABI
- [x] lifecycle ABI
- [x] asset naming and loader strategy

Compiler work:

- [x] Add ADR before implementation.
- [x] Generate or validate Go WASM entrypoints.
- [x] Emit loader that passes bootstrap data.
- [x] Decide whether WASM owns DOM updates or calls host JS helpers.

Tests:

- [x] WASM island receives initial state.
- [x] WASM island handles click event.
- [x] WASM island updates visible state.
- [x] JS default and WASM explicit modes can coexist on one page.

First implementation note: ADR 0004 defines a JS-hosted WASM island ABI with
component-scoped exports, JSON bootstrap data, host-captured events, validated
patch-list DOM updates, lifecycle calls, and stable island asset names. This is
an ABI decision only; code generation and browser-side Go validation remain
open.

Second implementation note: explicit WASM island loaders now collect
component-local `state`, `props`, `emits`, `refs`, and binding metadata into the
bootstrap object passed to `GOWDKMount<Component>` when that export exists. The
loader also captures compiler-bound DOM events for `GOWDKHandle<Component>`,
calls `GOWDKDestroy<Component>` on pagehide, and applies first-slice host patch
commands for text, hidden state, attributes, classes, styles, and emitted
events. Real user-authored Go WASM entrypoint generation/validation remains
open.

Third implementation note: components can now declare `@wasm <package>` to use
a real browser-side Go package for explicit `g:island="wasm"` calls. GOWDK
builds the package with `GOOS=js GOARCH=wasm` and rejects packages that do not
produce a browser WASM module. Full export/registration validation for the ADR
entrypoint names remains open.

### 19. Browser-Side Go Logic

Goal: allow explicit WASM islands to run real user-authored browser Go when the
ABI exists.

Support:

- [x] user package contract for WASM island functions
- [x] build tags or target selection for browser-only Go
- [x] import restrictions for browser-safe packages
- [ ] compile diagnostics for unsupported packages

Compiler work:

- [x] Discover browser Go entry packages.
- [x] Run `GOOS=js GOARCH=wasm go build` per explicit island target.
- [x] Record emitted WASM assets.
- [ ] Surface Go build errors as GOWDK diagnostics.

Tests:

- [x] Valid browser Go island builds.
- [x] Unsupported package import fails clearly.
- [x] Missing entrypoint fails clearly.

First implementation note: `@wasm <package>` on a component declares the
browser-side Go package used by explicit WASM island calls for that component.
The static generator compiles the package with `GOOS=js GOARCH=wasm`, writes
the resulting module to `assets/gowdk/islands/<Component>.wasm`, records it in
`gowdk-assets.json`, and fails clearly when the package produces a Go archive
instead of a browser WASM module or when `go build` rejects imports.
Browser-safe import restriction policy and ADR export validation remain open.

Second implementation note: local `@wasm` packages now get a first browser-safe
import policy before build. GOWDK rejects server/process/network packages such
as `net`, `net/http`, `os/exec`, `database/sql`, `plugin`, raw `syscall`, and
`unsafe` with component-scoped errors. ADR export validation and stable
diagnostic codes for these build errors remain open.

### 20. Diagnostics For Unsupported Syntax

Goal: make the compiler strict and helpful as the language grows.

Support:

- [ ] diagnostic codes for every unsupported feature
- [ ] source spans in `client {}` and view bindings
- [x] suggestions for common mistakes
- [x] JSON diagnostics for editor tooling

Compiler work:

- [ ] Extend parser recovery around `client {}`.
- [x] Store spans in client AST.
- [x] Add diagnostic code docs.
- [x] Add LSP validation for client syntax.

Tests:

- [x] Unsupported JS function call points to exact expression.
- [x] Unknown state field points to exact identifier.
- [x] Bad `g:for` syntax points to directive value.
- [x] Diagnostics appear in `gowdk check --json`.

First implementation note: `clientlang.Program` now keeps source spans for
functions, statements, lifecycle blocks, effects, and computed expressions;
`clientlang` expression AST nodes also keep 1-based expression-column spans.
Statement validation errors carry the failing statement index, and expression
validation errors carry the failing expression span, so compiler diagnostics and
`gowdk check --json` can report the offending client statement line and exact
columns for deterministic expression positions. `docs/reference/diagnostics.md`
documents current JSON fields, ranges, and known diagnostic codes. View-binding
spans remain open.

Second implementation note: component view validation now maps selected
directive and interpolation substrings back to `view {}` source ranges.
Unsupported `g:on:*` event expressions point at the failing expression, unknown
view fields point at the identifier, and malformed `g:for` directives point at
the directive value. Broader parser recovery, suggestions, and exhaustive
view-binding span coverage remain open.

Third implementation note: in-memory checks now classify page, component,
layout, asset, and plugin buffers before parsing. LSP diagnostics therefore
validate unsaved `.cmp.gwdk` component buffers through the compiler and publish
client syntax diagnostics with the same source ranges as `gowdk check --json`.
Broader parser recovery and exhaustive view-binding span coverage remain open.

Fourth implementation note: JSON diagnostics now include an optional
`suggestion` field for common fixable mistakes: missing SSR addons, dynamic
static routes without `paths {}`, load blocks on build-time render modes,
unknown client/view fields, unknown event handlers, unknown emitted events,
malformed `g:for`, and missing `g:key`. Parser recovery, exhaustive diagnostic
codes, and broader view-binding span coverage remain open.

Fifth implementation note: client parser failures now carry line metadata when
the parser can identify the offending `client {}` body line. Compiler
diagnostics map malformed client syntax to that line in the source file instead
of the whole client block. Multi-error parser recovery remains open.

## Phasing

### Phase A: Language Core

- [x] Expression AST
- [x] `client {}` block parser
- [x] component functions
- [x] computed values
- [ ] strict diagnostics

### Phase B: DOM Binding Core

- [x] text bindings
- [x] attr bindings
- [x] class/style bindings
- [x] conditional rendering
- [x] update scheduler

### Phase C: Product UI Workflows

- [x] two-way form bindings
- [x] event modifiers
- [x] list rendering
- [x] parent-to-child reactive props
- [x] child-to-parent typed events

### Phase D: Advanced Runtime

- [x] lifecycle hooks
- [x] effects
- [x] async functions
- [x] partial swap island remounting
- [ ] debug source maps

### Phase E: Explicit WASM

- [x] WASM ABI ADR
- [x] bootstrap ABI
- [x] event ABI
- [x] DOM update ABI
- [ ] real browser-side Go package build

## Files Expected To Change

- `.llm/features/golangish-reactive-islands.md`
- `.llm/plans/golangish-reactive-islands.md`
- `docs/language/components.md`
- `docs/language/markup.md`
- `docs/language/syntax.md`
- `docs/language/diagnostics.md`
- `docs/compiler/generated-output.md`
- `docs/engineering/architecture.md`
- `docs/engineering/decisions/*`
- `internal/parser`
- `internal/view`
- `internal/compiler`
- `internal/gotypes`
- `internal/staticgen`
- `internal/lang`
- `internal/lsp`
- `runtime/*` only for public generated-runtime contracts

## Data And API Impact

- Static output keeps HTML-first behavior.
- Default JS assets stay under `assets/gowdk/islands/`.
- `gowdk-assets.json` records generated JS, source maps, WASM, and loaders.
- Public manifest should eventually include component client capabilities for
  tooling.
- No npm package API is introduced.

## Verification Commands

```sh
go test ./internal/parser ./internal/view ./internal/compiler ./internal/staticgen
go test ./internal/lang ./internal/lsp
go test ./...
GOMODCACHE=/tmp/gowdk-go-mod-cache GOCACHE=/tmp/gowdk-go-build-cache go build -o /tmp/gowdk-build-check ./cmd/gowdk
```

## Rollback Plan

- Keep existing scalar event-expression support.
- Remove `client {}` parsing and generated JS lowering for the failed slice.
- Remove new directive support slice by slice.
- Keep typed props/state contracts because they are already useful for static
  rendering and current islands.

## Risks

- The language can become too much like JavaScript if expression support grows
  without constraints.
- The language can become too much like Go if it inherits packages, pointers,
  goroutines, and browser-hostile semantics.
- List reconciliation can introduce subtle UI bugs; keyed lists should be
  required before mutable lists ship.
- Async support can race user input; cancellation or stale-response guards are
  required.
- WASM can become confusing unless docs keep JS as default and WASM as explicit.
