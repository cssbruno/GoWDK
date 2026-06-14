package appgen

import (
	"fmt"
	"go/format"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
	gowdkroute "github.com/cssbruno/gowdk/runtime/route"
)

type auditTestMode string

const (
	auditTestGeneratedApp auditTestMode = "generated-app"
	auditTestStandalone   auditTestMode = "standalone"
)

type auditScenario struct {
	Name       string
	Method     string
	Path       string
	Actor      string
	WantStatus int
	WantHeader map[string]string
}

var (
	auditExpectStatusPattern = regexp.MustCompile(`^expect\s+([A-Za-z]+)\s+"([^"]+)"(?:\s+as\s+"([^"]+)")?\s+status\s+([0-9]{3})$`)
	auditExpectHeaderPattern = regexp.MustCompile(`^expect\s+header\s+"([^"]+)"\s+"([^"]+)"$`)
)

// GeneratedAuditTestSource returns the generated-app audit test file source for
// options. It returns nil when there is no IR-backed posture to exercise.
func GeneratedAuditTestSource(options Options) ([]byte, error) {
	if options.IR == nil {
		return nil, nil
	}
	manifest := securitymanifest.Build(options.Config, *options.IR)
	return auditTestSource("gowdkapp", auditTestGeneratedApp, options.Config, manifest, options.IR.AuditSpecs)
}

// StandaloneAuditTestSource returns a committable audit test file that drives
// runtime/app directly from the derived posture. The CLI uses this for
// `gowdk audit --emit-tests`; `gowdk audit --run` generates a temporary app and
// runs the generated-app audit test instead.
func StandaloneAuditTestSource(config gowdk.Config, manifest securitymanifest.SecurityManifest, specs []gwdkir.AuditSpec) ([]byte, error) {
	return auditTestSource("gowdkaudit_test", auditTestStandalone, config, manifest, specs)
}

// StandaloneAuditTestSourceWithPackage returns standalone audit test source
// using packageName. It exists so the CLI can emit into a directory that
// already has Go files without creating a mixed-package test setup.
func StandaloneAuditTestSourceWithPackage(packageName string, config gowdk.Config, manifest securitymanifest.SecurityManifest, specs []gwdkir.AuditSpec) ([]byte, error) {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		packageName = "gowdkaudit_test"
	}
	return auditTestSource(packageName, auditTestStandalone, config, manifest, specs)
}

