package gowdkcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cssbruno/gowdk/internal/safeasset"
	gowdkroute "github.com/cssbruno/gowdk/runtime/route"
)

func serve(args []string) error {
	dir, addr, err := parseServeOptions(args)
	if err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("serve directory %q is not a directory", dir)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           outputFileHandler(absDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	fmt.Printf("Serving %s at http://%s\n", absDir, addr)
	return server.ListenAndServe()
}

func parseServeOptions(args []string) (string, string, error) {
	addr := "127.0.0.1:8080"
	var dir string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if value, next, ok, missing := consumeValueFlag(args, i, "--dir", true); ok {
			if missing {
				return "", "", fmt.Errorf("usage: gowdk serve --dir <dir> [--addr <addr>]")
			}
			dir = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--addr", true); ok {
			if missing {
				return "", "", fmt.Errorf("usage: gowdk serve --dir <dir> [--addr <addr>]")
			}
			addr = value
			i = next
			continue
		}
		switch {
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("unknown serve flag %q", arg)
		default:
			return "", "", fmt.Errorf("unexpected serve argument %q", arg)
		}
	}
	if strings.TrimSpace(dir) == "" {
		return "", "", fmt.Errorf("usage: gowdk serve --dir <dir> [--addr <addr>]")
	}
	if strings.TrimSpace(addr) == "" {
		return "", "", fmt.Errorf("serve address is required")
	}
	return dir, addr, nil
}

func outputFileHandler(root string) http.Handler {
	files, err := newRootedOutputFiles(root)
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err != nil {
			http.Error(w, "static output unavailable", http.StatusInternalServerError)
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, staticMethodNotAllowedMessage(root, request), http.StatusMethodNotAllowed)
			return
		}

		file, info, _, ok := files.Open(request.URL.Path)
		if !ok {
			http.NotFound(w, request)
			return
		}
		defer func() {
			_ = file.Close()
		}()
		http.ServeContent(w, request, info.Name(), info.ModTime(), file)
	})
}

type staticRouteManifest struct {
	Endpoints []staticRouteEndpoint `json:"endpoints"`
}

type staticRouteEndpoint struct {
	Kind      string `json:"kind"`
	Directive string `json:"directive"`
	Method    string `json:"method"`
	Route     string `json:"route"`
	PageID    string `json:"page"`
	Symbol    string `json:"symbol"`
	Handler   string `json:"handler"`
}

func staticMethodNotAllowedMessage(root string, request *http.Request) string {
	if request == nil {
		return "method not allowed"
	}
	message := "method not allowed: " + request.Method + " " + request.URL.Path
	endpoint, ok := staticEndpointForRequest(root, request)
	if !ok {
		return message
	}
	message += "; " + endpoint.Method + " " + endpoint.Route + " is a generated endpoint"
	if endpoint.Directive != "" || endpoint.Symbol != "" {
		message += " (" + strings.TrimSpace(endpoint.Directive+" "+endpoint.Symbol) + ")"
	}
	return message + "; gowdk serve only serves static GET/HEAD output; run the generated app or binary for backend endpoints"
}

func staticEndpointForRequest(root string, request *http.Request) (staticRouteEndpoint, bool) {
	manifest, ok := readStaticRouteManifest(root)
	if !ok {
		return staticRouteEndpoint{}, false
	}
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	requestPath := request.URL.Path
	for _, endpoint := range manifest.Endpoints {
		if strings.ToUpper(strings.TrimSpace(endpoint.Method)) != method {
			continue
		}
		if staticEndpointRouteMatches(endpoint.Route, requestPath) {
			return endpoint, true
		}
	}
	return staticRouteEndpoint{}, false
}

func staticEndpointRouteMatches(endpointRoute string, requestPath string) bool {
	endpointRoute = strings.TrimSpace(endpointRoute)
	if endpointRoute == requestPath {
		return true
	}
	if !strings.Contains(endpointRoute, "{") {
		return false
	}
	_, ok := gowdkroute.Match(endpointRoute, requestPath)
	return ok
}

