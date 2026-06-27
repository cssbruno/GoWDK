package buildgen

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/cssscope"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	view "github.com/cssbruno/gowdk/internal/viewrender"
)

func buildComponents(components []gwdkir.Component) (map[string]view.Component, []string) {
	registry := map[string]view.Component{}
	var failures []string
	for _, component := range components {
		valid := true
		if component.Name == "" {
			failures = append(failures, "component missing name")
			continue
		}
		key := componentRegistryKey(component.Package, component.Name)
		if _, exists := registry[key]; exists {
			failures = append(failures, fmt.Sprintf("duplicate component %q", component.Name))
			continue
		}
		if !isComponentName(component.Name) {
			failures = append(failures, fmt.Sprintf("component %q must start with an uppercase letter", component.Name))
			continue
		}
		if !component.Blocks.View {
			failures = append(failures, fmt.Sprintf("component %s missing view {}", component.Name))
			continue
		}
		if strings.TrimSpace(component.Blocks.ViewBody) == "" {
			failures = append(failures, fmt.Sprintf("component %s view {} is empty", component.Name))
			continue
		}

		props, propTypes, propDefaults, propFailures := componentProps(component)
		for _, failure := range propFailures {
			failures = append(failures, failure)
			valid = false
		}
		state, stateTypes, stateJSON, err := componentInitialState(component)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s state: %v", component.Name, err))
			valid = false
		}
		emits := componentEmits(component)
		computeds, computedFailures := componentClientComputeds(component)
		for _, failure := range computedFailures {
			failures = append(failures, failure)
			valid = false
		}
		exports, exportNames, exportFailures := componentExports(component, propTypes, stateTypes, computeds)
		for _, failure := range exportFailures {
			failures = append(failures, failure)
			valid = false
		}
		handlers, handlersJSON, err := componentClientHandlers(component, exportNames)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s client: %v", component.Name, err))
			valid = false
		}
		refs, refFailures := componentClientRefs(component)
		for _, failure := range refFailures {
			failures = append(failures, failure)
			valid = false
		}
		if !valid {
			continue
		}
		compiled := view.Component{
			Name:          component.Name,
			Package:       component.Package,
			Uses:          componentUses(component.Uses),
			JS:            append([]string(nil), component.JS...),
			InlineJS:      viewInlineScripts(component.InlineJS),
			ScopeIDs:      componentScopeIDs(component),
			DefaultIsland: componentDefaultIsland(component),
			Props:         props,
			PropTypes:     propTypes,
			PropDefaults:  propDefaults,
			State:         state,
			StateJSON:     stateJSON,
			Handlers:      handlers,
			HandlersJSON:  handlersJSON,
			StateTypes:    stateTypes,
			Refs:          refs,
			Emits:         emits,
			Exports:       exports,
			Computed:      computeds,
			Body:          component.Blocks.ViewBody,
			Nodes:         append([]view.Node(nil), component.Blocks.ViewNodes...),
		}
		registry[key] = compiled
		if component.Package == "" {
			registry[component.Name] = compiled
		}
	}
	return registry, failures
}

func viewInlineScripts(scripts []source.InlineScript) []view.InlineScript {
	if len(scripts) == 0 {
		return nil
	}
	out := make([]view.InlineScript, 0, len(scripts))
	for _, script := range scripts {
		out = append(out, view.InlineScript{Name: script.Name, Body: script.Body})
	}
	return out
}

func componentDefaultIsland(component gwdkir.Component) string {
	if strings.TrimSpace(component.WASM.Package) != "" {
		return "wasm"
	}
	return ""
}

func componentScopeIDs(component gwdkir.Component) []string {
	if len(component.CSS) == 0 && strings.TrimSpace(component.Blocks.StyleBody) == "" {
		return nil
	}
	scopeIDs := make([]string, 0, len(component.CSS)+1)
	for _, css := range component.CSS {
		hashKey := cssscope.HashKey("component", component.Package, component.Name, component.Source, css)
		scopeIDs = append(scopeIDs, cssscope.ScopeID(hashKey))
	}
	if strings.TrimSpace(component.Blocks.StyleBody) != "" {
		hashKey := cssscope.HashKey("component", component.Package, component.Name, component.Source, inlineStyleAssetPath)
		scopeIDs = append(scopeIDs, cssscope.ScopeID(hashKey))
	}
	return scopeIDs
}

