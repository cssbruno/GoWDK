package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/viewanalysis"
)

func validateRedundantComponents(components []gwdkir.Component) []ValidationError {
	seenIdentities := map[string]bool{}
	seen := map[string]gwdkir.Component{}
	var diagnostics []ValidationError
	for _, component := range components {
		identity := componentIdentityKey(component.Package, component.Name)
		if component.Name == "" || seenIdentities[identity] {
			continue
		}
		seenIdentities[identity] = true
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
		defaultValue := ""
		if prop.DefaultSet {
			defaultValue = "=" + prop.Default
		}
		props = append(props, prop.Name+":"+prop.Type+defaultValue)
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
	return viewanalysis.CanonicalNodes(component.Blocks.ViewNodes)
}

func componentClientFingerprint(component gwdkir.Component) string {
	if !component.Blocks.Client || component.Blocks.ClientProgram == nil {
		return ""
	}
	return component.Blocks.ClientProgram.Canonical()
}

func canonicalGoType(imports []gwdkir.Import, ref gwdkir.GoRef) string {
	path, err := gotypes.ImportPathForAlias(imports, ref.Alias)
	if err != nil {
		return ref.Alias + "." + ref.Name
	}
	return path + "." + ref.Name
}

func canonicalGoFunc(imports []gwdkir.Import, ref gwdkir.GoRef) string {
	path, err := gotypes.ImportPathForAlias(imports, ref.Alias)
	if err != nil {
		return ref.Alias + "." + ref.Name
	}
	return path + "." + ref.Name
}
