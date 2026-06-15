package appgen

import (
	"path"
	"sort"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

type BackendAdapterIR struct {
	Registrations     []BackendEndpointRegistration
	Actions           []BackendActionAdapter
	APIs              []BackendAPIAdapter
	Fragments         []BackendFragmentAdapter
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
	Guards  []string
	Dynamic bool
	Source  string
	Span    source.SourceSpan
}

type BackendActionAdapter struct {
	Endpoint         BackendEndpointRegistration
	PageID           string
	ActionName       string
	Method           string
	Route            string
	Guards           []string
	InputName        string
	InputType        string
	InputFields      []string
	RequiredFields   []string
	RequiredMessages map[string]string
	ValidationRules  []ActionValidationRule
	ValidatesInput   bool
	Redirect         string
	Fragments        []ActionFragment
	ErrorPage        string
	Binding          source.BackendBinding
	BackendAlias     string
}

type BackendAPIAdapter struct {
	Endpoint     BackendEndpointRegistration
	PageID       string
	APIName      string
	Method       string
	Route        string
	Guards       []string
	ErrorPage    string
	Binding      source.BackendBinding
	BackendAlias string
}

type BackendFragmentAdapter struct {
	Endpoint     BackendEndpointRegistration
	PageID       string
	FragmentName string
	Method       string
	Route        string
	RouteParams  []source.RouteParam
	Target       string
	HTML         string
	Package      string
	Uses         map[string]string
	Guards       []string
	Binding      source.BackendBinding
	BackendAlias string
}

type BackendDecoder struct {
	Endpoint BackendEndpointRegistration
	Function string
	Input    string
	Fields   []string
}

type BackendHandlerCall struct {
	Endpoint   BackendEndpointRegistration
	Alias      string
	ImportPath string
	Function   string
	Signature  source.BackendSignatureKind
	InputType  string
}

type BackendResponse struct {
	Endpoint BackendEndpointRegistration
	NoStore  bool
	Partial  bool
	Redirect string
}

type BackendFallback struct {
	Endpoint BackendEndpointRegistration
	Status   source.BackendBindingStatus
	Message  string
}

type BackendContractExposure struct {
	Endpoint    BackendEndpointRegistration
	Contract    string
	ImportAlias string
	ImportPath  string
	Type        string
	Result      string
	Roles       []string
	Guards      []string
	InputFields []source.BackendInputField
	Status      gwdkir.ContractBindingStatus
	Handler     string
	Register    string
	Message     string
	OwnerKind   gwdkir.SourceKind
	OwnerID     string
	Package     string
	Source      string
	Span        source.SourceSpan
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
			Guards:  append([]string(nil), action.Guards...),
			Dynamic: backendRouteIsDynamic(action.Route),
			Source:  action.Source,
			Span:    action.SourceSpan,
		}
		actionAdapter := BackendActionAdapter{
			Endpoint:         endpoint,
			PageID:           action.PageID,
			ActionName:       action.ActionName,
			Method:           action.Method,
			Route:            action.Route,
			Guards:           append([]string(nil), action.Guards...),
			InputName:        action.InputName,
			InputType:        action.InputType,
			InputFields:      append([]string(nil), action.InputFields...),
			RequiredFields:   append([]string(nil), action.RequiredFields...),
			RequiredMessages: copyStringMap(action.RequiredMessages),
			ValidationRules:  append([]ActionValidationRule(nil), action.ValidationRules...),
			ValidatesInput:   action.ValidatesInput,
			Redirect:         action.Redirect,
			Fragments:        append([]ActionFragment(nil), action.Fragments...),
			ErrorPage:        action.ErrorPage,
			Binding:          action.Binding,
			BackendAlias:     action.BackendAlias,
		}
		ir.Registrations = append(ir.Registrations, endpoint)
		ir.Actions = append(ir.Actions, actionAdapter)
		if action.InputType != "" || action.Binding.InputType != "" {
			decoder := BackendDecoder{
				Endpoint: endpoint,
				Input:    action.InputType,
				Fields:   append([]string(nil), action.InputFields...),
			}
			if action.Binding.Status == source.BackendBindingBound && action.Binding.InputType != "" {
				decoder.Function = boundActionDecoderName(actionAdapter)
				decoder.Input = action.Binding.InputType
			} else if action.InputType != "" {
				decoder.Function = actionDecoderName(actionAdapter)
			}
			ir.Decoders = append(ir.Decoders, decoder)
		}
		if action.Binding.Status == source.BackendBindingBound {
			ir.Calls = append(ir.Calls, BackendHandlerCall{
				Endpoint:   endpoint,
				Alias:      action.BackendAlias,
				ImportPath: action.Binding.ImportPath,
				Function:   action.Binding.FunctionName,
				Signature:  action.Binding.Signature,
				InputType:  action.Binding.InputType,
			})
		}
		if action.Binding.Status != "" && action.Binding.Status != source.BackendBindingBound {
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
			Guards:  append([]string(nil), api.Guards...),
			Dynamic: backendRouteIsDynamic(api.Route),
			Source:  api.Source,
			Span:    api.SourceSpan,
		}
		apiAdapter := BackendAPIAdapter{
			Endpoint:     endpoint,
			PageID:       api.PageID,
			APIName:      api.APIName,
			Method:       api.Method,
			Route:        api.Route,
			Guards:       append([]string(nil), api.Guards...),
			ErrorPage:    api.ErrorPage,
			Binding:      api.Binding,
			BackendAlias: api.BackendAlias,
		}
		ir.Registrations = append(ir.Registrations, endpoint)
		ir.APIs = append(ir.APIs, apiAdapter)
		if api.Binding.Status == source.BackendBindingBound {
			ir.Calls = append(ir.Calls, BackendHandlerCall{
				Endpoint:   endpoint,
				Alias:      api.BackendAlias,
				ImportPath: api.Binding.ImportPath,
				Function:   api.Binding.FunctionName,
				Signature:  api.Binding.Signature,
			})
		}
		if api.Binding.Status != "" && api.Binding.Status != source.BackendBindingBound {
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
			Guards:  append([]string(nil), fragment.Guards...),
			Dynamic: backendRouteIsDynamic(fragment.Route),
			Source:  fragment.Source,
			Span:    fragment.SourceSpan,
		}
		fragmentAdapter := BackendFragmentAdapter{
			Endpoint:     endpoint,
			PageID:       fragment.PageID,
			FragmentName: fragment.FragmentName,
			Method:       fragment.Method,
			Route:        fragment.Route,
			RouteParams:  append([]source.RouteParam(nil), fragment.RouteParams...),
			Target:       fragment.Target,
			HTML:         fragment.HTML,
			Package:      fragment.Package,
			Uses:         copyStringMap(fragment.Uses),
			Guards:       append([]string(nil), fragment.Guards...),
			Binding:      fragment.Binding,
			BackendAlias: fragment.BackendAlias,
		}
		ir.Registrations = append(ir.Registrations, endpoint)
		ir.Fragments = append(ir.Fragments, fragmentAdapter)
		if fragment.Binding.Status == source.BackendBindingBound {
			ir.Calls = append(ir.Calls, BackendHandlerCall{
				Endpoint:   endpoint,
				Alias:      fragment.BackendAlias,
				ImportPath: fragment.Binding.ImportPath,
				Function:   fragment.Binding.FunctionName,
				Signature:  fragment.Binding.Signature,
			})
		}
		if fragment.Binding.Status != "" && fragment.Binding.Status != source.BackendBindingBound {
			ir.Fallbacks = append(ir.Fallbacks, BackendFallback{
				Endpoint: endpoint,
				Status:   fragment.Binding.Status,
				Message:  fragment.Binding.Message,
			})
		}
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
				Method:  source.BackendRouteMethod(ref.Method),
				Path:    ref.Path,
				Handler: string(ref.Kind),
				PageID:  ref.OwnerID,
				Name:    ref.Name,
				Guards:  append([]string(nil), ref.Guards...),
				Dynamic: backendRouteIsDynamic(ref.Path),
				Source:  ref.Source,
				Span:    ref.Span,
			}
			ir.ContractExposures = append(ir.ContractExposures, BackendContractExposure{
				Endpoint:    endpoint,
				Contract:    ref.Name,
				ImportAlias: ref.ImportAlias,
				ImportPath:  ref.ImportPath,
				Type:        ref.Type,
				Result:      ref.Result,
				Roles:       append([]string(nil), ref.Roles...),
				Guards:      append([]string(nil), ref.Guards...),
				InputFields: append([]source.BackendInputField(nil), ref.InputFields...),
				Status:      ref.Status,
				Handler:     ref.Handler,
				Register:    ref.Register,
				Message:     ref.Message,
				OwnerKind:   ref.OwnerKind,
				OwnerID:     ref.OwnerID,
				Package:     ref.Package,
				Source:      ref.Source,
				Span:        ref.Span,
			})
		}
	}
	reserveGeneratedBackendAdapterAliases(&ir)
	return ir
}

