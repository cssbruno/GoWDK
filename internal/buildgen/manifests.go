package buildgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/compiler"
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
)

type routeManifest struct {
	Version   int                          `json:"version"`
	Routes    []routeManifestEntry         `json:"routes"`
	Endpoints []routeManifestEndpointEntry `json:"endpoints,omitempty"`
}

type routeManifestEntry struct {
	PageID string `json:"page"`
	Route  string `json:"route"`
	Path   string `json:"path"`
}

type routeManifestEndpointEntry struct {
	Kind      compiler.EndpointKind `json:"kind"`
	Directive string                `json:"directive,omitempty"`
	Method    string                `json:"method"`
	Route     string                `json:"route"`
	PageID    string                `json:"page"`
	Symbol    string                `json:"symbol,omitempty"`
	Handler   string                `json:"handler"`
	Guards    []string              `json:"guards,omitempty"`
	CSRF      bool                  `json:"csrf,omitempty"`
}

func writeRouteManifest(outputDir string, artifacts []Artifact, endpoints []compiler.EndpointBinding) (string, error) {
	payload, err := routeManifestPayload(outputDir, artifacts, endpoints)
	if err != nil {
		return "", err
	}

	manifestPath := filepath.Join(outputDir, routeManifestFile)
	if err := writeFileIfChanged(manifestPath, payload); err != nil {
		return "", err
	}
	return manifestPath, nil
}

func routeManifestPayload(outputDir string, artifacts []Artifact, endpoints []compiler.EndpointBinding) ([]byte, error) {
	routes := make([]routeManifestEntry, 0, len(artifacts))
	for _, artifact := range artifacts {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return nil, err
		}
		routes = append(routes, routeManifestEntry{
			PageID: artifact.PageID,
			Route:  artifact.Route,
			Path:   rel,
		})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Route == routes[j].Route {
			return routes[i].PageID < routes[j].PageID
		}
		return routes[i].Route < routes[j].Route
	})

	endpointRoutes := routeManifestEndpointEntries(endpoints)
	payload, err := json.MarshalIndent(routeManifest{Version: 1, Routes: routes, Endpoints: endpointRoutes}, "", "  ")
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')
	return payload, nil
}

func routeManifestEndpointEntries(endpoints []compiler.EndpointBinding) []routeManifestEndpointEntry {
	if len(endpoints) == 0 {
		return nil
	}
	routes := make([]routeManifestEndpointEntry, 0, len(endpoints))
	for _, endpoint := range endpoints {
		routes = append(routes, routeManifestEndpointEntry{
			Kind:      endpoint.Kind,
			Directive: routeManifestEndpointDirective(endpoint.Kind),
			Method:    endpoint.Method,
			Route:     endpoint.Route,
			PageID:    endpoint.PageID,
			Symbol:    endpoint.Symbol,
			Handler:   endpoint.Handler,
			Guards:    append([]string(nil), endpoint.Guards...),
			CSRF:      endpoint.CSRF,
		})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Route == routes[j].Route {
			if routes[i].Method == routes[j].Method {
				return routes[i].Kind < routes[j].Kind
			}
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Route < routes[j].Route
	})
	return routes
}

func routeManifestEndpointDirective(kind compiler.EndpointKind) string {
	switch kind {
	case compiler.EndpointAction:
		return "act"
	case compiler.EndpointAPI:
		return "api"
	case compiler.EndpointFragment:
		return "fragment"
	case compiler.EndpointCommand:
		return "g:command"
	case compiler.EndpointQuery:
		return "g:query"
	default:
		return ""
	}
}

func readRouteManifestIfExists(outputDir string) (routeManifest, error) {
	manifestPath := filepath.Join(outputDir, routeManifestFile)
	payload, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		return routeManifest{}, nil
	}
	if err != nil {
		return routeManifest{}, err
	}
	var manifest routeManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return routeManifest{}, fmt.Errorf("read existing route manifest: %w", err)
	}
	return manifest, nil
}

