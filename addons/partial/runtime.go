package partial

import runtimepartial "github.com/cssbruno/gowdk/runtime/partial"

type ClientHook = runtimepartial.ClientHook
type SwapMode = runtimepartial.SwapMode

const (
	HookAfterSwap     = runtimepartial.HookAfterSwap
	HookBeforeRequest = runtimepartial.HookBeforeRequest
	HookRequestError  = runtimepartial.HookRequestError

	SwapInnerHTML = runtimepartial.SwapInnerHTML
	SwapOuterHTML = runtimepartial.SwapOuterHTML
)

var Fragment = runtimepartial.Fragment
var Swap = runtimepartial.Swap
