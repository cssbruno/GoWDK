package app

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/runtime/asset"
)

// HandlerFunc handles a generated request-time route and reports whether it
// wrote a response.
type HandlerFunc func(http.ResponseWriter, *http.Request) bool

// CSRFTokenSource generates tokens for generated action forms.
type CSRFTokenSource interface {
	Token(http.ResponseWriter) (string, error)
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
	Root       fs.FS
	Identity   Identity
	Assets     asset.Manifest
	Backend    HandlerFunc
	Action     HandlerFunc
	API        HandlerFunc
	CSRF       CSRFTokenSource
	ErrorPages ErrorPages
	Metrics    *Metrics
	SSRExact   HandlerFunc
	SSRDynamic HandlerFunc
}

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
		return asset.Manifest{Version: 1, Files: map[string]string{}}
	}
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return asset.Manifest{Version: 1, Files: map[string]string{}}
	}
	if manifest.Files == nil {
		manifest.Files = map[string]string{}
	}
	return manifest
}

func (handler Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	metrics := handler.Metrics
	metrics.recordRequest()
	handler.writeIdentityHeaders(response)
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
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
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
	if parsed.RawQuery != "" {
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

var (
	formStartTagPattern   = regexp.MustCompile(`(?is)<form\b[^>]*>`)
	formMethodPostPattern = regexp.MustCompile(`(?i)\bmethod\s*=\s*(?:"post"|'post'|post)(?:\s|/|>)`)
)

func (handler Handler) csrfAwarePayload(response http.ResponseWriter, request *http.Request, payload []byte, name string) ([]byte, bool) {
	if handler.CSRF == nil || request.Method != http.MethodGet || !strings.HasSuffix(name, ".html") {
		return payload, true
	}
	matches := formStartTagPattern.FindAllIndex(payload, -1)
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
		if !formMethodPostPattern.Match(tag) {
			continue
		}
		if token == "" {
			generated, err := handler.CSRF.Token(response)
			if err != nil {
				response.Header().Set("Cache-Control", "no-store")
				http.Error(response, "csrf token unavailable", http.StatusInternalServerError)
				return nil, false
			}
			token = generated
			hidden = csrfHiddenInput(handler.CSRF.FieldName(), token)
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
	_ = json.NewEncoder(response).Encode(payload)
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
	info, err := fs.Stat(root, name)
	if err != nil {
		return nil, nil, false
	}
	if info.IsDir() {
		name = path.Join(name, "index.html")
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
