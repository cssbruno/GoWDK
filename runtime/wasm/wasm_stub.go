//go:build !js || !wasm

package wasm

// DecodePayload reports ErrUnavailable outside browser WASM builds.
func DecodePayload(target any) error {
	return ErrUnavailable
}

// CurrentPayload reports ErrUnavailable outside browser WASM builds.
func CurrentPayload() (Payload, error) {
	return Payload{}, ErrUnavailable
}

// Return returns zero outside browser WASM builds.
func Return(value any) uint32 {
	return 0
}

// ReturnPatches returns zero outside browser WASM builds.
func ReturnPatches(patches any) uint32 {
	return 0
}

// ReturnResult returns zero outside browser WASM builds.
func ReturnResult(result Result) uint32 {
	return 0
}
