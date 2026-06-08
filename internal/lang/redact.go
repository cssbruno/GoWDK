package lang

import "regexp"

// RedactMessage masks values that commonly carry credentials so a diagnostic
// quoting .gwdk source content (attribute values, expressions, route or store
// literals) does not echo a hardcoded secret into terminal output, check
// --json payloads, or LSP diagnostics.
//
// It is deliberately conservative and mirrors the runtime panic-log policy: it
// favours over-masking a suspicious token over letting a real secret through.
func RedactMessage(message string) string {
	if message == "" {
		return message
	}
	for _, rule := range redactionRules {
		message = rule.pattern.ReplaceAllString(message, rule.replacement)
	}
	return message
}

const redactionMask = "[REDACTED]"

// Rules run in order. The DSN and Bearer/Basic scheme rules run before the
// generic key=value rule so an "Authorization: Bearer <token>" string has its
// token masked by the scheme rule rather than half-consumed by the key rule.
var redactionRules = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	// scheme://user:password@host  (DB DSNs, connection strings)
	{
		pattern:     regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^:/@\s]+:)[^@/\s]+(@)`),
		replacement: `${1}` + redactionMask + `${2}`,
	},
	// Authorization header style: "Bearer <token>" / "Basic <token>"
	{
		pattern:     regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}`),
		replacement: `${1} ` + redactionMask,
	},
	// key=value / key: value where key names a secret. Requires an explicit
	// : or = separator so it does not swallow following words by whitespace.
	{
		pattern:     regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|client[_-]?secret|private[_-]?key)\b(\s*[:=]\s*)("?)[^\s"',;)]+`),
		replacement: `${1}${2}${3}` + redactionMask,
	},
}
