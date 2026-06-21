package buildgen

import (
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// sourceBackendBinding converts an IR page load binding into the
// source.BackendBinding shape that SSRArtifact.LoadBinding exposes to appgen.
// Only the fields appgen reads (Status, ImportPath, PackageName, FunctionName,
// Signature) plus the binding-local metadata are populated; the kind/page/route
// fields of source.BackendBinding describe a binding site that is not part of
// a page load binding and are left empty.
func sourceBackendBinding(binding gwdkir.Binding) source.BackendBinding {
	return source.BackendBinding{
		ImportPath:    binding.ImportPath,
		PackageName:   binding.PackageName,
		FunctionName:  binding.FunctionName,
		Signature:     binding.Signature,
		InputType:     binding.InputType,
		InputPointer:  binding.InputPointer,
		InputFields:   binding.InputFields,
		ResultType:    binding.ResultType,
		ResultPointer: binding.ResultPointer,
		ResultFields:  append([]source.BackendResultField(nil), binding.ResultFields...),
		Status:        binding.Status,
		Message:       binding.Message,
	}
}
