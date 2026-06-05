package appgen

const moduleSource = `module gowdk-generated-app

go 1.26
`

const serverMainSource = `package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gowdk-generated-app/gowdkapp"
)

func main() {
	handler, err := gowdkapp.Handler()
	if err != nil {
		log.Fatal(err)
	}

	addr := env("GOWDK_ADDR", "127.0.0.1:8080")
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	log.Printf("serving embedded GOWDK static app at http://%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
`

const appPackageSourceTemplate = `package gowdkapp

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const maxActionBodyBytes int64 = 1 << 20

//go:embed static
var embeddedFiles embed.FS

func Handler() (http.Handler, error) {
	return ServeMux()
}

func ServeMux() (*http.ServeMux, error) {
	root, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", staticHandler{
		root:     root,
		identity: instanceIdentity(),
		assets:   loadAssetManifest(root),
	})
	return mux, nil
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

type identity struct {
	AppID      string
	ModuleName string
	InstanceID string
}

func instanceIdentity() identity {
	appID := env("GOWDK_APP_ID", "app")
	moduleName := env("GOWDK_MODULE_NAME", "app")
	instanceID := env("GOWDK_INSTANCE_ID", "")
	if instanceID == "" {
		instanceID = generatedInstanceID(moduleName)
	}

	return identity{
		AppID:      appID,
		ModuleName: moduleName,
		InstanceID: instanceID,
	}
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
	var out strings.Builder
	lastDash := false
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9'
		if valid {
			out.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			out.WriteByte('-')
			lastDash = true
		}
	}
	part := strings.Trim(out.String(), "-")
	if part == "" {
		return "instance"
	}
	return part
}

type staticHandler struct {
	root     fs.FS
	identity identity
	assets   assetManifest
}

type assetManifest struct {
	Version int
	Files   map[string]string
}

func loadAssetManifest(root fs.FS) assetManifest {
	var manifest assetManifest
	payload, err := fs.ReadFile(root, "gowdk-assets.json")
	if err != nil {
		return assetManifest{Version: 1, Files: map[string]string{}}
	}
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return assetManifest{Version: 1, Files: map[string]string{}}
	}
	if manifest.Files == nil {
		manifest.Files = map[string]string{}
	}
	return manifest
}

func (handler staticHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	handler.writeIdentityHeaders(response)
	if request.Method == http.MethodPost && handler.action(response, request) {
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		response.Header().Set("Allow", "GET, HEAD")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request.URL.Path == "/_gowdk/health" {
		handler.health(response)
		return
	}
	if handler.ssrExact(response, request) {
		return
	}

	payload, info, ok := handler.staticFile(request.URL.Path)
	if !ok {
		if handler.ssrDynamic(response, request) {
			return
		}
		http.NotFound(response, request)
		return
	}
	http.ServeContent(response, request, info.Name(), info.ModTime(), bytes.NewReader(payload))
}

{{ACTION_HANDLER}}

{{SSR_HANDLER}}

func writeSSRHTML(response http.ResponseWriter, request *http.Request, html string) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodHead {
		_, _ = response.Write([]byte(html))
	}
}

func matchSSRRoute(pattern, requestPath string) (map[string]string, bool) {
	patternParts := splitSSRPath(pattern)
	requestParts := splitSSRPath(requestPath)
	if len(patternParts) != len(requestParts) {
		return nil, false
	}
	params := map[string]string{}
	for index, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			value := requestParts[index]
			if value == "" || value == "." || value == ".." {
				return nil, false
			}
			params[name] = value
			continue
		}
		if part != requestParts[index] {
			return nil, false
		}
	}
	return params, true
}

func splitSSRPath(value string) []string {
	clean := path.Clean("/" + value)
	trimmed := strings.Trim(clean, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func escapeSSRValue(value string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	).Replace(value)
}

func (handler staticHandler) writeIdentityHeaders(response http.ResponseWriter) {
	response.Header().Set("X-GOWDK-App", handler.identity.AppID)
	response.Header().Set("X-GOWDK-Module", handler.identity.ModuleName)
	response.Header().Set("X-GOWDK-Instance-ID", handler.identity.InstanceID)
}

func (handler staticHandler) health(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(map[string]string{
		"status":      "ok",
		"app":         handler.identity.AppID,
		"module":      handler.identity.ModuleName,
		"instance_id": handler.identity.InstanceID,
		"assets":      strconv.Itoa(len(handler.assets.Files)),
	})
}

func (handler staticHandler) staticFile(requestPath string) ([]byte, fs.FileInfo, bool) {
	for _, candidate := range staticCandidates(requestPath) {
		payload, info, ok := readStaticFile(handler.root, candidate)
		if ok {
			return payload, info, true
		}
	}
	return nil, nil, false
}

func staticCandidates(requestPath string) []string {
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

func readStaticFile(root fs.FS, name string) ([]byte, fs.FileInfo, bool) {
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
`