func removeStaleChangedPageArtifacts(outputDir string, previous routeManifest, current []Artifact, changedPageIDs map[string]bool) error {
	if len(previous.Routes) == 0 || len(changedPageIDs) == 0 {
		return nil
	}
	keep := map[string]bool{}
	for _, artifact := range current {
		if !changedPageIDs[artifact.PageID] {
			continue
		}
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return err
		}
		keep[rel] = true
	}
	for _, route := range previous.Routes {
		if !changedPageIDs[route.PageID] || keep[route.Path] {
			continue
		}
		filePath, err := outputFilePath(outputDir, route.Path)
		if err != nil {
			return err
		}
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func outputFilePath(outputDir, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("route manifest path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("route manifest path %q must be relative", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("route manifest path %q must stay inside output directory", rel)
	}
	return filepath.Join(outputDir, clean), nil
}

func writeAssetManifest(outputDir string, pageArtifacts []Artifact, cssArtifacts []CSSArtifact, assetArtifacts []AssetArtifact) (string, error) {
	payload, err := assetManifestPayload(outputDir, pageArtifacts, cssArtifacts, assetArtifacts)
	if err != nil {
		return "", err
	}

	manifestPath := filepath.Join(outputDir, assetManifestFile)
	if err := writeFileIfChanged(manifestPath, payload); err != nil {
		return "", err
	}
	return manifestPath, nil
}

func assetManifestPayload(outputDir string, pageArtifacts []Artifact, cssArtifacts []CSSArtifact, assetArtifacts []AssetArtifact) ([]byte, error) {
	files := make(map[string]string, len(cssArtifacts)+len(assetArtifacts))
	hashes := make(map[string]string, len(cssArtifacts)+len(assetArtifacts))
	cache := make(map[string]string, len(pageArtifacts)+len(cssArtifacts)+len(assetArtifacts))
	sizes := make(map[string]int64, len(cssArtifacts)+len(assetArtifacts))
	obfuscated := make(map[string]bool, len(assetArtifacts))
	for _, artifact := range pageArtifacts {
		if artifact.CachePolicy == "" {
			continue
		}
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return nil, err
		}
		cache[rel] = artifact.CachePolicy
	}
	for _, artifact := range cssArtifacts {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return nil, err
		}
		logical := artifactLogicalPath(artifact.LogicalPath, rel)
		files[logical] = rel
		if artifact.Hash != "" {
			hashes[logical] = artifact.Hash
		}
		if artifact.CachePolicy != "" {
			cache[logical] = artifact.CachePolicy
			cache[rel] = artifact.CachePolicy
		}
		if artifact.SizeBytes > 0 {
			sizes[logical] = artifact.SizeBytes
			sizes[rel] = artifact.SizeBytes
		}
	}
	for _, artifact := range assetArtifacts {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return nil, err
		}
		logical := artifactLogicalPath(artifact.LogicalPath, rel)
		files[logical] = rel
		if artifact.Hash != "" {
			hashes[logical] = artifact.Hash
		}
		if artifact.CachePolicy != "" {
			cache[logical] = artifact.CachePolicy
			cache[rel] = artifact.CachePolicy
		}
		if artifact.SizeBytes > 0 {
			sizes[logical] = artifact.SizeBytes
			sizes[rel] = artifact.SizeBytes
		}
		if artifact.Obfuscated {
			obfuscated[logical] = true
			obfuscated[rel] = true
		}
	}

	manifest := runtimeasset.Manifest{Version: runtimeasset.ManifestVersion, Files: files}
	if len(hashes) > 0 {
		manifest.Hashes = hashes
	}
	if len(cache) > 0 {
		manifest.Cache = cache
	}
	if len(sizes) > 0 {
		manifest.Sizes = sizes
	}
	if len(obfuscated) > 0 {
		manifest.Obfuscated = obfuscated
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')
	return payload, nil
}

func artifactLogicalPath(logicalPath string, fallback string) string {
	logical := strings.TrimLeft(filepath.ToSlash(strings.TrimSpace(logicalPath)), "/")
	if logical == "" {
		return fallback
	}
	return logical
}

func writeFileIfChanged(filePath string, contents []byte) error {
	_, err := writeFileIfChangedStatus(filePath, contents)
	return err
}

func writeFileIfChangedStatus(filePath string, contents []byte) (bool, error) {
	current, err := os.ReadFile(filePath)
	if err == nil && bytes.Equal(current, contents) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(filePath, contents, 0o644)
}

func contentHash(contents []byte) string {
	sum := sha256.Sum256(contents)
	return fmt.Sprintf("sha256:%x", sum[:])
}

func relativeOutputPath(outputDir, filePath string) (string, error) {
	rel, err := filepath.Rel(outputDir, filePath)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact path %q must stay inside output directory", filePath)
	}
	return filepath.ToSlash(rel), nil
}
