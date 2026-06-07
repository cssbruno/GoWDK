package app

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

// DefaultActionBodyLimit is the generated action request body limit.
const DefaultActionBodyLimit int64 = 1 << 20

// BackendHandler handles one generated backend route and reports whether it
// wrote a response.
type BackendHandler func(http.ResponseWriter, *http.Request) bool

// BackendRoute describes one generated action or API route.
type BackendRoute struct {
	Method  string
	Path    string
	Kind    string
	Handler BackendHandler
}

// BackendRouter dispatches exact generated backend routes.
type BackendRouter struct {
	routes map[backendRouteKey]backendRouteEntry
}

type backendRouteKey struct {
	method string
	path   string
}

type backendRouteEntry struct {
	kind    string
	handler BackendHandler
}

// NewBackendRouter creates a backend router and registers the supplied routes.
func NewBackendRouter(routes ...BackendRoute) (*BackendRouter, error) {
	router := &BackendRouter{routes: map[backendRouteKey]backendRouteEntry{}}
	for _, route := range routes {
		if err := router.handle(route.Kind, route.Method, route.Path, route.Handler); err != nil {
			return nil, err
		}
	}
	return router, nil
}

// Handle registers a backend route with a normalized method and path.
func (router *BackendRouter) Handle(method string, routePath string, handler BackendHandler) error {
	return router.handle("backend", method, routePath, handler)
}

func (router *BackendRouter) handle(kind string, method string, routePath string, handler BackendHandler) error {
	if router == nil {
		return fmt.Errorf("backend router is nil")
	}
	if router.routes == nil {
		router.routes = map[backendRouteKey]backendRouteEntry{}
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return fmt.Errorf("backend route method is required")
	}
	if handler == nil {
		return fmt.Errorf("backend route %s %s handler is required", method, routePath)
	}
	key := backendRouteKey{method: method, path: normalizeBackendPath(routePath)}
	if _, exists := router.routes[key]; exists {
		return fmt.Errorf("duplicate backend route %s %s", key.method, key.path)
	}
	router.routes[key] = backendRouteEntry{kind: strings.ToLower(strings.TrimSpace(kind)), handler: BackendBoundary(kind, handler)}
	return nil
}

// Action registers a generated POST action route.
func (router *BackendRouter) Action(routePath string, handler BackendHandler) error {
	return router.handle("action", http.MethodPost, routePath, handler)
}

// API registers a generated API route.
func (router *BackendRouter) API(method string, routePath string, handler BackendHandler) error {
	return router.handle("api", method, routePath, handler)
}

// Dispatch writes a route response when the request matches a backend route.
func (router *BackendRouter) Dispatch(writer http.ResponseWriter, request *http.Request) bool {
	if router == nil || request == nil {
		return false
	}
	route := router.routes[backendRouteKey{
		method: strings.ToUpper(strings.TrimSpace(request.Method)),
		path:   normalizeBackendPath(request.URL.Path),
	}]
	if route.handler == nil {
		return false
	}
	if route.kind == "query" && !isContractQueryRequest(request) {
		return false
	}
	return route.handler(writer, request)
}

func isContractQueryRequest(request *http.Request) bool {
	if request == nil {
		return false
	}
	queryHeader := strings.TrimSpace(request.Header.Get("X-GOWDK-Query"))
	if strings.EqualFold(queryHeader, "1") || strings.EqualFold(queryHeader, "true") {
		return true
	}
	for _, accept := range request.Header.Values("Accept") {
		if acceptsJSON(accept) {
			return true
		}
	}
	return false
}

func acceptsJSON(header string) bool {
	for _, part := range strings.Split(header, ",") {
		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(part, ";")[0]))
		if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
			return true
		}
	}
	return false
}

// HandlerFunc returns the router as a generated runtime hook.
func (router *BackendRouter) HandlerFunc() HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) bool {
		return router.Dispatch(writer, request)
	}
}

