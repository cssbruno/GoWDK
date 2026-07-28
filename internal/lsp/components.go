package lsp

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/discover"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/source"
)

type componentDefinition struct {
	URI     string
	Text    string
	Package string
	Name    string
	Span    source.SourceSpan
}

func (server *Server) resolveComponentDefinition(doc document, name string) (componentDefinition, bool) {
	ownerPackage, ownerUses := server.ownerPackageAndUses(doc)
	definitions := server.componentDefinitions(doc)
	if alias, componentName, ok := strings.Cut(name, "."); ok {
		packageName, ok := ownerUses[alias]
		if !ok {
			return componentDefinition{}, false
		}
		definition, ok := definitions[componentDefinitionKey(packageName, componentName)]
		return definition, ok
	}
	if ownerPackage != "" {
		if definition, ok := definitions[componentDefinitionKey(ownerPackage, name)]; ok {
			return definition, true
		}
	}
	definition, ok := definitions[componentDefinitionKey("", name)]
	return definition, ok
}

func (server *Server) ownerPackageAndUses(doc document) (string, map[string]string) {
	topLevel := lang.ParseTopLevel(doc.Text)
	packageName := ""
	if topLevel.Package != nil {
		packageName = topLevel.Package.Name
	}
	packages := map[string]string{}
	for _, use := range topLevel.Uses {
		if _, exists := packages[use.Alias]; !exists {
			packages[use.Alias] = use.Package
		}
	}
	return packageName, packages
}

func (server *Server) componentDefinitions(doc document) map[string]componentDefinition {
	definitions := map[string]componentDefinition{}
	selection, ok := server.componentSelection(doc)
	if !ok {
		return definitions
	}
	for key, definition := range server.workspaceComponentDefinitions(selection) {
		definitions[key] = definition
	}
	for key, definition := range server.openComponentDefinitions(selection) {
		definitions[key] = definition
	}
	return definitions
}

func (server *Server) componentSelection(doc document) (discover.Selection, bool) {
	root := server.workspaceRootForPath(doc.Path)
	if root == "" {
		return discover.Selection{}, false
	}
	selection, err := discover.ConfiguredSelection(
		server.config,
		server.config.Build.Output,
		server.moduleNames,
		root,
	)
	if err != nil {
		server.logf("component discovery: %v", err)
		return discover.Selection{}, false
	}
	return selection, true
}

func (server *Server) openComponentDefinitions(selection discover.Selection) map[string]componentDefinition {
	definitions := map[string]componentDefinition{}
	docs := make([]document, 0, len(server.documents))
	for _, doc := range server.documents {
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Path < docs[j].Path
	})
	for _, doc := range docs {
		if !selection.Matches(doc.Path) {
			continue
		}
		payload := []byte(doc.Text)
		if lang.ClassifySource(doc.Path, payload) != lang.FileKindComponent {
			continue
		}
		component, diagnostics := lang.ParseComponentSource(doc.Path, payload)
		if diagnostics.HasErrors() {
			continue
		}
		if component.Name == "" {
			continue
		}
		definition := componentDefinition{
			URI:     doc.URI,
			Text:    doc.Text,
			Package: component.Package,
			Name:    component.Name,
			Span:    component.Span,
		}
		definitions[componentDefinitionKey(component.Package, component.Name)] = definition
		if component.Package == "" {
			definitions[componentDefinitionKey("", component.Name)] = definition
		}
	}
	return definitions
}

func (server *Server) workspaceComponentDefinitions(selection discover.Selection) map[string]componentDefinition {
	definitions := map[string]componentDefinition{}
	fingerprint := selection.Fingerprint()
	if server.workspaceComponentCache.root == selection.Root &&
		server.workspaceComponentCache.selectionFingerprint == fingerprint &&
		server.workspaceComponentCache.key != "" {
		key := workspaceComponentCacheKey(server.workspaceComponentCache.files, server.workspaceComponentCache.dirs)
		if key == server.workspaceComponentCache.key {
			return cloneComponentDefinitions(server.workspaceComponentCache.definitions)
		}
	}
	definitions, key, files, dirs, err := server.loadWorkspaceComponentDefinitions(selection)
	if err != nil {
		server.logf("component discovery: %v", err)
		return definitions
	}
	server.workspaceComponentCache = workspaceComponentDefinitionCache{
		root:                 selection.Root,
		selectionFingerprint: fingerprint,
		key:                  key,
		files:                files,
		dirs:                 dirs,
		definitions:          cloneComponentDefinitions(definitions),
	}
	return definitions
}

