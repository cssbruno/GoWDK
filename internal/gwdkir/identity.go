package gwdkir

import (
	"net/url"
	"strings"
)

// PageID, ComponentID, LayoutID, RouteID, and EndpointID are stable semantic
// identities used across compiler stages. Legacy string fields remain for now
// while consumers migrate toward typed stage boundaries.
type PageID string
type ComponentID string
type LayoutID string
type RouteID string
type EndpointID string

func (id PageID) String() string      { return string(id) }
func (id ComponentID) String() string { return string(id) }
func (id LayoutID) String() string    { return string(id) }
func (id RouteID) String() string     { return string(id) }
func (id EndpointID) String() string  { return string(id) }

func NewPageID(id string) PageID             { return PageID(id) }
func NewComponentID(name string) ComponentID { return ComponentID(name) }
func NewLayoutID(id string) LayoutID         { return LayoutID(id) }
func RouteIdentity(method, path, pageID string) RouteID {
	return RouteID(joinIdentityParts(method, path, pageID))
}

func EndpointIdentity(kind EndpointKind, pageID, symbol, method, path string) EndpointID {
	return EndpointID(joinIdentityParts(string(kind), pageID, symbol, method, path))
}

func (page Page) SemanticID() PageID {
	return NewPageID(page.ID)
}

func (component Component) SemanticID() ComponentID {
	return NewComponentID(component.Name)
}

func (layout Layout) SemanticID() LayoutID {
	return NewLayoutID(layout.ID)
}

func (route Route) ExpectedID() RouteID {
	return RouteIdentity(route.Method, route.Path, route.PageID)
}

func (route Route) SemanticID() RouteID {
	if route.ID != "" {
		return route.ID
	}
	return route.ExpectedID()
}

func (endpoint Endpoint) ExpectedID() EndpointID {
	return EndpointIdentity(endpoint.Kind, endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path)
}

func (endpoint Endpoint) SemanticID() EndpointID {
	if endpoint.ID != "" {
		return endpoint.ID
	}
	return endpoint.ExpectedID()
}

func joinIdentityParts(parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return strings.Join(escaped, ":")
}
