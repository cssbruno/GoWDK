package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/runtime/asset"
	"github.com/cssbruno/gowdk/runtime/route"
	gowdktrace "github.com/cssbruno/gowdk/runtime/trace"
)

// HandlerFunc handles a generated request-time route and reports whether it
// wrote a response.
type HandlerFunc func(http.ResponseWriter, *http.Request) bool

// Middleware wraps the generated app handler chain.
type Middleware func(http.Handler) http.Handler

// CSRFTokenSource generates tokens for generated action forms.
type CSRFTokenSource interface {
	Token(http.ResponseWriter, *http.Request) (string, error)
	FieldName() string
}

// Identity describes one running generated app instance.
type Identity struct {
	AppID      string
	ModuleName string
	InstanceID string
}

// Handler serves embedded generated output plus optional action and SSR hooks.
type Handler struct {
	Root            fs.FS
	Identity        Identity
	SecurityHeaders map[string]string
	Middlewares     []Middleware
	Assets          asset.Manifest
	Backend         HandlerFunc
	Action          HandlerFunc
	API             HandlerFunc
	CSRF            CSRFTokenSource
	ErrorPages      ErrorPages
	Metrics         *Metrics
	Tracer          *gowdktrace.Tracer
	TraceHandler    http.Handler
	TraceAccess     TraceAccess
	SSRExact        HandlerFunc
	SSRDynamic      HandlerFunc

	// Denied holds concrete page routes that declared no guard. Such a page is
	// not public by default: its GET/HEAD route returns 403 until the author
	// opts in with guard public (or a protective guard for request-time pages).
	// Keyed by exact route path.
	Denied map[string]bool

	// DeniedPatterns holds route patterns (e.g. /blog/{slug}) for guardless
	// build-time pages whose paths {} block expands to many concrete artifacts.
	// The exact Denied map cannot enumerate those concrete paths, so the request
	// path is matched against each pattern to deny every expanded instance.
	DeniedPatterns []string

	// RequestTimeout bounds how long a single request's handler context lives.
	// When > 0, the request context is cancelled after the deadline so slow
	// user Go (actions, contracts, SSR) sees ctx.Done() instead of running
	// unbounded and pinning a goroutine. Zero disables the deadline.
	RequestTimeout time.Duration
}

// DefaultRequestTimeout is the per-request handler deadline applied to
// generated apps. It sits below the server WriteTimeout (30s) so handler
// context cancellation fires before the connection write deadline.
const DefaultRequestTimeout = 25 * time.Second

// InstanceIdentity reads GOWDK identity settings from the environment.
func InstanceIdentity() Identity {
	appID := env("GOWDK_APP_ID", "app")
	moduleName := env("GOWDK_MODULE_NAME", "app")
	instanceID := env("GOWDK_INSTANCE_ID", "")
	if instanceID == "" {
		instanceID = generatedInstanceID(moduleName)
	}

	return Identity{
		AppID:      appID,
		ModuleName: moduleName,
		InstanceID: instanceID,
	}
}

// LoadAssetManifest reads gowdk-assets.json from generated app output.
func LoadAssetManifest(root fs.FS) asset.Manifest {
	var manifest asset.Manifest
	payload, err := fs.ReadFile(root, "gowdk-assets.json")
	if err != nil {
		return asset.Manifest{Version: asset.ManifestVersion, Files: map[string]string{}}
	}
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return asset.Manifest{Version: asset.ManifestVersion, Files: map[string]string{}}
	}
	if manifest.Files == nil {
		manifest.Files = map[string]string{}
	}
	return manifest
}

// ApplyMiddlewares wraps next with middlewares in declaration order. The first
// middleware in the slice runs first for each request.
func ApplyMiddlewares(next http.Handler, middlewares ...Middleware) http.Handler {
	if next == nil {
		next = middlewareErrorHandler("gowdk middleware chain missing handler")
	}
	for index := len(middlewares) - 1; index >= 0; index-- {
		middleware := middlewares[index]
		if middleware == nil {
			continue
		}
		wrapped := middleware(next)
		if wrapped == nil {
			next = middlewareErrorHandler("gowdk middleware returned nil handler")
			continue
		}
		next = wrapped
	}
	return next
}

