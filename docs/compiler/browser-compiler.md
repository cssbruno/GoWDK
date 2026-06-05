# Browser Compiler

GOWDK does not compile arbitrary Go or JavaScript for the browser by default.
The current browser-facing compiler slices are:

- Partial form enhancement runtime emitted as `assets/gowdk/gowdk.js`.
- Generated JavaScript islands for stateful components.
- Explicit WASM island asset emission for component calls that request
  `g:island="wasm"`.

## Partial Runtime

When a page uses a fragment-producing action form with `g:target` and `g:swap`,
static builds emit the client runtime:

```gwdk
<form g:post={refresh} g:target="#patients" g:swap="innerHTML">
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
- Dispatches `gowdk:before-request`, `gowdk:after-swap`, and
  `gowdk:request-error`.
- Toggles `aria-busy` on the submitting form.
- Restores focus by matching the active element's `id` or `name` when possible.
- Calls generated island destroy and mount hooks around replaced island DOM.

## JavaScript Islands

Stateful components use generated JavaScript by default:

```gwdk
@component Counter

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
JavaScript under `assets/gowdk/islands/`.

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

## Explicit WASM Islands

Components can opt an instance into a WASM island:

```gwdk
@component Counter
@wasm ./browser/counter

view {
  <button>Counter</button>
}
```

```gwdk
<Counter g:island="wasm" />
```

When `@wasm` points to a local package, GOWDK builds that package with
`GOOS=js GOARCH=wasm`. The package must be browser-safe and cannot import
server/process/network packages such as `net/http`, `os/exec`, `database/sql`,
raw `syscall`, `plugin`, or `unsafe`.

If a component is called with `g:island="wasm"` and no `@wasm` package is
declared, GOWDK emits the current placeholder module plus loader shape.

The production WASM island ABI and validation of required browser-side exports
are planned. See `docs/engineering/decisions/0004-production-wasm-island-abi.md`
for the target decision.

## Production Mode

`Build.Mode` affects generated island assets:

```go
Build: gowdk.BuildConfig{
	Mode: gowdk.Production,
	Output: "dist/site",
}
```

Development mode emits JavaScript island source maps. Production mode omits
`.js.map` artifacts and `sourceMappingURL` comments and trims formatting-only
whitespace from generated island JavaScript.
