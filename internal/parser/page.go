// Package parser turns .gwdk source files into syntax trees.
package parser

import "github.com/cssbruno/gowdk/internal/gwdkir"

// ParsePage extracts page metadata and top-level block declarations.
func ParsePage(source []byte) (gwdkir.Page, error) {
	return ParsePageWithDefaultID(source, "")
}

// ParsePageWithDefaultID extracts page metadata and uses defaultID when the
// source omits page.
func ParsePageWithDefaultID(source []byte, defaultID string) (gwdkir.Page, error) {
	ast, err := ParseSyntax(source)
	if err != nil {
		return gwdkir.Page{}, err
	}
	return lowerPageSyntax(source, ast, defaultID)
}