func auditTestSource(packageName string, mode auditTestMode, config gowdk.Config, manifest securitymanifest.SecurityManifest, specs []gwdkir.AuditSpec) ([]byte, error) {
	scenarios, err := auditTestScenarios(mode, config, manifest, specs)
	if err != nil {
		return nil, err
	}
	if len(scenarios) == 0 {
		return nil, nil
	}
	usesActor := auditScenariosUseActor(scenarios)
	installAuthProvider := mode == auditTestGeneratedApp && usesActor && auditManifestUsesNativeGuards(manifest)

	var builder strings.Builder
	fmt.Fprintf(&builder, "package %s\n\n", packageName)
	writeAuditTestImports(&builder, mode, installAuthProvider)
	builder.WriteString("\n")
	builder.WriteString("func TestGOWDKAuditGeneratedSecurityPosture(t *testing.T) {\n")
	switch mode {
	case auditTestGeneratedApp:
		writeGeneratedAuditEnvSeeds(&builder, config, manifest)
		builder.WriteString("\thandler, err := Handler()\n")
		builder.WriteString("\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n")
		if installAuthProvider {
			writeGeneratedAuditAuthProvider(&builder)
		}
	case auditTestStandalone:
		writeStandaloneAuditHandler(&builder, config, manifest)
	}
	builder.WriteString("\tgowdktestkit.Run(t, handler, []gowdktestkit.Scenario{\n")
	for _, scenario := range scenarios {
		writeAuditScenario(&builder, scenario)
	}
	builder.WriteString("\t})\n")
	builder.WriteString("}\n")

	formatted, err := format.Source([]byte(builder.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated audit tests: %w", err)
	}
	return formatted, nil
}

func writeAuditTestImports(builder *strings.Builder, mode auditTestMode, usesActor bool) {
	builder.WriteString("import (\n")
	builder.WriteString("\t\"net/http\"\n")
	if mode == auditTestGeneratedApp && usesActor {
		builder.WriteString("\t\"strings\"\n")
	}
	builder.WriteString("\t\"testing\"\n")
	if mode == auditTestStandalone {
		builder.WriteString("\t\"testing/fstest\"\n")
	}
	builder.WriteString("\n")
	if mode == auditTestGeneratedApp && usesActor {
		builder.WriteString("\tgowdkauth \"github.com/cssbruno/gowdk/runtime/auth\"\n")
	}
	if mode == auditTestStandalone {
		builder.WriteString("\tgowdkruntime \"github.com/cssbruno/gowdk/runtime/app\"\n")
		builder.WriteString("\truntimeasset \"github.com/cssbruno/gowdk/runtime/asset\"\n")
	}
	builder.WriteString("\tgowdktestkit \"github.com/cssbruno/gowdk/runtime/testkit\"\n")
	builder.WriteString(")\n")
}

func auditScenariosUseActor(scenarios []auditScenario) bool {
	for _, scenario := range scenarios {
		if strings.TrimSpace(scenario.Actor) != "" {
			return true
		}
	}
	return false
}

func auditManifestUsesNativeGuards(manifest securitymanifest.SecurityManifest) bool {
	for _, route := range manifest.Routes {
		if auditGuardsUseNativeRBAC(route.Guards) {
			return true
		}
	}
	for _, endpoint := range manifest.Endpoints {
		if auditGuardsUseNativeRBAC(endpoint.Guards) {
			return true
		}
	}
	return false
}

func auditGuardsUseNativeRBAC(guards []string) bool {
	for _, guard := range guards {
		if strings.HasPrefix(guard, "role:") || strings.HasPrefix(guard, "permission:") {
			return true
		}
	}
	return false
}

func auditTestScenarios(mode auditTestMode, config gowdk.Config, manifest securitymanifest.SecurityManifest, specs []gwdkir.AuditSpec) ([]auditScenario, error) {
	var scenarios []auditScenario
	routes := append([]securitymanifest.RouteEntry(nil), manifest.Routes...)
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Route != routes[j].Route {
			return routes[i].Route < routes[j].Route
		}
		return routes[i].PageID < routes[j].PageID
	})

	postEndpoints := map[string]bool{}
	for _, endpoint := range manifest.Endpoints {
		if strings.EqualFold(endpoint.Method, http.MethodPost) && endpoint.Path != "" {
			postEndpoints[path.Clean("/"+endpoint.Path)] = true
		}
	}

	for _, route := range routes {
		routePath := path.Clean("/" + route.Route)
		if !isConcreteAuditRoute(routePath) {
			continue
		}
		if route.DefaultDeny {
			scenarios = append(scenarios, auditScenario{
				Name:       "default-deny " + routePath,
				Method:     http.MethodGet,
				Path:       routePath,
				WantStatus: http.StatusForbidden,
			})
		} else if !strings.EqualFold(route.Render, string(gowdk.SSR)) {
			scenarios = append(scenarios, auditScenario{
				Name:       "route serves " + routePath,
				Method:     http.MethodGet,
				Path:       routePath,
				WantStatus: http.StatusOK,
			})
		}
		if !postEndpoints[routePath] {
			scenarios = append(scenarios, auditScenario{
				Name:       "method denied " + routePath,
				Method:     http.MethodPost,
				Path:       routePath,
				WantStatus: http.StatusMethodNotAllowed,
			})
		}
	}

	for _, header := range auditSecurityHeaders(config) {
		scenarios = append(scenarios, auditScenario{
			Name:       "security header " + header.Name,
			Method:     http.MethodGet,
			Path:       "/_gowdk/health",
			WantStatus: http.StatusOK,
			WantHeader: map[string]string{header.Name: header.Value},
		})
	}

	testScenarios, err := auditDeclaredTestScenarios(mode, manifest, specs)
	if err != nil {
		return nil, err
	}
	scenarios = append(scenarios, testScenarios...)
	return scenarios, nil
}

type auditHeaderExpectation struct {
	Name  string
	Value string
}

func auditSecurityHeaders(config gowdk.Config) []auditHeaderExpectation {
	if !config.Build.SecurityHeaders.Enabled || len(config.Build.SecurityHeaders.Headers) == 0 {
		return nil
	}
	normalized := normalizedSecurityHeaders(config.Build.SecurityHeaders.Headers)
	headers := make([]auditHeaderExpectation, 0, len(normalized))
	for _, header := range normalized {
		headers = append(headers, auditHeaderExpectation{Name: header.Name, Value: header.Value})
	}
	return headers
}

