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
//go:wasmexport GOWDKMountWasmCounter
func GOWDKMountWasmCounter() uint32 { return 0 }
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
host loader, and reports unsupported patch operations in the browser console.
User Go patch-memory decoding beyond the current `uint32` return contract is
still planned.
