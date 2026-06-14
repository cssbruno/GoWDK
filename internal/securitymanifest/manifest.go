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
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/safeasset"
	"github.com/cssbruno/gowdk/internal/securitytext"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/view"
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
// Policy evaluation consumes these posture facts without deciding whether they
// are acceptable here.
type FrontendSurface struct {
	UnguardedRoutes   []UnguardedRoute   `json:"unguardedRoutes"`
	BundleSecrets     []BundleLeak       `json:"bundleSecrets"`
	RawHTMLSinks      []RawHTMLSink      `json:"rawHtmlSinks"`
	ConfiguredHeaders []ConfiguredHeader `json:"configuredHeaders"`
}

// UnguardedRoute records one client-visible route that relies on generated
// default-deny handling because the source declared no guard.
type UnguardedRoute struct {
	Route  string `json:"route"`
	Source string `json:"source,omitempty"`
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

// ConfiguredHeader records one header configured for generated runtime output.
type ConfiguredHeader struct {
	Name string `json:"name"`
}

// Build projects validated IR into a SecurityManifest. It reuses
// compiler.BuildRouteMetadataFromIR so the posture matches the CLI routes and
// endpoints reports exactly.
func Build(config gowdk.Config, ir gwdkir.Program) SecurityManifest {
	metadata := compiler.BuildRouteMetadataFromIR(config, ir)
	manifest := SecurityManifest{
		Version:       SchemaVersion,
		GeneratedFrom: "ir",
		Frontend:      FrontendSurface{ConfiguredHeaders: configuredHeaders(config)},
	}

	var unguarded []UnguardedRoute
	for _, route := range metadata.Routes {
		routeSource := sourceRef(route.Source, route.SourceSpan)
		entry := RouteEntry{
			PageID:      route.PageID,
			Route:       route.Route,
			Kind:        string(route.Kind),
			Method:      route.Method,
			Render:      string(route.Render),
			Guards:      append([]string(nil), route.Guards...),
			Public:      hasPublicGuard(route.Guards),
			DefaultDeny: len(route.Guards) == 0,
			Source:      routeSource,
		}
		manifest.Routes = append(manifest.Routes, entry)
		if entry.DefaultDeny {
			unguarded = append(unguarded, UnguardedRoute{Route: route.Route, Source: routeSource})
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
	manifest.Frontend.BundleSecrets = bundleLeaks(ir)
	manifest.Frontend.RawHTMLSinks = rawHTMLSinks(ir)
	if manifest.Frontend.UnguardedRoutes == nil {
		manifest.Frontend.UnguardedRoutes = []UnguardedRoute{}
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

func configuredHeaders(config gowdk.Config) []ConfiguredHeader {
	if !config.Build.SecurityHeaders.Enabled || len(config.Build.SecurityHeaders.Headers) == 0 {
		return []ConfiguredHeader{}
	}
	type candidate struct {
		key  string
		name string
	}
	candidates := make([]candidate, 0, len(config.Build.SecurityHeaders.Headers))
	for name := range config.Build.SecurityHeaders.Headers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		candidates = append(candidates, candidate{
			key:  strings.ToLower(name),
			name: http.CanonicalHeaderKey(name),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].key != candidates[j].key {
			return candidates[i].key < candidates[j].key
		}
		return candidates[i].name < candidates[j].name
	})
	seen := map[string]bool{}
	headers := make([]ConfiguredHeader, 0, len(candidates))
	for _, candidate := range candidates {
		if seen[candidate.key] {
			continue
		}
		seen[candidate.key] = true
		headers = append(headers, ConfiguredHeader{Name: candidate.name})
	}
	return headers
}

func bundleLeaks(ir gwdkir.Program) []BundleLeak {
	var leaks []BundleLeak
	for _, asset := range ir.Assets {
		switch {
		case asset.Path != "" && safeasset.UnsafeEmbeddedFile(asset.Path):
			leaks = append(leaks, BundleLeak{
				Source: sourceRef(asset.Source, asset.Span),
				Kind:   "unsafe-asset:" + filepath.Base(filepath.ToSlash(asset.Path)),
			})
		case asset.Inline != "":
			if kind, ok := securitytext.FirstSecretKind(asset.Inline); ok {
				leaks = append(leaks, BundleLeak{
					Source: sourceRef(asset.Source, asset.Span),
					Kind:   "inline-asset:" + kind,
				})
			}
		case asset.Kind == gwdkir.AssetFile && asset.Path != "":
			if contents, ok := readSourceAsset(asset); ok {
				if kind, ok := securitytext.FirstSecretKind(contents); ok {
					leaks = append(leaks, BundleLeak{
						Source: sourceRef(asset.Source, asset.Span),
						Kind:   "file-asset:" + kind,
					})
				}
			}
		}
	}
	for _, page := range ir.Pages {
		if !page.Blocks.Build {
			continue
		}
		if kind, ok := securitytext.FirstSecretKind(page.Blocks.BuildBody); ok {
			leaks = append(leaks, BundleLeak{
				Source: sourceRef(page.Source, page.Blocks.Spans.Build),
				Kind:   "build-data:" + kind,
			})
		}
	}
	return leaks
}

func readSourceAsset(asset gwdkir.Asset) (string, bool) {
	if strings.TrimSpace(asset.Path) == "" || filepath.IsAbs(asset.Path) {
		return "", false
	}
	baseDir := "."
	if strings.TrimSpace(asset.Source) != "" {
		baseDir = filepath.Dir(filepath.FromSlash(asset.Source))
	}
	payload, err := os.ReadFile(filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(asset.Path))))
	if err != nil {
		return "", false
	}
	return string(payload), true
}

func rawHTMLSinks(ir gwdkir.Program) []RawHTMLSink {
	var sinks []RawHTMLSink
	for _, template := range ir.Templates {
		nodes, err := view.Parse(template.Body)
		if err != nil {
			continue
		}
		sinks = append(sinks, rawHTMLSinksForNodes(nodes, template)...)
	}
	return sinks
}

func rawHTMLSinksForNodes(nodes []view.Node, template gwdkir.Template) []RawHTMLSink {
	var sinks []RawHTMLSink
	var walk func([]view.Node)
	walk = func(nodes []view.Node) {
		for _, node := range nodes {
			switch typed := node.(type) {
			case view.Element:
				for _, attr := range typed.Attrs {
					if attr.Name != "g:html" {
						continue
					}
					sinks = append(sinks, RawHTMLSink{
						OwnerKind: string(template.OwnerKind),
						OwnerID:   template.OwnerID,
						Field:     strings.TrimSpace(attr.Value),
						Source:    sourceRef(template.Source, templateOffsetSpan(template, attr.Start)),
					})
				}
				walk(typed.Children)
			case view.ComponentCall:
				walk(typed.Children)
			}
		}
	}
	walk(nodes)
	return sinks
}

func templateOffsetSpan(template gwdkir.Template, offset int) source.SourceSpan {
	line := template.BodyStart.Line
	if line <= 0 {
		line = template.Span.Start.Line
	}
	if line <= 0 {
		return template.Span
	}
	if offset > 0 && offset <= len(template.Body) {
		line += strings.Count(template.Body[:offset], "\n")
	}
	return source.SourceSpan{
		Start: source.SourcePosition{Line: line, Column: 1},
		End:   source.SourcePosition{Line: line, Column: 2},
	}
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