func componentUses(uses []gwdkir.Use) map[string]string {
	if len(uses) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, use := range uses {
		out[use.Alias] = use.Package
	}
	return out
}

func componentClientComputeds(component gwdkir.Component) ([]clientlang.Computed, []string) {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return nil, nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, []string{fmt.Sprintf("component %s client: %v", component.Name, err)}
	}
	computeds, err := program.OrderedComputed()
	if err != nil {
		return nil, []string{fmt.Sprintf("component %s computed dependency graph: %v", component.Name, err)}
	}
	return computeds, nil
}

func componentClientRefs(component gwdkir.Component) (map[string]clientlang.Ref, []string) {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return nil, nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, []string{fmt.Sprintf("component %s client: %v", component.Name, err)}
	}
	return program.RefMap(), nil
}

func componentClientHandlers(component gwdkir.Component, exports []string) (map[string]clientlang.Handler, string, error) {
	emits := componentEmits(component)
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" && len(emits) == 0 && len(exports) == 0 {
		return nil, "", nil
	}
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		payload, err := json.Marshal(clientlang.Bootstrap{Emits: emits, Exports: exports})
		if err != nil {
			return nil, "", err
		}
		return nil, string(payload), nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, "", err
	}
	handlers := program.HandlerMap()
	helpers := program.HelperMap()
	if len(handlers) == 0 && len(helpers) == 0 && !program.NeedsBootstrap() && len(emits) == 0 && len(exports) == 0 {
		return nil, "", nil
	}
	computeds, err := program.OrderedComputed()
	if err != nil {
		return nil, "", err
	}
	var payload []byte
	if program.NeedsBootstrap() || len(emits) > 0 || len(exports) > 0 {
		payload, err = json.Marshal(clientlang.Bootstrap{
			Handlers: handlers,
			Helpers:  helpers,
			Emits:    emits,
			Exports:  exports,
			Stores:   program.StoreNames(),
			Mount:    append([]string(nil), program.Mount...),
			Destroy:  append([]string(nil), program.Destroy...),
			Effects:  append([]clientlang.Effect(nil), program.Effects...),
			Computed: computeds,
		})
	} else {
		payload, err = json.Marshal(handlers)
	}
	if err != nil {
		return nil, "", err
	}
	return handlers, string(payload), nil
}

func componentEmits(component gwdkir.Component) map[string]clientlang.Emit {
	if len(component.Emits) == 0 {
		return nil
	}
	out := map[string]clientlang.Emit{}
	for _, event := range component.Emits {
		params := make([]string, 0, len(event.Params))
		paramTypes := make([]clientlang.ValueType, 0, len(event.Params))
		for _, param := range event.Params {
			params = append(params, param.Name)
			paramTypes = append(paramTypes, clientlang.NormalizeType(param.Type))
		}
		out[event.Name] = clientlang.Emit{Name: event.Name, Params: params, ParamTypes: paramTypes}
	}
	return out
}

func componentExports(component gwdkir.Component, propTypes map[string]clientlang.ValueType, stateTypes map[string]clientlang.ValueType, computeds []clientlang.Computed) (map[string]clientlang.ValueType, []string, []string) {
	if len(component.Exports) == 0 {
		return nil, nil, nil
	}
	computedTypes := map[string]clientlang.ValueType{}
	for _, computed := range computeds {
		computedTypes[computed.Name] = clientlang.NormalizeType(computed.Type)
	}
	out := map[string]clientlang.ValueType{}
	names := make([]string, 0, len(component.Exports))
	seen := map[string]bool{}
	var failures []string
	for _, export := range component.Exports {
		if seen[export.Name] {
			failures = append(failures, fmt.Sprintf("component %s declares duplicate export %q", component.Name, export.Name))
			continue
		}
		if export.Name == gwdkir.ComponentExportActiveFlag {
			failures = append(failures, fmt.Sprintf("component %s export %q uses reserved name %q; the exports payload reserves it for the mount flag", component.Name, export.Name, gwdkir.ComponentExportActiveFlag))
			continue
		}
		seen[export.Name] = true
		expected := clientlang.NormalizeType(export.Type)
		if expected == clientlang.TypeUnknown || expected == clientlang.TypeArray || expected == clientlang.TypeObject {
			failures = append(failures, fmt.Sprintf("component %s export %s uses unsupported type %q", component.Name, export.Name, export.Type))
			continue
		}
		actual, ok := propTypes[export.Name]
		if !ok {
			actual, ok = stateTypes[export.Name]
		}
		if !ok {
			actual, ok = computedTypes[export.Name]
		}
		if !ok {
			failures = append(failures, fmt.Sprintf("component %s export %q must reference a declared prop, state field, or computed value", component.Name, export.Name))
			continue
		}
		if actual != clientlang.TypeUnknown && actual != expected && !compatibleClientNumericTypes(actual, expected) {
			failures = append(failures, fmt.Sprintf("component %s export %q declares %s but local symbol is %s", component.Name, export.Name, expected, actual))
			continue
		}
		out[export.Name] = expected
		names = append(names, export.Name)
	}
	if len(out) == 0 {
		out = nil
		names = nil
	}
	return out, names, failures
}