func middlewareErrorHandler(message string) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Error(response, message, http.StatusInternalServerError)
	})
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if handler == nil {
		middlewareErrorHandler("gowdk handler is nil").ServeHTTP(response, request)
		return
	}
	handler.middlewareChain().ServeHTTP(response, request)
}

func (handler *Handler) middlewareChain() http.Handler {
	next := http.Handler(http.HandlerFunc(handler.serveHTTP))
	if len(handler.Middlewares) > 0 {
		next = ApplyMiddlewares(next, handler.Middlewares...)
	}
	return next
}

func (handler *Handler) serveHTTP(response http.ResponseWriter, request *http.Request) {
	metrics := handler.Metrics
	metricsStart := metrics.startRequest()
	var metricsRecorder *traceResponseWriter
	if metrics != nil {
		metricsRecorder = &traceResponseWriter{ResponseWriter: response, status: http.StatusOK}
		response = wrapTraceResponseWriter(metricsRecorder)
		defer func() { metrics.finishRequest(metricsStart, metricsRecorder.status) }()
		request = request.WithContext(contextWithMetrics(request.Context(), metrics))
	}
	if handler.RequestTimeout > 0 {
		ctx, cancel := context.WithTimeout(request.Context(), handler.RequestTimeout)
		defer cancel()
		request = request.WithContext(ctx)
	}
	handler.writeSecurityHeaders(response)
	handler.writeIdentityHeaders(response)
	if handler.TraceHandler != nil && isTracePath(request.URL.Path) {
		if !handler.traceAccessAllowed(request) {
			http.NotFound(response, request)
			return
		}
		http.StripPrefix(tracePathPrefix, handler.TraceHandler).ServeHTTP(response, request)
		return
	}
	if handler.Tracer != nil {
		var span *gowdktrace.Span
		response, request, span = handler.startRequestTrace(response, request)
		defer finishRequestTrace(response, span)
	}
	if len(handler.ErrorPages.NotFound) > 0 || len(handler.ErrorPages.InternalServerError) > 0 || len(handler.ErrorPages.Custom) > 0 {
		request = request.WithContext(withErrorPages(request.Context(), handler.ErrorPages))
	}
	if request.Method == http.MethodPost && isCookieAckPath(request.URL.Path) {
		metrics.recordCookieAck()
		acknowledgeCookie(response, request)
		return
	}
	if request.URL.Path == "/_gowdk/health" {
		metrics.recordHealth()
		handler.health(response)
		return
	}
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		if canonical, ok := canonicalTrailingSlashPath(request.URL.Path); ok {
			location := canonical
			if request.URL.RawQuery != "" {
				location += "?" + request.URL.RawQuery
			}
			http.Redirect(response, request, location, http.StatusPermanentRedirect)
			return
		}
	}
	if handler.Backend != nil && Boundary("backend", handler.Backend)(response, request) {
		metrics.recordBackend()
		return
	}
	if handler.API != nil && Boundary("api", handler.API)(response, request) {
		metrics.recordAPI()
		return
	}
	if request.Method == http.MethodPost && handler.Action != nil && Boundary("action", handler.Action)(response, request) {
		metrics.recordAction()
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		metrics.recordMethodNotAllowed()
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, handler.methodNotAllowedMessage(request), http.StatusMethodNotAllowed)
		return
	}
	if handler.isDeniedPath(request.URL.Path) {
		metrics.recordForbidden()
		WriteErrorPage(response, request, http.StatusForbidden, "403 forbidden")
		return
	}
	if handler.SSRExact != nil && Boundary("ssr", handler.SSRExact)(response, request) {
		metrics.recordSSRExact()
		return
	}

	payload, info, assetName, ok := handler.SPAFile(request.URL.Path)
	if !ok {
		if handler.SSRDynamic != nil && Boundary("ssr", handler.SSRDynamic)(response, request) {
			metrics.recordSSRDynamic()
			return
		}
		metrics.recordNotFound()
		WriteErrorPage(response, request, http.StatusNotFound, "404 page not found")
		return
	}
	payload = handler.cookieAwarePayload(request, payload, info.Name())
	var csrfOK bool
	payload, csrfOK = handler.csrfAwarePayload(response, request, payload, info.Name())
	if !csrfOK {
		metrics.recordCSRFUnavailable()
		return
	}
	metrics.recordStatic()
	handler.setGeneratedStaticCache(response, assetName)
	http.ServeContent(response, request, info.Name(), info.ModTime(), bytes.NewReader(payload))
}

