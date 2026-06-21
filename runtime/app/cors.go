package app

import (
	"fmt"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

// CORSPolicy declares generated CORS behavior for API and web contract routes.
// The zero value is disabled.
type CORSPolicy struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
}

type corsPolicy struct {
	enabled          bool
	anyOrigin        bool
	origins          map[string]bool
	methods          map[string]bool
	allowedMethods   []string
	headers          map[string]bool
	allowedHeaders   []string
	exposedHeaders   []string
	allowCredentials bool
	maxAgeSeconds    int
}

func normalizeCORSPolicy(policy CORSPolicy) (corsPolicy, error) {
	if len(policy.AllowedOrigins) == 0 {
		return corsPolicy{}, nil
	}
	if policy.MaxAgeSeconds < 0 {
		return corsPolicy{}, fmt.Errorf("CORS max age must be non-negative")
	}
	normalized := corsPolicy{
		enabled:          true,
		origins:          map[string]bool{},
		methods:          map[string]bool{},
		headers:          map[string]bool{},
		allowCredentials: policy.AllowCredentials,
		maxAgeSeconds:    policy.MaxAgeSeconds,
	}
	for _, origin := range policy.AllowedOrigins {
		value, err := normalizeCORSOrigin(origin)
		if err != nil {
			return corsPolicy{}, err
		}
		if value == "*" {
			if policy.AllowCredentials {
				return corsPolicy{}, fmt.Errorf("CORS wildcard origin cannot be used with credentials")
			}
			normalized.anyOrigin = true
			continue
		}
		normalized.origins[value] = true
	}
	for _, method := range policy.AllowedMethods {
		value, err := normalizeCORSMethod(method)
		if err != nil {
			return corsPolicy{}, err
		}
		if !normalized.methods[value] {
			normalized.methods[value] = true
			normalized.allowedMethods = append(normalized.allowedMethods, value)
		}
	}
	for _, header := range policy.AllowedHeaders {
		value, key, err := normalizeCORSHeader(header)
		if err != nil {
			return corsPolicy{}, err
		}
		if !normalized.headers[key] {
			normalized.headers[key] = true
			normalized.allowedHeaders = append(normalized.allowedHeaders, value)
		}
	}
	seenExposed := map[string]bool{}
	for _, header := range policy.ExposedHeaders {
		value, key, err := normalizeCORSHeader(header)
		if err != nil {
			return corsPolicy{}, err
		}
		if !seenExposed[key] {
			seenExposed[key] = true
			normalized.exposedHeaders = append(normalized.exposedHeaders, value)
		}
	}
	return normalized, nil
}

func normalizeCORSOrigin(origin string) (string, error) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", fmt.Errorf("CORS origin is required")
	}
	if origin == "*" {
		return origin, nil
	}
	if strings.ContainsAny(origin, "\r\n") {
		return "", fmt.Errorf("CORS origin %q contains a control character", origin)
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("CORS origin %q is invalid: %w", origin, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("CORS origin %q must use http or https", origin)
	}
	if parsed.User != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("CORS origin %q must be an origin, not a URL with userinfo, query, or fragment", origin)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("CORS origin %q must not include a path", origin)
	}
	return scheme + "://" + strings.ToLower(parsed.Host), nil
}

func normalizeCORSMethod(method string) (string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return "", fmt.Errorf("CORS method is required")
	}
	if !httpgutsValidHeaderFieldName(method) {
		return "", fmt.Errorf("CORS method %q is invalid", method)
	}
	return method, nil
}

func normalizeCORSHeader(header string) (name string, key string, err error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", "", fmt.Errorf("CORS header name is required")
	}
	if !httpgutsValidHeaderFieldName(header) {
		return "", "", fmt.Errorf("CORS header name %q is invalid", header)
	}
	name = textproto.CanonicalMIMEHeaderKey(header)
	return name, strings.ToLower(name), nil
}

func httpgutsValidHeaderFieldName(name string) bool {
	for _, r := range name {
		if r > 127 {
			return false
		}
		if !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", r) {
			return false
		}
	}
	return name != ""
}

func (policy corsPolicy) writeActualHeaders(writer http.ResponseWriter, request *http.Request, method string) {
	if writer == nil || request == nil || !policy.enabled || !policy.methodAllowed(method) {
		return
	}
	origin, ok := policy.allowedOrigin(request.Header.Get("Origin"))
	if !ok {
		return
	}
	policy.writeSharedHeaders(writer, origin)
	if len(policy.exposedHeaders) > 0 {
		writer.Header().Set("Access-Control-Expose-Headers", strings.Join(policy.exposedHeaders, ", "))
	}
}

func (policy corsPolicy) writePreflight(writer http.ResponseWriter, request *http.Request, routeMethod string) bool {
	if writer == nil || request == nil || !policy.enabled {
		return false
	}
	origin, ok := policy.allowedOrigin(request.Header.Get("Origin"))
	if !ok {
		return false
	}
	requestedMethod, err := normalizeCORSMethod(request.Header.Get("Access-Control-Request-Method"))
	if err != nil || requestedMethod != routeMethod || !policy.methodAllowed(requestedMethod) {
		return false
	}
	requestedHeaders, ok := requestedCORSHeadersAllowed(request.Header.Get("Access-Control-Request-Headers"), policy.headers)
	if !ok {
		return false
	}
	policy.writeSharedHeaders(writer, origin)
	if len(policy.allowedMethods) > 0 {
		writer.Header().Set("Access-Control-Allow-Methods", strings.Join(policy.allowedMethods, ", "))
	} else {
		writer.Header().Set("Access-Control-Allow-Methods", routeMethod)
	}
	if len(requestedHeaders) > 0 {
		writer.Header().Set("Access-Control-Allow-Headers", strings.Join(policy.allowedHeaders, ", "))
	}
	if policy.maxAgeSeconds > 0 {
		writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(policy.maxAgeSeconds))
	}
	writer.WriteHeader(http.StatusNoContent)
	return true
}

func (policy corsPolicy) writeSharedHeaders(writer http.ResponseWriter, origin string) {
	if policy.anyOrigin {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		writer.Header().Set("Access-Control-Allow-Origin", origin)
		addVaryHeader(writer.Header(), "Origin")
	}
	if policy.allowCredentials {
		writer.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

func (policy corsPolicy) allowedOrigin(origin string) (string, bool) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", false
	}
	if policy.anyOrigin {
		return "*", true
	}
	normalized, err := normalizeCORSOrigin(origin)
	if err != nil {
		return "", false
	}
	if !policy.origins[normalized] {
		return "", false
	}
	return normalized, true
}

func (policy corsPolicy) methodAllowed(method string) bool {
	method, err := normalizeCORSMethod(method)
	if err != nil {
		return false
	}
	return len(policy.methods) == 0 || policy.methods[method]
}

func requestedCORSHeadersAllowed(header string, allowed map[string]bool) ([]string, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil, true
	}
	parts := strings.Split(header, ",")
	requested := make([]string, 0, len(parts))
	for _, part := range parts {
		name, key, err := normalizeCORSHeader(part)
		if err != nil {
			return nil, false
		}
		if !allowed[key] {
			return nil, false
		}
		requested = append(requested, name)
	}
	return requested, true
}

func addVaryHeader(header http.Header, value string) {
	for _, existing := range header.Values("Vary") {
		for _, part := range strings.Split(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(part), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}
