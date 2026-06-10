package compiler

import (
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/view"
	"sort"
	"strings"
)

func validateRedundantComponents(components []gwdkir.Component) []ValidationError {
	seenNames := map[string]bool{}
	seen := map[string]gwdkir.Component{}
	var diagnostics []ValidationError
	for _, component := range components {
		if component.Name == "" || seenNames[component.Name] {
			continue
		}
		seenNames[component.Name] = true
		fingerprint := componentFingerprint(component)
		if fingerprint == "" {
			continue
		}
		first, exists := seen[fingerprint]
		if !exists {
			seen[fingerprint] = component
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          "redundant_component_implementation",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Span, component.Blocks.Spans.View),
			Message: fmt.Sprintf(
				"component %q duplicates implementation of component %q; first declared in %s and duplicated in %s",
				component.Name,
				first.Name,
				first.Source,
				component.Source,
			),
		})
	}
	return diagnostics
}

func componentFingerprint(component gwdkir.Component) string {
	parts := []string{
		"props=" + componentPropsFingerprint(component),
		"state=" + componentStateFingerprint(component),
		"client=" + componentClientFingerprint(component),
		"view=" + componentViewFingerprint(component),
	}
	return strings.Join(parts, "\n")
}

func componentPropsFingerprint(component gwdkir.Component) string {
	if component.PropsType.Name != "" {
		return "type:" + canonicalGoType(component.Imports, component.PropsType)
	}
	if len(component.Props) == 0 {
		return "inline:"
	}
	props := make([]string, 0, len(component.Props))
	for _, prop := range component.Props {
		props = append(props, prop.Name+":"+prop.Type)
	}
	sort.Strings(props)
	return "inline:" + strings.Join(props, ",")
}

func componentStateFingerprint(component gwdkir.Component) string {
	if component.State.Type.Name == "" {
		return ""
	}
	return canonicalGoType(component.Imports, component.State.Type) + "=init:" + canonicalGoFunc(component.Imports, component.State.Init)
}

func componentViewFingerprint(component gwdkir.Component) string {
	canonical, err := view.Canonical(component.Blocks.ViewBody)
	if err == nil {
		return canonical
	}
	return strings.Join(strings.Fields(component.Blocks.ViewBody), " ")
}

func componentClientFingerprint(component gwdkir.Component) string {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return ""
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err == nil {
		return program.Canonical()
	}
	return strings.Join(strings.Fields(component.Blocks.ClientBody), " ")
}

func canonicalGoType(imports []gwdkir.Import, ref gwdkir.GoRef) string {
	path, err := gotypes.ImportPathForAlias(importsFromIR(imports), ref.Alias)
	if err != nil {
		return ref.Alias + "." + ref.Name
	}
	return path + "." + ref.Name
}

func canonicalGoFunc(imports []gwdkir.Import, ref gwdkir.GoRef) string {
	path, err := gotypes.ImportPathForAlias(importsFromIR(imports), ref.Alias)
	if err != nil {
		return ref.Alias + "." + ref.Name
	}
	return path + "." + ref.Name
}
