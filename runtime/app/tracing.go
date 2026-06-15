package app

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"

	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

const tracePathPrefix = "/_gowdk/traces"

// TraceAccess decides whether a request may use the generated trace viewer.
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

// LocalTraceAccess allows the trace viewer only from loopback clients. Generated
// apps use this as the default gate when the observability viewer is enabled.
func LocalTraceAccess(request *http.Request) bool {
	if request == nil {
		return false
	}
	if request.Header.Get("Forwarded") != "" || request.Header.Get("X-Forwarded-For") != "" || request.Header.Get("X-Real-IP") != "" {
		return false
	}
	if !loopbackHost(request.Host) {
		return false
	}
	return loopbackHost(request.RemoteAddr)
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
	return &traceResponseWriter{ResponseWriter: response, status: http.StatusOK}, request, span
}

func finishRequestTrace(response http.ResponseWriter, span *gowdktrace.Span) {
	if span == nil {
		return
	}
	status := http.StatusOK
	if recorder, ok := response.(*traceResponseWriter); ok {
		status = recorder.status
	}
	span.Set(gowdktrace.AttrHTTPResponseStatusCode, status)
	if status >= 500 {
		span.SetStatus(gowdktrace.StatusError, http.StatusText(status))
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