func auditDeclaredTestScenarios(mode auditTestMode, manifest securitymanifest.SecurityManifest, specs []gwdkir.AuditSpec) ([]auditScenario, error) {
	var scenarios []auditScenario
	for _, spec := range specs {
		for _, test := range spec.Tests {
			lines := strings.Split(test.Body, "\n")
			for lineIndex, raw := range lines {
				line := strings.TrimSpace(raw)
				if line == "" || strings.HasPrefix(line, "//") {
					continue
				}
				statusMatch := auditExpectStatusPattern.FindStringSubmatch(line)
				if statusMatch != nil {
					status, err := strconv.Atoi(statusMatch[4])
					if err != nil {
						return nil, fmt.Errorf("%s:%d: invalid audit test status %q", spec.Source, test.Span.Start.Line+lineIndex+1, statusMatch[4])
					}
					method := strings.ToUpper(statusMatch[1])
					requestPath := statusMatch[2]
					// The standalone harness installs no auth provider and only
					// models static serving, default-deny, and headers, so it
					// cannot enforce role/permission guards. An actor expectation
					// there would pass or fail for the wrong reason, so refuse it
					// and steer the author to the generated-app audit test, which
					// runs against the real guard pipeline.
					if mode == auditTestStandalone && statusMatch[3] != "" {
						return nil, fmt.Errorf("%s:%d: audit test actor %q requires the generated-app audit test (%s, emitted by gowdk build or run by gowdk audit --run); standalone audit tests cannot enforce role or permission guards", spec.Source, test.Span.Start.Line+lineIndex+1, statusMatch[3], auditTestFileName)
					}
					if mode == auditTestStandalone {
						if endpoint, ok := auditStandaloneEndpointExpectation(manifest, method, requestPath); ok {
							return nil, fmt.Errorf("%s:%d: audit test expectation %s %q targets %s endpoint %s and requires the generated-app audit test (%s, emitted by gowdk build or run by gowdk audit --run); standalone audit tests do not install Backend, Action, API, fragment, or contract callbacks", spec.Source, test.Span.Start.Line+lineIndex+1, method, requestPath, endpoint.Kind, endpoint.ID, auditTestFileName)
						}
					}
					name := test.Name + " " + method + " " + requestPath
					if statusMatch[3] != "" {
						name += " as " + statusMatch[3]
					}
					scenarios = append(scenarios, auditScenario{
						Name:       name,
						Method:     method,
						Path:       requestPath,
						Actor:      statusMatch[3],
						WantStatus: status,
					})
					continue
				}
				headerMatch := auditExpectHeaderPattern.FindStringSubmatch(line)
				if headerMatch != nil {
					scenarios = append(scenarios, auditScenario{
						Name:       test.Name + " header " + headerMatch[1],
						Method:     http.MethodGet,
						Path:       "/_gowdk/health",
						WantStatus: http.StatusOK,
						WantHeader: map[string]string{headerMatch[1]: headerMatch[2]},
					})
					continue
				}
				return nil, fmt.Errorf("%s:%d: unsupported audit test expectation %q", spec.Source, test.Span.Start.Line+lineIndex+1, line)
			}
		}
	}
	return scenarios, nil
}

func auditStandaloneEndpointExpectation(manifest securitymanifest.SecurityManifest, method string, requestPath string) (securitymanifest.EndpointEntry, bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	requestPath = path.Clean("/" + requestPath)
	for _, endpoint := range manifest.Endpoints {
		if strings.TrimSpace(endpoint.Method) == "" || strings.TrimSpace(endpoint.Path) == "" {
			continue
		}
		if !strings.EqualFold(endpoint.Method, method) {
			continue
		}
		if auditEndpointPathMatches(endpoint.Path, requestPath) {
			return endpoint, true
		}
	}
	return securitymanifest.EndpointEntry{}, false
}

func auditEndpointPathMatches(endpointPath string, requestPath string) bool {
	endpointPath = path.Clean("/" + endpointPath)
	requestPath = path.Clean("/" + requestPath)
	if endpointPath == requestPath {
		return true
	}
	if strings.Contains(endpointPath, "{") {
		_, ok := gowdkroute.Match(endpointPath, requestPath)
		return ok
	}
	return false
}

const generatedAuditEnvSeed = "gowdk-audit-test"
const generatedAuditCSRFSecretSeed = "gowdk-audit-test-csrf-secret-32-bytes"

