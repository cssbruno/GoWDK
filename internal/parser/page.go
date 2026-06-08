// Package parser turns .gwdk source files into syntax trees.
package parser

import "github.com/cssbruno/gowdk/internal/manifest"

// ParsePage extracts page metadata and top-level block declarations.
func ParsePage(source []byte) (manifest.Page, error) {
	return ParsePageWithDefaultID(source, "")
}

// ParsePageWithDefaultID extracts page metadata and uses defaultID when the
// source omits @page.
func ParsePageWithDefaultID(source []byte, defaultID string) (manifest.Page, error) {
	ast, err := ParseSyntax(source)
	if err != nil {
		return manifest.Page{}, err
	}
	return lowerPageSyntax(source, ast, defaultID)
}
