package app

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/runtime/asset"
)

// HandlerFunc handles a generated request-time route and reports whether it
// wrote a response.
type HandlerFunc func(http.ResponseWriter, *http.Request) bool

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
	Action     HandlerFunc
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

// LoadAssetManifest reads gowdk-assets.json from generated static output.
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
	handler.writeIdentityHeaders(response)
	if request.Method == http.MethodPost && handler.Action != nil && handler.Action(response, request) {
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
	if handler.SSRExact != nil && handler.SSRExact(response, request) {
		return
	}

	payload, info, ok := handler.staticFile(request.URL.Path)
	if !ok {
		if handler.SSRDynamic != nil && handler.SSRDynamic(response, request) {
			return
		}
		http.NotFound(response, request)
		return
	}
	http.ServeContent(response, request, info.Name(), info.ModTime(), bytes.NewReader(payload))
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

func (handler Handler) writeIdentityHeaders(response http.ResponseWriter) {
	response.Header().Set("X-GOWDK-App", handler.Identity.AppID)
	response.Header().Set("X-GOWDK-Module", handler.Identity.ModuleName)
	response.Header().Set("X-GOWDK-Instance-ID", handler.Identity.InstanceID)
}

func (handler Handler) health(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(map[string]string{
		"status":      "ok",
		"app":         handler.Identity.AppID,
		"module":      handler.Identity.ModuleName,
		"instance_id": handler.Identity.InstanceID,
		"assets":      strconv.Itoa(len(handler.Assets.Files)),
	})
}

func (handler Handler) staticFile(requestPath string) ([]byte, fs.FileInfo, bool) {
	for _, candidate := range staticCandidates(requestPath) {
		payload, info, ok := readStaticFile(handler.Root, candidate)
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
