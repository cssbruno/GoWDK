package project

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cssbruno/gowdk"
)

const (
	executableBridgeProtocolVersion = 1
	executableBridgeMaxPayload      = 4 << 20
	executableBridgeMaxStderr       = 64 << 10
	executableBridgeTimeout         = 30 * time.Second
)

type bridgeRequest struct {
	Version int             `json:"version"`
	ID      uint64          `json:"id"`
	Method  string          `json:"method"`
	Index   int             `json:"index,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type bridgeResponse struct {
	Version int             `json:"version"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *bridgeError    `json:"error,omitempty"`
}

type bridgeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ExecutableBridgeError is a stable, code-addressable failure returned by the
// versioned executable config/extension host.
type ExecutableBridgeError struct {
	Code    string
	Method  string
	Message string
	Cause   error
}

func (err *ExecutableBridgeError) Error() string {
	if err == nil {
		return ""
	}
	prefix := "executable config bridge"
	if err.Method != "" {
		prefix += " " + err.Method
	}
	message := strings.TrimSpace(err.Message)
	if message == "" && err.Cause != nil {
		message = err.Cause.Error()
	}
	if err.Code != "" {
		return fmt.Sprintf("%s: %s (%s)", prefix, message, err.Code)
	}
	return prefix + ": " + message
}

func (err *ExecutableBridgeError) Unwrap() error { return err.Cause }

func bridgeFailure(code, method, message string, cause error) error {
	return &ExecutableBridgeError{Code: code, Method: method, Message: message, Cause: cause}
}

type configBridge struct {
	key        string
	configPath string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	stderr     *boundedBuffer
	mu         sync.Mutex
	nextID     uint64
	dead       bool
}

type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (buffer *boundedBuffer) Write(payload []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	remaining := buffer.limit - len(buffer.data)
	if remaining > 0 {
		if remaining > len(payload) {
			remaining = len(payload)
		}
		buffer.data = append(buffer.data, payload[:remaining]...)
	}
	return len(payload), nil
}

func (buffer *boundedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return strings.TrimSpace(string(buffer.data))
}

var executableBridges = struct {
	sync.Mutex
	items map[string]*configBridge
}{items: map[string]*configBridge{}}

type executableConfig struct {
	AppName    string                       `json:"appName"`
	Source     gowdk.SourceConfig           `json:"source"`
	Modules    []gowdk.ModuleConfig         `json:"modules"`
	Render     gowdk.RenderConfig           `json:"render"`
	I18N       gowdk.I18NConfig             `json:"i18n"`
	Env        gowdk.EnvConfig              `json:"env"`
	Lifecycle  gowdk.LifecycleConfig        `json:"lifecycle"`
	Build      gowdk.BuildConfig            `json:"build"`
	CSS        gowdk.CSSConfig              `json:"css"`
	Features   gowdk.FeatureConfig          `json:"features"`
	Interop    gowdk.InteropConfig          `json:"interop"`
	Addons     []executableAddonDetails     `json:"addons"`
	Extensions []executableExtensionDetails `json:"extensions"`
}

type executableExtensionDetails struct {
	Index        int                         `json:"index"`
	Descriptor   gowdk.ExtensionDescriptor   `json:"descriptor"`
	Capabilities []executableAddonCapability `json:"capabilities,omitempty"`
}

type executableAddonDetails struct {
	Index        int                         `json:"index"`
	Name         string                      `json:"name"`
	Features     []gowdk.Feature             `json:"features"`
	Capabilities []executableAddonCapability `json:"capabilities,omitempty"`
}

type executableAddonCapability struct {
	Name               string                   `json:"name"`
	Required           bool                     `json:"required,omitempty"`
	GoBlockTargets     []string                 `json:"goBlockTargets,omitempty"`
	SEOOptions         gowdk.SEOOptions         `json:"seoOptions,omitempty"`
	AuthSessionOptions gowdk.AuthSessionOptions `json:"authSessionOptions,omitempty"`
}

const (
	executableCapabilityCSSProcessor        = "gowdk.css-processor"
	executableCapabilityGoBlockConsumer     = "gowdk.go-block-consumer"
	executableCapabilitySEOProvider         = "gowdk.seo-provider"
	executableCapabilityAuthSessionProvider = "gowdk.auth-session-provider"
)

type executableCSSResponse struct {
	Result gowdk.CSSResult `json:"result"`
	Error  string          `json:"error"`
}

type executableGoBlockRequest struct {
	Target  gowdk.GoBlockTarget  `json:"target"`
	Context gowdk.GoBlockContext `json:"context"`
}

type executableGoBlockResponse struct {
	Diagnostics []gowdk.GoBlockDiagnostic `json:"diagnostics,omitempty"`
	Files       []gowdk.GoBlockFile       `json:"files,omitempty"`
	Error       string                    `json:"error,omitempty"`
}

type goListPackage struct {
	ImportPath string `json:"ImportPath"`
	Name       string `json:"Name"`
	Dir        string `json:"Dir"`
	Module     *struct {
		Dir string `json:"Dir"`
	} `json:"Module"`
}

