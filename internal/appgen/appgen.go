// Package appgen emits a generated Go app that embeds static build output.
package appgen

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

const (
	staticDirName = "static"
	mainFileName  = "main.go"
	modFileName   = "go.mod"
)

// Result describes generated app artifacts.
type Result struct {
	AppDir     string
	MainPath   string
	ModulePath string
	StaticDir  string
	Files      []string
	BinaryPath string
}

// Options configures generated app output.
type Options struct {
	Actions []ActionRoute
}

// ActionRoute describes a generated static action handler.
type ActionRoute struct {
	PageID         string
	ActionName     string
	Route          string
	InputName      string
	InputType      string
	InputFields    []string
	RequiredFields []string
	ValidatesInput bool
	Redirect       string
}

// Generate writes a self-contained Go app that embeds staticDir.
func Generate(staticDir, appDir string) (Result, error) {
	return GenerateWithOptions(staticDir, appDir, Options{})
}

// GenerateWithOptions writes a self-contained Go app that embeds staticDir.
func GenerateWithOptions(staticDir, appDir string, options Options) (Result, error) {
	if strings.TrimSpace(staticDir) == "" {
		return Result{}, fmt.Errorf("static output directory is required")
	}
	if strings.TrimSpace(appDir) == "" {
		return Result{}, fmt.Errorf("generated app directory is required")
	}

	absStatic, err := filepath.Abs(staticDir)
	if err != nil {
		return Result{}, err
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return Result{}, err
	}
	if err := validateDirectories(absStatic, absApp); err != nil {
		return Result{}, err
	}
	if err := validateActionRoutes(options.Actions); err != nil {
		return Result{}, err
	}

	targetStatic := filepath.Join(absApp, staticDirName)
	if isSameOrWithin(targetStatic, absStatic) {
		return Result{}, fmt.Errorf("static output directory %q must not be inside generated app static directory %q", absStatic, targetStatic)
	}
	if err := os.MkdirAll(absApp, 0o755); err != nil {
		return Result{}, err
	}
	if err := os.RemoveAll(targetStatic); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(targetStatic, 0o755); err != nil {
		return Result{}, err
	}

	files, err := copyStaticFiles(absStatic, targetStatic)
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(absApp, modFileName), []byte(moduleSource), 0o644); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(absApp, mainFileName), []byte(mainSource(options.Actions)), 0o644); err != nil {
		return Result{}, err
	}

	return Result{
		AppDir:     absApp,
		MainPath:   filepath.Join(absApp, mainFileName),
		ModulePath: filepath.Join(absApp, modFileName),
		StaticDir:  targetStatic,
		Files:      files,
	}, nil
}

// ActionRoutes extracts generated action routes from a parsed manifest.
func ActionRoutes(app manifest.Manifest) ([]ActionRoute, error) {
	var routes []ActionRoute
	for _, page := range app.Pages {
		fieldsByAction, err := view.ActionFormSchema(page.Blocks.ViewBody)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		for _, action := range page.Blocks.Actions {
			if strings.TrimSpace(action.Redirect) == "" {
				continue
			}
			routes = append(routes, ActionRoute{
				PageID:         page.ID,
				ActionName:     action.Name,
				Route:          page.Route,
				InputName:      action.InputName,
				InputType:      action.InputType,
				InputFields:    actionInputFields(fieldsByAction[action.Name]),
				RequiredFields: actionRequiredFields(fieldsByAction[action.Name]),
				ValidatesInput: action.ValidatesInput,
				Redirect:       action.Redirect,
			})
		}
	}
	if err := validateActionRoutes(routes); err != nil {
		return nil, err
	}
	return routes, nil
}

func actionInputFields(fields []view.ActionFormField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

func actionRequiredFields(fields []view.ActionFormField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Required {
			names = append(names, field.Name)
		}
	}
	return names
}

// BuildBinary compiles the generated app into binaryPath.
func BuildBinary(appDir, binaryPath string) (string, error) {
	if strings.TrimSpace(appDir) == "" {
		return "", fmt.Errorf("generated app directory is required")
	}
	if strings.TrimSpace(binaryPath) == "" {
		return "", fmt.Errorf("binary output path is required")
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	absBinary, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absBinary), 0o755); err != nil {
		return "", err
	}

	command := exec.Command("go", "build", "-buildvcs=false", "-o", absBinary, ".")
	command.Dir = absApp
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build generated app failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return absBinary, nil
}

