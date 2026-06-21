package app

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
	gowdkroute "github.com/cssbruno/gowdk/runtime/route"
	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

// DefaultActionBodyLimit is the generated action request body limit.
const DefaultActionBodyLimit int64 = 1 << 20

// DefaultAPIBodyLimit is the generated API request body limit. API handlers
// receive the raw *http.Request and decode the body themselves, so the limit
// is enforced by capping request.Body before the handler runs.
const DefaultAPIBodyLimit int64 = 1 << 20

// BackendHandler handles one generated backend route and reports whether it
// wrote a response.
type BackendHandler func(http.ResponseWriter, *http.Request) bool

// BackendRoute describes one generated action or API route.
type BackendRoute struct {
	Method  string
	Path    string
	Kind    string
	Source  gowdktrace.SourceRef
	Handler BackendHandler
}

// BackendRouter dispatches generated backend routes.
type BackendRouter struct {
	routes   map[backendRouteKey]backendRouteEntry
	patterns []backendPatternRouteEntry
	cors     corsPolicy
}

type backendRouteKey struct {
	method string
	path   string
}

type backendRouteEntry struct {
	method  string
	kind    string
	path    string
	source  gowdktrace.SourceRef
	handler BackendHandler
}

type backendPatternRouteEntry struct {
	key     backendRouteKey
	kind    string
	source  gowdktrace.SourceRef
	handler BackendHandler
}

// NewBackendRouter creates a backend router and registers the supplied routes.
func NewBackendRouter(routes ...BackendRoute) (*BackendRouter, error) {
	router := &BackendRouter{routes: map[backendRouteKey]backendRouteEntry{}}
	for _, route := range routes {
		if err := router.handle(route); err != nil {
			return nil, err
		}
	}
	return router, nil
}

// Handle registers a backend route with a normalized method and path.
func (router *BackendRouter) Handle(method string, routePath string, handler BackendHandler) error {
	return router.handle(BackendRoute{Kind: "backend", Method: method, Path: routePath, Handler: handler})
}

func (router *BackendRouter) handle(route BackendRoute) error {
	if router == nil {
		return fmt.Errorf("backend router is nil")
	}
	if router.routes == nil {
		router.routes = map[backendRouteKey]backendRouteEntry{}
	}
	method := strings.ToUpper(strings.TrimSpace(route.Method))
	if method == "" {
		return fmt.Errorf("backend route method is required")
	}
	if route.Handler == nil {
		return fmt.Errorf("backend route %s %s handler is required", method, route.Path)
	}
	kind := strings.ToLower(strings.TrimSpace(route.Kind))
	if kind == "" {
		kind = "backend"
	}
	key := backendRouteKey{method: method, path: normalizeBackendPath(route.Path)}
	handler := BackendBoundary(kind, traceBackendRoute(kind, key.path, route.Source, route.Handler))
	if backendRouteIsDynamic(key.path) {
		for _, existing := range router.patterns {
			if existing.key == key {
				return fmt.Errorf("duplicate backend route %s %s", key.method, key.path)
			}
		}
		router.patterns = append(router.patterns, backendPatternRouteEntry{key: key, kind: kind, source: route.Source, handler: handler})
		return nil
	}
	if _, exists := router.routes[key]; exists {
		return fmt.Errorf("duplicate backend route %s %s", key.method, key.path)
	}
	router.routes[key] = backendRouteEntry{method: key.method, kind: kind, path: key.path, source: route.Source, handler: handler}
	return nil
}

// SetCORSPolicy enables CORS handling for generated API and web contract
// routes. The zero policy disables CORS.
func (router *BackendRouter) SetCORSPolicy(policy CORSPolicy) error {
	if router == nil {
		return fmt.Errorf("backend router is nil")
	}
	normalized, err := normalizeCORSPolicy(policy)
	if err != nil {
		return err
	}
	router.cors = normalized
	return nil
}

// Action registers a generated POST action route.
func (router *BackendRouter) Action(routePath string, handler BackendHandler) error {
	return router.handle(BackendRoute{Kind: "action", Method: http.MethodPost, Path: routePath, Handler: handler})
}

// API registers a generated API route.
func (router *BackendRouter) API(method string, routePath string, handler BackendHandler) error {
	return router.handle(BackendRoute{Kind: "api", Method: method, Path: routePath, Handler: handler})
}