type executableAddon struct {
	configPath         string
	environment        []string
	index              int
	lane               string
	name               string
	features           []gowdk.Feature
	cssProcessor       bool
	goBlockConsumer    bool
	goBlockTargets     []string
	seoProvider        bool
	seoOptions         gowdk.SEOOptions
	authSession        bool
	authSessionOptions gowdk.AuthSessionOptions
}

type executableExtension struct {
	addon      executableAddon
	descriptor gowdk.ExtensionDescriptor
}

type executableCSSCapability struct {
	addon executableAddon
}

type executableGoBlockCapability struct {
	addon executableAddon
}

type executableSEOCapability struct {
	options gowdk.SEOOptions
}

type executableAuthSessionCapability struct {
	options gowdk.AuthSessionOptions
}

func loadExecutableConfig(configPath string, environment []string) (gowdk.Config, error) {
	payload, err := runConfigHelper(configPath, environment, "config", nil)
	if err != nil {
		return gowdk.Config{}, err
	}
	var wire executableConfig
	if err := json.Unmarshal(payload, &wire); err != nil {
		return gowdk.Config{}, fmt.Errorf("decode executable config: %w", err)
	}
	return configFromExecutableWire(configPath, environment, wire)
}

func configFromExecutableWire(configPath string, environment []string, wire executableConfig) (gowdk.Config, error) {
	config := gowdk.Config{
		AppName:   wire.AppName,
		Source:    wire.Source,
		Modules:   wire.Modules,
		Render:    wire.Render,
		I18N:      wire.I18N,
		Env:       wire.Env,
		Lifecycle: wire.Lifecycle,
		Build:     wire.Build,
		CSS:       wire.CSS,
		Features:  wire.Features,
		Interop:   wire.Interop,
	}
	for _, addon := range wire.Addons {
		if len(addon.Capabilities) == 0 && executableFeaturesAreBuiltIn(addon.Features) {
			for _, feature := range addon.Features {
				enableExecutableBuiltInFeature(&config.Features, feature)
			}
			config.Addons = append(config.Addons, gowdk.NewBuiltinAddon(addon.Name, addon.Features...))
			continue
		}
		proxy := executableAddon{
			configPath:  configPath,
			environment: append([]string(nil), environment...),
			index:       addon.Index,
			name:        addon.Name,
			features:    append([]gowdk.Feature(nil), addon.Features...),
		}
		for _, capability := range addon.Capabilities {
			switch capability.Name {
			case executableCapabilityCSSProcessor:
				proxy.cssProcessor = true
			case executableCapabilityGoBlockConsumer:
				proxy.goBlockConsumer = true
				proxy.goBlockTargets = append([]string(nil), capability.GoBlockTargets...)
			case executableCapabilitySEOProvider:
				proxy.seoProvider = true
				proxy.seoOptions = cloneExecutableSEOOptions(capability.SEOOptions)
			case executableCapabilityAuthSessionProvider:
				proxy.authSession = true
				proxy.authSessionOptions = capability.AuthSessionOptions
			default:
				if capability.Required {
					return gowdk.Config{}, fmt.Errorf("addon %q requires unsupported executable capability %q", addon.Name, capability.Name)
				}
			}
		}
		config.Addons = append(config.Addons, proxy)
	}
	for _, extension := range wire.Extensions {
		proxy := executableAddon{configPath: configPath, environment: append([]string(nil), environment...), index: extension.Index, lane: "extension", name: extension.Descriptor.Name}
		for _, capability := range extension.Capabilities {
			switch capability.Name {
			case executableCapabilityCSSProcessor:
				proxy.cssProcessor = true
			case executableCapabilityGoBlockConsumer:
				proxy.goBlockConsumer = true
				proxy.goBlockTargets = append([]string(nil), capability.GoBlockTargets...)
			case executableCapabilitySEOProvider:
				proxy.seoProvider = true
				proxy.seoOptions = cloneExecutableSEOOptions(capability.SEOOptions)
			case executableCapabilityAuthSessionProvider:
				proxy.authSession = true
				proxy.authSessionOptions = capability.AuthSessionOptions
			default:
				if capability.Required {
					return gowdk.Config{}, fmt.Errorf("extension %q requires unsupported executable capability %q", extension.Descriptor.Name, capability.Name)
				}
			}
		}
		config.Extensions = append(config.Extensions, executableExtension{addon: proxy, descriptor: extension.Descriptor})
	}
	return config, nil
}

func (extension executableExtension) Name() string { return extension.descriptor.Name }

func (extension executableExtension) Descriptor() gowdk.ExtensionDescriptor {
	return extension.descriptor
}

func (extension executableExtension) ExtensionCapabilities() gowdk.AddonCapabilities {
	return extension.addon.AddonCapabilities()
}

func executableFeaturesAreBuiltIn(features []gowdk.Feature) bool {
	if len(features) == 0 {
		return false
	}
	for _, feature := range features {
		switch feature {
		case gowdk.FeatureSPA, gowdk.FeatureActions, gowdk.FeaturePartial, gowdk.FeatureSSR,
			gowdk.FeatureAPI, gowdk.FeatureEmbed, gowdk.FeatureCSS, gowdk.FeatureRateLimit,
			gowdk.FeatureContracts, gowdk.FeatureRealtime, gowdk.FeatureAuth, gowdk.FeatureDB,
			gowdk.FeatureSEO, gowdk.FeatureObservability:
		default:
			return false
		}
	}
	return true
}

