package buildgen

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

const openAPIFile = "openapi.json"

type openAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       openAPIInfo            `json:"info"`
	Servers    []openAPIServer        `json:"servers"`
	Paths      map[string]openAPIPath `json:"paths"`
	Components openAPIComponents      `json:"components,omitempty"`
	XGOWDK     map[string]string      `json:"x-gowdk,omitempty"`
}

type openAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type openAPIServer struct {
	URL string `json:"url"`
}

type openAPIPath map[string]openAPIOperation

type openAPIOperation struct {
	OperationID string                     `json:"operationId"`
	Summary     string                     `json:"summary,omitempty"`
	Tags        []string                   `json:"tags,omitempty"`
	Parameters  []openAPIParameter         `json:"parameters,omitempty"`
	RequestBody *openAPIRequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]openAPIResponse `json:"responses"`
	XGOWDK      openAPIGOWDKExtension      `json:"x-gowdk"`
}

type openAPIParameter struct {
	Name     string        `json:"name"`
	In       string        `json:"in"`
	Required bool          `json:"required"`
	Schema   openAPISchema `json:"schema"`
}

type openAPIRequestBody struct {
	Required bool                        `json:"required,omitempty"`
	Content  map[string]openAPIMediaType `json:"content"`
}

type openAPIMediaType struct {
	Schema openAPISchema `json:"schema"`
}

type openAPIResponse struct {
	Description string                      `json:"description"`
	Content     map[string]openAPIMediaType `json:"content,omitempty"`
}

type openAPIComponents struct {
	Schemas map[string]openAPISchema `json:"schemas,omitempty"`
}

type openAPISchema struct {
	Ref                  string                   `json:"$ref,omitempty"`
	Type                 string                   `json:"type,omitempty"`
	Format               string                   `json:"format,omitempty"`
	Items                *openAPISchema           `json:"items,omitempty"`
	Properties           map[string]openAPISchema `json:"properties,omitempty"`
	AdditionalProperties *bool                    `json:"additionalProperties,omitempty"`
	XGoType              string                   `json:"x-go-type,omitempty"`
}

type openAPIGOWDKExtension struct {
	Kind           string   `json:"kind"`
	Route          string   `json:"route,omitempty"`
	Source         string   `json:"source,omitempty"`
	PageID         string   `json:"pageId,omitempty"`
	Symbol         string   `json:"symbol,omitempty"`
	EndpointSource string   `json:"endpointSource,omitempty"`
	Cache          string   `json:"cache,omitempty"`
	Guards         []string `json:"guards,omitempty"`
	CSRF           bool     `json:"csrf,omitempty"`
	BindingStatus  string   `json:"bindingStatus,omitempty"`
	Signature      string   `json:"signature,omitempty"`
	InputType      string   `json:"inputType,omitempty"`
	Roles          []string `json:"roles,omitempty"`
}

