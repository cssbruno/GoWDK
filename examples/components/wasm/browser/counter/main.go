package main

//go:wasmexport GOWDKMountWasmCounter
func GOWDKMountWasmCounter() uint32 {
	return 0
}

//go:wasmexport GOWDKHandleWasmCounter
func GOWDKHandleWasmCounter() uint32 {
	return 0
}

//go:wasmexport GOWDKDestroyWasmCounter
func GOWDKDestroyWasmCounter() uint32 {
	return 0
}

func main() {}