func enableExecutableBuiltInFeature(config *gowdk.FeatureConfig, feature gowdk.Feature) {
	switch feature {
	case gowdk.FeatureSPA:
		config.SPA = true
	case gowdk.FeatureActions:
		config.Actions = true
	case gowdk.FeaturePartial:
		config.Partial = true
	case gowdk.FeatureSSR:
		config.SSR = true
	case gowdk.FeatureAPI:
		config.API = true
	case gowdk.FeatureEmbed:
		config.Embed = true
	case gowdk.FeatureCSS:
		config.CSS = true
	case gowdk.FeatureRateLimit:
		config.RateLimit = true
	case gowdk.FeatureContracts:
		config.Contracts = true
	case gowdk.FeatureRealtime:
		config.Realtime = true
	case gowdk.FeatureAuth:
		config.Auth.Enabled = true
	case gowdk.FeatureDB:
		config.DB = true
	case gowdk.FeatureSEO:
		config.SEO.Enabled = true
	case gowdk.FeatureObservability:
		config.Observability = true
	}
}

func (addon executableAddon) Name() string {
	return addon.name
}

func (addon executableAddon) Features() []gowdk.Feature {
	return append([]gowdk.Feature(nil), addon.features...)
}

func (addon executableAddon) AddonCapabilities() gowdk.AddonCapabilities {
	var capabilities gowdk.AddonCapabilities
	if addon.cssProcessor {
		capabilities.CSSProcessor = executableCSSCapability{addon: addon}
	}
	if addon.goBlockConsumer {
		capabilities.GoBlockConsumer = executableGoBlockCapability{addon: addon}
	}
	if addon.seoProvider {
		capabilities.SEOProvider = executableSEOCapability{options: cloneExecutableSEOOptions(addon.seoOptions)}
	}
	if addon.authSession {
		capabilities.AuthSessionProvider = executableAuthSessionCapability{options: addon.authSessionOptions}
	}
	return capabilities
}

func (capability executableAuthSessionCapability) AuthSessionOptions() gowdk.AuthSessionOptions {
	return capability.options
}

func (capability executableSEOCapability) SEOOptions() gowdk.SEOOptions {
	return cloneExecutableSEOOptions(capability.options)
}

func cloneExecutableSEOOptions(options gowdk.SEOOptions) gowdk.SEOOptions {
	options.Disallow = append([]string(nil), options.Disallow...)
	options.ExtraURLs = append([]gowdk.SEOURL(nil), options.ExtraURLs...)
	options.ExtraURLProvider = nil
	return options
}

func (capability executableCSSCapability) Name() string {
	return capability.addon.Name()
}

func (capability executableCSSCapability) Features() []gowdk.Feature {
	return capability.addon.Features()
}

func (capability executableCSSCapability) ProcessCSS(cssContext gowdk.CSSContext) (gowdk.CSSResult, error) {
	return capability.ProcessCSSContext(context.Background(), cssContext)
}

func (capability executableCSSCapability) ProcessCSSContext(ctx context.Context, cssContext gowdk.CSSContext) (gowdk.CSSResult, error) {
	return capability.addon.processCSSContext(ctx, cssContext)
}

func (addon executableAddon) processCSSContext(ctx context.Context, cssContext gowdk.CSSContext) (gowdk.CSSResult, error) {
	input, err := json.Marshal(cssContext)
	if err != nil {
		return gowdk.CSSResult{}, err
	}
	method := "css"
	if addon.lane == "extension" {
		method = "extension-css"
	}
	payload, err := runConfigHelperContext(ctx, addon.configPath, addon.environment, method, input, strconv.Itoa(addon.index))
	if err != nil {
		return gowdk.CSSResult{}, err
	}
	var response executableCSSResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return gowdk.CSSResult{}, fmt.Errorf("decode css processor response: %w", err)
	}
	if response.Error != "" {
		return gowdk.CSSResult{}, fmt.Errorf("%s", response.Error)
	}
	return response.Result, nil
}

func (capability executableGoBlockCapability) GoBlockTargets() []string {
	return capability.addon.goBlockTargetsCopy()
}

func (addon executableAddon) goBlockTargetsCopy() []string {
	return append([]string(nil), addon.goBlockTargets...)
}

