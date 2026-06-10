package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
		switch {
		case arg == "--dir":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("usage: gowdk serve --dir <dir> [--addr <addr>]")
			}
			dir = args[i]
		case strings.HasPrefix(arg, "--dir="):
			dir = strings.TrimPrefix(arg, "--dir=")
		case arg == "--addr":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("usage: gowdk serve --dir <dir> [--addr <addr>]")
			}
			addr = args[i]
		case strings.HasPrefix(arg, "--addr="):
			addr = strings.TrimPrefix(arg, "--addr=")
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
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		filePath, ok := outputFilePath(root, request.URL.Path)
		if !ok {
			http.NotFound(w, request)
			return
		}
		http.ServeFile(w, request, filePath)
	})
}

func liveReloadFileHandler(root string, reload *liveReloadBroker) http.Handler {
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
		filePath, ok := outputFilePath(root, request.URL.Path)
		if !ok || filepath.Ext(filePath) != ".html" {
			files.ServeHTTP(w, request)
			return
		}
		payload, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(injectLiveReloadScript(payload))
	})
}

func injectLiveReloadScript(html []byte) []byte {
	const script = `<script>
(() => {
  const overlayID = "__gowdk-error-overlay";
  const removeOverlay = () => {
    const current = document.getElementById(overlayID);
    if (current) current.remove();
  };
  const showOverlay = (message) => {
    let overlay = document.getElementById(overlayID);
    if (!overlay) {
      overlay = document.createElement("div");
      overlay.id = overlayID;
      overlay.setAttribute("role", "alert");
      overlay.style.cssText = "position:fixed;inset:0;z-index:2147483647;background:rgba(24,24,27,.96);color:#fff;font:14px/1.5 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;padding:24px;overflow:auto;white-space:pre-wrap;";
      document.body.appendChild(overlay);
    }
    overlay.textContent = "GOWDK build failed\n\n" + message;
  };
  const events = new EventSource("/__gowdk/reload");
  events.addEventListener("reload", () => {
    removeOverlay();
    window.location.reload();
  });
  events.addEventListener("build-error", (event) => showOverlay(event.data || "Check the terminal for details."));
})();
</script>`
	lower := strings.ToLower(string(html))
	index := strings.LastIndex(lower, "</body>")
	if index < 0 {
		out := make([]byte, 0, len(html)+len(script))
		out = append(out, html...)
		out = append(out, script...)
		return out
	}
	out := make([]byte, 0, len(html)+len(script))
	out = append(out, html[:index]...)
	out = append(out, script...)
	out = append(out, html[index:]...)
	return out
}

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

func outputFilePath(root, requestPath string) (string, bool) {
	clean := path.Clean("/" + requestPath)
	candidates := []string{clean}
	if strings.HasSuffix(requestPath, "/") {
		candidates = []string{path.Join(clean, "index.html")}
	} else if path.Ext(clean) == "" {
		candidates = append(candidates, path.Join(clean, "index.html"))
	}

	for _, candidate := range candidates {
		filePath, ok := outputCandidatePath(root, candidate)
		if ok {
			return filePath, true
		}
	}
	return "", false
}

func outputCandidatePath(root, candidate string) (string, bool) {
	rel := strings.TrimPrefix(path.Clean("/"+candidate), "/")
	filePath := filepath.Join(root, filepath.FromSlash(rel))
	relative, err := filepath.Rel(root, filePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		indexPath := filepath.Join(filePath, "index.html")
		if indexInfo, err := os.Stat(indexPath); err == nil && !indexInfo.IsDir() {
			return indexPath, true
		}
		return "", false
	}
	return filePath, true
}
