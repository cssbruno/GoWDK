package buildgen

import (
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// manifestBackendBinding converts an IR page load binding into the
// manifest.BackendBinding shape that SSRArtifact.LoadBinding exposes to appgen.
// Only the fields appgen reads (Status, ImportPath, PackageName, FunctionName,
// Signature) plus the binding-local metadata are populated; the kind/page/route
// fields of manifest.BackendBinding describe a binding site that is not part of
// a page load binding and are left empty.
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
