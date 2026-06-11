package partial

import "github.com/cssbruno/gowdk/runtime/response"

// SwapMode describes how a server fragment should be applied by the client runtime.
type SwapMode string

const (
	SwapInnerHTML SwapMode = "innerHTML"
	SwapOuterHTML SwapMode = "outerHTML"
)

// Swap returns a server fragment response with an explicit client swap mode.
func Swap(target string, mode SwapMode, html string) (response.Response, error) {
	return response.FragmentSwap(target, response.SwapMode(mode), html)
}