func writeGeneratedAuditEnvSeeds(builder *strings.Builder, config gowdk.Config, manifest securitymanifest.SecurityManifest) {
	csrfSecretName := ""
	if auditManifestHasCSRFProtectedEndpoint(manifest) {
		csrfSecretName = config.Build.CSRF.SecretEnvName()
	}
	for _, name := range auditRequiredEnvNames(config, manifest) {
		value := generatedAuditEnvSeed
		if name == csrfSecretName {
			value = generatedAuditCSRFSecretSeed
		}
		fmt.Fprintf(builder, "\tt.Setenv(%s, %s)\n", strconv.Quote(name), strconv.Quote(value))
	}
}

func auditRequiredEnvNames(config gowdk.Config, manifest securitymanifest.SecurityManifest) []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	for _, variable := range config.Env.Vars {
		if variable.Required && strings.TrimSpace(variable.Default) == "" {
			add(variable.Name)
		}
	}
	for _, secret := range config.Env.Secrets {
		if secret.Required {
			add(secret.Name)
		}
	}
	if auditManifestHasCSRFProtectedEndpoint(manifest) {
		add(config.Build.CSRF.SecretEnvName())
	}
	sort.Strings(names)
	return names
}

func auditManifestHasCSRFProtectedEndpoint(manifest securitymanifest.SecurityManifest) bool {
	for _, endpoint := range manifest.Endpoints {
		if endpoint.CSRF {
			return true
		}
	}
	return false
}

func writeGeneratedAuditAuthProvider(builder *strings.Builder) {
	builder.WriteString("\tpreviousAuditAuthProvider := authProvider\n")
	builder.WriteString("\tt.Cleanup(func() { authProvider = previousAuditAuthProvider })\n")
	builder.WriteString("\tRegisterAuthProvider(gowdkauth.ProviderFunc(func(request *http.Request) (*gowdkauth.Principal, error) {\n")
	builder.WriteString("\t\tactor := strings.TrimSpace(request.Header.Get(\"X-GOWDK-Audit-Actor\"))\n")
	builder.WriteString("\t\tswitch {\n")
	builder.WriteString("\t\tcase actor == \"\" || actor == \"anonymous\":\n")
	builder.WriteString("\t\t\treturn nil, nil\n")
	builder.WriteString("\t\tcase strings.HasPrefix(actor, \"role:\"):\n")
	builder.WriteString("\t\t\treturn &gowdkauth.Principal{ID: \"audit\", Roles: []string{strings.TrimPrefix(actor, \"role:\")}}, nil\n")
	builder.WriteString("\t\tcase strings.HasPrefix(actor, \"permission:\"):\n")
	builder.WriteString("\t\t\treturn &gowdkauth.Principal{ID: \"audit\", Permissions: []string{strings.TrimPrefix(actor, \"permission:\")}}, nil\n")
	builder.WriteString("\t\tdefault:\n")
	builder.WriteString("\t\t\treturn &gowdkauth.Principal{ID: \"audit\", Roles: []string{actor}}, nil\n")
	builder.WriteString("\t\t}\n")
	builder.WriteString("\t}))\n")
}

func writeStandaloneAuditHandler(builder *strings.Builder, config gowdk.Config, manifest securitymanifest.SecurityManifest) {
	builder.WriteString("\thandler := gowdkruntime.Handler{\n")
	builder.WriteString("\t\tRoot: fstest.MapFS{\n")
	for _, file := range auditStandaloneFiles(manifest) {
		fmt.Fprintf(builder, "\t\t\t%s: {Data: []byte(%s)},\n", strconv.Quote(file), strconv.Quote("<main>GOWDK audit</main>"))
	}
	builder.WriteString("\t\t},\n")
	builder.WriteString("\t\tIdentity: gowdkruntime.Identity{AppID: \"audit\", ModuleName: \"audit\", InstanceID: \"audit-test\"},\n")
	builder.WriteString("\t\tAssets: runtimeasset.Manifest{Version: runtimeasset.ManifestVersion, Files: map[string]string{}},\n")
	if headers := auditSecurityHeaders(config); len(headers) > 0 {
		builder.WriteString("\t\tSecurityHeaders: map[string]string{\n")
		for _, header := range headers {
			fmt.Fprintf(builder, "\t\t\t%s: %s,\n", strconv.Quote(header.Name), strconv.Quote(header.Value))
		}
		builder.WriteString("\t\t},\n")
	}
	if denied := auditDeniedRoutes(manifest); len(denied) > 0 {
		builder.WriteString("\t\tDenied: map[string]bool{\n")
		for _, route := range denied {
			fmt.Fprintf(builder, "\t\t\t%s: true,\n", strconv.Quote(route))
		}
		builder.WriteString("\t\t},\n")
	}
	if patterns := auditDeniedRoutePatterns(manifest); len(patterns) > 0 {
		builder.WriteString("\t\tDeniedPatterns: []string{\n")
		for _, route := range patterns {
			fmt.Fprintf(builder, "\t\t\t%s,\n", strconv.Quote(route))
		}
		builder.WriteString("\t\t},\n")
	}
	builder.WriteString("\t}\n")
}

