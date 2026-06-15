// Package security contains conservative runtime security helpers.
package security

import "regexp"

const RedactionMask = "[REDACTED]"

type rule struct {
	kind        string
	pattern     *regexp.Regexp
	replacement string
}

// Rules run in order. The DSN and Bearer/Basic scheme rules run before the
// generic key=value rule so an "Authorization: Bearer <token>" string has its
// token masked by the scheme rule rather than half-consumed by the key rule.
var rules = []rule{
	{
		kind:        "dsn-password",
		pattern:     regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^:/@\s]+:)[^@/\s]+(@)`),
		replacement: `${1}` + RedactionMask + `${2}`,
	},
	{
		kind:        "authorization-token",
		pattern:     regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}`),
		replacement: `${1} ` + RedactionMask,
	},
	{
		kind:        "secret-key-value",
		pattern:     regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|token|_?gowdk[_-]?csrf|_?csrf(?:[_-]?token)?|cookie|set-cookie|auth[_-]?token|session(?:[_-]?id)?|jwt|api[_-]?key|access(?:[_-]?(?:key|token))?|refresh[_-]?token|id[_-]?token|client[_-]?secret|private[_-]?key)\b(\s*[:=]\s*)("?)[^\s"',;)]+`),
		replacement: `${1}${2}${3}` + RedactionMask,
	},
	{
		// signed-token masks bare HMAC/JWT-shaped tokens that carry no preceding
		// keyword — e.g. a gowdk session cookie value (<b64payload>.<b64tag>) or a
		// JWT (<b64>.<b64>.<b64>) that surfaces inside a panic message or stack
		// frame. Two base64url runs of >= 20 chars joined by a dot effectively
		// never occur in ordinary text, so this stays high precision.
		kind:        "signed-token",
		pattern:     regexp.MustCompile(`[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}(?:\.[A-Za-z0-9_-]{10,})?`),
		replacement: RedactionMask,
	},
}

// RedactSecrets masks values that commonly carry credentials.
func RedactSecrets(message string) string {
	if message == "" {
		return message
	}
	for _, rule := range rules {
		message = rule.pattern.ReplaceAllString(message, rule.replacement)
	}
	return message
}

// FirstSecretKind returns the first secret-like rule kind matched in text.
func FirstSecretKind(text string) (string, bool) {
	for _, rule := range rules {
		if rule.pattern.MatchString(text) {
			return rule.kind, true
		}
	}
	return "", false
}
