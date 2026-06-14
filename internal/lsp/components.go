package lsp

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
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
	switch lang.ClassifySource(doc.Path, []byte(doc.Text)) {
	case lang.FileKindPage:
		page, diagnostics := lang.ParseSource(doc.Path, []byte(doc.Text))
		if diagnostics.HasErrors() {
			return "", nil
		}
		return page.Package, usePackagesByAlias(page.Uses)
	case lang.FileKindComponent:
		component, diagnostics := lang.ParseComponentSource(doc.Path, []byte(doc.Text))
		if diagnostics.HasErrors() {
			return "", nil
		}
		return component.Package, usePackagesByAlias(component.Uses)
	default:
		return "", nil
	}
}

func (server *Server) componentDefinitions(doc document) map[string]componentDefinition {
	definitions := map[string]componentDefinition{}
	for key, definition := range server.workspaceComponentDefinitions(doc) {
		definitions[key] = definition
	}
	for key, definition := range server.openComponentDefinitions() {
		definitions[key] = definition
	}
	return definitions
}

func (server *Server) openComponentDefinitions() map[string]componentDefinition {
	definitions := map[string]componentDefinition{}
	ir, docsBySource := server.openProjectIR()
	for _, component := range ir.Components {
		if component.Name == "" {
			continue
		}
		doc, ok := docsBySource[component.Source]
		if !ok {
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

func (server *Server) workspaceComponentDefinitions(doc document) map[string]componentDefinition {
	definitions := map[string]componentDefinition{}
	root := workspaceRootForPath(doc.Path)
	if root == "" {
		return definitions
	}
	if server.workspaceComponentCache.root == root && server.workspaceComponentCache.key != "" {
		key := workspaceComponentCacheKey(server.workspaceComponentCache.files, server.workspaceComponentCache.dirs)
		if key == server.workspaceComponentCache.key {
			return cloneComponentDefinitions(server.workspaceComponentCache.definitions)
		}
	}
	definitions, key, files, dirs := server.loadWorkspaceComponentDefinitions(root)
	server.workspaceComponentCache = workspaceComponentDefinitionCache{
		root:        root,
		key:         key,
		files:       files,
		dirs:        dirs,
		definitions: cloneComponentDefinitions(definitions),
	}
	return definitions
}

func (server *Server) loadWorkspaceComponentDefinitions(root string) (map[string]componentDefinition, string, []string, []string) {
	definitions := map[string]componentDefinition{}
	var paths []string
	var dirs []string
	payloads := map[string]string{}
	_ = filepath.WalkDir(root, func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if shouldSkipWorkspaceDir(entry.Name()) && filePath != root {
				return filepath.SkipDir
			}
			dirs = append(dirs, filePath)
			return nil
		}
		if !strings.HasSuffix(filePath, ".gwdk") {
			return nil
		}
		if _, open := server.openDocumentByPath(filePath); open {
			return nil
		}
		payload, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}
		if lang.ClassifySource(filePath, payload) != lang.FileKindComponent {
			return nil
		}
		paths = append(paths, filePath)
		payloads[filePath] = string(payload)
		return nil
	})
	sort.Strings(paths)
	sort.Strings(dirs)
	key := workspaceComponentCacheKey(paths, dirs)
	if len(paths) == 0 {
		return definitions, key, paths, dirs
	}
	for _, path := range paths {
		component, diagnostics := lang.ParseComponentSource(path, []byte(payloads[path]))
		if diagnostics.HasErrors() || component.Name == "" {
			continue
		}
		definition := componentDefinition{
			URI:     fileURI(component.Source),
			Text:    payloads[path],
			Package: component.Package,
			Name:    component.Name,
			Span:    component.Span,
		}
		definitions[componentDefinitionKey(component.Package, component.Name)] = definition
		if component.Package == "" {
			definitions[componentDefinitionKey("", component.Name)] = definition
		}
	}
	return definitions, key, paths, dirs
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

func workspaceRootForPath(filePath string) string {
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
	return filepath.Dir(filePath)
}

func shouldSkipWorkspaceDir(name string) bool {
	switch name {
	case ".git", ".gowdk", "bin", "dist", "gowdk_cache", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func fileURI(filePath string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(filePath)}
	return u.String()
}

func usePackagesByAlias(uses []gwdkir.Use) map[string]string {
	packages := map[string]string{}
	for _, use := range uses {
		if _, exists := packages[use.Alias]; !exists {
			packages[use.Alias] = use.Package
		}
	}
	return packages
}

func componentDefinitionKey(packageName, componentName string) string {
	return packageName + "\x00" + componentName
}