type generatedRouteManifest struct {
	Routes []struct {
		PageID string `json:"page"`
		Route  string `json:"route"`
	} `json:"routes"`
	Endpoints []generatedRouteEndpoint `json:"endpoints"`
}

type generatedRouteEndpoint struct {
	Kind      string `json:"kind"`
	Directive string `json:"directive"`
	Method    string `json:"method"`
	Route     string `json:"route"`
	PageID    string `json:"page"`
	Symbol    string `json:"symbol"`
	Handler   string `json:"handler"`
}

func (handler Handler) methodNotAllowedMessage(request *http.Request) string {
	if request == nil {
		return "method not allowed"
	}
	message := "method not allowed: " + request.Method + " " + request.URL.Path
	manifest, ok := handler.loadRouteManifest()
	if !ok {
		return message
	}
	for _, route := range manifest.Routes {
		if route.Route == request.URL.Path {
			message += "; page route " + route.Route + " accepts GET, HEAD"
			break
		}
	}
	if endpoint, ok := manifest.endpointForRequest(request.Method, request.URL.Path); ok {
		message += "; generated endpoint " + endpoint.Method + " " + endpoint.Route
		if endpoint.Directive != "" || endpoint.Symbol != "" {
			message += " (" + strings.TrimSpace(endpoint.Directive+" "+endpoint.Symbol) + ")"
		}
		if endpoint.PageID != "" {
			message += " exists for page " + endpoint.PageID
		}
	}
	return message
}

func (handler Handler) loadRouteManifest() (generatedRouteManifest, bool) {
	if handler.Root == nil {
		return generatedRouteManifest{}, false
	}
	payload, err := fs.ReadFile(handler.Root, "gowdk-routes.json")
	if err != nil {
		return generatedRouteManifest{}, false
	}
	var manifest generatedRouteManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return generatedRouteManifest{}, false
	}
	return manifest, true
}

func (manifest generatedRouteManifest) endpointForRequest(method string, path string) (generatedRouteEndpoint, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	for _, endpoint := range manifest.Endpoints {
		if strings.ToUpper(strings.TrimSpace(endpoint.Method)) != method {
			continue
		}
		if generatedEndpointRouteMatches(endpoint.Route, path) {
			return endpoint, true
		}
	}
	return generatedRouteEndpoint{}, false
}

func generatedEndpointRouteMatches(endpointRoute string, requestPath string) bool {
	endpointRoute = strings.TrimSpace(endpointRoute)
	if endpointRoute == requestPath {
		return true
	}
	if !strings.Contains(endpointRoute, "{") {
		return false
	}
	_, ok := route.Match(endpointRoute, requestPath)
	return ok
}

// isDeniedPath reports whether requestPath resolves to a guardless page that is
// denied by default. It normalizes the request to the page route it would serve
// so that direct index artifact paths (/dashboard/index.html) and trailing-slash
// directory forms are denied alongside the canonical route, and matches dynamic
// page patterns so every concrete paths {} artifact is denied, not just the
// pattern string.
func (handler Handler) isDeniedPath(requestPath string) bool {
	if len(handler.Denied) == 0 && len(handler.DeniedPatterns) == 0 {
		return false
	}
	routePath := deniedRouteForPath(requestPath)
	if handler.Denied[routePath] {
		return true
	}
	for _, pattern := range handler.DeniedPatterns {
		if _, ok := route.Match(pattern, routePath); ok {
			return true
		}
	}
	return false
}

