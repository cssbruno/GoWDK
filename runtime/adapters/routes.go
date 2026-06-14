// Package adapters provides framework-neutral helpers used by optional
// framework adapter modules.
package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"unicode"
)

// Route describes one routable generated HTTP surface from GOWDK metadata.
type Route struct {
	Method string
	Path   string
}

// PatternStyle selects a host router pattern syntax.
type PatternStyle string

const (
	// PatternChi translates GOWDK params to chi's {name} syntax.
	PatternChi PatternStyle = "chi"
	// PatternEcho translates GOWDK params to Echo's :name and * syntax.
	PatternEcho PatternStyle = "echo"
	// PatternGin translates GOWDK params to Gin's :name and *name syntax.
	PatternGin PatternStyle = "gin"
)

// RoutesFromOpenAPI extracts deterministic method/path records from a GOWDK
// OpenAPI report.
func RoutesFromOpenAPI(spec []byte) ([]Route, error) {
	var document struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(spec, &document); err != nil {
		return nil, fmt.Errorf("parse openapi spec: %w", err)
	}
	if len(document.Paths) == 0 {
		return nil, nil
	}
	var routes []Route
	for routePath, pathItem := range document.Paths {
		cleanPath, err := CleanRoutePath(routePath)
		if err != nil {
			return nil, fmt.Errorf("openapi path %q: %w", routePath, err)
		}
		for methodKey, operationPayload := range pathItem {
			method := strings.ToUpper(strings.TrimSpace(methodKey))
			if !isHTTPMethod(method) {
				continue
			}
			routePattern, err := routePathFromOperation(operationPayload, cleanPath)
			if err != nil {
				return nil, fmt.Errorf("openapi operation %s %s: %w", method, routePath, err)
			}
			routes = append(routes, Route{Method: method, Path: routePattern})
		}
	}
	sortRoutes(routes)
	return routes, nil
}

func routePathFromOperation(payload json.RawMessage, fallback string) (string, error) {
	var operation struct {
		XGOWDK struct {
			Route string `json:"route"`
		} `json:"x-gowdk"`
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &operation); err != nil {
			return "", err
		}
	}
	routePath := strings.TrimSpace(operation.XGOWDK.Route)
	if routePath == "" {
		return fallback, nil
	}
	return CleanRoutePath(routePath)
}

// OpenAPIWithServerURL returns spec with a single servers entry for the mount
// URL. Use this when generated routes are served below a host-app prefix.
func OpenAPIWithServerURL(spec []byte, serverURL string) ([]byte, error) {
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(spec))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse openapi spec: %w", err)
	}
	url, err := normalizeServerURL(serverURL)
	if err != nil {
		return nil, err
	}
	document["servers"] = []map[string]string{{"url": url}}
	out, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// TranslatePattern converts a GOWDK route pattern into the selected host router
// syntax. GOWDK params use {name}, {name:type}, or final {name...} segments.
func TranslatePattern(routePath string, style PatternStyle) (string, error) {
	cleanPath, err := CleanRoutePath(routePath)
	if err != nil {
		return "", err
	}
	switch style {
	case PatternChi, PatternEcho, PatternGin:
	default:
		return "", fmt.Errorf("unsupported route pattern style %q", style)
	}
	if cleanPath == "/" {
		return "/", nil
	}
	segments := strings.Split(strings.Trim(cleanPath, "/"), "/")
	for index, segment := range segments {
		param, ok, err := parseParamSegment(segment)
		if err != nil {
			return "", err
		}
		if !ok {
			if strings.ContainsAny(segment, "{}") {
				return "", fmt.Errorf("route segment %q has malformed parameter syntax", segment)
			}
			continue
		}
		if param.Rest && index != len(segments)-1 {
			return "", fmt.Errorf("rest route parameter %q must be the final segment", param.Name)
		}
		segments[index] = translateParam(param, style)
	}
	return "/" + strings.Join(segments, "/"), nil
}

// CleanRoutePath normalizes a route path while rejecting values that cannot be
// mounted as HTTP path patterns.
func CleanRoutePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("route path is required")
	}
	if strings.ContainsAny(value, "?#\\") || hasControl(value) {
		return "", fmt.Errorf("route path %q must not contain query, fragment, backslash, or control characters", value)
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if len(value) > 1 && (value[1] == '/' || value[1] == '\\') {
		return "", fmt.Errorf("route path %q must not start with // or /\\", value)
	}
	clean := path.Clean(value)
	if clean == "." {
		return "/", nil
	}
	return clean, nil
}

