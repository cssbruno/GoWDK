package view

import "github.com/cssbruno/gowdk/internal/viewanalysis"

// Canonical returns a deterministic AST-backed representation of a view body.
func Canonical(source string) (string, error) {
	return viewanalysis.Canonical(source)
}