func (server *Server) loadWorkspaceComponentDefinitions(selection discover.Selection) (map[string]componentDefinition, string, []string, []string, error) {
	definitions := map[string]componentDefinition{}
	paths, dirs, err := selection.FilesAndDirs()
	if err != nil {
		return definitions, "", nil, nil, err
	}
	payloads := map[string]string{}
	for _, filePath := range paths {
		if _, open := server.openDocumentByPath(filePath); open {
			continue
		}
		payload, ok := readWorkspaceComponentPayload(filePath)
		if !ok {
			continue
		}
		if lang.ClassifySource(filePath, payload) != lang.FileKindComponent {
			continue
		}
		payloads[filePath] = string(payload)
	}
	key := workspaceComponentCacheKey(paths, dirs)
	if len(paths) == 0 {
		return definitions, key, paths, dirs, nil
	}
	for _, path := range paths {
		payload, ok := payloads[path]
		if !ok {
			continue
		}
		component, diagnostics := lang.ParseComponentSource(path, []byte(payload))
		if diagnostics.HasErrors() || component.Name == "" {
			continue
		}
		definition := componentDefinition{
			URI:     fileURI(component.Source),
			Text:    payload,
			Package: component.Package,
			Name:    component.Name,
			Span:    component.Span,
		}
		definitions[componentDefinitionKey(component.Package, component.Name)] = definition
		if component.Package == "" {
			definitions[componentDefinitionKey("", component.Name)] = definition
		}
	}
	return definitions, key, paths, dirs, nil
}

func readWorkspaceComponentPayload(filePath string) ([]byte, bool) {
	payload, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}
	return payload, true
}

func workspaceComponentCacheKey(files, dirs []string) string {
	var parts []string
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			parts = append(parts, "file:"+file+":missing")
			continue
		}
		parts = append(parts, fmt.Sprintf("file:%s:%d:%d", file, info.ModTime().UnixNano(), info.Size()))
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			parts = append(parts, "dir:"+dir+":missing")
			continue
		}
		parts = append(parts, fmt.Sprintf("dir:%s:%d:%d", dir, info.ModTime().UnixNano(), info.Size()))
	}
	if len(parts) == 0 {
		return "empty"
	}
	sort.Strings(parts)
	return strings.Join(parts, "\x01")
}

func cloneComponentDefinitions(definitions map[string]componentDefinition) map[string]componentDefinition {
	if len(definitions) == 0 {
		return map[string]componentDefinition{}
	}
	clone := make(map[string]componentDefinition, len(definitions))
	for key, definition := range definitions {
		clone[key] = definition
	}
	return clone
}

func (server *Server) openDocumentByPath(filePath string) (document, bool) {
	cleanPath := filepath.Clean(filePath)
	for _, doc := range server.documents {
		if filepath.Clean(doc.Path) == cleanPath {
			return doc, true
		}
	}
	return document{}, false
}

func (server *Server) workspaceRootForPath(filePath string) string {
	if root := strings.TrimSpace(server.projectRoot); root != "" {
		return filepath.Clean(root)
	}
	if root := configuredWorkspaceRootForPath(filePath); root != "" {
		return root
	}
	if strings.TrimSpace(filePath) == "" {
		return ""
	}
	return filepath.Dir(filePath)
}

func configuredWorkspaceRootForPath(filePath string) string {
	if strings.TrimSpace(filePath) == "" {
		return ""
	}
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, "gowdk.config.go")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func fileURI(filePath string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(filePath)}
	return u.String()
}

func componentDefinitionKey(packageName, componentName string) string {
	return packageName + "\x00" + componentName
}