func (capability executableGoBlockCapability) ValidateGoBlock(target gowdk.GoBlockTarget, blockContext gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic {
	return capability.ValidateGoBlockContext(context.Background(), target, blockContext)
}

func (capability executableGoBlockCapability) ValidateGoBlockContext(ctx context.Context, target gowdk.GoBlockTarget, blockContext gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic {
	return capability.addon.validateGoBlockContext(ctx, target, blockContext)
}

func (addon executableAddon) validateGoBlockContext(ctx context.Context, target gowdk.GoBlockTarget, blockContext gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic {
	input, err := json.Marshal(executableGoBlockRequest{Target: target, Context: blockContext})
	if err != nil {
		return addon.goBlockProxyDiagnostics(target, fmt.Sprintf("encode addon go block validation request: %v", err))
	}
	method := "go-block-validate"
	if addon.lane == "extension" {
		method = "extension-go-block-validate"
	}
	payload, err := runConfigHelperContext(ctx, addon.configPath, addon.environment, method, input, strconv.Itoa(addon.index))
	if err != nil {
		return addon.goBlockProxyDiagnostics(target, fmt.Sprintf("run addon go block validation: %v", err))
	}
	var response executableGoBlockResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return addon.goBlockProxyDiagnostics(target, fmt.Sprintf("decode addon go block validation response: %v", err))
	}
	if response.Error != "" {
		return addon.goBlockProxyDiagnostics(target, response.Error)
	}
	return response.Diagnostics
}

func (capability executableGoBlockCapability) GeneratedGo(target gowdk.GoBlockTarget, blockContext gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error) {
	return capability.GeneratedGoContext(context.Background(), target, blockContext)
}

func (capability executableGoBlockCapability) GeneratedGoContext(ctx context.Context, target gowdk.GoBlockTarget, blockContext gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error) {
	return capability.addon.generatedGoContext(ctx, target, blockContext)
}

func (addon executableAddon) generatedGoContext(ctx context.Context, target gowdk.GoBlockTarget, blockContext gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error) {
	input, err := json.Marshal(executableGoBlockRequest{Target: target, Context: blockContext})
	if err != nil {
		return nil, err
	}
	method := "go-block-generate"
	if addon.lane == "extension" {
		method = "extension-go-block-generate"
	}
	payload, err := runConfigHelperContext(ctx, addon.configPath, addon.environment, method, input, strconv.Itoa(addon.index))
	if err != nil {
		return nil, err
	}
	var response executableGoBlockResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode addon go block generation response: %w", err)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("%s", response.Error)
	}
	seen := map[string]bool{}
	for index, file := range response.Files {
		path := filepath.Clean(strings.TrimSpace(file.Path))
		if path == "." || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("addon %q generated file %d has unsafe path %q", addon.name, index, file.Path)
		}
		if seen[path] {
			return nil, fmt.Errorf("addon %q generated duplicate file path %q", addon.name, file.Path)
		}
		seen[path] = true
		response.Files[index].Path = filepath.ToSlash(path)
	}
	return response.Files, nil
}

func (addon executableAddon) goBlockProxyDiagnostics(target gowdk.GoBlockTarget, message string) []gowdk.GoBlockDiagnostic {
	return []gowdk.GoBlockDiagnostic{{
		Code:    "addon_go_block_diagnostic",
		Message: fmt.Sprintf("addon %q go block proxy failed: %s", addon.name, message),
		Span:    target.Span,
	}}
}

func runConfigHelper(configPath string, environment []string, command string, input []byte, args ...string) ([]byte, error) {
	return runConfigHelperContext(context.Background(), configPath, environment, command, input, args...)
}

func runConfigHelperContext(parent context.Context, configPath string, environment []string, command string, input []byte, args ...string) ([]byte, error) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, executableBridgeTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, bridgeFailure("deadline_exceeded", command, "request canceled before dispatch", err)
	}
	bridge, err := executableConfigBridge(ctx, configPath, environment)
	if err != nil {
		return nil, err
	}
	index := 0
	if len(args) > 0 {
		index, err = strconv.Atoi(args[0])
		if err != nil {
			return nil, fmt.Errorf("invalid executable config addon index: %w", err)
		}
	}
	return bridge.request(ctx, command, index, input)
}

func executableConfigBridge(ctx context.Context, configPath string, environment []string) (*configBridge, error) {
	absolute, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	configPayload, err := os.ReadFile(absolute)
	if err != nil {
		return nil, err
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte(absolute))
	_, _ = digest.Write(configPayload)
	for _, item := range environment {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(item))
	}
	key := fmt.Sprintf("%x", digest.Sum(nil))
	executableBridges.Lock()
	if bridge := executableBridges.items[key]; bridge != nil && !bridge.dead {
		executableBridges.Unlock()
		return bridge, nil
	}
	executableBridges.Unlock()

	packageInfo, err := loadConfigPackage(configPath, environment)
	if err != nil {
		return nil, err
	}
	helperDir, err := makeConfigHelperDir(packageInfo)
	if err != nil {
		return nil, err
	}
	source, err := configHelperSource(packageInfo.ImportPath)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(helperDir, "main.go"), []byte(source), 0o644); err != nil {
		return nil, err
	}

	binaryPath := filepath.Join(helperDir, "gowdk-config-host")
	build := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", binaryPath, ".")
	build.Dir = helperDir
	if environment != nil {
		build.Env = append([]string(nil), environment...)
	}
	var buildStderr boundedBuffer
	buildStderr.limit = executableBridgeMaxStderr
	build.Stderr = &buildStderr
	if err := build.Run(); err != nil {
		message := redactBridgeOutput(buildStderr.String(), environment)
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("build executable config host: %s", message)
	}
	cmd := exec.Command(binaryPath)
	cmd.Dir = packageInfo.Dir
	if environment != nil {
		cmd.Env = append([]string(nil), environment...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &boundedBuffer{limit: executableBridgeMaxStderr}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start executable config host: %w", err)
	}
	bridge := &configBridge{key: key, configPath: absolute, cmd: cmd, stdin: stdin, stdout: bufio.NewReaderSize(stdout, executableBridgeMaxPayload+1), stderr: stderr}
	if _, err := bridge.request(ctx, "handshake", 0, nil); err != nil {
		terminateBridgeProcess(cmd)
		return nil, err
	}
	executableBridges.Lock()
	if existing := executableBridges.items[key]; existing != nil && !existing.dead {
		executableBridges.Unlock()
		terminateBridgeProcess(cmd)
		return existing, nil
	}
	for cacheKey, stale := range executableBridges.items {
		if stale != nil && stale.configPath == absolute && cacheKey != key {
			stale.mu.Lock()
			stale.dead = true
			terminateBridgeProcess(stale.cmd)
			stale.mu.Unlock()
			delete(executableBridges.items, cacheKey)
		}
	}
	executableBridges.items[key] = bridge
	executableBridges.Unlock()
	return bridge, nil
}

