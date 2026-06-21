package trace

import (
	"errors"
	"testing"
)

func TestCryptoIDGeneratorReportsEntropyFailure(t *testing.T) {
	previousRead := entropyRead
	previousHandler := onEntropyFailure.Load()
	t.Cleanup(func() {
		entropyRead = previousRead
		onEntropyFailure.Store(previousHandler)
	})

	wantErr := errors.New("no entropy")
	entropyRead = func([]byte) (int, error) { return 0, wantErr }

	handled := 0
	SetEntropyFailureHandler(func(err error) {
		if !errors.Is(err, wantErr) {
			t.Errorf("handler error = %v, want %v", err, wantErr)
		}
		handled++
	})

	before := EntropyFailureCount()
	if id := (CryptoIDGenerator{}).NewTraceID(); id != "" {
		t.Fatalf("NewTraceID under entropy failure = %q, want empty (no predictable fallback)", id)
	}
	if id := (CryptoIDGenerator{}).NewSpanID(); id != "" {
		t.Fatalf("NewSpanID under entropy failure = %q, want empty (no predictable fallback)", id)
	}
	if EntropyFailureCount() <= before {
		t.Fatalf("EntropyFailureCount did not increase: before=%d after=%d", before, EntropyFailureCount())
	}
	if handled == 0 {
		t.Fatal("entropy failure handler was not invoked")
	}
}

func TestCryptoIDGeneratorRecoversAfterTransientFailure(t *testing.T) {
	previousRead := entropyRead
	t.Cleanup(func() { entropyRead = previousRead })

	calls := 0
	entropyRead = func(buf []byte) (int, error) {
		calls++
		if calls == 1 {
			return 0, errors.New("transient")
		}
		for i := range buf {
			buf[i] = 0x42
		}
		return len(buf), nil
	}

	id := (CryptoIDGenerator{}).NewTraceID()
	if !id.Valid() {
		t.Fatalf("expected a valid trace id after a transient failure, got %q", id)
	}
}
