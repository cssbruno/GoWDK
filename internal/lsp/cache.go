package lsp

import "github.com/cssbruno/gowdk/internal/gwdkir"

type projectIRCache struct {
	key          string
	ir           gwdkir.Program
	docsBySource map[string]document
}

type workspaceComponentDefinitionCache struct {
	root        string
	key         string
	files       []string
	dirs        []string
	definitions map[string]componentDefinition
}

func (server *Server) invalidateProjectCaches() {
	server.projectCache = projectIRCache{}
	server.workspaceComponentCache = workspaceComponentDefinitionCache{}
}
