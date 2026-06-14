# Browser-Facing Output

GOWDK does not compile arbitrary Go or JavaScript for the browser by default.
The current browser-facing output slices are:

- Partial form enhancement runtime emitted as `assets/gowdk/gowdk.js`.
- Generated JavaScript islands for stateful components.
- Component-level WASM island asset emission for components that declare
  `wasm`, with `g:island="wasm"` still supported as a call-site override.

Framework-owned browser runtime source lives under `internal/clientrt/assets/`
as ordinary `.js` files embedded with `go:embed`. Buildgen consumes those
sources through small template helpers for component names, page IDs, asset
paths, and WASM export names; it should not grow new multi-hundred-line
JavaScript raw strings in Go files.

## Partial Runtime

When a page uses a fragment-producing action form with `g:target` and `g:swap`,
SPA builds emit the client runtime:

```gwdk
<form g:post={Refresh} g:target="#patients" g:swap="innerHTML">
  <input name="query" />
  <button>Refresh</button>
</form>
```

The compiler lowers this to normal form attributes plus `data-gowdk-*`
metadata and emits a deferred script tag for `assets/gowdk/gowdk.js`.

The runtime:

- Submits enhanced forms with `X-GOWDK-Partial`, `X-GOWDK-Target`, and
  `X-GOWDK-Swap` headers.
- Applies `innerHTML` or `outerHTML` swaps.
- Reloads the current page when the response carries `X-GOWDK-Reload: 1`.
- Dispatches `gowdk:before-request`, `gowdk:after-swap`, and
  `gowdk:request-error`. Failed enhanced requests include response `status`,
  `body`, and `response` in event detail when available.
- Toggles `aria-busy` on the submitting form.
- Restores focus by matching the active element's `id` or `name` when possible.
- Calls generated island destroy and mount hooks around replaced island DOM.

## JavaScript Islands

Stateful components use generated JavaScript by default:

```gwdk
component Counter

import ui "github.com/acme/app/ui"

state ui.CounterState = ui.NewCounterState()

client {
  fn Increment() {
    Count++
  }
}

view {
  <button g:on:click={Increment()}>{Count}</button>
}
```

The compiler validates a small Go-like client subset and emits browser
JavaScript under `assets/gowdk/islands/`: one shared `island.js` runtime plus
small package-scoped component registration stubs.

Supported island syntax is documented in `docs/language/syntax.md`,
`docs/language/markup.md`, and `docs/language/components.md`. The subset
includes scalar handler parameters, scalar locals, field assignment,
increment/decrement, arithmetic, comparisons, boolean logic, conditional
expressions, computed values, lifecycle/effect blocks, refs, simple bindings,
conditionals, keyed list rendering, and compiler-owned list/string/numeric
built-ins.

Unsupported today:

- Arbitrary Go syntax in the browser.
- Arbitrary JavaScript.
- Loops in client handlers.
- Event object reads.
- Broad date/time/browser APIs.
- Recursive helper functions.
- User-defined browser runtime imports.

## Component-Level WASM Islands

Components declare WASM at the component level:

```gwdk
component Counter
wasm ./browser/counter

view {
  <button>Counter</button>
}
```

```gwdk
<Counter />
```

When `wasm` points to a local package, GOWDK builds that package with
`GOOS=js GOARCH=wasm`. The package must be browser-safe and cannot import
server/process/network packages such as `net/http`, `os/exec`, `database/sql`,
raw `syscall`, `plugin`, or `unsafe`. Declared Go WASM island packages also
ship `assets/gowdk/islands/wasm_exec.js`; the generated component loader first
tries a direct WASM instantiate path and falls back to Go runtime imports when a
compiled Go module needs them.

Declared browser-side Go packages must produce a browser WASM module and export
the component-scoped ABI entrypoints with the current `func() uint32`
signature:

```go
//go:wasmexport GOWDKMountCounter
func GOWDKMountCounter() uint32 { return 0 }

//go:wasmexport GOWDKHandleCounter
func GOWDKHandleCounter() uint32 { return 0 }

//go:wasmexport GOWDKDestroyCounter
func GOWDKDestroyCounter() uint32 { return 0 }
```

The generated loader passes a `gowdk-wasm-island-v1` bootstrap object containing
component name, state, props, emits, refs, and compiler-owned binding metadata.
Event and destroy payloads carry the same ABI version. Returned patch lists may
use `setText`, `setAttr`, `removeAttr`, `toggleClass`, `setStyle`,
`setHidden`, `replaceList`, and `emit`; unsupported patch operations are
rejected with a console error. Missing required exports and startup failures are
reported to the browser console instead of silently disabling the island.

Normal calls to a component with `wasm` use the WASM island runtime. If a
component is called with `g:island="wasm"` and no `wasm` package is declared,
GOWDK emits the current placeholder module plus loader shape.

See `examples/components/wasm/` for a runnable component-level WASM ABI example
that emits the component `.wasm`, host loader, and `wasm_exec.js` assets.

## Production Mode

`Build.Mode` affects generated island assets:

```go
Build: gowdk.BuildConfig{
	Mode: gowdk.Production,
	Output: "dist/site",
}
```

Development mode emits JavaScript island source maps. Production mode omits
`.js.map` artifacts and `sourceMappingURL` comments and compacts generated
island JavaScript. `Build.ObfuscateAssets` or `gowdk build --obfuscate-assets`
enables stronger deterministic minification/identifier shortening for
compiler-owned generated browser JavaScript in production builds.
