package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/runtime/ratelimit"
)

const siteOutputDir = "dist/site"

func main() {
	root, err := siteRoot(siteOutputDir)
	if err != nil {
		log.Fatal(err)
	}

	addr := listenAddress()
	handler, err := rateLimitedHandler(staticHandler(root))
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	log.Printf("serving GOWDK page at http://%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func siteRoot(outputDir string) (fs.FS, error) {
	root := os.DirFS(outputDir)
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return nil, fmt.Errorf("generated site output missing at %q: run the docs-site build before starting the server: %w", outputDir, err)
	}
	return root, nil
}

func listenAddress() string {
	if addr := strings.TrimSpace(os.Getenv("GOWDK_ADDR")); addr != "" {
		return addr
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return "0.0.0.0:" + port
	}
	return "127.0.0.1:8080"
}

func rateLimitedHandler(next http.Handler) (http.Handler, error) {
	limit, err := intEnv("GOWDK_RATE_LIMIT", 240)
	if err != nil {
		return nil, err
	}
	window, err := durationEnv("GOWDK_RATE_WINDOW", time.Minute)
	if err != nil {
		return nil, err
	}
	limiter, err := ratelimit.New(ratelimit.Options{
		Limit:  limit,
		Window: window,
		Store:  ratelimit.NewInMemoryStore(ratelimit.InMemoryOptions{}),
	})
	if err != nil {
		return nil, err
	}
	return limiter.Middleware(next), nil
}

func intEnv(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

type siteServer struct {
	root fs.FS
}

func staticHandler(root fs.FS) http.Handler {
	return &siteServer{root: root}
}

func (server *siteServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	serveStaticFile(writer, request, server.root)
}

func serveStaticFile(writer http.ResponseWriter, request *http.Request, root fs.FS) {
	filePath, redirectPath, ok := resolveStaticPath(root, request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if redirectPath != "" {
		http.Redirect(writer, request, redirectPath, http.StatusFound)
		return
	}

	payload, err := fs.ReadFile(root, filePath)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	info, err := fs.Stat(root, filePath)
	if err != nil {
		http.NotFound(writer, request)
		return
	}

	if strings.HasSuffix(filePath, ".html") {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(writer, request, info.Name(), info.ModTime(), bytes.NewReader(payload))
		return
	}

	if contentType := mime.TypeByExtension(path.Ext(filePath)); contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(writer, request, info.Name(), info.ModTime(), bytes.NewReader(payload))
}

// resolveStaticPath maps a request path to a generated site file. The site root
// serves the documentation home, and extensionless paths resolve to their
// directory index.
func resolveStaticPath(root fs.FS, requestPath string) (filePath, redirectPath string, ok bool) {
	cleaned := path.Clean("/" + requestPath)
	if cleaned == "/" {
		return "index.html", "", true
	}

	name := strings.TrimPrefix(cleaned, "/")
	info, err := fs.Stat(root, name)
	if err == nil && info.IsDir() {
		if !strings.HasSuffix(requestPath, "/") {
			return "", cleaned + "/", true
		}
		indexPath := path.Join(name, "index.html")
		if _, err := fs.Stat(root, indexPath); err == nil {
			return indexPath, "", true
		}
		return "", "", false
	}
	if err == nil {
		return name, "", true
	}

	if path.Ext(name) == "" {
		indexPath := path.Join(name, "index.html")
		if _, err := fs.Stat(root, indexPath); err == nil {
			return indexPath, "", true
		}
	}
	return "", "", false
}