func readStaticRouteManifest(root string) (staticRouteManifest, bool) {
	payload, err := os.ReadFile(filepath.Join(root, "gowdk-routes.json"))
	if err != nil {
		return staticRouteManifest{}, false
	}
	var manifest staticRouteManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return staticRouteManifest{}, false
	}
	return manifest, true
}

func liveReloadFileHandler(root string, reload *liveReloadBroker) http.Handler {
	rooted, rootedErr := newRootedOutputFiles(root)
	files := outputFileHandler(root)
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/__gowdk/reload" {
			reload.serve(w, request)
			return
		}
		if request.Method != http.MethodGet || request.Method == http.MethodHead {
			files.ServeHTTP(w, request)
			return
		}
		if rootedErr != nil {
			files.ServeHTTP(w, request)
			return
		}
		file, _, rel, ok := rooted.Open(request.URL.Path)
		if !ok || path.Ext(rel) != ".html" {
			files.ServeHTTP(w, request)
			return
		}
		defer func() {
			_ = file.Close()
		}()
		payload, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(injectLiveReloadScript(payload))
	})
}

func devRuntimeProxyHandler(targetAddr string, reload *liveReloadBroker) http.Handler {
	target, err := url.Parse("http://" + targetAddr)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(proxyRequest *httputil.ProxyRequest) {
			proxyRequest.SetURL(target)
			proxyRequest.SetXForwarded()
			proxyRequest.Out.Header.Del("Accept-Encoding")
		},
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		return modifyDevRuntimeProxyResponse(response, reload)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/__gowdk/reload" {
			reload.serve(w, request)
			return
		}
		proxy.ServeHTTP(w, request)
	})
}

func injectLiveReloadScript(html []byte) []byte {
	return injectLiveReloadScriptWithInitialOverlay(html, nil)
}

func injectLiveReloadScriptWithInitialOverlay(html []byte, initialOverlay []byte) []byte {
	initial := "null"
	if len(initialOverlay) > 0 {
		initial = string(initialOverlay)
	}
	script := strings.Replace(liveReloadScriptTemplate, "__GOWDK_INITIAL_OVERLAY__", initial, 1)
	capacity := liveReloadScriptCapacityHint(len(html), len(script))
	lower := strings.ToLower(string(html))
	index := strings.LastIndex(lower, "</body>")
	if index < 0 {
		out := make([]byte, 0, capacity)
		out = append(out, html...)
		out = append(out, script...)
		return out
	}
	out := make([]byte, 0, capacity)
	out = append(out, html[:index]...)
	out = append(out, script...)
	out = append(out, html[index:]...)
	return out
}

func liveReloadScriptCapacityHint(htmlLen int, scriptLen int) int {
	if htmlLen < 0 || scriptLen < 0 {
		return 0
	}
	maxInt := int(^uint(0) >> 1)
	if htmlLen > maxInt-scriptLen {
		return 0
	}
	return htmlLen + scriptLen
}