func writeOpenAPI(outputDir string, config gowdk.Config, ir gwdkir.Program) (string, error) {
	payload, err := openAPIPayload(config, ir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(outputDir, openAPIFile)
	if err := writeFileIfChanged(path, payload); err != nil {
		return "", err
	}
	return path, nil
}

func openAPIPayload(config gowdk.Config, ir gwdkir.Program) ([]byte, error) {
	spec := buildOpenAPISpec(config, ir)
	payload, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func buildOpenAPISpec(config gowdk.Config, ir gwdkir.Program) openAPISpec {
	components := map[string]openAPISchema{}
	spec := openAPISpec{
		OpenAPI: "3.1.0",
		Info: openAPIInfo{
			Title:   "GOWDK routable web surface",
			Version: "0",
		},
		Servers: []openAPIServer{{URL: "/"}},
		Paths:   map[string]openAPIPath{},
		XGOWDK:  map[string]string{"schema": "gowdk.openapi.v1"},
	}
	seenOperationIDs := map[string]int{}
	endpoints := append([]gwdkir.Endpoint(nil), ir.Endpoints...)
	sort.Slice(endpoints, func(i, j int) bool {
		return endpointSortKey(endpoints[i]) < endpointSortKey(endpoints[j])
	})
	for _, endpoint := range endpoints {
		addOpenAPIEndpointOperation(spec.Paths, components, seenOperationIDs, endpoint)
	}
	refs := append([]gwdkir.ContractReference(nil), ir.ContractRefs...)
	sort.Slice(refs, func(i, j int) bool {
		return contractRefSortKey(refs[i]) < contractRefSortKey(refs[j])
	})
	for _, ref := range refs {
		if !contractRefIsWebRoutable(ref) {
			continue
		}
		addOpenAPIContractOperation(spec.Paths, components, seenOperationIDs, config, ref)
	}
	if len(components) > 0 {
		spec.Components = openAPIComponents{Schemas: components}
	}
	return spec
}

func addOpenAPIEndpointOperation(paths map[string]openAPIPath, components map[string]openAPISchema, seen map[string]int, endpoint gwdkir.Endpoint) {
	method := strings.ToLower(strings.TrimSpace(endpoint.Method))
	if method == "" || strings.TrimSpace(endpoint.Path) == "" {
		return
	}
	operationID := uniqueOperationID(seen, operationID("endpoint", string(endpoint.Kind), endpoint.PageID, endpoint.Symbol, method, endpoint.Path))
	operation := openAPIOperation{
		OperationID: operationID,
		Summary:     endpointSummary(string(endpoint.Kind), endpoint.Symbol, endpoint.Path),
		Tags:        []string{string(endpoint.Kind)},
		Parameters:  pathParameters(openAPIPathFromGOWDK(endpoint.Path), endpoint.RouteParams),
		Responses:   endpointResponsesForEndpoint(components, endpoint),
		XGOWDK: openAPIGOWDKExtension{
			Kind:           string(endpoint.Kind),
			Route:          endpoint.Path,
			Source:         endpoint.SourceFile,
			PageID:         endpoint.PageID,
			Symbol:         endpoint.Symbol,
			EndpointSource: string(endpoint.Source),
			Cache:          endpoint.Cache,
			Guards:         append([]string(nil), endpoint.Guards...),
			CSRF:           endpoint.CSRF,
			BindingStatus:  string(endpoint.Binding.Status),
			Signature:      string(endpoint.Binding.Signature),
			InputType:      endpoint.Binding.InputType,
		},
	}
	if len(endpoint.Binding.InputFields) > 0 {
		attachInputFields(&operation, components, method, endpoint.Binding.InputType, endpoint.Binding.InputFields, endpoint.Kind == gwdkir.EndpointAPI)
	}
	addOpenAPIOperation(paths, openAPIPathFromGOWDK(endpoint.Path), method, operation)
}

func endpointResponsesForEndpoint(components map[string]openAPISchema, endpoint gwdkir.Endpoint) map[string]openAPIResponse {
	if endpoint.Binding.ResultType == "" {
		return endpointResponses(components, string(endpoint.Kind), "", nil)
	}
	response := openAPIResponse{Description: "OK"}
	name := schemaComponentName(endpoint.Binding.ResultType)
	if _, ok := components[name]; !ok {
		schema := openAPISchema{Type: "object", XGoType: endpoint.Binding.ResultType}
		if len(endpoint.Binding.ResultFields) > 0 {
			schema = objectSchemaFromResultFields(endpoint.Binding.ResultFields)
			schema.XGoType = endpoint.Binding.ResultType
		}
		components[name] = schema
	}
	response.Content = map[string]openAPIMediaType{
		"application/json": {Schema: openAPISchema{Ref: "#/components/schemas/" + name}},
	}
	return map[string]openAPIResponse{"200": response}
}

func addOpenAPIContractOperation(paths map[string]openAPIPath, components map[string]openAPISchema, seen map[string]int, config gowdk.Config, ref gwdkir.ContractReference) {
	method := strings.ToLower(strings.TrimSpace(ref.Method))
	if method == "" || strings.TrimSpace(ref.Path) == "" {
		return
	}
	kind := string(ref.Kind)
	operationID := uniqueOperationID(seen, operationID("contract", kind, ref.OwnerID, ref.Name, method, ref.Path))
	operation := openAPIOperation{
		OperationID: operationID,
		Summary:     endpointSummary(kind, ref.Name, ref.Path),
		Tags:        []string{kind},
		Parameters:  pathParameters(openAPIPathFromGOWDK(ref.Path), nil),
		Responses:   endpointResponses(components, kind, ref.Result, ref.ResultFields),
		XGOWDK: openAPIGOWDKExtension{
			Kind:           kind,
			Route:          ref.Path,
			Source:         ref.Source,
			PageID:         ref.OwnerID,
			Symbol:         ref.Name,
			EndpointSource: "contract",
			Cache:          "no-store",
			Guards:         append([]string(nil), ref.Guards...),
			CSRF:           config.Build.CSRF.EnabledForGeneratedEndpoints() && ref.Kind == gwdkir.ContractCommand,
			BindingStatus:  string(ref.Status),
			InputType:      ref.Type,
			Roles:          append([]string(nil), ref.Roles...),
		},
	}
	if len(ref.InputFields) > 0 {
		attachInputFields(&operation, components, method, ref.Type, ref.InputFields, false)
	}
	addOpenAPIOperation(paths, openAPIPathFromGOWDK(ref.Path), method, operation)
}

func addOpenAPIOperation(paths map[string]openAPIPath, path string, method string, operation openAPIOperation) {
	if paths[path] == nil {
		paths[path] = openAPIPath{}
	}
	paths[path][method] = operation
}

func attachInputFields(operation *openAPIOperation, components map[string]openAPISchema, method string, inputType string, fields []source.BackendInputField, jsonBody bool) {
	if method == "get" {
		for _, field := range fields {
			name := field.FormName
			if name == "" {
				name = field.FieldName
			}
			operation.Parameters = append(operation.Parameters, openAPIParameter{
				Name:     name,
				In:       "query",
				Required: false,
				Schema:   schemaForGoType(field.Type),
			})
		}
		return
	}
	schema := objectSchemaFromFields(fields)
	if inputType != "" {
		components[schemaComponentName(inputType)] = schema
		schema = openAPISchema{Ref: "#/components/schemas/" + schemaComponentName(inputType)}
	}
	contentType := "application/x-www-form-urlencoded"
	if jsonBody {
		contentType = "application/json"
	}
	operation.RequestBody = &openAPIRequestBody{
		Content: map[string]openAPIMediaType{
			contentType: {Schema: schema},
		},
	}
}

func endpointResponses(components map[string]openAPISchema, kind string, resultType string, resultFields []source.BackendInputField) map[string]openAPIResponse {
	response := openAPIResponse{Description: "OK"}
	if resultType != "" {
		name := schemaComponentName(resultType)
		if _, ok := components[name]; !ok {
			schema := openAPISchema{Type: "object", XGoType: resultType}
			if len(resultFields) > 0 {
				schema = objectSchemaFromFields(resultFields)
				schema.XGoType = resultType
			}
			components[name] = schema
		}
		response.Content = map[string]openAPIMediaType{
			"application/json": {Schema: openAPISchema{Ref: "#/components/schemas/" + name}},
		}
	} else if kind == string(gwdkir.EndpointFragment) {
		response.Content = map[string]openAPIMediaType{
			"text/html": {Schema: openAPISchema{Type: "string"}},
		}
	}
	return map[string]openAPIResponse{"200": response}
}

func objectSchemaFromFields(fields []source.BackendInputField) openAPISchema {
	properties := map[string]openAPISchema{}
	for _, field := range fields {
		name := field.FormName
		if name == "" {
			name = field.FieldName
		}
		if name == "" {
			continue
		}
		properties[name] = schemaForGoType(field.Type)
	}
	return openAPISchema{Type: "object", Properties: properties}
}

func objectSchemaFromResultFields(fields []source.BackendResultField) openAPISchema {
	properties := map[string]openAPISchema{}
	for _, field := range fields {
		name := field.Path
		if name == "" {
			continue
		}
		setNestedOpenAPIProperty(properties, strings.Split(name, "."), schemaForOpenAPIGoType(field.Type))
	}
	return openAPISchema{Type: "object", Properties: properties}
}

func setNestedOpenAPIProperty(properties map[string]openAPISchema, parts []string, schema openAPISchema) {
	if len(parts) == 0 || parts[0] == "" {
		return
	}
	name := parts[0]
	if len(parts) == 1 {
		existing := properties[name]
		if len(existing.Properties) > 0 && schema.Type == "object" {
			schema.Properties = existing.Properties
		}
		properties[name] = schema
		return
	}
	parent := properties[name]
	if parent.Type == "" {
		parent.Type = "object"
	}
	if parent.Properties == nil {
		parent.Properties = map[string]openAPISchema{}
	}
	setNestedOpenAPIProperty(parent.Properties, parts[1:], schema)
	properties[name] = parent
}

func schemaForGoType(goType string) openAPISchema {
	return schemaForOpenAPIGoType(goType)
}

func schemaForOpenAPIGoType(goType string) openAPISchema {
	fieldType, ok := source.LookupBackendInputFieldType(goType)
	if !ok {
		return openAPISchema{Type: "object", XGoType: goType}
	}
	switch fieldType.Kind {
	case source.BackendInputFieldKindBool:
		return openAPISchema{Type: "boolean"}
	case source.BackendInputFieldKindSignedInt, source.BackendInputFieldKindUnsignedInt:
		return openAPISchema{Type: "integer", Format: "int64"}
	case source.BackendInputFieldKindStringSlice:
		item := openAPISchema{Type: "string"}
		return openAPISchema{Type: "array", Items: &item}
	case source.BackendInputFieldKindFile:
		return openAPISchema{Type: "string", Format: "binary"}
	case source.BackendInputFieldKindFileSlice:
		item := openAPISchema{Type: "string", Format: "binary"}
		return openAPISchema{Type: "array", Items: &item}
	case source.BackendInputFieldKindString:
		return openAPISchema{Type: "string"}
	default:
		panic("unsupported backend input field kind: " + string(fieldType.Kind))
	}
}

func pathParameters(path string, routeParams []source.RouteParam) []openAPIParameter {
	names := openAPIPathParamNames(path)
	if len(names) == 0 {
		return nil
	}
	types := map[string]string{}
	for _, param := range routeParams {
		types[param.Name] = param.Type
	}
	out := make([]openAPIParameter, 0, len(names))
	for _, name := range names {
		out = append(out, openAPIParameter{
			Name:     name,
			In:       "path",
			Required: true,
			Schema:   schemaForRouteParam(types[name]),
		})
	}
	return out
}

func schemaForRouteParam(paramType string) openAPISchema {
	switch strings.TrimSpace(paramType) {
	case "int", "int64":
		return openAPISchema{Type: "integer", Format: "int64"}
	case "uint", "uint64":
		return openAPISchema{Type: "integer", Format: "uint64"}
	case "bool":
		return openAPISchema{Type: "boolean"}
	case "float64":
		return openAPISchema{Type: "number", Format: "double"}
	default:
		return openAPISchema{Type: "string"}
	}
}

func openAPIPathParamNames(path string) []string {
	seen := map[string]bool{}
	var names []string
	for _, segment := range strings.Split(path, "/") {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		name = strings.TrimSuffix(name, "...")
		if before, _, ok := strings.Cut(name, ":"); ok {
			name = before
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func openAPIPathFromGOWDK(path string) string {
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		name = strings.TrimSuffix(name, "...")
		if before, _, ok := strings.Cut(name, ":"); ok {
			name = before
		}
		segments[index] = "{" + name + "}"
	}
	return strings.Join(segments, "/")
}

func endpointSummary(kind string, name string, path string) string {
	if strings.TrimSpace(name) == "" {
		return kind + " " + path
	}
	return kind + " " + name
}

func contractRefIsWebRoutable(ref gwdkir.ContractReference) bool {
	if ref.Status != "" && ref.Status != gwdkir.ContractBindingBound {
		return false
	}
	if strings.TrimSpace(ref.Method) == "" || strings.TrimSpace(ref.Path) == "" {
		return false
	}
	if len(ref.Roles) == 0 {
		return true
	}
	for _, role := range ref.Roles {
		if role == "web" {
			return true
		}
	}
	return false
}

func uniqueOperationID(seen map[string]int, value string) string {
	if seen[value] == 0 {
		seen[value] = 1
		return value
	}
	seen[value]++
	return value + "_" + strconv.Itoa(seen[value])
}

func operationID(parts ...string) string {
	var tokens []string
	for _, part := range parts {
		token := identifierToken(part)
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) == 0 {
		return "gowdkOperation"
	}
	return strings.Join(tokens, "_")
}

func schemaComponentName(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.LastIndex(value, "."); index >= 0 {
		value = value[index+1:]
	}
	token := identifierToken(value)
	if token == "" {
		return "GOWDKSchema"
	}
	return token
}

func identifierToken(value string) string {
	var out []rune
	upperNext := false
	for _, r := range value {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			if len(out) == 0 && unicode.IsDigit(r) {
				out = append(out, '_')
			}
			if upperNext {
				out = append(out, unicode.ToUpper(r))
				upperNext = false
				continue
			}
			out = append(out, r)
			continue
		}
		upperNext = len(out) > 0
	}
	return string(out)
}

func endpointSortKey(endpoint gwdkir.Endpoint) string {
	return strings.Join([]string{endpoint.Path, endpoint.Method, string(endpoint.Kind), endpoint.PageID, endpoint.Symbol}, "\x00")
}

func contractRefSortKey(ref gwdkir.ContractReference) string {
	return strings.Join([]string{ref.Path, ref.Method, string(ref.Kind), ref.OwnerID, ref.Name}, "\x00")
}