// deniedRouteForPath maps a request path to the page route that would serve it,
// stripping a trailing index.html artifact segment so a page emitted as
// "<route>/index.html" is denied when fetched directly by its file path.
func deniedRouteForPath(requestPath string) string {
	clean := path.Clean("/" + requestPath)
	if clean == "/index.html" {
		return "/"
	}
	if trimmed, ok := strings.CutSuffix(clean, "/index.html"); ok {
		return trimmed
	}
	return clean
}

// canonicalTrailingSlashPath reports the canonical redirect target for a
// GET/HEAD request path that carries a trailing slash. Declared routes are
// canonical without trailing slashes (except "/"), so /blog/hello/ permanently
// redirects to /blog/hello instead of serving duplicate content.
func canonicalTrailingSlashPath(requestPath string) (string, bool) {
	if requestPath == "" || requestPath == "/" || !strings.HasSuffix(requestPath, "/") {
		return "", false
	}
	if len(requestPath) > 1 && (requestPath[1] == '/' || requestPath[1] == '\\') {
		return "", false
	}
	canonical := path.Clean("/" + requestPath)
	if len(canonical) > 1 && (canonical[1] == '/' || canonical[1] == '\\') {
		return "", false
	}
	if canonical == requestPath {
		return "", false
	}
	return canonical, true
}

const cookieAckName = "gowdk_cookie_ack"

func isCookieAckPath(requestPath string) bool {
	return strings.TrimRight(requestPath, "/") == "/_gowdk/cookie-ack"
}

