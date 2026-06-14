package app

import "github.com/cssbruno/gowdk/runtime/security"

// redactSecrets masks values that commonly carry credentials so a recovered
// panic or error can be logged without leaking secrets into operator logs.
// It is deliberately conservative: it favours over-masking suspicious tokens
// over letting a real secret through, and it never touches the HTTP response
// (which already returns a generic message).
func redactSecrets(message string) string {
	return security.RedactSecrets(message)
}
