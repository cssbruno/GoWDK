package partial

// ClientHook names lifecycle hooks emitted for the partial update client runtime.
type ClientHook string

const (
	HookBeforeRequest ClientHook = "before-request"
	HookAfterSwap     ClientHook = "after-swap"
	HookRequestError  ClientHook = "request-error"
)
