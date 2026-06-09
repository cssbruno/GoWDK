package buildgen

import (
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// The gotypes package still consumes manifest model types. buildgen now works
// against the compiler IR (gwdkir), so these helpers translate the few IR types
// gotypes needs (Import, GoRef/GoTypeRef, StateContract) into their manifest
// equivalents. The conversions are field-for-field; gwdkir and manifest share
// the same leaf types via internal/source.

func manifestImports(imports []gwdkir.Import) []manifest.Import {
	if imports == nil {
		return nil
	}
	out := make([]manifest.Import, len(imports))
	for i, item := range imports {
		out[i] = manifest.Import{
			Alias: item.Alias,
			Path:  item.Path,
			Span:  item.Span,
		}
	}
	return out
}

func manifestGoTypeRef(ref gwdkir.GoRef) manifest.GoTypeRef {
	return manifest.GoTypeRef{
		Alias: ref.Alias,
		Name:  ref.Name,
		Span:  ref.Span,
	}
}

func manifestGoFuncRef(ref gwdkir.GoRef) manifest.GoFuncRef {
	return manifest.GoFuncRef{
		Alias: ref.Alias,
		Name:  ref.Name,
		Span:  ref.Span,
	}
}

func manifestStateContract(state gwdkir.StateContract) manifest.StateContract {
	return manifest.StateContract{
		Type: manifestGoTypeRef(state.Type),
		Init: manifestGoFuncRef(state.Init),
		Span: state.Span,
	}
}

// manifestBackendBinding converts an IR page load binding into the
// manifest.BackendBinding shape that SSRArtifact.LoadBinding exposes to appgen.
// Only the fields appgen reads (Status, ImportPath, PackageName, FunctionName,
// Signature) plus the binding-local metadata are populated; the kind/page/route
// fields of manifest.BackendBinding describe a binding site that is not part of
// a page load binding and are left empty (matching ManifestFromIR).
func manifestBackendBinding(binding gwdkir.Binding) manifest.BackendBinding {
	return manifest.BackendBinding{
		ImportPath:   binding.ImportPath,
		PackageName:  binding.PackageName,
		FunctionName: binding.FunctionName,
		Signature:    binding.Signature,
		InputType:    binding.InputType,
		InputPointer: binding.InputPointer,
		InputFields:  binding.InputFields,
		Status:       binding.Status,
		Message:      binding.Message,
	}
}
