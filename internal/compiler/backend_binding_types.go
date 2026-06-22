package compiler

import (
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/source"
)

const (
	actionHandlerKind   = "action"
	apiHandlerKind      = "api"
	fragmentHandlerKind = "fragment"
	loadHandlerKind     = "load"

	contextImportPath    = "context"
	formImportPath       = "github.com/cssbruno/gowdk/runtime/form"
	httpImportPath       = "net/http"
	guardImportPath      = "github.com/cssbruno/gowdk/runtime/guard"
	responseImportPath   = "github.com/cssbruno/gowdk/runtime/response"
	ssrImportPath        = "github.com/cssbruno/gowdk/addons/ssr"
	runtimeSSRImportPath = "github.com/cssbruno/gowdk/runtime/ssr"
)

type featurePackage struct {
	Dir        string
	ImportPath string
	Name       string
	Functions  map[string]featureFunction
	// Unexported holds the names of package-level functions that exist but are
	// not exported, so binding can explain a same-named lowercase near-miss.
	Unexported map[string]bool
	LoadError  string
}

// hasUnexported reports whether a same-named unexported function exists for the
// exported handler name (the canonical first-letter-lowercased near-miss).
func (pkg featurePackage) hasUnexported(exportedName string) bool {
	if len(pkg.Unexported) == 0 {
		return false
	}
	return pkg.Unexported[firstRuneLower(exportedName)]
}

func firstRuneLower(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

type featureFunction struct {
	Name           string
	Signature      source.BackendSignatureKind
	InputType      string
	InputPointer   bool
	InputFields    []source.BackendInputField
	ResultType     string
	ResultPointer  bool
	ResultFields   []source.BackendResultField
	SupportMessage string
}

func (function featureFunction) Action() bool {
	switch function.Signature {
	case source.BackendSignatureAction0, source.BackendSignatureActionValues, source.BackendSignatureActionData, source.BackendSignatureActionForm, source.BackendSignatureActionFormPtr:
		return true
	default:
		return false
	}
}

func (function featureFunction) API() bool {
	return function.Signature == source.BackendSignatureAPI
}

func (function featureFunction) Fragment() bool {
	return function.Signature == source.BackendSignatureAction0 || function.Signature == source.BackendSignatureFragment
}

func (function featureFunction) Load() bool {
	switch function.Signature {
	case source.BackendSignatureLoad, source.BackendSignatureLoadError, source.BackendSignatureLoadStruct, source.BackendSignatureLoadStructError:
		return true
	default:
		return false
	}
}

func bindingPackageName(pkgName string, fallback string) string {
	if strings.TrimSpace(pkgName) != "" {
		return pkgName
	}
	return fallback
}

func bindingPackageLabel(binding source.BackendBinding, pkg featurePackage) string {
	if pkg.ImportPath != "" {
		return pkg.ImportPath
	}
	if pkg.Name != "" {
		return pkg.Name
	}
	if binding.PackageName != "" {
		return binding.PackageName
	}
	return "feature"
}

func packageLabel(pkg featurePackage) string {
	if pkg.ImportPath != "" {
		return pkg.ImportPath
	}
	if pkg.Name != "" {
		return pkg.Name
	}
	return "feature"
}

type inputStruct struct {
	Fields  []source.BackendInputField
	Message string
}

type resultStruct struct {
	Fields  []source.BackendResultField
	Message string
}
