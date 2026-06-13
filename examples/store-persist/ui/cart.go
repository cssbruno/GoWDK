// Package ui holds the user-owned Go state for the store-persistence example.
// GOWDK never touches this struct; it only serializes its declared fields.
package ui

// CartState is the cart store's Go type. When the page store opts into
// persist "local", only these declared fields are written to browser storage,
// keyed by their exported Go field name (so the seed and persisted blob use
// "Count", not a json tag).
type CartState struct {
	Count int
}

// NewCartState is the build-time initializer for the cart store.
func NewCartState() CartState {
	return CartState{Count: 0}
}
