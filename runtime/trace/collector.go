package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync"
)

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
<header><h1>GOWDK Trace</h1><div class="meta"><span id="count">0</span> spans · <span id="dropped">0</span> dropped</div></header>
<main><section id="list"><div class="empty">Waiting for spans...</div></section><section id="detail"><pre id="json">{}</pre></section></main>
<script>
(() => {
  const spans = [];
  const list = document.getElementById("list");
  const detail = document.getElementById("json");
  const count = document.getElementById("count");
  const dropped = document.getElementById("dropped");
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
  fetch("data").then((response) => response.json()).then((payload) => {
    dropped.textContent = String(payload.dropped || 0);
    (payload.spans || []).forEach((span) => spans.push(span));
    render();
  }).catch(() => {});
  const source = new EventSource("events");
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
	ring        *RingSink
	mu          sync.Mutex
	subscribers map[chan Snapshot]struct{}
}

// NewCollector creates a collector backed by a RingSink.
func NewCollector(limit int) *Collector {
	return &Collector{
		ring:        NewRingSink(limit),
		subscribers: map[chan Snapshot]struct{}{},
	}
}

// RecordSpan implements Sink.
func (collector *Collector) RecordSpan(ctx context.Context, span Snapshot) error {
	if collector == nil {
		return nil
	}
	if err := collector.ring.RecordSpan(ctx, span); err != nil {
		return err
	}
	collector.mu.Lock()
	for subscriber := range collector.subscribers {
		select {
		case subscriber <- span:
		default:
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

// Dropped returns the ring overflow count.
func (collector *Collector) Dropped() uint64 {
	if collector == nil {
		return 0
	}
	return collector.ring.Dropped()
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
			if err := collector.recordJSON(request.Context(), request); err != nil {
				http.Error(response, err.Error(), http.StatusBadRequest)
				return
			}
			response.WriteHeader(http.StatusNoContent)
			return
		}
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if request.URL.Path == "/events" || strings.Contains(request.Header.Get("Accept"), "text/event-stream") {
			collector.serveSSE(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(struct {
			Spans   []Snapshot `json:"spans"`
			Dropped uint64     `json:"dropped"`
		}{Spans: collector.Spans(), Dropped: collector.Dropped()})
	})
}

// ViewerHandler serves a self-contained local trace viewer plus JSON and SSE
// endpoints. It is intentionally unprotected; generated apps must mount it
// behind an access gate when enabling it outside dev.
func (collector *Collector) ViewerHandler() http.Handler {
	if collector == nil {
		collector = NewCollector(defaultRingLimit)
	}
	api := collector.Handler()
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch strings.Trim(request.URL.Path, "/") {
		case "":
			if request.Method != http.MethodGet {
				response.Header().Set("Allow", http.MethodGet)
				http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = viewerTemplate.Execute(response, nil)
		case "data":
			cloned := request.Clone(request.Context())
			cloned.URL.Path = "/"
			api.ServeHTTP(response, cloned)
		case "events":
			cloned := request.Clone(request.Context())
			cloned.URL.Path = "/events"
			api.ServeHTTP(response, cloned)
		case "browser":
			cloned := request.Clone(request.Context())
			cloned.URL.Path = "/"
			api.ServeHTTP(response, cloned)
		default:
			http.NotFound(response, request)
		}
	})
}

func (collector *Collector) recordJSON(ctx context.Context, request *http.Request) error {
	defer request.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		return err
	}
	var spans []Snapshot
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&spans); err == nil {
		for _, span := range spans {
			if err := collector.RecordSpan(ctx, span); err != nil {
				return err
			}
		}
		return nil
	}
	var span Snapshot
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&span); err != nil {
		return err
	}
	if !span.TraceID.Valid() || !span.SpanID.Valid() {
		return fmt.Errorf("invalid trace payload")
	}
	return collector.RecordSpan(ctx, span)
}

func (collector *Collector) serveSSE(response http.ResponseWriter, request *http.Request) {
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	events := make(chan Snapshot, 16)
	collector.mu.Lock()
	collector.subscribers[events] = struct{}{}
	collector.mu.Unlock()
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
