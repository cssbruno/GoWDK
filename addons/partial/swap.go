package partial

// SwapMode describes how a server fragment should be applied by the client runtime.
type SwapMode string

const (
	SwapReplace SwapMode = "replace"
	SwapAppend  SwapMode = "append"
	SwapPrepend SwapMode = "prepend"
	SwapBefore  SwapMode = "before"
	SwapAfter   SwapMode = "after"
)
