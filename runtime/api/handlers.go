package api

import "net/http"

// Handler is a generated API endpoint.
type Handler func(http.ResponseWriter, *http.Request)

// Registry maps generated API handler names to handlers.
type Registry map[string]Handler