func auditStandaloneFiles(manifest securitymanifest.SecurityManifest) []string {
	seen := map[string]bool{}
	var files []string
	for _, route := range manifest.Routes {
		routePath := path.Clean("/" + route.Route)
		if !isConcreteAuditRoute(routePath) {
			continue
		}
		file := auditRouteFile(routePath)
		if seen[file] {
			continue
		}
		seen[file] = true
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func auditRouteFile(route string) string {
	route = path.Clean("/" + route)
	if route == "/" {
		return "index.html"
	}
	return strings.TrimPrefix(route, "/") + "/index.html"
}

func auditDeniedRoutes(manifest securitymanifest.SecurityManifest) []string {
	var routes []string
	for _, route := range manifest.Routes {
		routePath := path.Clean("/" + route.Route)
		if route.DefaultDeny && isConcreteAuditRoute(routePath) {
			routes = append(routes, routePath)
		}
	}
	sort.Strings(routes)
	return routes
}

func auditDeniedRoutePatterns(manifest securitymanifest.SecurityManifest) []string {
	var routes []string
	for _, route := range manifest.Routes {
		routePath := path.Clean("/" + route.Route)
		if route.DefaultDeny && !isConcreteAuditRoute(routePath) {
			routes = append(routes, routePath)
		}
	}
	sort.Strings(routes)
	return routes
}

func isConcreteAuditRoute(route string) bool {
	return !strings.Contains(route, "{") && !strings.Contains(route, "}")
}

func writeAuditScenario(builder *strings.Builder, scenario auditScenario) {
	builder.WriteString("\t\t{\n")
	fmt.Fprintf(builder, "\t\t\tName: %s,\n", strconv.Quote(scenario.Name))
	fmt.Fprintf(builder, "\t\t\tMethod: %s,\n", auditMethodExpr(scenario.Method))
	fmt.Fprintf(builder, "\t\t\tPath: %s,\n", strconv.Quote(path.Clean("/"+scenario.Path)))
	if strings.TrimSpace(scenario.Actor) != "" {
		builder.WriteString("\t\t\tHeaders: map[string]string{\n")
		fmt.Fprintf(builder, "\t\t\t\t%s: %s,\n", strconv.Quote("X-GOWDK-Audit-Actor"), strconv.Quote(strings.TrimSpace(scenario.Actor)))
		builder.WriteString("\t\t\t},\n")
	}
	if scenario.WantStatus != 0 {
		fmt.Fprintf(builder, "\t\t\tWantStatus: %s,\n", auditStatusExpr(scenario.WantStatus))
	}
	if len(scenario.WantHeader) > 0 {
		builder.WriteString("\t\t\tWantHeader: map[string]string{\n")
		names := make([]string, 0, len(scenario.WantHeader))
		for name := range scenario.WantHeader {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(builder, "\t\t\t\t%s: %s,\n", strconv.Quote(name), strconv.Quote(scenario.WantHeader[name]))
		}
		builder.WriteString("\t\t\t},\n")
	}
	builder.WriteString("\t\t},\n")
}

func auditMethodExpr(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet:
		return "http.MethodGet"
	case http.MethodHead:
		return "http.MethodHead"
	case http.MethodPost:
		return "http.MethodPost"
	case http.MethodPut:
		return "http.MethodPut"
	case http.MethodPatch:
		return "http.MethodPatch"
	case http.MethodDelete:
		return "http.MethodDelete"
	default:
		return strconv.Quote(strings.ToUpper(strings.TrimSpace(method)))
	}
}

func auditStatusExpr(status int) string {
	switch status {
	case http.StatusOK:
		return "http.StatusOK"
	case http.StatusNoContent:
		return "http.StatusNoContent"
	case http.StatusSeeOther:
		return "http.StatusSeeOther"
	case http.StatusBadRequest:
		return "http.StatusBadRequest"
	case http.StatusForbidden:
		return "http.StatusForbidden"
	case http.StatusNotFound:
		return "http.StatusNotFound"
	case http.StatusMethodNotAllowed:
		return "http.StatusMethodNotAllowed"
	case http.StatusInternalServerError:
		return "http.StatusInternalServerError"
	default:
		return strconv.Itoa(status)
	}
}