func (bridge *configBridge) request(ctx context.Context, method string, index int, payload []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, bridgeFailure("deadline_exceeded", method, "request canceled before dispatch", err)
	}
	if len(payload) > executableBridgeMaxPayload {
		return nil, bridgeFailure("request_too_large", method, fmt.Sprintf("request exceeds %d bytes", executableBridgeMaxPayload), nil)
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.dead {
		return nil, bridgeFailure("host_unavailable", method, "host is unavailable", nil)
	}
	bridge.nextID++
	requestPayload, err := json.Marshal(bridgeRequest{Version: executableBridgeProtocolVersion, ID: bridge.nextID, Method: method, Index: index, Payload: payload})
	if err != nil {
		return nil, err
	}
	if len(requestPayload) > executableBridgeMaxPayload {
		return nil, bridgeFailure("request_too_large", method, fmt.Sprintf("request envelope exceeds %d bytes", executableBridgeMaxPayload), nil)
	}
	if _, err := bridge.stdin.Write(append(requestPayload, '\n')); err != nil {
		bridge.dead = true
		terminateBridgeProcess(bridge.cmd)
		return nil, bridgeFailure("host_write_failed", method, "write request", err)
	}
	type readResult struct {
		line []byte
		err  error
	}
	read := make(chan readResult, 1)
	go func() {
		line, readErr := bridge.stdout.ReadSlice('\n')
		read <- readResult{line: append([]byte(nil), line...), err: readErr}
	}()
	select {
	case <-ctx.Done():
		bridge.dead = true
		terminateBridgeProcess(bridge.cmd)
		return nil, bridgeFailure("deadline_exceeded", method, "request timed out", ctx.Err())
	case result := <-read:
		if result.err != nil {
			bridge.dead = true
			terminateBridgeProcess(bridge.cmd)
			if errors.Is(result.err, bufio.ErrBufferFull) {
				return nil, bridgeFailure("response_too_large", method, fmt.Sprintf("response exceeds %d bytes", executableBridgeMaxPayload), result.err)
			}
			message := redactBridgeOutput(bridge.stderr.String(), bridge.cmd.Env)
			if message != "" {
				return nil, bridgeFailure("host_read_failed", method, message, result.err)
			}
			return nil, bridgeFailure("host_read_failed", method, "read response", result.err)
		}
		if len(result.line) > executableBridgeMaxPayload {
			bridge.dead = true
			terminateBridgeProcess(bridge.cmd)
			return nil, bridgeFailure("response_too_large", method, fmt.Sprintf("response exceeds %d bytes", executableBridgeMaxPayload), nil)
		}
		var response bridgeResponse
		if err := json.Unmarshal(result.line, &response); err != nil {
			bridge.dead = true
			terminateBridgeProcess(bridge.cmd)
			return nil, bridgeFailure("malformed_response", method, "decode response", err)
		}
		if response.Version != executableBridgeProtocolVersion || response.ID != bridge.nextID {
			bridge.dead = true
			terminateBridgeProcess(bridge.cmd)
			return nil, bridgeFailure("protocol_mismatch", method, "response version or request id mismatch", nil)
		}
		if response.Error != nil {
			return nil, bridgeFailure(response.Error.Code, method, response.Error.Message, nil)
		}
		return append([]byte(nil), response.Result...), nil
	}
}

func terminateBridgeProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

func redactBridgeOutput(message string, environment []string) string {
	for _, item := range environment {
		name, value, ok := strings.Cut(item, "=")
		if ok && value != "" && (len(value) >= 4 || bridgeSecretEnvName(name)) {
			message = strings.ReplaceAll(message, value, "[REDACTED]")
		}
	}
	return message
}