// ServeHTTP serves the backend router directly.
func (router *BackendRouter) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !router.Dispatch(writer, request) {
		http.NotFound(writer, request)
	}
}

// Action0 adapts a no-input action handler.
func Action0(handler func(context.Context) (response.Response, error)) BackendHandler {
	if handler == nil {
		return NotImplemented("GOWDK action handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx, ok := prepareAction(writer, request)
		if !ok {
			return true
		}
		result, err := handler(ctx)
		writeBackendResult(writer, result, err)
		return true
	}
}

// ActionForm adapts a typed value action handler with an explicit decoder.
func ActionForm[T any](decode func(form.Values) (T, error), handler func(context.Context, T) (response.Response, error)) BackendHandler {
	if decode == nil || handler == nil {
		return NotImplemented("GOWDK action handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx, values, ok := prepareActionValues(writer, request)
		if !ok {
			return true
		}
		input, err := decode(values)
		if err != nil {
			response.WriteNoStoreError(writer, http.StatusBadRequest, "invalid form")
			return true
		}
		result, err := handler(ctx, input)
		writeBackendResult(writer, result, err)
		return true
	}
}

// ActionFormPtr adapts a typed pointer action handler with an explicit decoder.
func ActionFormPtr[T any](decode func(form.Values) (T, error), handler func(context.Context, *T) (response.Response, error)) BackendHandler {
	if decode == nil || handler == nil {
		return NotImplemented("GOWDK action handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx, values, ok := prepareActionValues(writer, request)
		if !ok {
			return true
		}
		input, err := decode(values)
		if err != nil {
			response.WriteNoStoreError(writer, http.StatusBadRequest, "invalid form")
			return true
		}
		result, err := handler(ctx, &input)
		writeBackendResult(writer, result, err)
		return true
	}
}

// ActionValues adapts a low-level form.Values action handler.
func ActionValues(handler func(context.Context, form.Values) (response.Response, error)) BackendHandler {
	if handler == nil {
		return NotImplemented("GOWDK action handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx, values, ok := prepareActionValues(writer, request)
		if !ok {
			return true
		}
		result, err := handler(ctx, values)
		writeBackendResult(writer, result, err)
		return true
	}
}

// APIHandler adapts an API handler.
func APIHandler(handler func(context.Context, *http.Request) (response.Response, error)) BackendHandler {
	if handler == nil {
		return NotImplemented("GOWDK API handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx := WithRequest(request.Context(), request)
		result, err := handler(ctx, request)
		writeBackendResult(writer, result, err)
		return true
	}
}

// NotImplemented returns a generated 501 backend handler.
func NotImplemented(message string) BackendHandler {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "GOWDK backend handler is not implemented"
	}
	return func(writer http.ResponseWriter, _ *http.Request) bool {
		response.WriteNoStoreError(writer, http.StatusNotImplemented, message)
		return true
	}
}

func prepareAction(writer http.ResponseWriter, request *http.Request) (context.Context, bool) {
	ctx, _, ok := prepareActionValues(writer, request)
	return ctx, ok
}

func prepareActionValues(writer http.ResponseWriter, request *http.Request) (context.Context, form.Values, bool) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		response.WriteNoStoreError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return nil, nil, false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, DefaultActionBodyLimit)
	if err := request.ParseForm(); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			response.WriteNoStoreError(writer, http.StatusRequestEntityTooLarge, "request body too large")
			return nil, nil, false
		}
		response.WriteNoStoreError(writer, http.StatusBadRequest, "invalid form")
		return nil, nil, false
	}
	ctx := WithRequest(request.Context(), request)
	return ctx, form.FromURLValues(request.PostForm), true
}

func writeBackendResult(writer http.ResponseWriter, result response.Response, err error) {
	if err != nil {
		response.WriteNoStoreError(writer, response.HandlerStatus(err, http.StatusInternalServerError), err.Error())
		return
	}
	_ = response.WriteNoStoreHTTP(writer, result)
}

func normalizeBackendPath(routePath string) string {
	return path.Clean("/" + routePath)
}