func validateDirectories(staticDir, appDir string) error {
	info, err := os.Stat(staticDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("static output %q is not a directory", staticDir)
	}
	rel, err := filepath.Rel(staticDir, appDir)
	if err != nil {
		return err
	}
	if rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..") {
		return fmt.Errorf("generated app directory %q must be outside static output directory %q", appDir, staticDir)
	}
	return nil
}

func isSameOrWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func copyStaticFiles(sourceRoot, targetRoot string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		targetPath := filepath.Join(targetRoot, rel)
		if entry.IsDir() {
			if unsafeEmbeddedDirectory(rel) {
				return filepath.SkipDir
			}
			return os.MkdirAll(targetPath, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if unsafeEmbeddedFile(rel) {
			return nil
		}
		if err := copyFile(sourcePath, targetPath); err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func unsafeEmbeddedDirectory(rel string) bool {
	base := path.Base(filepath.ToSlash(rel))
	switch base {
	case ".git", ".hg", ".svn", "node_modules", "tmp", "temp", ".tmp":
		return true
	default:
		return false
	}
}

func unsafeEmbeddedFile(rel string) bool {
	rel = filepath.ToSlash(rel)
	base := path.Base(rel)
	ext := path.Ext(base)
	switch {
	case base == ".env" || strings.HasPrefix(base, ".env."):
		return true
	case ext == ".map" || ext == ".gwdk" || ext == ".go":
		return true
	case ext == ".tmp" || ext == ".temp" || strings.HasSuffix(base, "~"):
		return true
	case strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, ".swo"):
		return true
	default:
		return false
	}
}

func copyFile(sourcePath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

func validateActionRoutes(routes []ActionRoute) error {
	seen := map[string]ActionRoute{}
	for _, route := range routes {
		if strings.TrimSpace(route.ActionName) == "" {
			return fmt.Errorf("generated action route for page %q is missing action name", route.PageID)
		}
		if err := validateActionRoutePath(route.Route); err != nil {
			return fmt.Errorf("generated action %s.%s: %w", route.PageID, route.ActionName, err)
		}
		if err := validateActionRedirect(route.Redirect); err != nil {
			return fmt.Errorf("generated action %s.%s: %w", route.PageID, route.ActionName, err)
		}
		if err := validateInputFields(route); err != nil {
			return err
		}
		if err := validateRequiredFields(route); err != nil {
			return err
		}
		if previous, exists := seen[route.Route]; exists {
			return fmt.Errorf("generated action %s.%s route %q duplicates action %s.%s", route.PageID, route.ActionName, route.Route, previous.PageID, previous.ActionName)
		}
		seen[route.Route] = route
	}
	return nil
}

func validateInputFields(route ActionRoute) error {
	seen := map[string]bool{}
	for _, field := range route.InputFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("generated action %s.%s declares an empty input field", route.PageID, route.ActionName)
		}
		if seen[field] {
			return fmt.Errorf("generated action %s.%s declares duplicate input field %q", route.PageID, route.ActionName, field)
		}
		if strings.ContainsAny(field, "{}") {
			return fmt.Errorf("generated action %s.%s input field %q must be static", route.PageID, route.ActionName, field)
		}
		seen[field] = true
	}
	return nil
}

func validateRequiredFields(route ActionRoute) error {
	expected := map[string]bool{}
	for _, field := range route.InputFields {
		expected[field] = true
	}
	seen := map[string]bool{}
	for _, field := range route.RequiredFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("generated action %s.%s declares an empty required field", route.PageID, route.ActionName)
		}
		if seen[field] {
			return fmt.Errorf("generated action %s.%s declares duplicate required field %q", route.PageID, route.ActionName, field)
		}
		if !expected[field] {
			return fmt.Errorf("generated action %s.%s required field %q is not an expected input field", route.PageID, route.ActionName, field)
		}
		seen[field] = true
	}
	return nil
}

func validateActionRoutePath(value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("route %q must be an absolute path", value)
	}
	if strings.ContainsAny(value, "?#{}") {
		return fmt.Errorf("route %q must be a concrete path without query, fragment, or params", value)
	}
	return nil
}

func validateActionRedirect(value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("redirect %q must be a local absolute path", value)
	}
	if strings.HasPrefix(value, "//") {
		return fmt.Errorf("redirect %q must not be protocol-relative", value)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("redirect %q must not contain newlines", value)
	}
	return nil
}

