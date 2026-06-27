package gwdkanalysis

import (
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func standaloneEndpointPageID(endpoint gwdkir.StandaloneEndpointDeclaration) string {
	if endpoint.Package == "" {
		return endpoint.Name
	}
	return endpoint.Package + "." + endpoint.Name
}

func assetUse(uses []gwdkir.Use, path string) (name string, useAlias string, usePackage string) {
	alias, assetName, ok := strings.Cut(path, ".")
	if !ok {
		return path, "", ""
	}
	for _, use := range uses {
		if use.Alias == alias {
			return assetName, alias, use.Package
		}
	}
	return assetName, alias, ""
}

func spanForName(spans []source.NamedSpan, name string, fallback source.SourceSpan) source.SourceSpan {
	for _, span := range spans {
		if span.Name == name {
			return span.Span
		}
	}
	return fallback
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func hasImport(values []gwdkir.Import, value gwdkir.Import) bool {
	for _, item := range values {
		if item.Alias == value.Alias && item.Path == value.Path {
			return true
		}
	}
	return false
}

func hasUse(values []gwdkir.Use, value gwdkir.Use) bool {
	for _, item := range values {
		if item.Alias == value.Alias && item.Package == value.Package {
			return true
		}
	}
	return false
}

func routeKind(mode gowdk.RenderMode) gwdkir.RouteKind {
	switch mode {
	case gowdk.SSR:
		return gwdkir.RouteSSR
	case gowdk.Hybrid:
		return gwdkir.RouteHybrid
	default:
		return gwdkir.RouteSPA
	}
}

func copyRouteParams(params []source.RouteParam) []source.RouteParam {
	if len(params) == 0 {
		return nil
	}
	out := make([]source.RouteParam, len(params))
	copy(out, params)
	return out
}

func appendPackageImports(pkg *gwdkir.Package, imports []gwdkir.Import) {
	for _, item := range imports {
		if !hasImport(pkg.Imports, item) {
			pkg.Imports = append(pkg.Imports, item)
		}
	}
}

func appendPackageUses(pkg *gwdkir.Package, uses []gwdkir.Use) {
	for _, item := range uses {
		if !hasUse(pkg.Uses, item) {
			pkg.Uses = append(pkg.Uses, item)
		}
	}
}

func appendPackageStores(pkg *gwdkir.Package, stores []gwdkir.Store) {
	pkg.Stores = append(pkg.Stores, stores...)
}
