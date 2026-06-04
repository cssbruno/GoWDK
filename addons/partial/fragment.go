package partial

import "github.com/gowdk/gowdk/runtime/response"

// Fragment returns a server fragment response for a DOM target.
func Fragment(target, html string) response.Response {
	return response.FragmentFor(target, html)
}
