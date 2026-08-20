package appgen

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/runtime/i18n"
)

func errorCatalogDecl(options Options) ast.Decl {
	defaultLocale := strings.TrimSpace(options.Config.I18N.Errors.DefaultLocale)
	if defaultLocale == "" {
		defaultLocale = options.Config.I18N.DefaultLocaleCode()
	}
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names:  []*ast.Ident{id("userErrorCatalog")},
		Values: []ast.Expr{call(sel("gowdki18n", "NewErrorBundleStrings"), stringLit(defaultLocale), errorCatalogMapExpr(options.Config.I18N.Errors))},
	}}}
}

func errorCatalogMapExpr(bundle i18n.ErrorBundle) ast.Expr {
	outer := &ast.CompositeLit{Type: &ast.MapType{Key: id("string"), Value: &ast.MapType{Key: id("string"), Value: id("string")}}}
	locales := make([]string, 0, len(bundle.Catalogs))
	for locale := range bundle.Catalogs {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		catalog := bundle.Catalogs[locale]
		inner := &ast.CompositeLit{Type: &ast.MapType{Key: id("string"), Value: id("string")}}
		codes := make([]string, 0, len(catalog.Messages))
		for code := range catalog.Messages {
			codes = append(codes, string(code))
		}
		sort.Strings(codes)
		for _, code := range codes {
			inner.Elts = append(inner.Elts, &ast.KeyValueExpr{Key: stringLit(code), Value: stringLit(catalog.Messages[i18n.ErrorCode(code)])})
		}
		outer.Elts = append(outer.Elts, &ast.KeyValueExpr{Key: stringLit(locale), Value: inner})
	}
	return outer
}

func requestLocaleExpr() ast.Expr {
	return call(sel("gowdkruntime", "Locale"), call(selExpr(id("request"), "Context")))
}

func generatedErrorCode(message string) string {
	switch {
	case strings.Contains(message, "csrf"):
		return "invalid_csrf_token"
	case strings.Contains(message, "request body too large"):
		return "request_body_too_large"
	case strings.Contains(message, "invalid form"):
		return "invalid_form"
	case strings.Contains(message, "validation failed"):
		return "validation_failed"
	case strings.Contains(message, "partial fragment not found"):
		return "fragment_not_found"
	case strings.Contains(message, "route parameter"):
		return "invalid_route_parameter"
	case strings.Contains(message, "method not allowed"):
		return "method_not_allowed"
	case strings.Contains(message, "not implemented"), strings.Contains(message, "not registered"):
		return "handler_not_implemented"
	case strings.Contains(message, "403 forbidden"):
		return "forbidden"
	default:
		return "request_failed"
	}
}
