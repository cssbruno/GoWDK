package gwdkir

import (
	"fmt"
	"strings"
)

// CheckInvariants reports structural defects in a Program that indicate a
// compiler bug rather than an authoring error. The checks only assert what IR
// construction guarantees regardless of source validity: deterministic
// ordering, closed enum values, and cross-slice references that the builder
// creates together (routes to pages, templates and assets to their owners).
// Authoring problems such as duplicate page IDs or conflicting routes are
// user-facing diagnostics owned by the compiler validators, not invariants.
func CheckInvariants(program Program) error {
	var violations []string
	report := func(format string, args ...any) {
		violations = append(violations, fmt.Sprintf(format, args...))
	}

	pages := make(map[string]bool, len(program.Pages))
	for _, page := range program.Pages {
		pages[page.ID] = true
	}
	components := make(map[string]bool, len(program.Components))
	for _, component := range program.Components {
		components[component.Name] = true
	}
	layouts := make(map[string]bool, len(program.Layouts))
	for _, layout := range program.Layouts {
		layouts[layout.ID] = true
	}

	for index, pkg := range program.Packages {
		if index == 0 {
			continue
		}
		previous := program.Packages[index-1].Name
		if pkg.Name == previous {
			report("duplicate package %q", pkg.Name)
		} else if pkg.Name < previous {
			report("packages are not sorted: %q after %q", pkg.Name, previous)
		}
	}

	for index, route := range program.Routes {
		switch route.Kind {
		case RouteStatic, RouteSPA, RouteSSR, RouteHybrid:
		default:
			report("route %q has unknown kind %q", route.Path, route.Kind)
		}
		if !pages[route.PageID] {
			report("route %q references unknown page %q", route.Path, route.PageID)
		}
		if index > 0 && route.Path < program.Routes[index-1].Path {
			report("routes are not sorted: %q after %q", route.Path, program.Routes[index-1].Path)
		}
	}

	for index, endpoint := range program.Endpoints {
		switch endpoint.Kind {
		case EndpointAction, EndpointAPI, EndpointFragment:
		default:
			report("endpoint %q has unknown kind %q", endpoint.Path, endpoint.Kind)
		}
		switch endpoint.Source {
		case EndpointSourceGOWDK, EndpointSourceGo:
		default:
			report("endpoint %q has unknown source %q", endpoint.Path, endpoint.Source)
		}
		if endpoint.Method == "" {
			report("endpoint %q has no method", endpoint.Path)
		}
		if index > 0 {
			previous := program.Endpoints[index-1]
			if endpoint.Path < previous.Path || (endpoint.Path == previous.Path && endpoint.Method < previous.Method) {
				report("endpoints are not sorted: %s %q after %s %q", endpoint.Method, endpoint.Path, previous.Method, previous.Path)
			}
		}
	}

	for _, endpoint := range program.GoEndpoints {
		switch endpoint.SourceKind {
		case EndpointSourceGOWDK, EndpointSourceGo:
		default:
			report("go endpoint %q has unknown source %q", endpoint.Name, endpoint.SourceKind)
		}
	}

	for _, template := range program.Templates {
		reportOwnerReference("template", template.OwnerKind, template.OwnerID, pages, components, layouts, report)
	}

	for _, asset := range program.Assets {
		switch asset.Kind {
		case AssetCSS, AssetJS, AssetFile, AssetWASM:
		default:
			report("asset %q has unknown kind %q", asset.Path, asset.Kind)
		}
		if !pages[asset.OwnerID] && !components[asset.OwnerID] {
			report("asset %q references unknown owner %q", asset.Path, asset.OwnerID)
		}
	}

	for _, behavior := range program.ClientBehaviors {
		if !components[behavior.Component] {
			report("client behavior references unknown component %q", behavior.Component)
		}
	}

	for _, ref := range program.ContractRefs {
		switch ref.Kind {
		case ContractCommand, ContractQuery:
		default:
			report("contract reference %q has unknown kind %q", ref.Name, ref.Kind)
		}
		switch ref.Status {
		case "", ContractBindingUnknown, ContractBindingBound, ContractBindingMissing, ContractBindingInvalid:
		default:
			report("contract reference %q has unknown binding status %q", ref.Name, ref.Status)
		}
		reportOwnerReference("contract reference", ref.OwnerKind, ref.OwnerID, pages, components, layouts, report)
	}

	if len(violations) == 0 {
		return nil
	}
	return fmt.Errorf("invalid IR: %s", strings.Join(violations, "; "))
}

func reportOwnerReference(what string, kind SourceKind, ownerID string, pages, components, layouts map[string]bool, report func(string, ...any)) {
	switch kind {
	case SourcePage:
		if !pages[ownerID] {
			report("%s references unknown page %q", what, ownerID)
		}
	case SourceComponent:
		if !components[ownerID] {
			report("%s references unknown component %q", what, ownerID)
		}
	case SourceLayout:
		if !layouts[ownerID] {
			report("%s references unknown layout %q", what, ownerID)
		}
	default:
		report("%s has unknown owner kind %q", what, kind)
	}
}
