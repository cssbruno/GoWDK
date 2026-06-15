package gwdkir

import "strings"

// HTTPMethodRequiresCSRF reports whether a browser-reachable HTTP method should
// require generated CSRF validation by default.
func HTTPMethodRequiresCSRF(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "", "GET", "HEAD", "OPTIONS", "TRACE":
		return false
	default:
		return true
	}
}
