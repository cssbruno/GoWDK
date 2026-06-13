// Package securitymanifest projects the stable compiler IR into a declarative,
// machine-readable security posture (gowdk-security.json). It records what the
// generated app actually exposes — every route, backend endpoint, and contract
// with its guards, CSRF state, body limit, and source location, plus the
// frontend surface (raw-HTML sinks, secret scan, header configuration, and
// client route-guard coverage).
//
// The manifest is "what is": it never decides whether the posture is acceptable.
// Policy evaluation and findings live in internal/auditspec, which consumes this
// manifest. Keeping the projection free of policy keeps gowdk-security.json
// stable and equally auditable by a human or an LLM regardless of which policies
// are declared.
package securitymanifest

import (
	"fmt"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// SchemaVersion is the gowdk-security.json schema version.
const SchemaVersion = 1

// PublicGuardID is the guard ID that marks an intentionally public target.
const PublicGuardID = "public"

// SecurityManifest is the declarative posture of one built module.
type SecurityManifest struct {
	Version       int             `json:"version"`
	GeneratedFrom string          `json:"generatedFrom"`
	Routes        []RouteEntry    `json:"routes,omitempty"`
	Endpoints     []EndpointEntry `json:"endpoints,omitempty"`
	Contracts     []ContractEntry `json:"contracts,omitempty"`
	Frontend      FrontendSurface `json:"frontend"`
}

// RouteEntry is the posture of one page/file route.
type RouteEntry struct {
	PageID      string   `json:"pageId"`
	Route       string   `json:"route"`
	Kind        string   `json:"kind"`
	Method      string   `json:"method,omitempty"`
	Render      string   `json:"render,omitempty"`
	Guards      []string `json:"guards,omitempty"`
	Public      bool     `json:"public"`
	DefaultDeny bool     `json:"defaultDeny"`
	Source      string   `json:"source,omitempty"`
}

// EndpointEntry is the posture of one backend action/api/fragment/contract
// endpoint.
type EndpointEntry struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Method         string   `json:"method,omitempty"`
	Path           string   `json:"path,omitempty"`
	Guards         []string `json:"guards,omitempty"`
	CSRF           bool     `json:"csrf"`
	BodyLimitBytes int64    `json:"bodyLimitBytes,omitempty"`
	Public         bool     `json:"public"`
	DefaultDeny    bool     `json:"defaultDeny"`
	PageID         string   `json:"pageId,omitempty"`
	Source         string   `json:"source,omitempty"`
}

// ContractEntry is the posture of one command/query contract reference.
type ContractEntry struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Roles  []string `json:"roles,omitempty"`
	Status string   `json:"status,omitempty"`
}

// FrontendSurface describes the build-time / client-facing security surface.
// Phase 1 populates UnguardedRoutes from route posture; the remaining fields are
// enriched by the frontend audits.
type FrontendSurface struct {
	UnguardedRoutes   []string      `json:"unguardedRoutes"`
	BundleSecrets     []BundleLeak  `json:"bundleSecrets"`
	RawHTMLSinks      []RawHTMLSink `json:"rawHtmlSinks"`
	ConfiguredHeaders []string      `json:"configuredHeaders"`
}

// BundleLeak records a secret-shaped value found in embedded output or
// build-time data.
type BundleLeak struct {
	Source string `json:"source"`
	Kind   string `json:"kind"`
}

// RawHTMLSink records one raw-HTML (g:html) render site.
type RawHTMLSink struct {
	OwnerKind string `json:"ownerKind"`
	OwnerID   string `json:"ownerId"`
	Field     string `json:"field"`
	Source    string `json:"source"`
}

// Build projects validated IR into a SecurityManifest. It reuses
// compiler.BuildRouteMetadataFromIR so the posture matches the CLI routes and
// endpoints reports exactly.
func Build(config gowdk.Config, ir gwdkir.Program) SecurityManifest {
	metadata := compiler.BuildRouteMetadataFromIR(config, ir)
	manifest := SecurityManifest{
		Version:       SchemaVersion,
		GeneratedFrom: "ir",
		Frontend:      FrontendSurface{ConfiguredHeaders: []string{}},
	}

	var unguarded []string
	for _, route := range metadata.Routes {
		entry := RouteEntry{
			PageID:      route.PageID,
			Route:       route.Route,
			Kind:        string(route.Kind),
			Method:      route.Method,
			Render:      string(route.Render),
			Guards:      append([]string(nil), route.Guards...),
			Public:      hasPublicGuard(route.Guards),
			DefaultDeny: len(route.Guards) == 0,
			Source:      sourceRef(route.Source, route.SourceSpan),
		}
		manifest.Routes = append(manifest.Routes, entry)
		if entry.DefaultDeny {
			unguarded = append(unguarded, route.Route)
		}
	}

	for _, endpoint := range metadata.Endpoints {
		manifest.Endpoints = append(manifest.Endpoints, EndpointEntry{
			ID:             endpointID(endpoint),
			Kind:           string(endpoint.Kind),
			Method:         endpoint.Method,
			Path:           endpoint.Route,
			Guards:         append([]string(nil), endpoint.Guards...),
			CSRF:           endpoint.CSRF,
			BodyLimitBytes: bodyLimitFor(config, endpoint.Kind),
			Public:         hasPublicGuard(endpoint.Guards),
			DefaultDeny:    len(endpoint.Guards) == 0,
			PageID:         endpoint.PageID,
			Source:         sourceRef(endpoint.Source, endpoint.SourceSpan),
		})
		if contract := endpoint.Contract; contract.Name != "" {
			manifest.Contracts = append(manifest.Contracts, ContractEntry{
				Name:   contract.Name,
				Kind:   string(contract.Kind),
				Roles:  append([]string(nil), contract.Roles...),
				Status: string(contract.Status),
			})
		}
	}

	manifest.Frontend.UnguardedRoutes = unguarded
	if manifest.Frontend.UnguardedRoutes == nil {
		manifest.Frontend.UnguardedRoutes = []string{}
	}
	if manifest.Frontend.BundleSecrets == nil {
		manifest.Frontend.BundleSecrets = []BundleLeak{}
	}
	if manifest.Frontend.RawHTMLSinks == nil {
		manifest.Frontend.RawHTMLSinks = []RawHTMLSink{}
	}
	return manifest
}

func hasPublicGuard(guards []string) bool {
	for _, guard := range guards {
		if guard == PublicGuardID {
			return true
		}
	}
	return false
}

func endpointID(endpoint compiler.EndpointBinding) string {
	for _, candidate := range []string{endpoint.Symbol, endpoint.Contract.Name, endpoint.Handler, endpoint.PageID} {
		if candidate != "" {
			return candidate
		}
	}
	return endpoint.Route
}

func bodyLimitFor(config gowdk.Config, kind compiler.EndpointKind) int64 {
	switch kind {
	case compiler.EndpointAPI, compiler.EndpointQuery:
		return config.Build.BodyLimits.APILimitBytes()
	default:
		return config.Build.BodyLimits.ActionLimitBytes()
	}
}

func sourceRef(file string, span source.SourceSpan) string {
	if file == "" {
		return ""
	}
	if span.Start.Line > 0 {
		return fmt.Sprintf("%s:%d", file, span.Start.Line)
	}
	return file
}