// NormalizeMountPrefix cleans a host-app mount prefix. The empty string and "/"
// both mean no prefix.
func NormalizeMountPrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return "", nil
	}
	rawPrefix := prefix
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if len(prefix) > 1 && (prefix[1] == '/' || prefix[1] == '\\') {
		return "", fmt.Errorf("mount prefix %q must not start with // or /\\", rawPrefix)
	}
	if strings.ContainsAny(prefix, "?#\\") || hasControl(prefix) {
		return "", fmt.Errorf("mount prefix %q must not contain query, fragment, backslash, or control characters", rawPrefix)
	}
	clean := path.Clean(prefix)
	if clean == "/" || clean == "." {
		return "", nil
	}
	return clean, nil
}

// JoinPrefix prefixes a route path or translated host route pattern.
func JoinPrefix(prefix string, routePath string) (string, error) {
	cleanPrefix, err := NormalizeMountPrefix(prefix)
	if err != nil {
		return "", err
	}
	cleanPath, err := CleanRoutePath(routePath)
	if err != nil {
		return "", err
	}
	if cleanPrefix == "" {
		return cleanPath, nil
	}
	if cleanPath == "/" {
		return cleanPrefix, nil
	}
	return path.Join(cleanPrefix, cleanPath), nil
}

// HandlerWithPrefix strips a host-app mount prefix before dispatching to the
// generated GOWDK handler and rebases local generated URLs back under the mount
// prefix in Location headers and HTML bodies.
func HandlerWithPrefix(prefix string, handler http.Handler) (http.Handler, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}
	cleanPrefix, err := NormalizeMountPrefix(prefix)
	if err != nil {
		return nil, err
	}
	if cleanPrefix == "" {
		return handler, nil
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request == nil || request.URL == nil {
			http.NotFound(writer, request)
			return
		}
		requestPath := request.URL.Path
		if requestPath != cleanPrefix && !strings.HasPrefix(requestPath, cleanPrefix+"/") {
			http.NotFound(writer, request)
			return
		}
		strippedPath := strings.TrimPrefix(requestPath, cleanPrefix)
		if strippedPath == "" {
			strippedPath = "/"
		}
		copied := request.Clone(request.Context())
		urlCopy := *request.URL
		urlCopy.Path = strippedPath
		urlCopy.RawPath = ""
		copied.URL = &urlCopy
		recorder := newPrefixResponseRecorder()
		handler.ServeHTTP(recorder, copied)
		recorder.writeTo(writer, cleanPrefix)
	}), nil
}

type prefixResponseRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newPrefixResponseRecorder() *prefixResponseRecorder {
	return &prefixResponseRecorder{header: http.Header{}}
}

func (recorder *prefixResponseRecorder) Header() http.Header {
	return recorder.header
}

func (recorder *prefixResponseRecorder) WriteHeader(status int) {
	if recorder.status == 0 {
		recorder.status = status
	}
}

func (recorder *prefixResponseRecorder) Write(data []byte) (int, error) {
	if recorder.status == 0 {
		recorder.status = http.StatusOK
	}
	return recorder.body.Write(data)
}

func (recorder *prefixResponseRecorder) writeTo(writer http.ResponseWriter, prefix string) {
	status := recorder.status
	if status == 0 {
		status = http.StatusOK
	}
	headers := recorder.header.Clone()
	rebaseLocationHeaders(headers, prefix)
	body := recorder.body.Bytes()
	if shouldRebaseHTML(headers, body) {
		rebased := rebaseHTMLRootURLs(prefix, body)
		if !bytes.Equal(rebased, body) {
			body = rebased
			headers.Del("Content-Length")
		}
	} else if shouldRebaseCSS(headers) {
		rebased := rebaseCSSRootURLs(prefix, body)
		if !bytes.Equal(rebased, body) {
			body = rebased
			headers.Del("Content-Length")
		}
	}
	for key, values := range headers {
		for _, value := range values {
			writer.Header().Add(key, value)
		}
	}
	writer.WriteHeader(status)
	if len(body) > 0 {
		_, _ = writer.Write(body)
	}
}

func rebaseLocationHeaders(headers http.Header, prefix string) {
	values := headers.Values("Location")
	if len(values) == 0 {
		return
	}
	headers.Del("Location")
	for _, value := range values {
		headers.Add("Location", RebaseLocalURL(prefix, value))
	}
}