func reserveGeneratedBackendAdapterAliases(ir *BackendAdapterIR) {
	paths := map[string]string{}
	for _, call := range ir.Calls {
		if call.ImportPath != "" && call.Alias != "" {
			paths[call.ImportPath] = call.Alias
		}
	}
	for _, exposure := range ir.ContractExposures {
		if exposure.ImportPath != "" && exposure.ImportAlias != "" {
			paths[exposure.ImportPath] = exposure.ImportAlias
		}
	}
	if len(paths) == 0 {
		return
	}
	importPaths := make([]string, 0, len(paths))
	for importPath := range paths {
		importPaths = append(importPaths, importPath)
	}
	sort.Strings(importPaths)
	used := generatedImportAliasUseCounts()
	aliases := map[string]string{}
	for _, importPath := range importPaths {
		base := safeImportAlias(paths[importPath])
		if base == "" {
			base = safeImportAlias(path.Base(importPath))
		}
		if base == "" {
			base = "feature"
		}
		aliases[importPath] = nextImportAlias(base, used)
	}
	for index := range ir.Actions {
		if alias := aliases[ir.Actions[index].Binding.ImportPath]; alias != "" {
			ir.Actions[index].BackendAlias = alias
		}
	}
	for index := range ir.APIs {
		if alias := aliases[ir.APIs[index].Binding.ImportPath]; alias != "" {
			ir.APIs[index].BackendAlias = alias
		}
	}
	for index := range ir.Fragments {
		if alias := aliases[ir.Fragments[index].Binding.ImportPath]; alias != "" {
			ir.Fragments[index].BackendAlias = alias
		}
	}
	for index := range ir.Calls {
		if alias := aliases[ir.Calls[index].ImportPath]; alias != "" {
			ir.Calls[index].Alias = alias
		}
	}
	for index := range ir.ContractExposures {
		if alias := aliases[ir.ContractExposures[index].ImportPath]; alias != "" {
			ir.ContractExposures[index].ImportAlias = alias
		}
	}
}