func mainSource(actions []ActionRoute) string {
	return strings.ReplaceAll(mainSourceTemplate, "{{ACTION_HANDLER}}", actionHandlerSource(actions))
}

func actionHandlerSource(actions []ActionRoute) string {
	if len(actions) == 0 {
		return `func (handler staticHandler) action(response http.ResponseWriter, request *http.Request) bool {
	return false
}`
	}

	sorted := append([]ActionRoute(nil), actions...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Route == sorted[j].Route {
			return sorted[i].ActionName < sorted[j].ActionName
		}
		return sorted[i].Route < sorted[j].Route
	})

	var builder strings.Builder
	builder.WriteString("func (handler staticHandler) action(response http.ResponseWriter, request *http.Request) bool {\n")
	builder.WriteString("\tswitch request.URL.Path {\n")
	for _, action := range sorted {
		builder.WriteString("\tcase ")
		builder.WriteString(quote(action.Route))
		builder.WriteString(":\n")
		builder.WriteString("\t\trequest.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)\n")
		builder.WriteString("\t\tif err := request.ParseForm(); err != nil {\n")
		builder.WriteString("\t\t\tif strings.Contains(err.Error(), \"request body too large\") {\n")
		builder.WriteString("\t\t\t\twriteActionError(response, http.StatusRequestEntityTooLarge, actionErrorRequestTooLarge)\n")
		builder.WriteString("\t\t\t\treturn true\n")
		builder.WriteString("\t\t\t}\n")
		builder.WriteString("\t\t\twriteActionError(response, http.StatusBadRequest, actionErrorInvalidForm)\n")
		builder.WriteString("\t\t\treturn true\n")
		builder.WriteString("\t\t}\n")
		builder.WriteString("\t\tvalues := formValuesFromURLValues(request.PostForm)\n")
		if action.InputType != "" {
			builder.WriteString("\t\tinput, err := ")
			builder.WriteString(actionDecoderName(action))
			builder.WriteString("(values)\n")
			builder.WriteString("\t\tif err != nil {\n")
			builder.WriteString("\t\t\twriteActionError(response, http.StatusBadRequest, actionErrorInvalidForm)\n")
			builder.WriteString("\t\t\treturn true\n")
			builder.WriteString("\t\t}\n")
			builder.WriteString("\t\t_ = input\n")
			if action.ValidatesInput {
				builder.WriteString("\t\tvalidation := validateRequiredFields(input.Values, ")
				builder.WriteString(stringSliceLiteral(action.RequiredFields))
				builder.WriteString(")\n")
				builder.WriteString("\t\tif !validation.OK() {\n")
				builder.WriteString("\t\t\twriteActionError(response, http.StatusUnprocessableEntity, actionErrorValidationFailed)\n")
				builder.WriteString("\t\t\treturn true\n")
				builder.WriteString("\t\t}\n")
			}
		} else {
			builder.WriteString("\t\tif _, err := decodeExpectedFields(values, ")
			builder.WriteString(stringSliceLiteral(action.InputFields))
			builder.WriteString("); err != nil {\n")
			builder.WriteString("\t\t\twriteActionError(response, http.StatusBadRequest, actionErrorInvalidForm)\n")
			builder.WriteString("\t\t\treturn true\n")
			builder.WriteString("\t\t}\n")
		}
		builder.WriteString("\t\thttp.Redirect(response, request, ")
		builder.WriteString(quote(action.Redirect))
		builder.WriteString(", http.StatusSeeOther)\n")
		builder.WriteString("\t\treturn true\n")
	}
	builder.WriteString("\tdefault:\n")
	builder.WriteString("\t\treturn false\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}")
	builder.WriteString("\n\n")
	builder.WriteString(actionDecoderSource(sorted))
	return builder.String()
}