func compatibleClientNumericTypes(actual clientlang.ValueType, expected clientlang.ValueType) bool {
	return (actual == clientlang.TypeInt || actual == clientlang.TypeFloat) &&
		(expected == clientlang.TypeInt || expected == clientlang.TypeFloat)
}

func componentProps(component gwdkir.Component) ([]string, map[string]clientlang.ValueType, map[string]string, []string) {
	if component.PropsType.Name != "" {
		resolved, err := gotypes.ResolveStruct(component.Imports, component.PropsType)
		if err != nil {
			return nil, nil, nil, []string{fmt.Sprintf("component %s props: %v", component.Name, err)}
		}
		propTypes := map[string]clientlang.ValueType{}
		for _, field := range resolved.Fields {
			propTypes[field.Name] = clientlang.NormalizeType(field.Type)
		}
		for field, typ := range resolved.FieldTypes {
			propTypes[field] = clientlang.NormalizeType(typ)
		}
		return resolved.FieldNames(), propTypes, nil, nil
	}
	props := make([]string, 0, len(component.Props))
	propTypes := map[string]clientlang.ValueType{}
	propDefaults := map[string]string{}
	seen := map[string]bool{}
	var failures []string
	for _, prop := range component.Props {
		propType := clientlang.NormalizeType(prop.Type)
		if propType == clientlang.TypeUnknown || propType == clientlang.TypeArray || propType == clientlang.TypeObject {
			failures = append(failures, fmt.Sprintf("component %s prop %s uses unsupported type %q", component.Name, prop.Name, prop.Type))
			continue
		}
		if seen[prop.Name] {
			failures = append(failures, fmt.Sprintf("component %s declares duplicate prop %q", component.Name, prop.Name))
			continue
		}
		seen[prop.Name] = true
		props = append(props, prop.Name)
		propTypes[prop.Name] = propType
		if prop.DefaultSet {
			propDefaults[prop.Name] = prop.Default
		}
	}
	if len(propDefaults) == 0 {
		propDefaults = nil
	}
	return props, propTypes, propDefaults, failures
}

func componentInitialState(component gwdkir.Component) (map[string]string, map[string]clientlang.ValueType, string, error) {
	state := map[string]string{}
	stateTypes := map[string]clientlang.ValueType{}
	raw := map[string]any{}
	stateJSON := ""

	if component.State.Type.Name != "" {
		resolved, err := gotypes.ResolveStruct(component.Imports, component.State.Type)
		if err != nil {
			return nil, nil, "", err
		}
		rawJSON, err := gotypes.RunStateInitJSON(component.Imports, component.State)
		if err != nil {
			return nil, nil, "", err
		}
		if err := json.Unmarshal(rawJSON, &raw); err != nil {
			return nil, nil, "", fmt.Errorf("decode state JSON: %w", err)
		}
		for _, field := range resolved.Fields {
			value, ok := raw[field.Name]
			if !ok {
				return nil, nil, "", fmt.Errorf("init JSON missing field %q", field.Name)
			}
			scalar, ok := stateValueString(value)
			if !ok {
				return nil, nil, "", fmt.Errorf("field %s must initialize to JSON-compatible state", field.Name)
			}
			state[field.Name] = scalar
			stateTypes[field.Name] = clientlang.NormalizeType(field.Type)
		}
		for field, typ := range resolved.FieldTypes {
			stateTypes[field] = clientlang.NormalizeType(typ)
		}
		stateJSON = string(rawJSON)
	}

	// A typed `use <store> <Type>` declaration contributes the store's fields to
	// the island seed so SSR and the initial client state carry the right shape;
	// the store registry merges its actual (init or persisted) value on mount.
	added := mergeComponentStoreSeed(component, state, stateTypes, raw)
	if len(state) == 0 && !added {
		return nil, nil, "", nil
	}
	// Preserve the exact state init serialization when no store fields were added,
	// so existing state-only components keep their generated seed verbatim.
	if added {
		merged, err := json.Marshal(raw)
		if err != nil {
			return nil, nil, "", err
		}
		stateJSON = string(merged)
	}
	return state, stateTypes, stateJSON, nil
}

