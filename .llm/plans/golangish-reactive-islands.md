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
  fn SetQuery(value string) {
    Query = value
  }

  computed HasQuery bool {
    return Query != ""
  }

  computed VisibleItems []ui.Item {
    return filter(Items, item => contains(lower(item.Name), lower(Query)))
  }
}

view {
  <input value="{Query}" g:on:input={SetQuery(event.value)} />

  <p g:if={HasQuery}>{len(VisibleItems)} results</p>

  <ul>
    <li g:for={item in VisibleItems} g:key={item.ID}>{item.Name}</li>
  </ul>
}
```

The example shows target direction. Each syntax piece must land in a separate
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
- [ ] Add expression source spans.
- [x] Type-check against props, state, and params.
- [x] Type-check against computed values, locals, and built-ins.
- [x] Lower expression behavior to generated JavaScript.
- [x] Add diagnostics for unsupported operators, unknown fields, and type
  mismatch.
- [ ] Add diagnostics for invalid nil use beyond scalar comparison.

Tests:

- [x] Parse valid expression fixtures.
- [ ] Reject unsupported expressions with spans.
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
- [ ] Emit stable comment/marker anchors.
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

- [ ] `async fn`
- [ ] `await` only inside async functions
- [ ] compiler-owned `fetchJSON[T]`
- [ ] loading/error conventions
- [ ] cancellation or stale-response guard

Compiler work:

- [ ] Type-check async functions separately.
- [ ] Generate Promise-based JS.
- [ ] Add optional AbortController support.
- [ ] Define safe JSON decode expectations.

Tests:

- [ ] Async function sets loading and result.
- [ ] Error path sets error state.
- [ ] Stale response does not overwrite newer state.

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

- [ ] `emits { event(args...) }`
- [ ] `emit event(args...)`
- [ ] parent event listeners on component calls
- [ ] typed payloads

Compiler work:

- [ ] Add event contract metadata to manifest.
- [ ] Validate child emit calls.
- [ ] Validate parent listeners and payload fields.
- [ ] Generate CustomEvent dispatch/listen code.

Tests:

- [ ] Child emits event and parent state updates.
- [ ] Unknown event rejected.
- [ ] Payload type mismatch rejected.

### 13. Parent/Child State Passing

Goal: allow parent state to flow into child props without two-way mutation.

Syntax:

```gwdk
<UserCard name={SelectedName} active={Open} />
```

Support:

- [ ] reactive prop expressions on component calls
- [ ] child receives updated prop values
- [ ] props remain read-only in child client logic
- [ ] no implicit child mutation of parent state

Compiler work:

- [ ] Track component call prop expressions.
- [ ] Generate parent update code for child prop changes.
- [ ] Decide whether child rerender is marker-based or function-based.

Tests:

- [ ] Parent state update changes child displayed prop.
- [ ] Child cannot assign to prop.
- [ ] Missing child prop remains a compile error.

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

- [ ] compiler-owned island root markers
- [ ] binding IDs for text, attrs, classes, styles, conditionals, and lists
- [ ] keyed list updates
- [ ] fragment replacement reattaches islands
- [ ] no full-page hydration

Compiler work:

- [ ] Assign stable binding IDs during view compilation.
- [ ] Emit compact binding table into JS asset.
- [ ] Generate per-binding update functions.
- [ ] Add island mount registry.
- [ ] Integrate with partial runtime after server fragment swaps.

Tests:

- [ ] Text/attr/list updates share one scheduler.
- [ ] Partial swap remounts new islands.
- [ ] Removed island runs destroy hook.

### 17. Source Maps And Debug Output

Goal: make generated JS debuggable.

Support:

- [ ] readable generated function names
- [ ] optional unminified dev output
- [ ] source maps from `.gwdk` source spans to generated JS
- [ ] CLI flag or config for dev/prod asset mode

Compiler work:

- [ ] Preserve source spans in client AST.
- [ ] Add generated JS source map writer.
- [ ] Record source map assets in `gowdk-assets.json`.

Tests:

- [ ] Source map JSON validates.
- [ ] Generated JS includes sourceMappingURL in dev mode.

### 18. Production WASM Island ABI

Goal: define what explicit WASM islands actually execute.

Support:

- [ ] WASM island entrypoint naming convention
- [ ] props/state bootstrap ABI
- [ ] event dispatch ABI
- [ ] DOM update ABI or host callback ABI
- [ ] lifecycle ABI
- [ ] asset naming and loader strategy

Compiler work:

- [ ] Add ADR before implementation.
- [ ] Generate or validate Go WASM entrypoints.
- [ ] Emit loader that passes bootstrap data.
- [ ] Decide whether WASM owns DOM updates or calls host JS helpers.

Tests:

- [ ] WASM island receives initial state.
- [ ] WASM island handles click event.
- [ ] WASM island updates visible state.
- [ ] JS default and WASM explicit modes can coexist on one page.

### 19. Browser-Side Go Logic

Goal: allow explicit WASM islands to run real user-authored browser Go when the
ABI exists.

Support:

- [ ] user package contract for WASM island functions
- [ ] build tags or target selection for browser-only Go
- [ ] import restrictions for browser-safe packages
- [ ] compile diagnostics for unsupported packages

Compiler work:

- [ ] Discover browser Go entry packages.
- [ ] Run `GOOS=js GOARCH=wasm go build` per explicit island target.
- [ ] Record emitted WASM assets.
- [ ] Surface Go build errors as GOWDK diagnostics.

Tests:

- [ ] Valid browser Go island builds.
- [ ] Unsupported package import fails clearly.
- [ ] Missing entrypoint fails clearly.

### 20. Diagnostics For Unsupported Syntax

Goal: make the compiler strict and helpful as the language grows.

Support:

- [ ] diagnostic codes for every unsupported feature
- [ ] source spans in `client {}` and view bindings
- [ ] suggestions for common mistakes
- [ ] JSON diagnostics for editor tooling

Compiler work:

- [ ] Extend parser recovery around `client {}`.
- [ ] Store spans in client AST.
- [ ] Add diagnostic code docs.
- [ ] Add LSP validation for client syntax.

Tests:

- [ ] Unsupported JS function call points to exact expression.
- [ ] Unknown state field points to exact identifier.
- [ ] Bad `g:for` syntax points to directive value.
- [ ] Diagnostics appear in `gowdk check --json`.

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
- [ ] update scheduler

### Phase C: Product UI Workflows

- [x] two-way form bindings
- [x] event modifiers
- [x] list rendering
- [ ] parent-to-child reactive props
- [ ] child-to-parent typed events

### Phase D: Advanced Runtime

- [ ] lifecycle hooks
- [ ] effects
- [ ] async functions
- [ ] partial swap island remounting
- [ ] debug source maps

### Phase E: Explicit WASM

- [ ] WASM ABI ADR
- [ ] bootstrap ABI
- [ ] event ABI
- [ ] DOM update ABI
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
