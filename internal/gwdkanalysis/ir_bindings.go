package gwdkanalysis

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/source"
)

func attachBackendBindings(program *gwdkir.Program, bindings []manifest.BackendBinding) {
	byEndpoint := map[string]manifest.BackendBinding{}
	byLoadPage := map[string]manifest.BackendBinding{}
	for _, binding := range bindings {
		if binding.Kind == "load" {
			byLoadPage[binding.PageID] = binding
			continue
		}
		kind := gwdkir.EndpointAction
		if binding.Kind == "api" {
			kind = gwdkir.EndpointAPI
		} else if binding.Kind == "fragment" {
			kind = gwdkir.EndpointFragment
		}
		byEndpoint[endpointKey(kind, binding.PageID, binding.BlockName, binding.Method, binding.Route)] = binding
	}
	for index := range program.Endpoints {
		endpoint := &program.Endpoints[index]
		binding := byEndpoint[endpointKey(endpoint.Kind, endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path)]
		endpoint.Binding = gwdkir.Binding{
			Status:       binding.Status,
			Message:      binding.Message,
			ImportPath:   binding.ImportPath,
			PackageName:  binding.PackageName,
			FunctionName: binding.FunctionName,
			Signature:    binding.Signature,
			InputType:    binding.InputType,
			InputPointer: binding.InputPointer,
			InputFields:  append([]source.BackendInputField(nil), binding.InputFields...),
		}
	}
	for index := range program.Pages {
		page := &program.Pages[index]
		binding := byLoadPage[page.ID]
		page.LoadBinding = gwdkir.Binding{
			Status:       binding.Status,
			Message:      binding.Message,
			ImportPath:   binding.ImportPath,
			PackageName:  binding.PackageName,
			FunctionName: binding.FunctionName,
			Signature:    binding.Signature,
		}
	}
}

func endpointKey(kind gwdkir.EndpointKind, pageID, symbol, method, route string) string {
	return strings.Join([]string{string(kind), pageID, symbol, method, route}, "\x00")
}