// Dispatch writes a route response when the request matches a backend route.
func (router *BackendRouter) Dispatch(writer http.ResponseWriter, request *http.Request) bool {
	if router == nil || request == nil {
		return false
	}
	if router.dispatchCORSPreflight(writer, request) {
		return true
	}
	key := backendRouteKey{
		method: strings.ToUpper(strings.TrimSpace(request.Method)),
		path:   normalizeBackendPath(request.URL.Path),
	}
	route := router.routes[key]
	if route.handler == nil {
		return router.dispatchPattern(writer, request, key.method)
	}
	if route.kind == "query" && !isContractQueryRequest(request) {
		return false
	}
	if backendRouteSupportsCORS(route.kind) {
		router.cors.writeActualHeaders(writer, request, route.method)
	}
	return route.handler(writer, request)
}

func (router *BackendRouter) dispatchPattern(writer http.ResponseWriter, request *http.Request, method string) bool {
	for _, route := range router.patterns {
		if route.key.method != method {
			continue
		}
		if route.kind == "query" && !isContractQueryRequest(request) {
			continue
		}
		if _, ok := gowdkroute.Match(route.key.path, request.URL.Path); !ok {
			continue
		}
		if backendRouteSupportsCORS(route.kind) {
			router.cors.writeActualHeaders(writer, request, route.key.method)
		}
		return route.handler(writer, request)
	}
	return false
}

func (router *BackendRouter) dispatchCORSPreflight(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method != http.MethodOptions || strings.TrimSpace(request.Header.Get("Access-Control-Request-Method")) == "" {
		return false
	}
	requestedMethod, err := normalizeCORSMethod(request.Header.Get("Access-Control-Request-Method"))
	if err != nil {
		return false
	}
	route, ok := router.corsRoute(requestedMethod, request.URL.Path)
	if !ok {
		return false
	}
	if router.cors.writePreflight(writer, request, route.method) {
		return true
	}
	response.WriteNoStoreError(writer, http.StatusForbidden, "cors preflight denied")
	return true
}

func (router *BackendRouter) corsRoute(method string, requestPath string) (backendRouteEntry, bool) {
	key := backendRouteKey{method: method, path: normalizeBackendPath(requestPath)}
	if route := router.routes[key]; route.handler != nil && backendRouteSupportsCORS(route.kind) {
		return route, true
	}
	for _, route := range router.patterns {
		if route.key.method != method || !backendRouteSupportsCORS(route.kind) {
			continue
		}
		if _, ok := gowdkroute.Match(route.key.path, requestPath); ok {
			return backendRouteEntry{method: route.key.method, kind: route.kind, path: route.key.path, source: route.source, handler: route.handler}, true
		}
	}
	return backendRouteEntry{}, false
}

func backendRouteSupportsCORS(kind string) bool {
	switch kind {
	case "api", "command", "query":
		return true
	default:
		return false
	}
}

func traceBackendRoute(kind, routePath string, source gowdktrace.SourceRef, handler BackendHandler) BackendHandler {
	if handler == nil {
		return nil
	}
	lane := backendTraceLane(kind)
	name := strings.TrimSpace(kind)
	if name == "" {
		name = "backend"
	}
	name += " " + routePath
	return func(writer http.ResponseWriter, request *http.Request) bool {
		if request == nil {
			return handler(writer, request)
		}
		if _, ok := gowdktrace.TracerFromContext(request.Context()); !ok {
			return handler(writer, request)
		}
		recorder, ok := writer.(interface{ traceRecorder() *traceResponseWriter })
		var traceRecorder *traceResponseWriter
		if ok {
			traceRecorder = recorder.traceRecorder()
		} else {
			traceRecorder = &traceResponseWriter{ResponseWriter: writer, status: http.StatusOK}
			writer = wrapTraceResponseWriter(traceRecorder)
		}
		ctx, span := gowdktrace.Start(request.Context(), name,
			gowdktrace.WithSurface(gowdktrace.SurfaceBackend),
			gowdktrace.WithLane(lane),
			gowdktrace.WithSource(source),
			gowdktrace.WithAttributes(map[string]any{
				gowdktrace.AttrHTTPRoute: routePath,
				"gowdk.endpoint.kind":    kind,
			}),
		)
		defer func() {
			if recovered := recover(); recovered != nil {
				span.SetStatus(gowdktrace.StatusError, redactSecrets(strings.TrimSpace(fmt.Sprint(recovered))))
				span.End()
				panic(recovered)
			}
			span.Set(gowdktrace.AttrHTTPResponseStatusCode, traceRecorder.status)
			if traceRecorder.status >= 500 {
				span.SetStatus(gowdktrace.StatusError, http.StatusText(traceRecorder.status))
			} else {
				span.SetStatus(gowdktrace.StatusOK, "")
			}
			span.End()
		}()
		handled := handler(writer, request.WithContext(ctx))
		return handled
	}
}

