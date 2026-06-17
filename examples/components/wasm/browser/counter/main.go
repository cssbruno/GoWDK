package main

import gowdkwasm "github.com/cssbruno/gowdk/runtime/wasm"

//go:wasmexport GOWDKMountWasmCounter
func GOWDKMountWasmCounter() uint32 {
	return gowdkwasm.ReturnResult(gowdkwasm.Result{Patches: []any{}})
}

//go:wasmexport GOWDKHandleWasmCounter
func GOWDKHandleWasmCounter() uint32 {
	_, _ = gowdkwasm.CurrentPayload()
	return gowdkwasm.ReturnResult(gowdkwasm.Result{Patches: []any{}})
}

//go:wasmexport GOWDKDestroyWasmCounter
func GOWDKDestroyWasmCounter() uint32 {
	return gowdkwasm.ReturnPatches([]any{})
}

func main() {}
