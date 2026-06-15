package gwdkir

import (
	"strings"
	"testing"
)

func validProgram() Program {
	return Program{
		Packages:   []Package{{Name: "main"}, {Name: "shop"}},
		Pages:      []Page{{ID: "home", Package: "main", Route: "/"}},
		Components: []Component{{Name: "ProductCard", Package: "shop"}},
		Layouts:    []Layout{{ID: "base", Package: "main"}},
		Routes: []Route{
			{Kind: RouteSPA, Method: "GET", Path: "/", PageID: "home"},
		},
		Endpoints: []Endpoint{
			{Kind: EndpointAction, Source: EndpointSourceGOWDK, PageID: "home", Method: "POST", Path: "/actions/save"},
			{Kind: EndpointAPI, Source: EndpointSourceGo, Method: "GET", Path: "/api/items"},
		},
		GoEndpoints: []GoEndpoint{
			{Kind: "api", SourceKind: EndpointSourceGo, Name: "ListItems", Route: "/api/items"},
		},
		Templates: []Template{
			{OwnerKind: SourcePage, OwnerID: "home"},
			{OwnerKind: SourceComponent, OwnerID: "ProductCard"},
			{OwnerKind: SourceLayout, OwnerID: "base"},
		},
		ContractRefs: []ContractReference{
			{Kind: ContractQuery, Name: "shop.ListProducts", OwnerKind: SourcePage, OwnerID: "home"},
			{Kind: ContractCommand, Name: "shop.AddToCart", Status: ContractBindingBound, OwnerKind: SourceComponent, OwnerID: "ProductCard"},
		},
		RealtimeSubscriptions: []RealtimeSubscription{
			{Query: "shop.ListProducts", Event: "shop.ProductNotice", Status: ContractBindingBound, OwnerKind: SourcePage, OwnerID: "home"},
		},
		QueryInvalidations: []QueryInvalidation{
			{Query: "shop.ListProducts", Event: "shop.ProductCreated", Status: ContractBindingBound, OwnerKind: SourcePage, OwnerID: "home"},
		},
		ClientBehaviors: []ClientBehavior{{Component: "ProductCard"}},
		Assets: []Asset{
			{Kind: AssetCSS, OwnerID: "home", Path: "home.css"},
			{Kind: AssetWASM, OwnerID: "ProductCard", Path: "cart"},
		},
	}
}

func TestCheckInvariantsAcceptsValidProgram(t *testing.T) {
	if err := CheckInvariants(validProgram()); err != nil {
		t.Fatalf("CheckInvariants() = %v, want nil", err)
	}
}

func TestCheckInvariantsAcceptsEmptyProgram(t *testing.T) {
	if err := CheckInvariants(Program{}); err != nil {
		t.Fatalf("CheckInvariants() = %v, want nil", err)
	}
}