// RebaseLocalURL returns value under prefix when value is a same-origin
// root-relative URL. Absolute URLs, protocol-relative or slash-backslash URLs,
// relative URLs, and already-prefixed URLs are returned unchanged.
func RebaseLocalURL(prefix string, value string) string {
	cleanPrefix, err := NormalizeMountPrefix(prefix)
	if err != nil || cleanPrefix == "" || len(value) == 0 || value[0] != '/' || (len(value) > 1 && (value[1] == '/' || value[1] == '\\')) {
		return value
	}
	if localURLHasPrefix(value, cleanPrefix) {
		return value
	}
	if value == "/" {
		return cleanPrefix + "/"
	}
	return cleanPrefix + value
}

func localURLHasPrefix(value string, prefix string) bool {
	if value == prefix {
		return true
	}
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	if len(value) == len(prefix) {
		return true
	}
	switch value[len(prefix)] {
	case '/', '?', '#':
		return true
	default:
		return false
	}
}

func shouldRebaseHTML(headers http.Header, body []byte) bool {
	contentType := strings.ToLower(headers.Get("Content-Type"))
	if strings.Contains(contentType, "text/html") {
		return true
	}
	trimmed := bytes.ToLower(bytes.TrimSpace(body))
	return bytes.HasPrefix(trimmed, []byte("<!doctype html")) || bytes.HasPrefix(trimmed, []byte("<html"))
}

func shouldRebaseCSS(headers http.Header) bool {
	return strings.Contains(strings.ToLower(headers.Get("Content-Type")), "text/css")
}

func rebaseHTMLRootURLs(prefix string, body []byte) []byte {
	out := string(body)
	for _, attr := range []string{"href", "src", "action", "formaction", "poster", "cite", "data", "longdesc", "manifest", "xlink:href", "content"} {
		out = rebaseQuotedAttributeURLs(out, attr, prefix)
	}
	out = rebaseQuotedAttributeURLsWith(out, "srcset", prefix, rebaseSrcsetLocalURLs)
	out = rebaseCSSRootURLString(prefix, out)
	return []byte(out)
}

func rebaseCSSRootURLs(prefix string, body []byte) []byte {
	return []byte(rebaseCSSRootURLString(prefix, string(body)))
}

func rebaseQuotedAttributeURLs(input string, attr string, prefix string) string {
	return rebaseQuotedAttributeURLsWith(input, attr, prefix, RebaseLocalURL)
}

func rebaseQuotedAttributeURLsWith(input string, attr string, prefix string, rebase func(string, string) string) string {
	needle := strings.ToLower(attr) + "="
	lower := strings.ToLower(input)
	var builder strings.Builder
	start := 0
	for {
		index := strings.Index(lower[start:], needle)
		if index < 0 {
			builder.WriteString(input[start:])
			return builder.String()
		}
		index += start
		quoteIndex := index + len(needle)
		if !isHTMLAttributeBoundary(input, index) || quoteIndex >= len(input) || (input[quoteIndex] != '"' && input[quoteIndex] != '\'') {
			builder.WriteString(input[start : index+len(needle)])
			start = index + len(needle)
			continue
		}
		quote := input[quoteIndex]
		valueStart := quoteIndex + 1
		valueEndOffset := strings.IndexByte(input[valueStart:], quote)
		if valueEndOffset < 0 {
			builder.WriteString(input[start:])
			return builder.String()
		}
		valueEnd := valueStart + valueEndOffset
		builder.WriteString(input[start:valueStart])
		builder.WriteString(rebase(prefix, input[valueStart:valueEnd]))
		start = valueEnd
	}
}

func rebaseSrcsetLocalURLs(prefix string, value string) string {
	candidates := strings.Split(value, ",")
	for index, candidate := range candidates {
		urlStart := 0
		for urlStart < len(candidate) && isHTMLSpace(candidate[urlStart]) {
			urlStart++
		}
		urlEnd := urlStart
		for urlEnd < len(candidate) && !isHTMLSpace(candidate[urlEnd]) {
			urlEnd++
		}
		if urlStart == urlEnd {
			continue
		}
		candidates[index] = candidate[:urlStart] + RebaseLocalURL(prefix, candidate[urlStart:urlEnd]) + candidate[urlEnd:]
	}
	return strings.Join(candidates, ",")
}

func isHTMLAttributeBoundary(input string, index int) bool {
	if index == 0 {
		return true
	}
	switch input[index-1] {
	case '<', ' ', '\n', '\r', '\t':
		return true
	default:
		return false
	}
}

func isHTMLSpace(value byte) bool {
	switch value {
	case ' ', '\n', '\r', '\t', '\f':
		return true
	default:
		return false
	}
}

func rebaseCSSRootURLString(prefix string, input string) string {
	return rebaseCSSImportStringURLs(rebaseCSSURLFunctions(input, prefix), prefix)
}

