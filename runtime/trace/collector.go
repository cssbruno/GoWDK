package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxCollectorBodyBytes     = 1 << 20
	maxCollectorBatchSpans    = 128
	defaultCollectorSSELimit  = 32
	defaultCollectorRateLimit = 120
	maxCollectorRateClients   = 1024
)

const defaultCollectorRateWindow = time.Minute

var errTracePayloadTooLarge = errors.New("trace payload exceeds byte limit")

var viewerTemplate = template.Must(template.New("gowdk-trace-viewer").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>GOWDK Trace</title>
<style>
body{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:0;color:#111827;background:#f8fafc}
header{display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid #d1d5db;background:#fff}
h1{font-size:18px;margin:0}.meta{color:#4b5563;font-size:13px}
main{display:grid;grid-template-columns:minmax(260px,360px) 1fr;min-height:calc(100vh - 58px)}
#list{border-right:1px solid #d1d5db;background:#fff;overflow:auto}.span{display:block;width:100%;text-align:left;border:0;border-bottom:1px solid #e5e7eb;background:#fff;padding:10px 12px;cursor:pointer}
.span:hover,.span.active{background:#eef2ff}.name{font-weight:600}.sub{font-size:12px;color:#4b5563;margin-top:3px}
#detail{padding:18px;overflow:auto}pre{white-space:pre-wrap;background:#111827;color:#f9fafb;padding:14px;border-radius:6px;overflow:auto}
.empty{padding:24px;color:#6b7280}
</style>
</head>
<body>
<header><h1>GOWDK Trace</h1><div class="meta"><span id="count">0</span> spans · <span id="dropped">0</span> dropped · <span id="rejected">0</span> rejected</div></header>
<main><section id="list"><div class="empty">Waiting for spans...</div></section><section id="detail"><pre id="json">{}</pre></section></main>
<script>
(() => {
  const spans = [];
  const list = document.getElementById("list");
  const detail = document.getElementById("json");
  const count = document.getElementById("count");
  const dropped = document.getElementById("dropped");
  const rejected = document.getElementById("rejected");
  const mount = window.location.pathname.endsWith("/") ? window.location.pathname : window.location.pathname + "/";
  function render() {
    count.textContent = String(spans.length);
    if (spans.length === 0) {
      list.innerHTML = '<div class="empty">Waiting for spans...</div>';
      detail.textContent = "{}";
      return;
    }
    list.innerHTML = "";
    spans.slice().reverse().forEach((span, index) => {
      const button = document.createElement("button");
      button.className = "span";
      button.innerHTML = '<div class="name"></div><div class="sub"></div>';
      button.querySelector(".name").textContent = span.name || "(unnamed span)";
      const source = span.source && span.source.file ? " · " + span.source.file + ":" + (span.source.line || 1) : "";
      button.querySelector(".sub").textContent = [span.surface, span.lane, span.status && span.status.code].filter(Boolean).join("/") + source;
      button.addEventListener("click", () => {
        detail.textContent = JSON.stringify(span, null, 2);
        document.querySelectorAll(".span.active").forEach((node) => node.classList.remove("active"));
        button.classList.add("active");
      });
      if (index === 0) {
        button.classList.add("active");
        detail.textContent = JSON.stringify(span, null, 2);
      }
      list.appendChild(button);
    });
  }
  fetch(mount + "data").then((response) => response.json()).then((payload) => {
    dropped.textContent = String(payload.dropped || 0);
    rejected.textContent = String(payload.rejected || 0);
    (payload.spans || []).forEach((span) => spans.push(span));
    render();
  }).catch(() => {});
  const source = new EventSource(mount + "events");
  source.addEventListener("gowdk-trace", (event) => {
    spans.push(JSON.parse(event.data));
    render();
  });
})();
</script>
</body>
</html>`))

// Collector combines an in-memory ring sink with a small HTTP JSON/SSE
// handler for local inspection.
type Collector struct {
	ring             *RingSink
	mu               sync.Mutex
	subscribers      map[chan Snapshot]struct{}
	sseLimit         int
	ingestRateLimit  int
	ingestRateWindow time.Duration
	rateMu           sync.Mutex
	rateWindows      map[string]collectorRateWindow
	dropped          atomic.Uint64
	rejected         atomic.Uint64
}

type collectorRateWindow struct {
	reset time.Time
	count int
}

// CollectorHealthSnapshot is a point-in-time view of local collector state.
type CollectorHealthSnapshot struct {
	Spans                    int    `json:"spans"`
	Dropped                  uint64 `json:"dropped"`
	Rejected                 uint64 `json:"rejected"`
	Subscribers              int    `json:"subscribers"`
	SubscriberQueueDepth     int    `json:"subscriberQueueDepth"`
	SubscriberQueueCapacity  int    `json:"subscriberQueueCapacity"`
	SSELimit                 int    `json:"sseLimit"`
	IngestRateLimit          int    `json:"ingestRateLimit"`
	IngestRateWindowDuration string `json:"ingestRateWindowDuration"`
}

// CollectorOption configures a Collector.
type CollectorOption func(*Collector)

// WithCollectorSSELimit sets the maximum number of concurrent SSE subscribers.
// A non-positive limit disables the cap.
func WithCollectorSSELimit(limit int) CollectorOption {
	return func(collector *Collector) {
		collector.sseLimit = limit
	}
}

// WithCollectorIngestRate sets a fixed-window POST ingest rate per remote
// address. A non-positive limit or window disables rate limiting.
func WithCollectorIngestRate(limit int, window time.Duration) CollectorOption {
	return func(collector *Collector) {
		collector.ingestRateLimit = limit
		collector.ingestRateWindow = window
	}
}

// NewCollector creates a collector backed by a RingSink.
func NewCollector(limit int, options ...CollectorOption) *Collector {
	collector := &Collector{
		ring:             NewRingSink(limit),
		subscribers:      map[chan Snapshot]struct{}{},
		sseLimit:         defaultCollectorSSELimit,
		ingestRateLimit:  defaultCollectorRateLimit,
		ingestRateWindow: defaultCollectorRateWindow,
		rateWindows:      map[string]collectorRateWindow{},
	}
	for _, option := range options {
		option(collector)
	}
	return collector
}

// RecordSpan implements Sink.
func (collector *Collector) RecordSpan(ctx context.Context, span Snapshot) error {
	if collector == nil {
		return nil
	}
	// Normalize here so externally ingested spans (browser POST) are subject to
	// the same source-path policy as locally produced spans before they reach
	// the JSON, SSE, or viewer surfaces. Locally produced spans are already
	// normalized; re-applying the policy is idempotent.
	span = cloneSnapshot(span)
	if err := collector.ring.RecordSpan(ctx, span); err != nil {
		return err
	}
	collector.mu.Lock()
	for subscriber := range collector.subscribers {
		select {
		case subscriber <- span:
		default:
			collector.dropped.Add(1)
		}
	}
	collector.mu.Unlock()
	return nil
}

// Spans returns completed spans from oldest to newest.
func (collector *Collector) Spans() []Snapshot {
	if collector == nil {
		return nil
	}
	return collector.ring.Spans()
}

// Dropped returns the ring overflow and slow-subscriber drop count.
func (collector *Collector) Dropped() uint64 {
	if collector == nil {
		return 0
	}
	return collector.ring.Dropped() + collector.dropped.Load()
}

// Rejected returns the number of rejected ingest or stream requests.
func (collector *Collector) Rejected() uint64 {
	if collector == nil {
		return 0
	}
	return collector.rejected.Load()
}

// HealthSnapshot returns local collector storage, rejection, and stream queue
// health.
func (collector *Collector) HealthSnapshot() CollectorHealthSnapshot {
	if collector == nil {
		return CollectorHealthSnapshot{}
	}
	snapshot := CollectorHealthSnapshot{
		Spans:                    len(collector.Spans()),
		Dropped:                  collector.Dropped(),
		Rejected:                 collector.Rejected(),
		SSELimit:                 collector.sseLimit,
		IngestRateLimit:          collector.ingestRateLimit,
		IngestRateWindowDuration: collector.ingestRateWindow.String(),
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	snapshot.Subscribers = len(collector.subscribers)
	for subscriber := range collector.subscribers {
		snapshot.SubscriberQueueDepth += len(subscriber)
		snapshot.SubscriberQueueCapacity += cap(subscriber)
	}
	return snapshot
}

// Handler serves recent spans as JSON. Requests to /events or requests with an
// Accept header containing text/event-stream receive an SSE stream of existing
// and future spans. POST requests accept one Snapshot or a JSON array of
// Snapshots so generated browser runtimes can join frontend spans to backend
// traces without another collector dependency.
func (collector *Collector) Handler() http.Handler {
	if collector == nil {
		collector = NewCollector(defaultRingLimit)
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			if !requestHasJSONContentType(request) {
				collector.rejectHTTP(response, "unsupported media type", http.StatusUnsupportedMediaType)
				return
			}
			if !sameOriginRequest(request) {
				collector.rejectHTTP(response, "forbidden", http.StatusForbidden)
				return
			}
			if !collector.allowIngest(request) {
				collector.rejectHTTP(response, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			if err := collector.recordJSON(request.Context(), request); err != nil {
				if errors.Is(err, errTracePayloadTooLarge) {
					collector.rejectHTTP(response, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
					return
				}
				collector.rejectHTTP(response, err.Error(), http.StatusBadRequest)
				return
			}
			response.WriteHeader(http.StatusNoContent)
			return
		}
		if request.Method != http.MethodGet {
			collector.methodNotAllowed(response, http.MethodGet+", "+http.MethodPost)
			return
		}
		if request.URL.Path == "/events" || strings.Contains(request.Header.Get("Accept"), "text/event-stream") {
			collector.serveSSE(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(struct {
			Spans    []Snapshot              `json:"spans"`
			Dropped  uint64                  `json:"dropped"`
			Rejected uint64                  `json:"rejected"`
			Health   CollectorHealthSnapshot `json:"health"`
		}{Spans: collector.Spans(), Dropped: collector.Dropped(), Rejected: collector.Rejected(), Health: collector.HealthSnapshot()})
	})
}

// ViewerHandler serves a self-contained local trace viewer plus JSON and SSE
// endpoints. Generated apps mount it behind an access gate when enabling it
// outside dev; raw callers should do the same for internet-facing handlers.
func (collector *Collector) ViewerHandler() http.Handler {
	if collector == nil {
		collector = NewCollector(defaultRingLimit)
	}
	api := collector.Handler()
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch strings.Trim(request.URL.Path, "/") {
		case "":
			if request.Method != http.MethodGet {
				collector.methodNotAllowed(response, http.MethodGet)
				return
			}
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = viewerTemplate.Execute(response, nil)
		case "data":
			if request.Method != http.MethodGet {
				collector.methodNotAllowed(response, http.MethodGet)
				return
			}
			cloned := request.Clone(request.Context())
			cloned.URL.Path = "/"
			api.ServeHTTP(response, cloned)
		case "events":
			if request.Method != http.MethodGet {
				collector.methodNotAllowed(response, http.MethodGet)
				return
			}
			cloned := request.Clone(request.Context())
			cloned.URL.Path = "/events"
			api.ServeHTTP(response, cloned)
		case "browser":
			if request.Method != http.MethodPost {
				collector.methodNotAllowed(response, http.MethodPost)
				return
			}
			cloned := request.Clone(request.Context())
			cloned.URL.Path = "/"
			api.ServeHTTP(response, cloned)
		default:
			http.NotFound(response, request)
		}
	})
}

func (collector *Collector) methodNotAllowed(response http.ResponseWriter, allow string) {
	response.Header().Set("Allow", allow)
	collector.rejectHTTP(response, "method not allowed", http.StatusMethodNotAllowed)
}

func (collector *Collector) rejectHTTP(response http.ResponseWriter, message string, status int) {
	collector.rejected.Add(1)
	http.Error(response, message, status)
}

func requestHasJSONContentType(request *http.Request) bool {
	contentType := request.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	mediaType = strings.ToLower(mediaType)
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func sameOriginRequest(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	scheme := requestScheme(request)
	return strings.EqualFold(parsed.Scheme, scheme) && strings.EqualFold(canonicalOriginHost(parsed.Scheme, parsed.Host), canonicalOriginHost(scheme, request.Host))
}

func requestScheme(request *http.Request) string {
	if request.TLS != nil {
		return "https"
	}
	if scheme := forwardedRequestProto(request.Header); scheme != "" {
		return scheme
	}
	if request.URL != nil && request.URL.Scheme != "" {
		return request.URL.Scheme
	}
	return "http"
}

func forwardedRequestProto(header http.Header) string {
	for _, value := range header.Values("Forwarded") {
		for _, forwarded := range strings.Split(value, ",") {
			for _, part := range strings.Split(forwarded, ";") {
				name, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
				if !ok || !strings.EqualFold(name, "proto") {
					continue
				}
				if scheme := cleanForwardedProto(raw); scheme != "" {
					return scheme
				}
			}
		}
	}
	for _, value := range header.Values("X-Forwarded-Proto") {
		if scheme := cleanForwardedProto(strings.Split(value, ",")[0]); scheme != "" {
			return scheme
		}
	}
	return ""
}

func cleanForwardedProto(value string) string {
	value = strings.ToLower(strings.Trim(strings.TrimSpace(value), `"`))
	if value == "http" || value == "https" {
		return value
	}
	return ""
}

func canonicalOriginHost(scheme string, host string) string {
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

func (collector *Collector) allowIngest(request *http.Request) bool {
	if collector.ingestRateLimit <= 0 || collector.ingestRateWindow <= 0 {
		return true
	}
	key := collectorRateKey(request)
	now := time.Now()
	collector.rateMu.Lock()
	defer collector.rateMu.Unlock()
	if collector.rateWindows == nil {
		collector.rateWindows = map[string]collectorRateWindow{}
	}
	window, ok := collector.rateWindows[key]
	if !ok && len(collector.rateWindows) >= maxCollectorRateClients {
		collector.pruneExpiredRateWindowsLocked(now)
		if len(collector.rateWindows) >= maxCollectorRateClients {
			return false
		}
	}
	if !ok || !now.Before(window.reset) {
		collector.rateWindows[key] = collectorRateWindow{reset: now.Add(collector.ingestRateWindow), count: 1}
		return true
	}
	if window.count >= collector.ingestRateLimit {
		return false
	}
	window.count++
	collector.rateWindows[key] = window
	return true
}

func (collector *Collector) pruneExpiredRateWindowsLocked(now time.Time) {
	for key, window := range collector.rateWindows {
		if !now.Before(window.reset) {
			delete(collector.rateWindows, key)
		}
	}
}

func collectorRateKey(request *http.Request) string {
	if request == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if request.RemoteAddr != "" {
		return request.RemoteAddr
	}
	return "unknown"
}

func (collector *Collector) recordJSON(ctx context.Context, request *http.Request) error {
	defer func() {
		_ = request.Body.Close()
	}()
	payload, err := io.ReadAll(io.LimitReader(request.Body, maxCollectorBodyBytes+1))
	if err != nil {
		return err
	}
	if len(payload) > maxCollectorBodyBytes {
		return errTracePayloadTooLarge
	}
	spans, err := decodeSnapshotPayload(payload)
	if err != nil {
		return err
	}
	for _, span := range spans {
		if err := validateSnapshot(span); err != nil {
			return err
		}
	}
	for _, span := range spans {
		if err := collector.RecordSpan(ctx, span); err != nil {
			return err
		}
	}
	return nil
}

func decodeSnapshotPayload(payload []byte) ([]Snapshot, error) {
	var spans []Snapshot
	if err := decodeStrict(payload, &spans); err == nil {
		if len(spans) == 0 {
			return nil, fmt.Errorf("empty trace batch")
		}
		if len(spans) > maxCollectorBatchSpans {
			return nil, fmt.Errorf("trace batch exceeds %d spans", maxCollectorBatchSpans)
		}
		return spans, nil
	}
	var span Snapshot
	if err := decodeStrict(payload, &span); err != nil {
		return nil, err
	}
	return []Snapshot{span}, nil
}

func decodeStrict(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("trace payload contains trailing data")
	}
	return nil
}

func (collector *Collector) serveSSE(response http.ResponseWriter, request *http.Request) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	events := make(chan Snapshot, 16)
	collector.mu.Lock()
	if collector.sseLimit > 0 && len(collector.subscribers) >= collector.sseLimit {
		collector.mu.Unlock()
		collector.rejectHTTP(response, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	collector.subscribers[events] = struct{}{}
	collector.mu.Unlock()
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	defer func() {
		collector.mu.Lock()
		delete(collector.subscribers, events)
		collector.mu.Unlock()
		close(events)
	}()
	for _, span := range collector.Spans() {
		if !writeSSE(response, flusher, span) {
			return
		}
	}
	for {
		select {
		case <-request.Context().Done():
			return
		case span := <-events:
			if !writeSSE(response, flusher, span) {
				return
			}
		}
	}
}

func writeSSE(response http.ResponseWriter, flusher http.Flusher, span Snapshot) bool {
	payload, err := json.Marshal(span)
	if err != nil {
		return true
	}
	if _, err := fmt.Fprintf(response, "event: gowdk-trace\ndata: %s\n\n", payload); err != nil {
		return false
	}
	flusher.Flush()
	return true
}