const liveReloadScriptTemplate = `<script>
(() => {
  const overlayID = "__gowdk-error-overlay";
  const parsePayload = (data) => {
    if (!data) return { message: "Check the terminal for details." };
    try {
      const parsed = JSON.parse(data);
      if (parsed && typeof parsed === "object") return parsed;
    } catch (_) {}
    return { message: data };
  };
  const formatPosition = (position) => {
    if (!position || !position.line || !position.column) return "";
    return position.line + ":" + position.column;
  };
  const formatRange = (range) => {
    if (!range) return "";
    const start = formatPosition(range.start);
    const end = formatPosition(range.end);
    if (!start) return "";
    if (!end || end === start) return start;
    return start + "-" + end;
  };
  const formatTime = (value) => {
    if (!value) return "";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString();
  };
  const DEV_UPDATE_VERSION = 1;
  const selectorValue = (value) => String(value || "").replace(/\\/g, "\\\\").replace(/"/g, "\\\"");
  const normalizeDevUpdate = (payload, fallbackAction) => {
    payload = typeof payload === "string" ? parsePayload(payload) : (payload || {});
    if (!payload.version) payload.version = DEV_UPDATE_VERSION;
    if (!payload.action && fallbackAction) payload.action = fallbackAction;
    return payload;
  };
  const formatDiagnostic = (diagnostic) => {
    const tags = [];
    if (diagnostic.code) tags.push("[" + diagnostic.code + "]");
    if (diagnostic.severity) tags.push(diagnostic.severity);
    if (diagnostic.file) {
      const range = formatRange(diagnostic.range);
      tags.push(diagnostic.file + (range ? ":" + range : ""));
    }
    if (diagnostic.route) tags.push("route " + diagnostic.route);
    if (diagnostic.endpoint) tags.push("endpoint " + diagnostic.endpoint);
    if (diagnostic.pageId) tags.push("page " + diagnostic.pageId);
    if (diagnostic.component) tags.push("component " + diagnostic.component);
    return "- " + (tags.length ? tags.join(" ") + ": " : "") + (diagnostic.message || "diagnostic");
  };
  const addSection = (lines, title, values) => {
    const list = Array.isArray(values) ? values.filter(Boolean) : [];
    if (list.length === 0) return;
    if (lines.length > 0 && lines[lines.length - 1] !== "") lines.push("");
    lines.push(title);
    for (const value of list) lines.push(value);
  };
  const removeOverlay = () => {
    const current = document.getElementById(overlayID);
    if (current) current.remove();
  };
  const showOverlay = (payload) => {
    payload = typeof payload === "string" ? parsePayload(payload) : (payload || {});
    let overlay = document.getElementById(overlayID);
    if (!overlay) {
      overlay = document.createElement("div");
      overlay.id = overlayID;
      overlay.setAttribute("role", "alert");
      overlay.style.cssText = "position:fixed;inset:0;z-index:2147483647;background:rgba(24,24,27,.96);color:#fff;font:14px/1.5 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;padding:24px;overflow:auto;white-space:pre-wrap;";
      document.body.appendChild(overlay);
    }
    const lines = [payload.title || "GOWDK build failed", ""];
    lines.push(payload.message || "Check the terminal for details.");
    addSection(lines, "Status", [payload.status ? "HTTP " + payload.status : ""]);
    addSection(lines, "Diagnostics", (payload.diagnostics || []).map(formatDiagnostic));
    addSection(lines, "Last successful build", [formatTime(payload.lastSuccessfulBuild)]);
    addSection(lines, "Changed files", payload.changedFiles || []);
    addSection(lines, "Runtime attribution", [payload.route ? "route " + payload.route : "", payload.endpoint ? "endpoint " + payload.endpoint : ""]);
    overlay.textContent = lines.join("\n").trimEnd();
  };
  const pathMatchesRoute = (route, pathname) => {
    route = String(route || "");
    pathname = String(pathname || "/");
    if (!route) return false;
    if (route === pathname) return true;
    const routeParts = route.split("/").filter(Boolean);
    const pathParts = pathname.split("/").filter(Boolean);
    let index = 0;
    for (; index < routeParts.length; index++) {
      const part = routeParts[index];
      if (/^\{[^}]+\.\.\.\}$/.test(part)) return index < pathParts.length;
      if (index >= pathParts.length) return false;
      if (/^\{[^}]+\}$/.test(part)) continue;
      if (part !== pathParts[index]) return false;
    }
    return index === pathParts.length;
  };
  const componentSelectors = (component) => {
    const id = selectorValue(component && component.id);
    const name = selectorValue(component && component.name);
    const selectors = [];
    if (id) selectors.push('gowdk-island[data-gowdk-component-id="' + id + '"]');
    if (name) selectors.push('gowdk-island:not([data-gowdk-component-id])[data-gowdk-component="' + name + '"]');
    return selectors.join(",");
  };
  const fetchFreshDocument = async () => {
    const url = new URL(window.location.href);
    url.searchParams.set("__gowdk_hmr", Date.now().toString());
    const response = await fetch(url.href, { headers: { "Accept": "text/html", "X-GOWDK-HMR": "1" }, cache: "no-store" });
    if (!response.ok) throw new Error("HMR document fetch failed with status " + response.status);
    const type = response.headers.get("Content-Type") || "";
    if (type && !type.includes("text/html")) throw new Error("HMR document fetch did not return HTML");
    return new DOMParser().parseFromString(await response.text(), "text/html");
  };
  const applyComponentHMR = async (payload) => {
    payload = typeof payload === "string" ? parsePayload(payload) : (payload || {});
    if (payload.version && payload.version !== DEV_UPDATE_VERSION) {
      window.location.reload();
      return;
    }
    const components = Array.isArray(payload.components) ? payload.components : [];
    const routes = Array.isArray(payload.routes) ? payload.routes : [];
    const routeAffected = routes.some((route) => pathMatchesRoute(route, window.location.pathname));
    const hasCurrentRoot = components.some((component) => {
      const selector = componentSelectors(component);
      return selector && document.querySelector(selector);
    });
    if (!routeAffected && !hasCurrentRoot) return;
    let next;
    try {
      next = await fetchFreshDocument();
    } catch (_) {
      window.location.reload();
      return;
    }
    let replaced = 0;
    for (const component of components) {
      const selector = componentSelectors(component);
      if (!selector) continue;
      const currentRoots = Array.from(document.querySelectorAll(selector));
      const nextRoots = Array.from(next.querySelectorAll(selector));
      if (currentRoots.length === 0 || nextRoots.length < currentRoots.length) continue;
      currentRoots.forEach((root, index) => {
        const replacement = nextRoots[index] && nextRoots[index].cloneNode(true);
        if (!replacement || !root.parentNode) return;
        if (typeof window.__gowdkDestroyIslands === "function") window.__gowdkDestroyIslands(root, true);
        root.parentNode.replaceChild(replacement, root);
        replaced++;
      });
    }
    if (replaced === 0) {
      window.location.reload();
      return;
    }
    if (window.__gowdkStores && typeof window.__gowdkStores.hydrate === "function") window.__gowdkStores.hydrate();
    if (typeof window.__gowdkMountIslands === "function") window.__gowdkMountIslands();
    if (typeof window.__gowdkMountClientGoBlocks === "function") window.__gowdkMountClientGoBlocks();
    document.dispatchEvent(new CustomEvent("gowdk:component-hmr", { detail: payload }));
  };
  const applyDevUpdate = async (payload) => {
    payload = normalizeDevUpdate(payload, "reload");
    removeOverlay();
    document.dispatchEvent(new CustomEvent("gowdk:dev-update", { detail: payload }));
    if (payload.version !== DEV_UPDATE_VERSION) {
      window.location.reload();
      return;
    }
    if (payload.action === "component-remount") {
      await applyComponentHMR(payload);
      return;
    }
    if (payload.action === "reload") {
      const routes = Array.isArray(payload.routes) ? payload.routes : [];
      if (routes.length > 0 && !routes.some((route) => pathMatchesRoute(route, window.location.pathname))) return;
      window.location.reload();
      return;
    }
    window.location.reload();
  };
  const events = new EventSource("/__gowdk/reload");
  events.addEventListener("reload", () => {
    applyDevUpdate({ version: DEV_UPDATE_VERSION, action: "reload", reason: "legacy-reload" });
  });
  events.addEventListener("build-error", (event) => showOverlay(parsePayload(event.data)));
  events.addEventListener("runtime-error", (event) => showOverlay(parsePayload(event.data)));
  events.addEventListener("dev-update", (event) => {
    applyDevUpdate(parsePayload(event.data)).catch(() => window.location.reload());
  });
  events.addEventListener("component-hmr", (event) => {
    applyDevUpdate(normalizeDevUpdate(parsePayload(event.data), "component-remount")).catch(() => window.location.reload());
  });
  const initialOverlay = __GOWDK_INITIAL_OVERLAY__;
  if (initialOverlay) showOverlay(initialOverlay);
})();
</script>`