// mergeComponentStoreSeed adds zero-value seeds for the fields of any typed
// `use <store> <Type>` declaration that are not already seeded by a local state
// contract. It reports whether any field was added so the caller knows to
// re-marshal the seed JSON. Type-resolution failures are reported by contract
// validation, so they are swallowed here.
func mergeComponentStoreSeed(component gwdkir.Component, state map[string]string, stateTypes map[string]clientlang.ValueType, raw map[string]any) bool {
	if strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return false
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return false
	}
	added := false
	for _, use := range program.Uses {
		if use.Type == "" {
			continue
		}
		resolved, err := gotypes.ResolveStruct(component.Imports, gwdkir.GoRefFromLiteral(use.Type))
		if err != nil {
			continue
		}
		for _, field := range resolved.Fields {
			if _, exists := raw[field.Name]; exists {
				continue
			}
			typ := clientlang.NormalizeType(field.Type)
			zero := zeroStateValue(typ)
			raw[field.Name] = zero
			scalar, _ := stateValueString(zero)
			state[field.Name] = scalar
			stateTypes[field.Name] = typ
			added = true
		}
		for field, typ := range resolved.FieldTypes {
			if _, exists := stateTypes[field]; !exists {
				stateTypes[field] = clientlang.NormalizeType(typ)
			}
		}
	}
	return added
}

func zeroStateValue(typ clientlang.ValueType) any {
	switch typ {
	case clientlang.TypeInt, clientlang.TypeFloat:
		return float64(0)
	case clientlang.TypeBool:
		return false
	case clientlang.TypeString:
		return ""
	case clientlang.TypeArray:
		return []any{}
	case clientlang.TypeObject:
		return map[string]any{}
	default:
		return nil
	}
}

func stateValueString(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", true
	case string:
		return typed, true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	case []any, map[string]any:
		payload, err := json.Marshal(typed)
		if err != nil {
			return "", false
		}
		return string(payload), true
	default:
		return "", false
	}
}

func buildLayouts(layouts []gwdkir.Layout) (map[string]gwdkir.Layout, []string) {
	registry := map[string]gwdkir.Layout{}
	var failures []string
	for _, layout := range layouts {
		if layout.ID == "" {
			failures = append(failures, "layout missing ID")
			continue
		}
		key := layoutRegistryKey(layout.Package, layout.ID)
		if _, exists := registry[key]; exists {
			failures = append(failures, fmt.Sprintf("duplicate layout %q", layoutRegistryDisplayName(layout.Package, layout.ID)))
			continue
		}
		if !layout.Blocks.View {
			failures = append(failures, fmt.Sprintf("layout %s missing view {}", layout.ID))
			continue
		}
		if strings.TrimSpace(layout.Blocks.ViewBody) == "" {
			failures = append(failures, fmt.Sprintf("layout %s view {} is empty", layout.ID))
			continue
		}
		registry[key] = layout
	}
	return registry, failures
}

func layoutRegistryKey(packageName, layoutID string) string {
	if packageName == "" {
		return layoutID
	}
	return packageName + "." + layoutID
}

func layoutRegistryDisplayName(packageName, layoutID string) string {
	if packageName == "" {
		return layoutID
	}
	return packageName + "." + layoutID
}

func isComponentName(value string) bool {
	if value == "" {
		return false
	}
	first := []rune(value)[0]
	return first >= 'A' && first <= 'Z'
}
