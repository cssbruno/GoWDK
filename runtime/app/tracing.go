package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

const tracePathPrefix = "/_gowdk/traces"
const localTraceViewerAddr = "127.0.0.1:0"

// TraceAccess decides whether a request may use a generated trace endpoint.
type TraceAccess func(*http.Request) bool

func isTracePath(requestPath string) bool {
	return requestPath == tracePathPrefix || strings.HasPrefix(requestPath, tracePathPrefix+"/")
}

func (handler Handler) traceAccessAllowed(request *http.Request) bool {
	if handler.TraceAccess != nil {
		return handler.TraceAccess(request)
	}
	return LocalTraceAccess(request)
}

// BrowserTraceIngestAccess allows only generated browser span ingest on the
// main app handler. Readable trace viewer, JSON, and SSE routes should use
// LocalTraceViewerService or an app-owned TraceAccess policy.
func BrowserTraceIngestAccess(request *http.Request) bool {
	if request == nil || request.URL == nil {
		return false
	}
	if request.Method != http.MethodPost || request.URL.Path != tracePathPrefix+"/browser" {
		return false
	}
	return traceBrowserIngestOriginAllowed(request)
}

func traceBrowserIngestOriginAllowed(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	scheme := traceExternalRequestScheme(request)
	if scheme == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, scheme) &&
		strings.EqualFold(traceCanonicalOriginHost(parsed.Scheme, parsed.Host), traceCanonicalOriginHost(scheme, request.Host))
}

func traceExternalRequestScheme(request *http.Request) string {
	if request.TLS != nil {
		return "https"
	}
	if scheme := traceForwardedRequestProto(request.Header); scheme != "" {
		return scheme
	}
	if request.URL != nil && request.URL.Scheme != "" {
		return request.URL.Scheme
	}
	return "http"
}

func traceForwardedRequestProto(header http.Header) string {
	for _, value := range header.Values("Forwarded") {
		for _, forwarded := range strings.Split(value, ",") {
			for _, part := range strings.Split(forwarded, ";") {
				name, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
				if !ok || !strings.EqualFold(name, "proto") {
					continue
				}
				if scheme := traceCleanForwardedProto(raw); scheme != "" {
					return scheme
				}
			}
		}
	}
	for _, value := range header.Values("X-Forwarded-Proto") {
		if scheme := traceCleanForwardedProto(strings.Split(value, ",")[0]); scheme != "" {
			return scheme
		}
	}
	return ""
}

func traceCleanForwardedProto(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), `"`))
	if value == "http" || value == "https" {
		return value
	}
	return ""
}

func traceCanonicalOriginHost(scheme string, host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	name, port, err := net.SplitHostPort(host)
	if err == nil {
		name = strings.ToLower(strings.Trim(name, "[]"))
		if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
			return name
		}
		return name + ":" + port
	}
	return strings.Trim(host, "[]")
}

// LocalTraceAccess allows a trace endpoint only on direct loopback connections.
// Applications that mount trace handlers on their own can use it as a local-only
// gate. The decision intentionally ignores request.Host because Host is
// client-controlled and can be forwarded by a reverse proxy.
func LocalTraceAccess(request *http.Request) bool {
	if request == nil {
		return false
	}
	if hasForwardedProxyHeader(request.Header) {
		return false
	}
	if !loopbackHost(request.RemoteAddr) {
		return false
	}
	return loopbackRequestLocalAddr(request)
}

func hasForwardedProxyHeader(header http.Header) bool {
	for name := range header {
		if strings.EqualFold(name, "Forwarded") || strings.EqualFold(name, "X-Real-IP") || strings.HasPrefix(strings.ToLower(name), "x-forwarded-") {
			return true
		}
	}
	return false
}