type liveReloadBroker struct {
	mu      sync.Mutex
	clients map[chan liveReloadEvent]bool
}

func newLiveReloadBroker() *liveReloadBroker {
	return &liveReloadBroker{clients: map[chan liveReloadEvent]bool{}}
}

type liveReloadEvent struct {
	Name string
	Data string
}

func (broker *liveReloadBroker) notify(event string) {
	broker.notifyData(event, fmt.Sprint(time.Now().UnixMilli()))
}

// notifyData is a no-op on a nil broker so callers without live reload
// (for example dev runtime mode) can skip wiring a broker entirely.
func (broker *liveReloadBroker) notifyData(event string, data string) {
	if broker == nil {
		return
	}
	broker.mu.Lock()
	defer broker.mu.Unlock()
	for client := range broker.clients {
		select {
		case client <- liveReloadEvent{Name: event, Data: data}:
		default:
		}
	}
}

func (broker *liveReloadBroker) serve(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	client := make(chan liveReloadEvent, 4)
	broker.mu.Lock()
	broker.clients[client] = true
	broker.mu.Unlock()
	defer func() {
		broker.mu.Lock()
		delete(broker.clients, client)
		broker.mu.Unlock()
		close(client)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	_, _ = fmt.Fprintln(w, "event: ready")
	_, _ = fmt.Fprintln(w, "data: connected")
	_, _ = fmt.Fprintln(w)
	flusher.Flush()

	for {
		select {
		case <-request.Context().Done():
			return
		case event := <-client:
			writeLiveReloadEvent(w, event)
			flusher.Flush()
		}
	}
}

func writeLiveReloadEvent(w io.Writer, event liveReloadEvent) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Name)
	for _, line := range strings.Split(event.Data, "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprintln(w)
}

