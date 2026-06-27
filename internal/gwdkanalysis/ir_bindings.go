package gwdkanalysis

import (
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// AttachBackendBindings copies backend handler binding records onto the
// normalized IR endpoints and page load bindings they describe. Records are
// matched by (kind, page, block, method, route); endpoints and pages without a
// matching record get a zero binding.
func AttachBackendBindings(program *gwdkir.Program, bindings []source.BackendBinding) {
	byEndpoint := map[gwdkir.EndpointID]source.BackendBinding{}
	byLoadPage := map[string]source.BackendBinding{}
	for _, binding := range bindings {
		if binding.Kind == "load" {
			byLoadPage[binding.PageID] = binding
			continue
		}
		kind := gwdkir.EndpointAction
		switch binding.Kind {
		case "api":
			kind = gwdkir.EndpointAPI
		case "fragment":
			kind = gwdkir.EndpointFragment
		}
		byEndpoint[gwdkir.EndpointIdentity(kind, binding.PageID, binding.BlockName, binding.Method, binding.Route)] = binding
	}
	for index := range program.Endpoints {
		endpoint := &program.Endpoints[index]
		binding := byEndpoint[endpoint.SemanticID()]
		endpoint.Binding = gwdkir.Binding{
			Status:        binding.Status,
			Message:       binding.Message,
			ImportPath:    binding.ImportPath,
			PackageName:   binding.PackageName,
			FunctionName:  binding.FunctionName,
			Signature:     binding.Signature,
			InputType:     binding.InputType,
			InputPointer:  binding.InputPointer,
			InputFields:   append([]source.BackendInputField(nil), binding.InputFields...),
			ResultType:    binding.ResultType,
			ResultPointer: binding.ResultPointer,
			ResultFields:  append([]source.BackendResultField(nil), binding.ResultFields...),
		}
	}
	for index := range program.Pages {
		page := &program.Pages[index]
		binding := byLoadPage[page.ID]
		page.LoadBinding = gwdkir.Binding{
			Status:        binding.Status,
			Message:       binding.Message,
			ImportPath:    binding.ImportPath,
			PackageName:   binding.PackageName,
			FunctionName:  binding.FunctionName,
			Signature:     binding.Signature,
			ResultType:    binding.ResultType,
			ResultPointer: binding.ResultPointer,
			ResultFields:  append([]source.BackendResultField(nil), binding.ResultFields...),
		}
	}
}
