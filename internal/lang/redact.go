package lang

import "github.com/cssbruno/gowdk/internal/securitytext"

// RedactMessage masks values that commonly carry credentials so a diagnostic
// quoting .gwdk source content (attribute values, expressions, route or store
// literals) does not echo a hardcoded secret into terminal output, check
// --json payloads, or LSP diagnostics.
//
// It is deliberately conservative and mirrors the runtime panic-log policy: it
// favours over-masking a suspicious token over letting a real secret through.
func RedactMessage(message string) string {
	return securitytext.RedactSecrets(message)
}