func backendTraceLane(kind string) gowdktrace.Lane {
	switch kind {
	case "action", "command":
		return gowdktrace.LaneAction
	case "api", "query":
		return gowdktrace.LaneAPI
	case "fragment":
		return gowdktrace.LaneFragment
	case "ssr":
		return gowdktrace.LaneSSR
	default:
		return gowdktrace.LaneHandler
	}
}

func backendRouteIsDynamic(routePath string) bool {
	return strings.Contains(routePath, "{") && strings.Contains(routePath, "}")
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
	return Action0WithBodyLimit(DefaultActionBodyLimit, handler)
}

// Action0WithBodyLimit adapts a no-input action handler with a custom request
// body limit. Non-positive limits use DefaultActionBodyLimit.
func Action0WithBodyLimit(bodyLimit int64, handler func(context.Context) (response.Response, error)) BackendHandler {
	if handler == nil {
		return NotImplemented("GOWDK action handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx, ok := prepareAction(writer, request, bodyLimit)
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
	return ActionFormWithBodyLimit(DefaultActionBodyLimit, decode, handler)
}

// ActionFormWithBodyLimit adapts a typed value action handler with an explicit
// decoder and custom request body limit. Non-positive limits use
// DefaultActionBodyLimit.
func ActionFormWithBodyLimit[T any](bodyLimit int64, decode func(form.Values) (T, error), handler func(context.Context, T) (response.Response, error)) BackendHandler {
	if decode == nil || handler == nil {
		return NotImplemented("GOWDK action handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx, values, ok := prepareActionValues(writer, request, bodyLimit)
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
	return ActionFormPtrWithBodyLimit(DefaultActionBodyLimit, decode, handler)
}

// ActionFormPtrWithBodyLimit adapts a typed pointer action handler with an
// explicit decoder and custom request body limit. Non-positive limits use
// DefaultActionBodyLimit.
func ActionFormPtrWithBodyLimit[T any](bodyLimit int64, decode func(form.Values) (T, error), handler func(context.Context, *T) (response.Response, error)) BackendHandler {
	if decode == nil || handler == nil {
		return NotImplemented("GOWDK action handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx, values, ok := prepareActionValues(writer, request, bodyLimit)
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
	return ActionValuesWithBodyLimit(DefaultActionBodyLimit, handler)
}

// ActionValuesWithBodyLimit adapts a low-level form.Values action handler with
// a custom request body limit. Non-positive limits use DefaultActionBodyLimit.
func ActionValuesWithBodyLimit(bodyLimit int64, handler func(context.Context, form.Values) (response.Response, error)) BackendHandler {
	if handler == nil {
		return NotImplemented("GOWDK action handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		ctx, values, ok := prepareActionValues(writer, request, bodyLimit)
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
	return APIHandlerWithBodyLimit(DefaultAPIBodyLimit, handler)
}

// APIHandlerWithBodyLimit adapts an API handler with a custom request body
// limit. Non-positive limits use DefaultAPIBodyLimit.
func APIHandlerWithBodyLimit(bodyLimit int64, handler func(context.Context, *http.Request) (response.Response, error)) BackendHandler {
	if handler == nil {
		return NotImplemented("GOWDK API handler is not implemented")
	}
	return func(writer http.ResponseWriter, request *http.Request) bool {
		request.Body = http.MaxBytesReader(writer, request.Body, normalizeBodyLimit(bodyLimit, DefaultAPIBodyLimit))
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

func prepareAction(writer http.ResponseWriter, request *http.Request, bodyLimit int64) (context.Context, bool) {
	ctx, _, ok := prepareActionValues(writer, request, bodyLimit)
	return ctx, ok
}

func prepareActionValues(writer http.ResponseWriter, request *http.Request, bodyLimit int64) (context.Context, form.Values, bool) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		response.WriteNoStoreError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return nil, nil, false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, normalizeBodyLimit(bodyLimit, DefaultActionBodyLimit))
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

func normalizeBodyLimit(bodyLimit int64, fallback int64) int64 {
	if bodyLimit > 0 {
		return bodyLimit
	}
	return fallback
}

func writeBackendResult(writer http.ResponseWriter, result response.Response, err error) {
	if err != nil {
		response.WriteNoStoreHandlerError(writer, err, http.StatusInternalServerError)
		return
	}
	_ = response.WriteNoStoreHTTP(writer, result)
}

func normalizeBackendPath(routePath string) string {
	return path.Clean("/" + routePath)
}