func rebaseCSSURLFunctions(input string, prefix string) string {
	lower := strings.ToLower(input)
	var builder strings.Builder
	start := 0
	for {
		index := strings.Index(lower[start:], "url(")
		if index < 0 {
			builder.WriteString(input[start:])
			return builder.String()
		}
		index += start
		valueStart := index + len("url(")
		for valueStart < len(input) && (input[valueStart] == ' ' || input[valueStart] == '\t' || input[valueStart] == '\n' || input[valueStart] == '\r') {
			valueStart++
		}
		if valueStart >= len(input) {
			builder.WriteString(input[start:])
			return builder.String()
		}
		quote := byte(0)
		if input[valueStart] == '"' || input[valueStart] == '\'' {
			quote = input[valueStart]
			valueStart++
		}
		valueEnd := valueStart
		for valueEnd < len(input) {
			if quote != 0 {
				if input[valueEnd] == quote {
					break
				}
			} else if input[valueEnd] == ')' || input[valueEnd] == ' ' || input[valueEnd] == '\t' || input[valueEnd] == '\n' || input[valueEnd] == '\r' {
				break
			}
			valueEnd++
		}
		if valueEnd >= len(input) {
			builder.WriteString(input[start:])
			return builder.String()
		}
		builder.WriteString(input[start:valueStart])
		builder.WriteString(RebaseLocalURL(prefix, input[valueStart:valueEnd]))
		start = valueEnd
	}
}

func rebaseCSSImportStringURLs(input string, prefix string) string {
	lower := strings.ToLower(input)
	var builder strings.Builder
	start := 0
	for {
		index := strings.Index(lower[start:], "@import")
		if index < 0 {
			builder.WriteString(input[start:])
			return builder.String()
		}
		index += start
		valueStart := index + len("@import")
		if valueStart < len(input) && !isHTMLSpace(input[valueStart]) {
			builder.WriteString(input[start:valueStart])
			start = valueStart
			continue
		}
		for valueStart < len(input) && isHTMLSpace(input[valueStart]) {
			valueStart++
		}
		if valueStart >= len(input) || (input[valueStart] != '"' && input[valueStart] != '\'') {
			builder.WriteString(input[start:valueStart])
			start = valueStart
			continue
		}
		quote := input[valueStart]
		valueStart++
		valueEnd := valueStart
		for valueEnd < len(input) {
			if input[valueEnd] == '\\' {
				valueEnd += 2
				continue
			}
			if input[valueEnd] == quote {
				break
			}
			valueEnd++
		}
		if valueEnd >= len(input) {
			builder.WriteString(input[start:])
			return builder.String()
		}
		builder.WriteString(input[start:valueStart])
		builder.WriteString(RebaseLocalURL(prefix, input[valueStart:valueEnd]))
		start = valueEnd
	}
}

func normalizeServerURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "/", nil
	}
	if strings.ContainsAny(value, "\r\n\t") || hasControl(value) {
		return "", fmt.Errorf("openapi server URL %q must not contain control characters", value)
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return strings.TrimRight(value, "/"), nil
	}
	prefix, err := NormalizeMountPrefix(value)
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return "/", nil
	}
	return prefix, nil
}

type routeParam struct {
	Name string
	Rest bool
}

func parseParamSegment(segment string) (routeParam, bool, error) {
	if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
		return routeParam{}, false, nil
	}
	name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
	rest := strings.HasSuffix(name, "...")
	name = strings.TrimSuffix(name, "...")
	if before, _, ok := strings.Cut(name, ":"); ok {
		name = before
	}
	if !validParamName(name) {
		return routeParam{}, false, fmt.Errorf("route parameter name %q is invalid", name)
	}
	return routeParam{Name: name, Rest: rest}, true, nil
}

func translateParam(param routeParam, style PatternStyle) string {
	switch style {
	case PatternChi:
		if param.Rest {
			return "*"
		}
		return "{" + param.Name + "}"
	case PatternEcho:
		if param.Rest {
			return "*"
		}
		return ":" + param.Name
	case PatternGin:
		if param.Rest {
			return "*" + param.Name
		}
		return ":" + param.Name
	default:
		return ""
	}
}

func sortRoutes(routes []Route) {
	sort.Slice(routes, func(i, j int) bool {
		left := routes[i].Method + "\x00" + routes[i].Path
		right := routes[j].Method + "\x00" + routes[j].Path
		return left < right
	})
}

func isHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return true
	default:
		return false
	}
}

func validParamName(name string) bool {
	if name == "" {
		return false
	}
	for index, r := range name {
		if r == '_' || unicode.IsLetter(r) || index > 0 && unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
