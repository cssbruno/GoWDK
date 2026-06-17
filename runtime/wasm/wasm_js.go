//go:build js && wasm

package wasm

import (
	"encoding/json"
	"errors"
	"syscall/js"
	"unsafe"
)

var returnBuffer []byte

// DecodePayload decodes the current island mount or event payload into target.
func DecodePayload(target any) error {
	if target == nil {
		return errors.New("decode gowdk wasm payload: target is nil")
	}
	value := js.Global().Get("__gowdkWASMIslandPayload")
	if value.Type() == js.TypeUndefined || value.Type() == js.TypeNull {
		return nil
	}
	raw := value.String()
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), target)
}

// CurrentPayload returns the current island mount or event payload.
func CurrentPayload() (Payload, error) {
	var payload Payload
	return payload, DecodePayload(&payload)
}

// Return marshals value and returns a pointer to a null-terminated JSON buffer.
func Return(value any) uint32 {
	payload, err := json.Marshal(value)
	if err != nil {
		payload = []byte("[]")
	}
	returnBuffer = append(returnBuffer[:0], payload...)
	returnBuffer = append(returnBuffer, 0)
	return uint32(uintptr(unsafe.Pointer(&returnBuffer[0])))
}

// ReturnPatches returns a legacy patch array result.
func ReturnPatches(patches any) uint32 {
	return Return(patches)
}

// ReturnResult returns the extended patches plus stores result.
func ReturnResult(result Result) uint32 {
	return Return(result)
}
