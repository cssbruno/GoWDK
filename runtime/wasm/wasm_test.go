package wasm

import "testing"

func TestStubHelpersReportUnavailable(t *testing.T) {
	var payload Payload
	if err := DecodePayload(&payload); err != ErrUnavailable {
		t.Fatalf("DecodePayload error = %v, want ErrUnavailable", err)
	}
	if _, err := CurrentPayload(); err != ErrUnavailable {
		t.Fatalf("CurrentPayload error = %v, want ErrUnavailable", err)
	}
	if pointer := Return(Result{}); pointer != 0 {
		t.Fatalf("Return pointer = %d, want 0", pointer)
	}
}