func actionDecoderSource(actions []ActionRoute) string {
	var builder strings.Builder
	inputTypes := uniqueInputTypes(actions)
	for _, inputType := range inputTypes {
		builder.WriteString("type ")
		builder.WriteString(inputType)
		builder.WriteString(" struct {\n\tValues formValues\n}\n\n")
	}
	for _, action := range actions {
		if action.InputType == "" {
			continue
		}
		builder.WriteString("func ")
		builder.WriteString(actionDecoderName(action))
		builder.WriteString("(values formValues) (")
		builder.WriteString(action.InputType)
		builder.WriteString(", error) {\n")
		builder.WriteString("\tdecoded, err := decodeExpectedFields(values, ")
		builder.WriteString(stringSliceLiteral(action.InputFields))
		builder.WriteString(")\n")
		builder.WriteString("\tif err != nil {\n")
		builder.WriteString("\t\treturn ")
		builder.WriteString(action.InputType)
		builder.WriteString("{}, err\n")
		builder.WriteString("\t}\n")
		builder.WriteString("\treturn ")
		builder.WriteString(action.InputType)
		builder.WriteString("{Values: decoded}, nil\n")
		builder.WriteString("}\n\n")
	}
	builder.WriteString(`type formValues map[string][]string

func formValuesFromURLValues(values map[string][]string) formValues {
	out := formValues{}
	for key, list := range values {
		out[key] = append([]string(nil), list...)
	}
	return out
}

func decodeExpectedFields(values formValues, expected []string) (formValues, error) {
	allowed := map[string]bool{}
	for _, field := range expected {
		if field == "" {
			return nil, formDecodeError("expected form field name is required")
		}
		if allowed[field] {
			return nil, formDecodeError("duplicate expected form field")
		}
		allowed[field] = true
	}
	for field := range values {
		if !allowed[field] {
			return nil, formDecodeError("unexpected form field")
		}
	}
	out := formValues{}
	for _, field := range expected {
		if submitted, ok := values[field]; ok {
			out[field] = append([]string(nil), submitted...)
		}
	}
	return out, nil
}

func validateRequiredFields(values formValues, required []string) validationResult {
	var result validationResult
	for _, field := range required {
		if !hasSubmittedValue(values[field]) {
			result.Add(field, "required")
		}
	}
	return result
}

func hasSubmittedValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

type validationError struct {
	Field   string
	Message string
}

type validationResult struct {
	Errors []validationError
}

func (result *validationResult) Add(field, message string) {
	result.Errors = append(result.Errors, validationError{Field: field, Message: message})
}

func (result validationResult) OK() bool {
	return len(result.Errors) == 0
}

type formDecodeError string

func (err formDecodeError) Error() string {
	return string(err)
}

const (
	actionErrorInvalidForm      = "invalid form"
	actionErrorRequestTooLarge  = "request body too large"
	actionErrorValidationFailed = "validation failed"
)

func writeActionError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Cache-Control", "no-store")
	http.Error(response, message, status)
}
`)
	return strings.TrimSpace(builder.String())
}

func uniqueInputTypes(actions []ActionRoute) []string {
	seen := map[string]bool{}
	var types []string
	for _, action := range actions {
		if action.InputType == "" || seen[action.InputType] {
			continue
		}
		seen[action.InputType] = true
		types = append(types, action.InputType)
	}
	sort.Strings(types)
	return types
}

func actionDecoderName(action ActionRoute) string {
	return "decode" + exportedIdentifier(action.PageID) + exportedIdentifier(action.ActionName) + "Input"
}

func exportedIdentifier(value string) string {
	var builder strings.Builder
	upperNext := true
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
		if !valid {
			upperNext = true
			continue
		}
		if builder.Len() == 0 && char >= '0' && char <= '9' {
			builder.WriteByte('X')
		}
		if upperNext && char >= 'a' && char <= 'z' {
			char -= 'a' - 'A'
		}
		builder.WriteRune(char)
		upperNext = false
	}
	if builder.Len() == 0 {
		return "Action"
	}
	return builder.String()
}

func stringSliceLiteral(values []string) string {
	if len(values) == 0 {
		return "nil"
	}
	var builder strings.Builder
	builder.WriteString("[]string{")
	for index, value := range values {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("%q", value))
	}
	builder.WriteString("}")
	return builder.String()
}

func quote(value string) string {
	return fmt.Sprintf("%q", path.Clean("/"+value))
}

const moduleSource = `module gowdk-generated-app

go 1.26
`

const mainSourceTemplate = `package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"embed"
	"io/fs"
	"log"
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

func main() {
	root, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		log.Fatal(err)
	}

	addr := env("GOWDK_ADDR", "127.0.0.1:8080")
	identity := instanceIdentity()
	assets := loadAssetManifest(root)
	server := &http.Server{
		Addr:              addr,
		Handler:           staticHandler{root: root, identity: identity, assets: assets},
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

	payload, info, ok := handler.staticFile(request.URL.Path)
	if !ok {
		http.NotFound(response, request)
		return
	}
	http.ServeContent(response, request, info.Name(), info.ModTime(), bytes.NewReader(payload))
}

{{ACTION_HANDLER}}

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
