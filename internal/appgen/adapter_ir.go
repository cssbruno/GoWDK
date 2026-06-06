package appgen

import "github.com/cssbruno/gowdk/internal/manifest"

type BackendAdapterIR struct {
	Registrations []BackendEndpointRegistration
	Decoders      []BackendDecoder
	Calls         []BackendHandlerCall
	Responses     []BackendResponse
	Fallbacks     []BackendFallback
}

type BackendEndpointKind string

const (
	BackendEndpointAction   BackendEndpointKind = "action"
	BackendEndpointAPI      BackendEndpointKind = "api"
	BackendEndpointFragment BackendEndpointKind = "fragment"
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
	return ir
}

func (ir BackendAdapterIR) HasRegistrations() bool {
	return len(ir.Registrations) > 0
}