func acknowledgeCookie(response http.ResponseWriter, request *http.Request) {
	http.SetCookie(response, &http.Cookie{
		Name:     cookieAckName,
		Value:    "accepted",
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		HttpOnly: true,
		Secure:   requestIsHTTPS(request),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(response, request, safeRedirectPath(request), http.StatusSeeOther)
}

func requestIsHTTPS(request *http.Request) bool {
	if request.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")), "https")
}

func safeRedirectPath(request *http.Request) string {
	referer := strings.TrimSpace(request.Referer())
	if referer == "" {
		return "/"
	}
	parsed, err := url.Parse(referer)
	if err != nil {
		return "/"
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, request.Host) {
		return "/"
	}
	target := parsed.EscapedPath()
	if target == "" {
		target = "/"
	}
	if len(target) == 0 || target[0] != '/' || (len(target) > 1 && (target[1] == '/' || target[1] == '\\')) {
		return "/"
	}
	if parsed.RawQuery != "" {
		if strings.ContainsAny(parsed.RawQuery, "\r\n") {
			return "/"
		}
		target += "?" + parsed.RawQuery
	}
	return target
}

func (handler Handler) cookieAwarePayload(request *http.Request, payload []byte, name string) []byte {
	if !strings.HasSuffix(name, ".html") || !cookieAcknowledged(request) {
		return payload
	}
	marker := []byte("data-cookie-notice")
	hidden := []byte("data-cookie-notice hidden")
	if !bytes.Contains(payload, marker) || bytes.Contains(payload, hidden) {
		return payload
	}
	return bytes.Replace(payload, marker, hidden, 1)
}

func (handler Handler) csrfAwarePayload(response http.ResponseWriter, request *http.Request, payload []byte, name string) ([]byte, bool) {
	if handler.CSRF == nil || request.Method != http.MethodGet || !strings.HasSuffix(name, ".html") {
		return payload, true
	}
	return CSRFInjectHTML(response, request, payload, handler.CSRF)
}

// CSRFInjectHTML injects a hidden CSRF token into HTML POST forms. It returns
// false after writing a no-store 500 response when token generation fails.
func CSRFInjectHTML(response http.ResponseWriter, request *http.Request, payload []byte, source CSRFTokenSource) ([]byte, bool) {
	if source == nil || request.Method != http.MethodGet {
		return payload, true
	}
	matches := formStartTagRanges(payload)
	if len(matches) == 0 {
		return payload, true
	}

	var token string
	var hidden []byte
	var builder bytes.Buffer
	cursor := 0
	injected := false
	for _, match := range matches {
		tag := payload[match[0]:match[1]]
		if !formStartTagHasPostMethod(tag) {
			continue
		}
		if !formStartTagHasSameOriginAction(request, tag) {
			continue
		}
		if token == "" {
			generated, err := source.Token(response, request)
			if err != nil {
				response.Header().Set("Cache-Control", "no-store")
				http.Error(response, "csrf token unavailable", http.StatusInternalServerError)
				return nil, false
			}
			token = generated
			hidden = csrfHiddenInput(source.FieldName(), token)
			response.Header().Set("Cache-Control", "no-store")
		}
		builder.Write(payload[cursor:match[1]])
		builder.Write(hidden)
		cursor = match[1]
		injected = true
	}
	if !injected {
		return payload, true
	}
	builder.Write(payload[cursor:])
	return builder.Bytes(), true
}

func formStartTagRanges(payload []byte) [][2]int {
	var matches [][2]int
	for index := 0; index < len(payload); index++ {
		if payload[index] != '<' || !bytesHasFoldPrefix(payload[index+1:], "form") {
			continue
		}
		afterName := index + len("<form")
		if afterName < len(payload) && isHTMLNameChar(payload[afterName]) {
			continue
		}
		end := htmlTagEnd(payload, afterName)
		if end < 0 {
			break
		}
		matches = append(matches, [2]int{index, end + 1})
		index = end
	}
	return matches
}

func formStartTagHasPostMethod(tag []byte) bool {
	value, ok := formStartTagAttrValue(tag, "method")
	return ok && strings.EqualFold(html.UnescapeString(string(value)), http.MethodPost)
}

func formStartTagHasSameOriginAction(request *http.Request, tag []byte) bool {
	value, ok := formStartTagAttrValue(tag, "action")
	if !ok {
		return true
	}
	action := strings.TrimSpace(html.UnescapeString(string(value)))
	if action == "" {
		return true
	}
	if formActionHasBrowserNetworkPath(action) {
		return false
	}
	if request == nil {
		return false
	}
	scheme, host := requestOrigin(request)
	if scheme == "" || host == "" {
		return false
	}
	actionURL, err := url.Parse(action)
	if err != nil {
		return false
	}
	resolved := (&url.URL{Scheme: scheme, Host: host, Path: "/"}).ResolveReference(actionURL)
	return strings.EqualFold(resolved.Scheme, scheme) &&
		strings.EqualFold(canonicalOriginHost(resolved.Scheme, resolved.Host), canonicalOriginHost(scheme, host))
}

func requestOrigin(request *http.Request) (string, string) {
	scheme := ""
	if requestIsHTTPS(request) {
		scheme = "https"
	} else if request.URL != nil {
		scheme = request.URL.Scheme
	}
	if scheme == "" {
		scheme = "http"
	}
	host := request.Host
	if host == "" && request.URL != nil {
		host = request.URL.Host
	}
	return scheme, host
}

func formActionHasBrowserNetworkPath(action string) bool {
	if len(action) < 2 {
		return false
	}
	first := action[0]
	second := action[1]
	return (first == '/' || first == '\\') && (second == '/' || second == '\\')
}

func canonicalOriginHost(scheme string, host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	name, port, err := net.SplitHostPort(host)
	if err == nil {
		defaultPort := (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
		if defaultPort {
			return strings.ToLower(strings.Trim(name, "[]"))
		}
		return strings.ToLower(strings.Trim(name, "[]") + ":" + port)
	}
	return strings.Trim(host, "[]")
}

func formStartTagAttrValue(tag []byte, attrName string) ([]byte, bool) {
	cursor := len("<form")
	for cursor < len(tag) {
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		if cursor >= len(tag) || tag[cursor] == '>' || tag[cursor] == '/' {
			return nil, false
		}
		nameStart := cursor
		for cursor < len(tag) && !isHTMLSpace(tag[cursor]) && tag[cursor] != '=' && tag[cursor] != '/' && tag[cursor] != '>' {
			cursor++
		}
		name := tag[nameStart:cursor]
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		if cursor >= len(tag) || tag[cursor] != '=' {
			continue
		}
		cursor++
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		value, next := htmlAttrValue(tag, cursor)
		cursor = next
		if bytes.EqualFold(name, []byte(attrName)) {
			return value, true
		}
	}
	return nil, false
}

func htmlTagEnd(payload []byte, cursor int) int {
	var quote byte
	for cursor < len(payload) {
		if quote != 0 {
			if payload[cursor] == '\\' {
				cursor += 2
				continue
			}
			if payload[cursor] == quote {
				quote = 0
			}
			cursor++
			continue
		}
		switch payload[cursor] {
		case '\'', '"':
			quote = payload[cursor]
		case '>':
			return cursor
		}
		cursor++
	}
	return -1
}

func htmlAttrValue(tag []byte, cursor int) ([]byte, int) {
	if cursor >= len(tag) {
		return nil, cursor
	}
	if tag[cursor] == '\'' || tag[cursor] == '"' {
		quote := tag[cursor]
		cursor++
		start := cursor
		for cursor < len(tag) && tag[cursor] != quote {
			cursor++
		}
		if cursor < len(tag) {
			return tag[start:cursor], cursor + 1
		}
		return tag[start:], cursor
	}
	start := cursor
	for cursor < len(tag) && !isHTMLSpace(tag[cursor]) && tag[cursor] != '/' && tag[cursor] != '>' {
		cursor++
	}
	return tag[start:cursor], cursor
}

func bytesHasFoldPrefix(value []byte, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	for index := 0; index < len(prefix); index++ {
		if asciiLower(value[index]) != prefix[index] {
			return false
		}
	}
	return true
}

func asciiLower(char byte) byte {
	if char >= 'A' && char <= 'Z' {
		return char + ('a' - 'A')
	}
	return char
}

func isHTMLNameChar(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-'
}

func isHTMLSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func csrfHiddenInput(fieldName string, token string) []byte {
	return []byte(`<input type="hidden" name="` + html.EscapeString(fieldName) + `" value="` + html.EscapeString(token) + `">`)
}

func (handler Handler) setGeneratedStaticCache(response http.ResponseWriter, assetName string) {
	if response.Header().Get("Cache-Control") != "" {
		return
	}
	if policy := handler.Assets.CachePolicy(assetName); policy != "" {
		response.Header().Set("Cache-Control", policy)
		return
	}
	response.Header().Set("Cache-Control", "no-cache")
}

func cookieAcknowledged(request *http.Request) bool {
	cookie, err := request.Cookie(cookieAckName)
	return err == nil && cookie.Value == "accepted"
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func generatedInstanceID(moduleName string) string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "host"
	}

	token := randomToken()
	if token == "" {
		token = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return identityPart(moduleName) + "-" + identityPart(hostname) + "-" + token
}

func randomToken() string {
	var token [6]byte
	if _, err := rand.Read(token[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(token[:])
}

func identityPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	out := make([]rune, 0, len(value))
	lastDash := false
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9'
		if valid {
			out = append(out, char)
			lastDash = false
			continue
		}
		if !lastDash {
			out = append(out, '-')
			lastDash = true
		}
	}
	part := strings.Trim(string(out), "-")
	if part == "" {
		return "instance"
	}
	return part
}

func (handler Handler) writeIdentityHeaders(response http.ResponseWriter) {
	response.Header().Set("X-GOWDK-App", handler.Identity.AppID)
	response.Header().Set("X-GOWDK-Module", handler.Identity.ModuleName)
	response.Header().Set("X-GOWDK-Instance-ID", handler.Identity.InstanceID)
}

func (handler Handler) writeSecurityHeaders(response http.ResponseWriter) {
	for _, header := range canonicalSecurityHeaders(handler.SecurityHeaders) {
		response.Header().Set(header.Name, header.Value)
	}
}

type canonicalSecurityHeader struct {
	Name  string
	Value string
}

func canonicalSecurityHeaders(headers map[string]string) []canonicalSecurityHeader {
	type candidate struct {
		key   string
		name  string
		value string
	}
	candidates := make([]candidate, 0, len(headers))
	for name, value := range headers {
		clean := strings.TrimSpace(name)
		if clean == "" {
			continue
		}
		candidates = append(candidates, candidate{
			key:   strings.ToLower(clean),
			name:  http.CanonicalHeaderKey(clean),
			value: value,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].key != candidates[j].key {
			return candidates[i].key < candidates[j].key
		}
		if candidates[i].name != candidates[j].name {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].value < candidates[j].value
	})
	seen := map[string]bool{}
	out := make([]canonicalSecurityHeader, 0, len(candidates))
	for _, candidate := range candidates {
		if seen[candidate.key] {
			continue
		}
		seen[candidate.key] = true
		out = append(out, canonicalSecurityHeader{Name: candidate.name, Value: candidate.value})
	}
	return out
}

func (handler Handler) health(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "application/json")
	payload := map[string]any{
		"status":      "ok",
		"app":         handler.Identity.AppID,
		"module":      handler.Identity.ModuleName,
		"instance_id": handler.Identity.InstanceID,
		"assets":      strconv.Itoa(len(handler.Assets.Files)),
	}
	if handler.Metrics != nil {
		payload["metrics"] = handler.Metrics.Snapshot()
	}
	if handler.Tracer != nil {
		payload["trace"] = handler.Tracer.HealthSnapshot()
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		_, _ = response.Write([]byte(`{"status":"error"}` + "\n"))
		return
	}
	encoded = append(encoded, '\n')
	_, _ = response.Write(encoded)
}

func (handler Handler) SPAFile(requestPath string) ([]byte, fs.FileInfo, string, bool) {
	for _, candidate := range SPACandidates(requestPath) {
		payload, info, ok := readSPAFile(handler.Root, candidate)
		if ok {
			return payload, info, candidate, true
		}
	}
	return nil, nil, "", false
}

func SPACandidates(requestPath string) []string {
	if unsafeRequestPath(requestPath) {
		return nil
	}
	clean := path.Clean("/" + requestPath)
	if strings.HasSuffix(requestPath, "/") {
		return []string{strings.TrimPrefix(path.Join(clean, "index.html"), "/")}
	}

	candidate := strings.TrimPrefix(clean, "/")
	if path.Ext(clean) == "" {
		return []string{candidate, strings.TrimPrefix(path.Join(clean, "index.html"), "/")}
	}
	return []string{candidate}
}

func readSPAFile(root fs.FS, name string) ([]byte, fs.FileInfo, bool) {
	if name == "" {
		name = "index.html"
	}
	if !publicSPAFile(name) {
		return nil, nil, false
	}
	info, err := fs.Stat(root, name)
	if err != nil {
		return nil, nil, false
	}
	if info.IsDir() {
		name = path.Join(name, "index.html")
		if !publicSPAFile(name) {
			return nil, nil, false
		}
		info, err = fs.Stat(root, name)
		if err != nil || info.IsDir() {
			return nil, nil, false
		}
	}
	payload, err := fs.ReadFile(root, name)
	if err != nil {
		return nil, nil, false
	}
	return payload, info, true
}

func publicSPAFile(name string) bool {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if name == "" || privateSPAMetadata(name) {
		return false
	}
	if strings.HasPrefix(name, "assets/") {
		return !unsafeSPAExtension(name)
	}
	switch path.Base(name) {
	case "sitemap.xml", "robots.txt", "openapi.json", "asyncapi.json":
		return true
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".css", ".js", ".wasm", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".avif", ".woff", ".woff2", ".ttf", ".otf":
		return true
	default:
		return false
	}
}

func privateSPAMetadata(name string) bool {
	switch strings.ToLower(path.Base(name)) {
	case "gowdk-security.json", "gowdk-build-report.json", "gowdk-build-timings.json", "gowdk-routes.json", "gowdk-assets.json":
		return true
	default:
		return false
	}
}

func unsafeSPAExtension(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".map", ".gwdk", ".go", ".tmp", ".temp", ".key", ".pem", ".p12", ".pfx":
		return true
	default:
		return strings.HasSuffix(strings.ToLower(path.Base(name)), "~")
	}
}

func unsafeRequestPath(requestPath string) bool {
	trimmed := strings.Trim(requestPath, "/")
	if trimmed == "" {
		return false
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return true
		}
	}
	return false
}