type rootedOutputFiles struct {
	root *os.Root
}

func newRootedOutputFiles(root string) (*rootedOutputFiles, error) {
	opened, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	return &rootedOutputFiles{root: opened}, nil
}

func (files *rootedOutputFiles) Open(requestPath string) (*os.File, os.FileInfo, string, bool) {
	clean := path.Clean("/" + requestPath)
	candidates := []string{clean}
	if strings.HasSuffix(requestPath, "/") {
		candidates = []string{path.Join(clean, "index.html")}
	} else if path.Ext(clean) == "" {
		candidates = append(candidates, path.Join(clean, "index.html"))
	}

	for _, candidate := range candidates {
		file, info, rel, ok := files.openCandidate(candidate)
		if ok {
			return file, info, rel, true
		}
	}
	return nil, nil, "", false
}

func (files *rootedOutputFiles) openCandidate(candidate string) (*os.File, os.FileInfo, string, bool) {
	rel := strings.TrimPrefix(path.Clean("/"+candidate), "/")
	if rel == "" {
		rel = "index.html"
	}
	file, info, ok := files.openPublicRegularFile(rel)
	if ok {
		return file, info, rel, true
	}
	indexRel := path.Join(rel, "index.html")
	file, info, ok = files.openPublicRegularFile(indexRel)
	if ok {
		return file, info, indexRel, true
	}
	return nil, nil, "", false
}

func (files *rootedOutputFiles) openPublicRegularFile(rel string) (*os.File, os.FileInfo, bool) {
	rel = strings.TrimPrefix(path.Clean("/"+rel), "/")
	if !safeasset.PublicGeneratedOutputFile(rel) || files.pathContainsLink(rel) {
		return nil, nil, false
	}
	file, err := files.root.Open(rel)
	if err != nil {
		return nil, nil, false
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, false
	}
	return file, info, true
}

func (files *rootedOutputFiles) pathContainsLink(rel string) bool {
	rel = strings.Trim(path.Clean("/"+rel), "/")
	if rel == "" {
		return false
	}
	var current string
	for _, segment := range strings.Split(rel, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return true
		}
		current = path.Join(current, segment)
		info, err := files.root.Lstat(current)
		if err != nil {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}
