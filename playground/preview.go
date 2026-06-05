package playground

import (
	"crypto/sha256"
	"encoding/hex"
	"mime"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const defaultPreviewCacheEntries = 32

// PreviewOptions configures compiler-owned browser preview rendering.
type PreviewOptions struct {
	AssetPathPrefix string
	ActionPath      string
	MaxCachedBuilds int
}

// Preview is a browser-ready view of a compiled playground result.
type Preview struct {
	HTMLPath string
	HTML     string
	Token    string
}

// PreviewServer renders preview HTML from compiler output and serves the
// generated assets that preview HTML references.
type PreviewServer struct {
	assetPathPrefix string
	actionPath      string
	maxCachedBuilds int

	mu      sync.RWMutex
	entries map[string]map[string]string
	order   []string
}

// NewPreviewServer creates a preview server for Result values returned by
// Compile.
func NewPreviewServer(options PreviewOptions) *PreviewServer {
	prefix := strings.TrimSpace(options.AssetPathPrefix)
	if prefix == "" {
		prefix = "/playground/assets/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	actionPath := strings.TrimSpace(options.ActionPath)
	if actionPath == "" {
		actionPath = "/playground/preview-post"
	}

	maxCachedBuilds := options.MaxCachedBuilds
	if maxCachedBuilds <= 0 {
		maxCachedBuilds = defaultPreviewCacheEntries
	}

	return &PreviewServer{
		assetPathPrefix: prefix,
		actionPath:      actionPath,
		maxCachedBuilds: maxCachedBuilds,
		entries:         map[string]map[string]string{},
	}
}

// Render returns preview HTML for the first emitted HTML artifact and registers
// generated assets so ServeHTTP can return them.
func (server *PreviewServer) Render(result Result) Preview {
	paths := sortedResultPaths(result.HTML)
	if len(paths) == 0 {
		return Preview{HTML: "<p>No HTML output.</p>"}
	}

	token := server.store(result.Files)
	htmlPath := paths[0]
	return Preview{
		HTMLPath: htmlPath,
		HTML:     server.prepareHTML(result.HTML[htmlPath], result.Files, token),
		Token:    token,
	}
}

func (server *PreviewServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token, filePath, ok := server.assetPath(request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	content, ok := server.asset(token, filePath)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if contentType := mime.TypeByExtension(path.Ext(filePath)); contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}
	if request.Method == http.MethodHead {
		return
	}
	_, _ = writer.Write([]byte(content))
}

func (server *PreviewServer) MatchesAssetPath(requestPath string) bool {
	return strings.HasPrefix(requestPath, server.assetPathPrefix)
}

func (server *PreviewServer) store(files map[string]string) string {
	token := previewToken(files)
	if token == "" {
		return ""
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if _, ok := server.entries[token]; !ok {
		server.order = append(server.order, token)
	}
	server.entries[token] = files
	for len(server.order) > server.maxCachedBuilds {
		oldest := server.order[0]
		server.order = server.order[1:]
		delete(server.entries, oldest)
	}
	return token
}

func (server *PreviewServer) asset(token, filePath string) (string, bool) {
	server.mu.RLock()
	defer server.mu.RUnlock()
	files, ok := server.entries[token]
	if !ok {
		return "", false
	}
	content, ok := files[filePath]
	return content, ok
}

func (server *PreviewServer) assetPath(requestPath string) (string, string, bool) {
	rest := strings.TrimPrefix(requestPath, server.assetPathPrefix)
	token, filePath, ok := strings.Cut(rest, "/")
	if !ok || token == "" || !validPreviewAssetPath(filePath) {
		return "", "", false
	}
	return token, filePath, true
}

func (server *PreviewServer) prepareHTML(pageHTML string, generatedFiles map[string]string, token string) string {
	out := rewritePreviewForms(pageHTML, server.actionPath)
	if token == "" {
		return out
	}
	for _, filePath := range sortedResultPaths(generatedFiles) {
		if !validPreviewAssetPath(filePath) {
			continue
		}
		out = rewritePreviewAssetURL(out, filePath, server.assetURL(token, filePath))
	}
	return out
}

func (server *PreviewServer) assetURL(token, filePath string) string {
	escapedPath := strings.TrimLeft(path.Clean("/"+filePath), "/")
	return server.assetPathPrefix + token + "/" + escapedPath
}

func previewToken(files map[string]string) string {
	paths := sortedResultPaths(files)
	if len(paths) == 0 {
		return ""
	}

	digest := sha256.New()
	for _, filePath := range paths {
		digest.Write([]byte(strconv.Itoa(len(filePath))))
		digest.Write([]byte{0})
		digest.Write([]byte(filePath))
		digest.Write([]byte{0})
		digest.Write([]byte(strconv.Itoa(len(files[filePath]))))
		digest.Write([]byte{0})
		digest.Write([]byte(files[filePath]))
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))[:24]
}

func sortedResultPaths(files map[string]string) []string {
	paths := make([]string, 0, len(files))
	for filePath := range files {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	return paths
}

func validPreviewAssetPath(filePath string) bool {
	return strings.HasPrefix(filePath, "assets/") && !strings.Contains(filePath, "..")
}

var previewFormAttrPattern = regexp.MustCompile(`(?i)\s(?:action|method|target)\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
var previewFormTagPattern = regexp.MustCompile(`(?is)<form\b[^>]*>`)

func rewritePreviewForms(pageHTML, actionPath string) string {
	return previewFormTagPattern.ReplaceAllStringFunc(pageHTML, func(tag string) string {
		cleaned := previewFormAttrPattern.ReplaceAllString(tag, "")
		cleaned = strings.TrimSuffix(cleaned, ">")
		return cleaned + ` method="post" action="` + actionPath + `">`
	})
}

func rewritePreviewAssetURL(pageHTML, filePath, assetURL string) string {
	for _, prefix := range []string{"/", ""} {
		pattern := regexp.MustCompile(`(?i)(\b(?:src|href)\s*=\s*["'])` + regexp.QuoteMeta(prefix+filePath) + `(["'])`)
		pageHTML = pattern.ReplaceAllString(pageHTML, "${1}"+assetURL+"${2}")
	}
	return pageHTML
}
