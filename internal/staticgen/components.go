package staticgen

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

func buildComponents(components []manifest.Component) (map[string]view.Component, []string) {
	registry := map[string]view.Component{}
	var failures []string
	for _, component := range components {
		valid := true
		if component.Name == "" {
			failures = append(failures, "component missing name")
			continue
		}
		if _, exists := registry[component.Name]; exists {
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

		props, propFailures := componentPropNames(component)
		for _, failure := range propFailures {
			failures = append(failures, failure)
			valid = false
		}
		state, stateTypes, stateJSON, err := componentInitialState(component)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s state: %v", component.Name, err))
			valid = false
		}
		handlers, handlersJSON, err := componentClientHandlers(component)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s client: %v", component.Name, err))
			valid = false
		}
		refs, refFailures := componentClientRefs(component)
		for _, failure := range refFailures {
			failures = append(failures, failure)
			valid = false
		}
		emits := componentEmits(component)
		computeds, computedFailures := componentClientComputeds(component)
		for _, failure := range computedFailures {
			failures = append(failures, failure)
			valid = false
		}
		if !valid {
			continue
		}
		registry[component.Name] = view.Component{
			Name:         component.Name,
			Props:        props,
			State:        state,
			StateJSON:    stateJSON,
			Handlers:     handlers,
			HandlersJSON: handlersJSON,
			StateTypes:   stateTypes,
			Refs:         refs,
			Emits:        emits,
			Computed:     computeds,
			Body:         component.Blocks.ViewBody,
		}
	}
	return registry, failures
}

func componentClientComputeds(component manifest.Component) ([]clientlang.Computed, []string) {
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

func componentClientRefs(component manifest.Component) (map[string]clientlang.Ref, []string) {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return nil, nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, []string{fmt.Sprintf("component %s client: %v", component.Name, err)}
	}
	return program.RefMap(), nil
}

func componentClientHandlers(component manifest.Component) (map[string]clientlang.Handler, string, error) {
	emits := componentEmits(component)
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" && len(emits) == 0 {
		return nil, "", nil
	}
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		payload, err := json.Marshal(clientlang.Bootstrap{Emits: emits})
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
	if len(handlers) == 0 && len(helpers) == 0 && !program.NeedsBootstrap() && len(emits) == 0 {
		return nil, "", nil
	}
	computeds, err := program.OrderedComputed()
	if err != nil {
		return nil, "", err
	}
	var payload []byte
	if program.NeedsBootstrap() || len(emits) > 0 {
		payload, err = json.Marshal(clientlang.Bootstrap{
			Handlers: handlers,
			Helpers:  helpers,
			Emits:    emits,
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

func componentEmits(component manifest.Component) map[string]clientlang.Emit {
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

func componentPropNames(component manifest.Component) ([]string, []string) {
	if component.PropsType.Name != "" {
		resolved, err := gotypes.ResolveStruct(component.Imports, component.PropsType)
		if err != nil {
			return nil, []string{fmt.Sprintf("component %s props: %v", component.Name, err)}
		}
		return resolved.FieldNames(), nil
	}
	props := make([]string, 0, len(component.Props))
	seen := map[string]bool{}
	var failures []string
	for _, prop := range component.Props {
		if prop.Type != "string" {
			failures = append(failures, fmt.Sprintf("component %s prop %s uses unsupported type %q", component.Name, prop.Name, prop.Type))
			continue
		}
		if seen[prop.Name] {
			failures = append(failures, fmt.Sprintf("component %s declares duplicate prop %q", component.Name, prop.Name))
			continue
		}
		seen[prop.Name] = true
		props = append(props, prop.Name)
	}
	return props, failures
}

func componentInitialState(component manifest.Component) (map[string]string, map[string]clientlang.ValueType, string, error) {
	if component.State.Type.Name == "" {
		return nil, nil, "", nil
	}
	resolved, err := gotypes.ResolveStruct(component.Imports, component.State.Type)
	if err != nil {
		return nil, nil, "", err
	}
	rawJSON, err := gotypes.RunStateInitJSON(component.Imports, component.State)
	if err != nil {
		return nil, nil, "", err
	}
	var raw map[string]any
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, nil, "", fmt.Errorf("decode state JSON: %w", err)
	}
	state := map[string]string{}
	stateTypes := map[string]clientlang.ValueType{}
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
	return state, stateTypes, string(rawJSON), nil
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

func buildLayouts(layouts []manifest.Layout) (map[string]manifest.Layout, []string) {
	registry := map[string]manifest.Layout{}
	var failures []string
	for _, layout := range layouts {
		if layout.ID == "" {
			failures = append(failures, "layout missing ID")
			continue
		}
		if _, exists := registry[layout.ID]; exists {
			failures = append(failures, fmt.Sprintf("duplicate layout %q", layout.ID))
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
		registry[layout.ID] = layout
	}
	return registry, failures
}

func isComponentName(value string) bool {
	if value == "" {
		return false
	}
	first := []rune(value)[0]
	return first >= 'A' && first <= 'Z'
}