func loopbackHost(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	if host == "" {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loopbackRequestLocalAddr(request *http.Request) bool {
	localAddr, ok := request.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok || localAddr == nil {
		return false
	}
	return loopbackHost(localAddr.String())
}

// LocalTraceViewerService serves the readable trace viewer, JSON data, and SSE
// stream on a separate loopback listener. Generated apps use this so the main
// app handler does not expose trace reads through public or reverse-proxied
// routes.
func LocalTraceViewerService(handler http.Handler) Service {
	return localTraceViewerService{handler: handler}
}

type localTraceViewerService struct {
	handler http.Handler
	onStart func(net.Addr)
}

func (service localTraceViewerService) Name() string {
	return "trace-viewer"
}

func (service localTraceViewerService) Mount(ServiceContext) error {
	return nil
}

func (service localTraceViewerService) Run(ctx context.Context, _ ServiceContext) error {
	if ctx == nil {
		ctx = context.Background()
	}
	listener, err := net.Listen("tcp", localTraceViewerAddr)
	if err != nil {
		return fmt.Errorf("listen local trace viewer: %w", err)
	}
	if service.onStart != nil {
		service.onStart(listener.Addr())
	}
	traceHandler := service.traceHandler()
	mux := http.NewServeMux()
	mux.Handle(tracePathPrefix, http.StripPrefix(tracePathPrefix, traceHandler))
	mux.Handle(tracePathPrefix+"/", http.StripPrefix(tracePathPrefix, traceHandler))
	serverContext, cancelServer := context.WithCancel(ctx)
	defer cancelServer()
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
		BaseContext: func(net.Listener) context.Context {
			return serverContext
		},
	}
	server.RegisterOnShutdown(cancelServer)
	fmt.Fprintf(os.Stderr, "GOWDK trace viewer: http://%s%s\n", listener.Addr().String(), tracePathPrefix)
	done := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("serve local trace viewer: %w", err)
		}
		return nil
	case <-ctx.Done():
		cancelServer()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		timedOut := errors.Is(err, context.DeadlineExceeded)
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !timedOut {
			return fmt.Errorf("shutdown local trace viewer: %w", err)
		}
		if timedOut {
			if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
				return fmt.Errorf("close local trace viewer after shutdown timeout: %w", closeErr)
			}
		}
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("serve local trace viewer: %w", err)
			}
			return nil
		case <-time.After(time.Second):
			if timedOut {
				return nil
			}
			return fmt.Errorf("shutdown local trace viewer timed out")
		}
	}
}

func (service localTraceViewerService) traceHandler() http.Handler {
	if service.handler != nil {
		return service.handler
	}
	return http.NotFoundHandler()
}

func (handler Handler) startRequestTrace(response http.ResponseWriter, request *http.Request) (http.ResponseWriter, *http.Request, *gowdktrace.Span) {
	ctx := gowdktrace.Extract(request.Context(), request.Header)
	ctx, span := handler.Tracer.Start(ctx, request.Method+" "+request.URL.Path,
		gowdktrace.WithSurface(gowdktrace.SurfaceBackend),
		gowdktrace.WithLane(gowdktrace.LaneRoute),
		gowdktrace.WithAttributes(map[string]any{
			gowdktrace.AttrHTTPRequestMethod: request.Method,
			gowdktrace.AttrURLPath:           request.URL.Path,
			"gowdk.route.query":              redactSecrets(request.URL.RawQuery),
		}),
	)
	request = request.WithContext(ctx)
	if span == nil {
		return response, request, nil
	}
	recorder := &traceResponseWriter{ResponseWriter: response, status: http.StatusOK}
	return wrapTraceResponseWriter(recorder), request, span
}

func finishRequestTrace(response http.ResponseWriter, span *gowdktrace.Span) {
	FinishHTTPTrace(response, span)
}

// FinishHTTPTrace completes a generated route span with the response status.
func FinishHTTPTrace(response http.ResponseWriter, span *gowdktrace.Span) {
	if span == nil {
		return
	}
	status := http.StatusOK
	if recorder, ok := response.(interface{ traceRecorder() *traceResponseWriter }); ok {
		status = recorder.traceRecorder().status
	}
	span.Set(gowdktrace.AttrHTTPResponseStatusCode, status)
	if status >= 500 {
		span.SetStatus(gowdktrace.StatusError, http.StatusText(status))
	} else {
		span.SetStatus(gowdktrace.StatusOK, "")
	}
	span.End()
}

// FinishTrace completes a generated non-HTTP child span with an optional error.
func FinishTrace(span *gowdktrace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.SetStatus(gowdktrace.StatusError, redactSecrets(strings.TrimSpace(err.Error())))
	} else {
		span.SetStatus(gowdktrace.StatusOK, "")
	}
	span.End()
}

type traceResponseWriter struct {
	http.ResponseWriter
	status int
}

func (writer *traceResponseWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *traceResponseWriter) Write(payload []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	return writer.ResponseWriter.Write(payload)
}

func (writer *traceResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *traceResponseWriter) traceRecorder() *traceResponseWriter {
	return writer
}

// Trace records a redacted user event on the active span. It is intentionally
// small so Go handlers can call app.Trace(ctx, "loaded patient", attrs) without
// depending on a hosted tracing backend.
func Trace(ctx context.Context, message string, attrs map[string]any) {
	span := gowdktrace.SpanFrom(ctx)
	if span == nil {
		return
	}
	span.Event("info", redactSecrets(message), redactTraceAttrs(attrs))
}

func redactTraceAttrs(attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]any, len(attrs))
	for key, value := range attrs {
		switch typed := value.(type) {
		case string:
			out[key] = redactSecrets(typed)
		case []byte:
			out[key] = redactSecrets(string(typed))
		case int:
			out[key] = typed
		case int64:
			out[key] = typed
		case uint:
			out[key] = typed
		case uint64:
			out[key] = typed
		case float64:
			out[key] = typed
		case bool:
			out[key] = typed
		case nil:
			out[key] = nil
		default:
			out[key] = "[redacted " + strconv.FormatInt(int64(len(strings.TrimSpace(key))), 10) + " byte value]"
		}
	}
	return out
}