func TestCheckInvariantsReportsViolations(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*Program)
		want    string
	}{
		{
			name:    "duplicate package",
			corrupt: func(p *Program) { p.Packages = []Package{{Name: "main"}, {Name: "main"}} },
			want:    `duplicate package "main"`,
		},
		{
			name:    "unsorted packages",
			corrupt: func(p *Program) { p.Packages = []Package{{Name: "shop"}, {Name: "main"}} },
			want:    "packages are not sorted",
		},
		{
			name:    "unknown route kind",
			corrupt: func(p *Program) { p.Routes[0].Kind = "island" },
			want:    `unknown kind "island"`,
		},
		{
			name:    "route to missing page",
			corrupt: func(p *Program) { p.Routes[0].PageID = "ghost" },
			want:    `route "/" references unknown page "ghost"`,
		},
		{
			name: "unsorted routes",
			corrupt: func(p *Program) {
				p.Pages = append(p.Pages, Page{ID: "about", Route: "/about"})
				p.Routes = []Route{
					{Kind: RouteSPA, Path: "/about", PageID: "about"},
					{Kind: RouteSPA, Path: "/", PageID: "home"},
				}
			},
			want: "routes are not sorted",
		},
		{
			name:    "unknown endpoint kind",
			corrupt: func(p *Program) { p.Endpoints[0].Kind = "stream" },
			want:    `unknown kind "stream"`,
		},
		{
			name:    "unknown endpoint source",
			corrupt: func(p *Program) { p.Endpoints[0].Source = "yaml" },
			want:    `unknown source "yaml"`,
		},
		{
			name:    "endpoint without method",
			corrupt: func(p *Program) { p.Endpoints[0].Method = "" },
			want:    "has no method",
		},
		{
			name: "unsorted endpoints",
			corrupt: func(p *Program) {
				p.Endpoints[0], p.Endpoints[1] = p.Endpoints[1], p.Endpoints[0]
			},
			want: "endpoints are not sorted",
		},
		{
			name:    "unknown go endpoint source",
			corrupt: func(p *Program) { p.GoEndpoints[0].SourceKind = "yaml" },
			want:    `go endpoint "ListItems" has unknown source "yaml"`,
		},
		{
			name:    "template with unknown owner kind",
			corrupt: func(p *Program) { p.Templates[0].OwnerKind = "partial" },
			want:    `template has unknown owner kind "partial"`,
		},
		{
			name:    "template to missing page",
			corrupt: func(p *Program) { p.Templates[0].OwnerID = "ghost" },
			want:    `template references unknown page "ghost"`,
		},
		{
			name:    "template to missing component",
			corrupt: func(p *Program) { p.Templates[1].OwnerID = "Ghost" },
			want:    `template references unknown component "Ghost"`,
		},
		{
			name:    "template to missing layout",
			corrupt: func(p *Program) { p.Templates[2].OwnerID = "ghost" },
			want:    `template references unknown layout "ghost"`,
		},
		{
			name:    "unknown asset kind",
			corrupt: func(p *Program) { p.Assets[0].Kind = "font" },
			want:    `asset "home.css" has unknown kind "font"`,
		},
		{
			name:    "asset to missing owner",
			corrupt: func(p *Program) { p.Assets[0].OwnerID = "ghost" },
			want:    `asset "home.css" references unknown owner "ghost"`,
		},
		{
			name:    "client behavior to missing component",
			corrupt: func(p *Program) { p.ClientBehaviors[0].Component = "Ghost" },
			want:    `client behavior references unknown component "Ghost"`,
		},
		{
			name:    "unknown contract reference kind",
			corrupt: func(p *Program) { p.ContractRefs[0].Kind = "event" },
			want:    `contract reference "shop.ListProducts" has unknown kind "event"`,
		},
		{
			name:    "unknown contract binding status",
			corrupt: func(p *Program) { p.ContractRefs[0].Status = "maybe" },
			want:    `unknown binding status "maybe"`,
		},
		{
			name:    "contract reference to missing owner",
			corrupt: func(p *Program) { p.ContractRefs[0].OwnerID = "ghost" },
			want:    `contract reference references unknown page "ghost"`,
		},
		{
			name:    "unknown realtime subscription status",
			corrupt: func(p *Program) { p.RealtimeSubscriptions[0].Status = "maybe" },
			want:    `realtime subscription "shop.ProductNotice" has unknown binding status "maybe"`,
		},
		{
			name:    "realtime subscription without query",
			corrupt: func(p *Program) { p.RealtimeSubscriptions[0].Query = "" },
			want:    `realtime subscription "shop.ProductNotice" has no query boundary`,
		},
		{
			name:    "realtime subscription without event",
			corrupt: func(p *Program) { p.RealtimeSubscriptions[0].Event = "" },
			want:    `realtime subscription has no event reference`,
		},
		{
			name:    "realtime subscription to missing owner",
			corrupt: func(p *Program) { p.RealtimeSubscriptions[0].OwnerID = "ghost" },
			want:    `realtime subscription references unknown page "ghost"`,
		},
		{
			name:    "unknown query invalidation status",
			corrupt: func(p *Program) { p.QueryInvalidations[0].Status = "maybe" },
			want:    `query invalidation "shop.ListProducts" has unknown binding status "maybe"`,
		},
		{
			name:    "query invalidation without query",
			corrupt: func(p *Program) { p.QueryInvalidations[0].Query = "" },
			want:    `query invalidation for event "shop.ProductCreated" has no query boundary`,
		},
		{
			name:    "query invalidation without event",
			corrupt: func(p *Program) { p.QueryInvalidations[0].Event = "" },
			want:    `query invalidation has no event reference`,
		},
		{
			name:    "query invalidation to missing owner",
			corrupt: func(p *Program) { p.QueryInvalidations[0].OwnerID = "ghost" },
			want:    `query invalidation references unknown page "ghost"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program := validProgram()
			test.corrupt(&program)
			err := CheckInvariants(program)
			if err == nil {
				t.Fatalf("CheckInvariants() = nil, want error containing %q", test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("CheckInvariants() = %q, want error containing %q", err, test.want)
			}
		})
	}
}

func TestCheckInvariantsJoinsAllViolations(t *testing.T) {
	program := validProgram()
	program.Routes[0].PageID = "ghost"
	program.Assets[0].OwnerID = "ghost"
	err := CheckInvariants(program)
	if err == nil {
		t.Fatal("CheckInvariants() = nil, want error")
	}
	for _, want := range []string{`route "/" references unknown page "ghost"`, `asset "home.css" references unknown owner "ghost"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("CheckInvariants() = %q, want error containing %q", err, want)
		}
	}
}
