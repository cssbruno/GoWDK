package ssr

import "context"

// LoadFunc is generated from a request-time load {} block.
type LoadFunc func(context.Context) (map[string]any, error)