func bridgeSecretEnvName(name string) bool {
	name = strings.ToUpper(strings.TrimSpace(name))
	for _, marker := range []string{"SECRET", "TOKEN", "PASSWORD", "PASSWD", "PRIVATE_KEY", "CREDENTIAL", "DATABASE_URL"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func loadConfigPackage(configPath string, environment []string) (goListPackage, error) {
	absolute, err := filepath.Abs(configPath)
	if err != nil {
		return goListPackage{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), executableBridgeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-buildvcs=false", "-json", ".")
	cmd.Dir = filepath.Dir(absolute)
	if environment != nil {
		cmd.Env = append([]string(nil), environment...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return goListPackage{}, fmt.Errorf("go list config package: %s", message)
	}
	var packageInfo goListPackage
	if err := json.Unmarshal(output, &packageInfo); err != nil {
		return goListPackage{}, err
	}
	if packageInfo.ImportPath == "" {
		return goListPackage{}, fmt.Errorf("config package has no import path")
	}
	if packageInfo.Name == "main" {
		return goListPackage{}, fmt.Errorf("config package %s is package main and cannot be imported", packageInfo.ImportPath)
	}
	if packageInfo.Module == nil || packageInfo.Module.Dir == "" {
		return goListPackage{}, fmt.Errorf("config package %s is not inside a Go module", packageInfo.ImportPath)
	}
	return packageInfo, nil
}

func makeConfigHelperDir(packageInfo goListPackage) (string, error) {
	cacheRoot := filepath.Join(packageInfo.Module.Dir, ".gowdk")
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(cacheRoot, "config-loader-*")
}

const configHelperImportPlaceholder = "gowdk.local/config-placeholder"

func configHelperSource(configImportPath string) (string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "gowdk-config-helper.go", configHelperSourceTemplate, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return "", fmt.Errorf("parse config helper source: %w", err)
	}
	replaced := false
	for _, item := range file.Imports {
		if item.Name == nil || item.Name.Name != "configpkg" {
			continue
		}
		importPath, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			return "", fmt.Errorf("parse config helper import path: %w", err)
		}
		if importPath != configHelperImportPlaceholder {
			continue
		}
		item.Path.Value = strconv.Quote(configImportPath)
		replaced = true
	}
	if !replaced {
		return "", fmt.Errorf("config helper source is missing config package import placeholder")
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fileSet, file); err != nil {
		return "", fmt.Errorf("format config helper source: %w", err)
	}
	return buffer.String(), nil
}

const configHelperSourceTemplate = `package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cssbruno/gowdk"
	configpkg "gowdk.local/config-placeholder"
)

type executableConfig struct {
	AppName   string                    ` + "`json:\"appName\"`" + `
	Source    gowdk.SourceConfig       ` + "`json:\"source\"`" + `
	Modules   []gowdk.ModuleConfig     ` + "`json:\"modules\"`" + `
	Render    gowdk.RenderConfig       ` + "`json:\"render\"`" + `
	I18N      gowdk.I18NConfig         ` + "`json:\"i18n\"`" + `
	Env       gowdk.EnvConfig          ` + "`json:\"env\"`" + `
	Lifecycle gowdk.LifecycleConfig    ` + "`json:\"lifecycle\"`" + `
	Build     gowdk.BuildConfig        ` + "`json:\"build\"`" + `
	CSS       gowdk.CSSConfig          ` + "`json:\"css\"`" + `
	Features  gowdk.FeatureConfig      ` + "`json:\"features\"`" + `
	Interop   gowdk.InteropConfig      ` + "`json:\"interop\"`" + `
	Addons    []executableAddonDetails ` + "`json:\"addons\"`" + `
	Extensions []executableExtensionDetails ` + "`json:\"extensions\"`" + `
}

type executableExtensionDetails struct {
	Index int ` + "`json:\"index\"`" + `
	Descriptor gowdk.ExtensionDescriptor ` + "`json:\"descriptor\"`" + `
	Capabilities []executableAddonCapability ` + "`json:\"capabilities,omitempty\"`" + `
}

type executableAddonDetails struct {
	Index        int                         ` + "`json:\"index\"`" + `
	Name         string                      ` + "`json:\"name\"`" + `
	Features     []gowdk.Feature             ` + "`json:\"features\"`" + `
	Capabilities []executableAddonCapability ` + "`json:\"capabilities,omitempty\"`" + `
}

type executableAddonCapability struct {
	Name               string                   ` + "`json:\"name\"`" + `
	Required           bool                     ` + "`json:\"required,omitempty\"`" + `
	GoBlockTargets     []string                 ` + "`json:\"goBlockTargets,omitempty\"`" + `
	SEOOptions         gowdk.SEOOptions         ` + "`json:\"seoOptions,omitempty\"`" + `
	AuthSessionOptions gowdk.AuthSessionOptions ` + "`json:\"authSessionOptions,omitempty\"`" + `
}

const (
	executableCapabilityCSSProcessor        = "gowdk.css-processor"
	executableCapabilityGoBlockConsumer     = "gowdk.go-block-consumer"
	executableCapabilitySEOProvider         = "gowdk.seo-provider"
	executableCapabilityAuthSessionProvider = "gowdk.auth-session-provider"
)

type executableCSSResponse struct {
	Result gowdk.CSSResult ` + "`json:\"result\"`" + `
	Error  string          ` + "`json:\"error\"`" + `
}

type executableGoBlockRequest struct {
	Target  gowdk.GoBlockTarget  ` + "`json:\"target\"`" + `
	Context gowdk.GoBlockContext ` + "`json:\"context\"`" + `
}

type executableGoBlockResponse struct {
	Diagnostics []gowdk.GoBlockDiagnostic ` + "`json:\"diagnostics,omitempty\"`" + `
	Files       []gowdk.GoBlockFile       ` + "`json:\"files,omitempty\"`" + `
	Error       string                    ` + "`json:\"error,omitempty\"`" + `
}

const bridgeProtocolVersion = 1

type bridgeRequest struct {
	Version int             ` + "`json:\"version\"`" + `
	ID      uint64          ` + "`json:\"id\"`" + `
	Method  string          ` + "`json:\"method\"`" + `
	Index   int             ` + "`json:\"index,omitempty\"`" + `
	Payload json.RawMessage ` + "`json:\"payload,omitempty\"`" + `
}

type bridgeResponse struct {
	Version int         ` + "`json:\"version\"`" + `
	ID      uint64      ` + "`json:\"id\"`" + `
	Result  any         ` + "`json:\"result,omitempty\"`" + `
	Error   *bridgeError ` + "`json:\"error,omitempty\"`" + `
}

type bridgeError struct {
	Code string ` + "`json:\"code\"`" + `
	Message string ` + "`json:\"message\"`" + `
}

func main() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for {
		var request bridgeRequest
		if err := decoder.Decode(&request); err != nil {
			if err.Error() == "EOF" { return }
			_ = encoder.Encode(bridgeResponse{Version: bridgeProtocolVersion, Error: &bridgeError{Code: "decode_request", Message: err.Error()}})
			return
		}
		response := bridgeResponse{Version: bridgeProtocolVersion, ID: request.ID}
		if request.Version != bridgeProtocolVersion {
			response.Error = &bridgeError{Code: "protocol_mismatch", Message: fmt.Sprintf("host protocol %d does not support client protocol %d", bridgeProtocolVersion, request.Version)}
		} else {
			response.Result, response.Error = dispatch(request)
		}
		if err := encoder.Encode(response); err != nil { return }
	}
}

func dispatch(request bridgeRequest) (any, *bridgeError) {
	switch request.Method {
	case "handshake":
		return map[string]any{"protocolVersion": bridgeProtocolVersion}, nil
	case "config":
		return configWire(), nil
	case "css":
		return processCSS(request.Index, request.Payload), nil
	case "go-block-validate":
		return validateGoBlock(request.Index, request.Payload), nil
	case "go-block-generate":
		return generateGoBlock(request.Index, request.Payload), nil
	case "extension-css":
		return processExtensionCSS(request.Index, request.Payload), nil
	case "extension-go-block-validate":
		return validateExtensionGoBlock(request.Index, request.Payload), nil
	case "extension-go-block-generate":
		return generateExtensionGoBlock(request.Index, request.Payload), nil
	default:
		return nil, &bridgeError{Code: "unknown_method", Message: "unknown method " + request.Method}
	}
}

func configWire() executableConfig {
	config := configpkg.Config
	wire := executableConfig{
		AppName:   config.AppName,
		Source:    config.Source,
		Modules:   config.Modules,
		Render:    config.Render,
		I18N:      config.I18N,
		Env:       config.Env,
		Lifecycle: config.Lifecycle,
		Build:     config.Build,
		CSS:       config.CSS,
		Features:  config.Features,
		Interop:   config.Interop,
	}
	for index, addon := range config.Addons {
		resolved := gowdk.ResolveAddonCapabilities(addon)
		var capabilities []executableAddonCapability
		if resolved.CSSProcessor != nil {
			capabilities = append(capabilities, executableAddonCapability{
				Name:     executableCapabilityCSSProcessor,
				Required: true,
			})
		}
		if resolved.GoBlockConsumer != nil {
			capabilities = append(capabilities, executableAddonCapability{
				Name:           executableCapabilityGoBlockConsumer,
				Required:       true,
				GoBlockTargets: resolved.GoBlockConsumer.GoBlockTargets(),
			})
		}
		if resolved.SEOProvider != nil {
			capabilities = append(capabilities, executableAddonCapability{
				Name:       executableCapabilitySEOProvider,
				Required:   true,
				SEOOptions: resolved.SEOProvider.SEOOptions(),
			})
		}
		if resolved.AuthSessionProvider != nil {
			capabilities = append(capabilities, executableAddonCapability{
				Name:               executableCapabilityAuthSessionProvider,
				Required:           true,
				AuthSessionOptions: resolved.AuthSessionProvider.AuthSessionOptions(),
			})
		}
		wire.Addons = append(wire.Addons, executableAddonDetails{
			Index:        index,
			Name:         addon.Name(),
			Features:     addon.Features(),
			Capabilities: capabilities,
		})
	}
	for index, extension := range config.Extensions {
		resolved := gowdk.ResolveExtensionCapabilities(extension)
		var capabilities []executableAddonCapability
		if resolved.CSSProcessor != nil {
			capabilities = append(capabilities, executableAddonCapability{Name: executableCapabilityCSSProcessor, Required: true})
		}
		if resolved.GoBlockConsumer != nil {
			capabilities = append(capabilities, executableAddonCapability{Name: executableCapabilityGoBlockConsumer, Required: true, GoBlockTargets: resolved.GoBlockConsumer.GoBlockTargets()})
		}
		if resolved.SEOProvider != nil {
			capabilities = append(capabilities, executableAddonCapability{Name: executableCapabilitySEOProvider, Required: true, SEOOptions: resolved.SEOProvider.SEOOptions()})
		}
		if resolved.AuthSessionProvider != nil {
			capabilities = append(capabilities, executableAddonCapability{Name: executableCapabilityAuthSessionProvider, Required: true, AuthSessionOptions: resolved.AuthSessionProvider.AuthSessionOptions()})
		}
		wire.Extensions = append(wire.Extensions, executableExtensionDetails{Index: index, Descriptor: extension.Descriptor(), Capabilities: capabilities})
	}
	return wire
}

func processExtensionCSS(index int, payload json.RawMessage) executableCSSResponse {
	config := configpkg.Config
	if index < 0 || index >= len(config.Extensions) { return executableCSSResponse{Error: fmt.Sprintf("extension index %d is out of range", index)} }
	processor := gowdk.ResolveExtensionCapabilities(config.Extensions[index]).CSSProcessor
	if processor == nil { return executableCSSResponse{Error: fmt.Sprintf("extension %s does not provide CSSProcessor", config.Extensions[index].Name())} }
	var context gowdk.CSSContext
	if err := json.Unmarshal(payload, &context); err != nil { return executableCSSResponse{Error: err.Error()} }
	result, err := processor.ProcessCSS(context)
	if err != nil { return executableCSSResponse{Error: err.Error()} }
	return executableCSSResponse{Result: result}
}

func extensionGoBlockConsumer(config gowdk.Config, index int) (gowdk.GoBlockConsumer, error) {
	if index < 0 || index >= len(config.Extensions) { return nil, fmt.Errorf("extension index %d is out of range", index) }
	consumer := gowdk.ResolveExtensionCapabilities(config.Extensions[index]).GoBlockConsumer
	if consumer == nil { return nil, fmt.Errorf("extension %s does not provide GoBlockConsumer", config.Extensions[index].Name()) }
	return consumer, nil
}

func validateExtensionGoBlock(index int, payload json.RawMessage) executableGoBlockResponse {
	consumer, err := extensionGoBlockConsumer(configpkg.Config, index)
	if err != nil { return executableGoBlockResponse{Error: err.Error()} }
	var request executableGoBlockRequest
	if err := json.Unmarshal(payload, &request); err != nil { return executableGoBlockResponse{Error: err.Error()} }
	return executableGoBlockResponse{Diagnostics: consumer.ValidateGoBlock(request.Target, request.Context)}
}

func generateExtensionGoBlock(index int, payload json.RawMessage) executableGoBlockResponse {
	consumer, err := extensionGoBlockConsumer(configpkg.Config, index)
	if err != nil { return executableGoBlockResponse{Error: err.Error()} }
	var request executableGoBlockRequest
	if err := json.Unmarshal(payload, &request); err != nil { return executableGoBlockResponse{Error: err.Error()} }
	files, err := consumer.GeneratedGo(request.Target, request.Context)
	if err != nil { return executableGoBlockResponse{Error: err.Error()} }
	return executableGoBlockResponse{Files: files}
}

func processCSS(index int, payload json.RawMessage) executableCSSResponse {
	config := configpkg.Config
	if index < 0 || index >= len(config.Addons) {
		return executableCSSResponse{Error: fmt.Sprintf("addon index %d is out of range", index)}
	}
	processor := gowdk.ResolveAddonCapabilities(config.Addons[index]).CSSProcessor
	if processor == nil {
		return executableCSSResponse{Error: fmt.Sprintf("addon %s does not implement CSSProcessor", config.Addons[index].Name())}
	}
	var context gowdk.CSSContext
	if err := json.Unmarshal(payload, &context); err != nil {
		return executableCSSResponse{Error: err.Error()}
	}
	result, err := processor.ProcessCSS(context)
	if err != nil {
		return executableCSSResponse{Error: err.Error()}
	}
	return executableCSSResponse{Result: result}
}

func validateGoBlock(index int, payload json.RawMessage) executableGoBlockResponse {
	config := configpkg.Config
	consumer, err := goBlockConsumer(config, index)
	if err != nil {
		return executableGoBlockResponse{Error: err.Error()}
	}
	var request executableGoBlockRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return executableGoBlockResponse{Error: err.Error()}
	}
	return executableGoBlockResponse{
		Diagnostics: consumer.ValidateGoBlock(request.Target, request.Context),
	}
}

func generateGoBlock(index int, payload json.RawMessage) executableGoBlockResponse {
	config := configpkg.Config
	consumer, err := goBlockConsumer(config, index)
	if err != nil {
		return executableGoBlockResponse{Error: err.Error()}
	}
	var request executableGoBlockRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return executableGoBlockResponse{Error: err.Error()}
	}
	files, err := consumer.GeneratedGo(request.Target, request.Context)
	if err != nil {
		return executableGoBlockResponse{Error: err.Error()}
	}
	return executableGoBlockResponse{Files: files}
}

func goBlockConsumer(config gowdk.Config, index int) (gowdk.GoBlockConsumer, error) {
	if index < 0 || index >= len(config.Addons) {
		return nil, fmt.Errorf("addon index %d is out of range", index)
	}
	consumer := gowdk.ResolveAddonCapabilities(config.Addons[index]).GoBlockConsumer
	if consumer == nil {
		return nil, fmt.Errorf("addon %s does not implement GoBlockConsumer", config.Addons[index].Name())
	}
	return consumer, nil
}

`
