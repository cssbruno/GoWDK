// Package securitytext wraps runtime security text helpers for compiler-private
// callers.
package securitytext

import "github.com/cssbruno/gowdk/runtime/security"

const RedactionMask = security.RedactionMask

// RedactSecrets masks values that commonly carry credentials.
func RedactSecrets(message string) string {
	return security.RedactSecrets(message)
}

// FirstSecretKind returns the first secret-like rule kind matched in text.
func FirstSecretKind(text string) (string, bool) {
	return security.FirstSecretKind(text)
}