func (ir BackendAdapterIR) HasRegistrations() bool {
	return len(ir.Registrations) > 0 || len(routableContractExposures(ir.ContractExposures)) > 0
}

func (ir BackendAdapterIR) HasEndpointKind(kind BackendEndpointKind) bool {
	for _, registration := range ir.Registrations {
		if registration.Kind == kind {
			return true
		}
	}
	for _, exposure := range routableContractExposures(ir.ContractExposures) {
		if exposure.Endpoint.Kind == kind {
			return true
		}
	}
	return false
}

func (ir BackendAdapterIR) HasDynamicRoutes() bool {
	for _, registration := range ir.Registrations {
		if registration.Dynamic {
			return true
		}
	}
	for _, exposure := range routableContractExposures(ir.ContractExposures) {
		if exposure.Endpoint.Dynamic {
			return true
		}
	}
	return false
}

func (ir BackendAdapterIR) GuardNames() []string {
	var guards []string
	for _, registration := range ir.Registrations {
		guards = append(guards, registration.Guards...)
	}
	for _, exposure := range routableContractExposures(ir.ContractExposures) {
		guards = append(guards, exposure.Guards...)
	}
	return guards
}

func (ir BackendAdapterIR) BackendImports() map[string]string {
	imports := map[string]string{}
	for _, call := range ir.Calls {
		if call.ImportPath != "" && call.Alias != "" {
			imports[call.ImportPath] = call.Alias
		}
	}
	return imports
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

func backendRouteIsDynamic(route string) bool {
	return len(ssrRoutePatternParams(route)) > 0
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
