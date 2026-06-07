package appgen

import (
	"sort"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

type BackendAdapterIR struct {
	Registrations     []BackendEndpointRegistration
	ContractExposures []BackendContractExposure
	Decoders          []BackendDecoder
	Calls             []BackendHandlerCall
	Responses         []BackendResponse
	Fallbacks         []BackendFallback
}

type BackendEndpointKind string

const (
	BackendEndpointAction   BackendEndpointKind = "action"
	BackendEndpointAPI      BackendEndpointKind = "api"
	BackendEndpointFragment BackendEndpointKind = "fragment"
	BackendEndpointCommand  BackendEndpointKind = "command"
	BackendEndpointQuery    BackendEndpointKind = "query"
)

type BackendEndpointRegistration struct {
	Kind    BackendEndpointKind
	Method  string
	Path    string
	Handler string
	PageID  string
	Name    string
}

type BackendDecoder struct {
	Endpoint BackendEndpointRegistration
	Function string
	Input    string
	Fields   []string
}

type BackendHandlerCall struct {
	Endpoint  BackendEndpointRegistration
	Alias     string
	Function  string
	Signature manifest.BackendSignatureKind
	InputType string
}

type BackendResponse struct {
	Endpoint BackendEndpointRegistration
	NoStore  bool
	Partial  bool
	Redirect string
}

type BackendFallback struct {
	Endpoint BackendEndpointRegistration
	Status   manifest.BackendBindingStatus
	Message  string
}

type BackendContractExposure struct {
	Endpoint    BackendEndpointRegistration
	Contract    string
	ImportAlias string
	ImportPath  string
	Type        string
	Result      string
	Status      gwdkir.ContractBindingStatus
	Handler     string
	Register    string
	Message     string
	OwnerKind   gwdkir.SourceKind
	OwnerID     string
	Package     string
	Source      string
}

func backendAdapterIR(options Options) BackendAdapterIR {
	var ir BackendAdapterIR
	for _, action := range sortedActionEndpoints(options.Actions) {
		endpoint := BackendEndpointRegistration{
			Kind:    BackendEndpointAction,
			Method:  actionMethod(action),
			Path:    action.Route,
			Handler: "action",
			PageID:  action.PageID,
			Name:    action.ActionName,
		}
		ir.Registrations = append(ir.Registrations, endpoint)
		if action.InputType != "" || action.Binding.InputType != "" {
			decoder := BackendDecoder{
				Endpoint: endpoint,
				Input:    action.InputType,
				Fields:   append([]string(nil), action.InputFields...),
			}
			if action.Binding.Status == manifest.BackendBindingBound && action.Binding.InputType != "" {
				decoder.Function = boundActionDecoderName(action)
				decoder.Input = action.Binding.InputType
			} else if action.InputType != "" {
				decoder.Function = actionDecoderName(action)
			}
			ir.Decoders = append(ir.Decoders, decoder)
		}
		if action.Binding.Status == manifest.BackendBindingBound {
			ir.Calls = append(ir.Calls, BackendHandlerCall{
				Endpoint:  endpoint,
				Alias:     action.BackendAlias,
				Function:  action.Binding.FunctionName,
				Signature: action.Binding.Signature,
				InputType: action.Binding.InputType,
			})
		}
		if action.Binding.Status != "" && action.Binding.Status != manifest.BackendBindingBound {
			ir.Fallbacks = append(ir.Fallbacks, BackendFallback{
				Endpoint: endpoint,
				Status:   action.Binding.Status,
				Message:  action.Binding.Message,
			})
		}
		ir.Responses = append(ir.Responses, BackendResponse{
			Endpoint: endpoint,
			NoStore:  true,
			Partial:  len(action.Fragments) > 0,
			Redirect: action.Redirect,
		})
	}
	for _, api := range sortedAPIEndpoints(options.APIs) {
		endpoint := BackendEndpointRegistration{
			Kind:    BackendEndpointAPI,
			Method:  api.Method,
			Path:    api.Route,
			Handler: "api",
			PageID:  api.PageID,
			Name:    api.APIName,
		}
		ir.Registrations = append(ir.Registrations, endpoint)
		if api.Binding.Status == manifest.BackendBindingBound {
			ir.Calls = append(ir.Calls, BackendHandlerCall{
				Endpoint:  endpoint,
				Alias:     api.BackendAlias,
				Function:  api.Binding.FunctionName,
				Signature: api.Binding.Signature,
			})
		}
		if api.Binding.Status != "" && api.Binding.Status != manifest.BackendBindingBound {
			ir.Fallbacks = append(ir.Fallbacks, BackendFallback{
				Endpoint: endpoint,
				Status:   api.Binding.Status,
				Message:  api.Binding.Message,
			})
		}
		ir.Responses = append(ir.Responses, BackendResponse{Endpoint: endpoint, NoStore: true})
	}
	for _, fragment := range sortedFragmentEndpoints(options.Fragments) {
		endpoint := BackendEndpointRegistration{
			Kind:    BackendEndpointFragment,
			Method:  fragment.Method,
			Path:    fragment.Route,
			Handler: "fragment",
			PageID:  fragment.PageID,
			Name:    fragment.FragmentName,
		}
		ir.Registrations = append(ir.Registrations, endpoint)
		ir.Responses = append(ir.Responses, BackendResponse{
			Endpoint: endpoint,
			NoStore:  true,
			Partial:  true,
		})
	}
	if options.IR != nil {
		for _, ref := range sortedContractReferences(options.IR.ContractRefs) {
			endpoint := BackendEndpointRegistration{
				Kind:    backendContractEndpointKind(ref.Kind),
				Method:  ref.Method,
				Path:    ref.Path,
				Handler: string(ref.Kind),
				PageID:  ref.OwnerID,
				Name:    ref.Name,
			}
			ir.ContractExposures = append(ir.ContractExposures, BackendContractExposure{
				Endpoint:    endpoint,
				Contract:    ref.Name,
				ImportAlias: ref.ImportAlias,
				ImportPath:  ref.ImportPath,
				Type:        ref.Type,
				Result:      ref.Result,
				Status:      ref.Status,
				Handler:     ref.Handler,
				Register:    ref.Register,
				Message:     ref.Message,
				OwnerKind:   ref.OwnerKind,
				OwnerID:     ref.OwnerID,
				Package:     ref.Package,
				Source:      ref.Source,
			})
		}
	}
	return ir
}

func (ir BackendAdapterIR) HasRegistrations() bool {
	return len(ir.Registrations) > 0 || len(routableContractExposures(ir.ContractExposures)) > 0
}

func backendContractEndpointKind(kind gwdkir.ContractKind) BackendEndpointKind {
	switch kind {
	case gwdkir.ContractQuery:
		return BackendEndpointQuery
	default:
		return BackendEndpointCommand
	}
}

func sortedContractReferences(refs []gwdkir.ContractReference) []gwdkir.ContractReference {
	out := append([]gwdkir.ContractReference(nil), refs...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].OwnerKind != out[j].OwnerKind {
			return out[i].OwnerKind < out[j].OwnerKind
		}
		if out[i].OwnerID != out[j].OwnerID {
			return out[i].OwnerID < out[j].OwnerID
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}
