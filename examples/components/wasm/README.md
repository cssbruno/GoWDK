# WASM Island ABI

This example shows the supported component-level WASM island package shape.

```gwdk
component WasmCounter
wasm ./examples/components/wasm/browser/counter
```

The browser Go package is a normal `package main` compiled with
`GOOS=js GOARCH=wasm`. It must export the component-scoped ABI functions with
`//go:wasmexport`:

```go
import gowdkwasm "github.com/cssbruno/gowdk/runtime/wasm"

//go:wasmexport GOWDKMountWasmCounter
func GOWDKMountWasmCounter() uint32 {
	return gowdkwasm.ReturnResult(gowdkwasm.Result{Patches: []any{}})
}
```

## Build It

Run from the repository root:

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-wasm-island examples/components/wasm/*.gwdk
test -f /tmp/gowdk-wasm-island/components/wasm/index.html
test -f /tmp/gowdk-wasm-island/assets/gowdk/islands/componentwasm/WasmCounter.wasm
test -f /tmp/gowdk-wasm-island/assets/gowdk/islands/componentwasm/WasmCounter.wasm.js
test -f /tmp/gowdk-wasm-island/assets/gowdk/islands/wasm_exec.js
```

The current ABI validates required exports and browser-safe imports, emits the
host loader, decodes Go-style `uint32` JSON result pointers, and reports
unsupported patch operations in the browser console. Use `runtime/wasm` to read
the current mount/event/destroy payload and return either a legacy patch array or
the extended `{ "patches": [...], "stores": { ... } }` result shape.

## Page stores

A WASM island that declares `use <store>` in its client block participates in
page stores like a JS island. The host loader:

- **reads** every used store and merges its current (and persisted) value into the
  `state` field of the mount/handle/destroy payload, and passes the used store
  names in `payload.stores`;
- **writes back** when an export returns the extended result shape
  `{ "patches": [...], "stores": { "<name>": <value> } }` — each returned store
  value is written to `window.__gowdkStores`. The legacy bare patch array is still
  accepted (no store write-back);
- **syncs** on external changes: when another island updates a used store, the
  loader re-invokes the mount export with the refreshed `state` and applies the
  returned patches, without echoing the update back into the registry.

Surfacing serialized state from the `uint32` export contract (so a Go island can
return the `stores` map) is implemented by the `runtime/wasm` helper and host
loader pointer decoding.
