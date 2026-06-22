package trace

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
)

// IDGenerator produces trace and span identifiers for new spans.
//
// The default generator (CryptoIDGenerator) draws identifiers from
// crypto/rand. Provide a custom generator with WithIDGenerator to make IDs
// deterministic in tests, or to source identity from another subsystem. A
// generator may report transient failure by returning an empty (invalid) ID;
// the tracer treats that as "cannot start a span" rather than substituting a
// predictable identifier.
type IDGenerator interface {
	// NewTraceID returns a valid W3C trace ID, or "" if one cannot be produced.
	NewTraceID() TraceID
	// NewSpanID returns a valid W3C span ID, or "" if one cannot be produced.
	NewSpanID() SpanID
}

// maxEntropyAttempts bounds how many times the crypto generator retries a
// failed read before giving up and returning an invalid ID. crypto/rand
// failures are extraordinary; the bound prevents an unbounded spin while still
// tolerating a transient hiccup.
const maxEntropyAttempts = 16

// entropyRead reads cryptographic randomness for the default ID generator. It
// is a package variable so tests can simulate a CSPRNG failure without making
// crypto/rand actually fail.
var entropyRead = rand.Read

// entropyFailures counts crypto/rand failures observed by the default ID
// generator since process start.
var entropyFailures atomic.Uint64

// onEntropyFailure, when non-nil, is invoked for each crypto/rand failure
// observed by the default ID generator.
var onEntropyFailure atomic.Pointer[func(error)]

// EntropyFailureCount reports how many crypto/rand failures the default ID
// generator has observed since process start. It is zero on healthy systems.
// A non-zero value means trace/span identity could not be generated from the
// system CSPRNG and the affected spans were dropped rather than assigned a
// predictable identifier.
func EntropyFailureCount() uint64 {
	return entropyFailures.Load()
}

// SetEntropyFailureHandler installs a callback invoked for each crypto/rand
// failure observed by the default ID generator. Pass nil to clear it.
//
// The handler must not start spans or otherwise re-enter the tracer: doing so
// during an entropy failure can recurse. Use it to increment an external
// metric or emit a single rate-limited alert.
func SetEntropyFailureHandler(handler func(error)) {
	if handler == nil {
		onEntropyFailure.Store(nil)
		return
	}
	onEntropyFailure.Store(&handler)
}

func reportEntropyFailure(err error) {
	entropyFailures.Add(1)
	if handler := onEntropyFailure.Load(); handler != nil {
		(*handler)(err)
	}
}

// CryptoIDGenerator is the default IDGenerator. It reads identifiers from
// crypto/rand and never falls back to predictable values such as timestamps or
// sequence counters. On the (extraordinary) event of a crypto/rand failure it
// records the failure via EntropyFailureCount and the entropy failure handler,
// then returns an invalid ID so the caller can observe the loss instead of
// emitting a guessable identifier.
type CryptoIDGenerator struct{}

// NewTraceID implements IDGenerator.
func (CryptoIDGenerator) NewTraceID() TraceID {
	var buf [16]byte
	if !readEntropy(buf[:]) {
		return ""
	}
	id := TraceID(hex.EncodeToString(buf[:]))
	if !id.Valid() {
		return ""
	}
	return id
}

// NewSpanID implements IDGenerator.
func (CryptoIDGenerator) NewSpanID() SpanID {
	var buf [8]byte
	if !readEntropy(buf[:]) {
		return ""
	}
	id := SpanID(hex.EncodeToString(buf[:]))
	if !id.Valid() {
		return ""
	}
	return id
}

// readEntropy fills buf from crypto/rand, retrying a bounded number of times on
// failure. Each failure is reported. It returns false only when entropy could
// not be read at all, which is the signal to drop identity rather than degrade
// to a predictable value.
func readEntropy(buf []byte) bool {
	for attempt := 0; attempt < maxEntropyAttempts; attempt++ {
		if _, err := entropyRead(buf); err == nil {
			return true
		} else {
			reportEntropyFailure(err)
		}
	}
	return false
}

// defaultIDGenerator backs the package-level NewTraceID/NewSpanID helpers and
// any tracer constructed without WithIDGenerator.
var defaultIDGenerator IDGenerator = CryptoIDGenerator{}
