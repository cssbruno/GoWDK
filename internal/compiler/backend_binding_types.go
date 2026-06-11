package compiler

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

const (
	actionHandlerKind   = "action"
	apiHandlerKind      = "api"
	fragmentHandlerKind = "fragment"
	loadHandlerKind     = "load"

	contextImportPath  = "context"
	formImportPath     = "github.com/cssbruno/gowdk/runtime/form"
	httpImportPath     = "net/http"
	guardImportPath    = "github.com/cssbruno/gowdk/runtime/guard"
	responseImportPath = "github.com/cssbruno/gowdk/runtime/response"
	ssrImportPath      = "github.com/cssbruno/gowdk/addons/ssr"
)

type featurePackage struct {
	Dir        string
	ImportPath string
	Name       string
	Functions  map[string]featureFunction
	LoadError  string
}

type featureFunction struct {
	Name           string
	Signature      source.BackendSignatureKind
	InputType      string
	InputPointer   bool
	InputFields    []source.BackendInputField
	SupportMessage string
}

func (function featureFunction) Action() bool {
	switch function.Signature {
	case source.BackendSignatureAction0, source.BackendSignatureActionValues, source.BackendSignatureActionForm, source.BackendSignatureActionFormPtr:
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
	return function.Signature == source.BackendSignatureLoad || function.Signature == source.BackendSignatureLoadError
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
