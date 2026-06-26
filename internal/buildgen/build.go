package buildgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func Build(config gowdk.Config, sources gwdkanalysis.Sources, outputDir string) (Result, error) {
	ir, bindings, err := compiler.AssembleProgram(config, sources)
	if err != nil {
		return Result{}, err
	}
	return buildFromIR(config, ir, bindings, outputDir, true)
}

// BuildFromIR writes SPA build artifacts from normalized compiler IR.
func BuildFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) (Result, error) {
	return buildFromIR(config, ir, compiler.BackendBindingsFromIR(ir), outputDir, true)
}

// BuildFromValidatedIR is BuildFromIR for orchestrators that already ran
// compiler.ValidateProgram on the IR (the CLI build path). It skips the
// defensive re-validation, which type-checks feature Go packages on disk and
// is too expensive to run twice per build, but still runs cheap IR invariant
// checks before planning generated output.
func BuildFromValidatedIR(config gowdk.Config, ir gwdkir.Program, outputDir string) (Result, error) {
	return buildFromIR(config, ir, compiler.BackendBindingsFromIR(ir), outputDir, false)
}

func buildFromIR(config gowdk.Config, ir gwdkir.Program, backendBindings []source.BackendBinding, outputDir string, validate bool) (Result, error) {
	reporter, planned, err := prepareBuildPlan("build", "SPA build started", outputDir, config, ir, backendBindings, validate, true)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Artifacts:      make([]Artifact, 0, len(planned.pages)),
		CSSArtifacts:   make([]CSSArtifact, 0, len(planned.css)),
		AssetArtifacts: make([]AssetArtifact, 0, len(planned.assets)),
	}
	for _, artifact := range planned.css {
		wrote, err := writeFileIfChangedStatus(artifact.Path, artifact.contents)
		if err != nil {
			return Result{}, reporter.fail("write", err)
		}
		recordWriteStat(&result, wrote)
		reporter.debug("write", "css_written", "CSS artifact written", BuildEvent{Path: eventPath(outputDir, artifact.Path)})
		finalizeCSSArtifact(&artifact)
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
	}
	for _, artifact := range planned.assets {
		wrote, err := writeFileIfChangedStatus(artifact.Path, artifact.contents)
		if err != nil {
			return Result{}, reporter.fail("write", err)
		}
		recordWriteStat(&result, wrote)
		reporter.debug("write", "asset_written", "runtime asset written", BuildEvent{Path: eventPath(outputDir, artifact.Path)})
		finalizeAssetArtifact(&artifact)
		result.AssetArtifacts = append(result.AssetArtifacts, artifact.AssetArtifact)
	}
	for _, artifact := range planned.pages {
		wrote, err := writeFileIfChangedStatus(artifact.Path, artifact.contents)
		if err != nil {
			return Result{}, reporter.fail("write", err)
		}
		recordWriteStat(&result, wrote)
		reporter.debug("write", "page_written", "page artifact written", BuildEvent{
			PageID: artifact.PageID,
			Route:  artifact.Route,
			Path:   eventPath(outputDir, artifact.Path),
		})
		result.Artifacts = append(result.Artifacts, artifact.Artifact)
	}
	endpoints := compiler.BuildRouteMetadataFromIR(config, ir).Endpoints
	manifestPath, err := writeRouteManifest(outputDir, result.Artifacts, endpoints)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.RouteManifestPath = manifestPath
	reporter.info("manifest", "route_manifest_written", "route manifest written", BuildEvent{Path: eventPath(outputDir, manifestPath)})
	seoPlan, err := planSEOArtifacts(config, ir, result.Artifacts)
	if err != nil {
		return Result{}, reporter.fail("seo", err)
	}
	reportSEOExclusions(reporter, seoPlan.Exclusions)
	sitemapPath, robotsPath, sitemapWrote, robotsWrote, err := writeSEOArtifacts(outputDir, seoPlan)
	if err != nil {
		return Result{}, reporter.fail("seo", err)
	}
	if sitemapPath != "" {
		recordWriteStat(&result, sitemapWrote)
		result.SitemapPath = sitemapPath
		reporter.info("seo", "sitemap_written", "sitemap written", BuildEvent{
			Path: eventPath(outputDir, sitemapPath),
			Data: map[string]string{"urls": fmt.Sprint(len(seoPlan.URLs))},
		})
	}
	if robotsPath != "" {
		recordWriteStat(&result, robotsWrote)
		result.RobotsPath = robotsPath
		reporter.info("seo", "robots_written", "robots.txt written", BuildEvent{Path: eventPath(outputDir, robotsPath)})
	}
	assetManifestPath, err := writeAssetManifest(outputDir, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.AssetManifestPath = assetManifestPath
	reporter.info("manifest", "asset_manifest_written", "asset manifest written", BuildEvent{Path: eventPath(outputDir, assetManifestPath)})
	reportCachePolicies(reporter, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	reportAssetSizes(reporter, outputDir, result.AssetArtifacts)
	openAPIPath, err := writeOpenAPI(outputDir, config, ir)
	if err != nil {
		return Result{}, reporter.fail("report", err)
	}
	result.OpenAPIPath = openAPIPath
	reporter.info("report", "openapi_written", "OpenAPI report written", BuildEvent{Path: eventPath(outputDir, openAPIPath)})
	securityManifestPath, err := writeSecurityManifest(outputDir, config, ir)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.SecurityManifestPath = securityManifestPath
	reporter.info("manifest", "security_manifest_written", "security manifest written", BuildEvent{Path: eventPath(outputDir, securityManifestPath)})
	reporter.info("complete", "build_complete", "SPA build completed", BuildEvent{
		Data: map[string]string{
			"pages":  fmt.Sprint(len(result.Artifacts)),
			"css":    fmt.Sprint(len(result.CSSArtifacts)),
			"assets": fmt.Sprint(len(result.AssetArtifacts)),
		},
	})
	result.Report = reporter.result()
	buildReportPath, err := writeBuildReport(outputDir, result.Report)
	if err != nil {
		return Result{}, reporter.fail("report", err)
	}
	result.BuildReportPath = buildReportPath
	return result, nil
}

func recordWriteStat(result *Result, wrote bool) {
	if wrote {
		result.WriteStats.FilesWritten++
		return
	}
	result.WriteStats.IdenticalWritesSkipped++
}

func finalizeCSSArtifact(artifact *plannedCSSArtifact) {
	artifact.CSSArtifact.Hash = contentHash(artifact.contents)
	artifact.CSSArtifact.CachePolicy = immutableAssetCachePolicy
	artifact.CSSArtifact.SizeBytes = int64(len(artifact.contents))
}

func finalizeAssetArtifact(artifact *plannedAssetArtifact) {
	if artifact.AssetArtifact.Hash == "" {
		artifact.AssetArtifact.Hash = contentHash(artifact.contents)
	}
	if artifact.AssetArtifact.CachePolicy == "" {
		artifact.AssetArtifact.CachePolicy = noCacheAssetCachePolicy
	}
	artifact.AssetArtifact.SizeBytes = int64(len(artifact.contents))
}

func BuildMemory(config gowdk.Config, sources gwdkanalysis.Sources, outputDir string) (MemoryResult, error) {
	ir, bindings, err := compiler.AssembleProgram(config, sources)
	if err != nil {
		return MemoryResult{}, err
	}
	return buildMemoryFromIR(config, ir, bindings, outputDir, true)
}

// BuildMemoryWithOptions plans SPA build artifacts without requiring a real
// output directory. Empty MemoryBuildOptions.OutputBase defaults to ".".
func BuildMemoryWithOptions(config gowdk.Config, sources gwdkanalysis.Sources, options MemoryBuildOptions) (MemoryResult, error) {
	ir, bindings, err := compiler.AssembleProgram(config, sources)
	if err != nil {
		return MemoryResult{}, err
	}
	return buildMemoryFromIR(config, ir, bindings, memoryOutputBase(options), false)
}

// BuildMemoryFromIR plans SPA build artifacts from normalized compiler IR
// without writing them to disk.
func BuildMemoryFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) (MemoryResult, error) {
	return buildMemoryFromIR(config, ir, compiler.BackendBindingsFromIR(ir), outputDir, true)
}

// BuildMemoryFromIRWithOptions is BuildMemoryWithOptions for orchestrators
// that already have normalized compiler IR.
func BuildMemoryFromIRWithOptions(config gowdk.Config, ir gwdkir.Program, options MemoryBuildOptions) (MemoryResult, error) {
	return buildMemoryFromIR(config, ir, compiler.BackendBindingsFromIR(ir), memoryOutputBase(options), false)
}

func memoryOutputBase(options MemoryBuildOptions) string {
	if strings.TrimSpace(options.OutputBase) == "" {
		return "."
	}
	return options.OutputBase
}

func buildMemoryFromIR(config gowdk.Config, ir gwdkir.Program, backendBindings []source.BackendBinding, outputDir string, requireOutputDir bool) (MemoryResult, error) {
	reporter, planned, err := prepareBuildPlan("memory", "in-memory SPA build started", outputDir, config, ir, backendBindings, true, requireOutputDir)
	if err != nil {
		return MemoryResult{}, err
	}

	result := MemoryResult{
		Result: Result{
			Artifacts:         make([]Artifact, 0, len(planned.pages)),
			CSSArtifacts:      make([]CSSArtifact, 0, len(planned.css)),
			AssetArtifacts:    make([]AssetArtifact, 0, len(planned.assets)),
			RouteManifestPath: filepath.Join(outputDir, routeManifestFile),
			AssetManifestPath: filepath.Join(outputDir, assetManifestFile),
			OpenAPIPath:       filepath.Join(outputDir, openAPIFile),
			BuildReportPath:   buildReportPath(outputDir),
		},
		Files: map[string][]byte{},
	}
	manifestPath, err := memorySecurityManifestPath(outputDir, requireOutputDir)
	if err != nil {
		return MemoryResult{}, reporter.fail("manifest", err)
	}
	result.SecurityManifestPath = manifestPath
	for _, artifact := range planned.css {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return MemoryResult{}, reporter.fail("memory", err)
		}
		finalizeCSSArtifact(&artifact)
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
		result.Files[rel] = append([]byte(nil), artifact.contents...)
		reporter.debug("memory", "css_collected", "CSS artifact collected", BuildEvent{Path: rel})
	}
	for _, artifact := range planned.assets {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return MemoryResult{}, reporter.fail("memory", err)
		}
		finalizeAssetArtifact(&artifact)
		result.AssetArtifacts = append(result.AssetArtifacts, artifact.AssetArtifact)
		result.Files[rel] = append([]byte(nil), artifact.contents...)
		reporter.debug("memory", "asset_collected", "runtime asset collected", BuildEvent{Path: rel})
	}
	for _, artifact := range planned.pages {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return MemoryResult{}, reporter.fail("memory", err)
		}
		result.Artifacts = append(result.Artifacts, artifact.Artifact)
		result.Files[rel] = append([]byte(nil), artifact.contents...)
		reporter.debug("memory", "page_collected", "page artifact collected", BuildEvent{
			PageID: artifact.PageID,
			Route:  artifact.Route,
			Path:   rel,
		})
	}

	endpoints := compiler.BuildRouteMetadataFromIR(config, ir).Endpoints
	routeManifest, err := routeManifestPayload(outputDir, result.Artifacts, endpoints)
	if err != nil {
		return MemoryResult{}, reporter.fail("manifest", err)
	}
	result.Files[routeManifestFile] = routeManifest
	reporter.info("manifest", "route_manifest_collected", "route manifest collected", BuildEvent{Path: routeManifestFile})
	seoPlan, err := planSEOArtifacts(config, ir, result.Artifacts)
	if err != nil {
		return MemoryResult{}, reporter.fail("seo", err)
	}
	reportSEOExclusions(reporter, seoPlan.Exclusions)
	if seoPlan.Enabled {
		result.SitemapPath = filepath.Join(outputDir, sitemapFile)
		result.RobotsPath = filepath.Join(outputDir, robotsFile)
		result.Files[sitemapFile] = seoPlan.Sitemap
		result.Files[robotsFile] = seoPlan.Robots
		reporter.info("seo", "sitemap_collected", "sitemap collected", BuildEvent{
			Path: sitemapFile,
			Data: map[string]string{"urls": fmt.Sprint(len(seoPlan.URLs))},
		})
		reporter.info("seo", "robots_collected", "robots.txt collected", BuildEvent{Path: robotsFile})
	}
	assetManifest, err := assetManifestPayload(outputDir, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	if err != nil {
		return MemoryResult{}, reporter.fail("manifest", err)
	}
	result.Files[assetManifestFile] = assetManifest
	reporter.info("manifest", "asset_manifest_collected", "asset manifest collected", BuildEvent{Path: assetManifestFile})
	reportCachePolicies(reporter, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	reportAssetSizes(reporter, outputDir, result.AssetArtifacts)
	openAPI, err := openAPIPayload(config, ir)
	if err != nil {
		return MemoryResult{}, reporter.fail("report", err)
	}
	result.Files[openAPIFile] = openAPI
	reporter.info("report", "openapi_collected", "OpenAPI report collected", BuildEvent{Path: openAPIFile})
	reporter.info("manifest", "security_manifest_planned", "security manifest planned outside served output", BuildEvent{Path: eventPath(outputDir, result.SecurityManifestPath)})
	reporter.info("complete", "build_complete", "in-memory SPA build completed", BuildEvent{
		Data: map[string]string{
			"pages":  fmt.Sprint(len(result.Artifacts)),
			"css":    fmt.Sprint(len(result.CSSArtifacts)),
			"assets": fmt.Sprint(len(result.AssetArtifacts)),
			"files":  fmt.Sprint(len(result.Files) + 1),
		},
	})
	result.Report = reporter.result()
	buildReport, err := buildReportPayload(result.Report)
	if err != nil {
		return MemoryResult{}, reporter.fail("report", err)
	}
	result.Files[buildReportFile] = buildReport
	return result, nil
}

func prepareBuildPlan(kind string, startMessage string, outputDir string, config gowdk.Config, ir gwdkir.Program, backendBindings []source.BackendBinding, validate bool, requireOutputDir bool) (*buildReporter, buildPlan, error) {
	reporter := newBuildReporter(kind, outputDir)
	reporter.info("start", "build_started", startMessage, BuildEvent{
		Data: map[string]string{
			"pages":      fmt.Sprint(len(ir.Pages)),
			"components": fmt.Sprint(len(ir.Components)),
			"layouts":    fmt.Sprint(len(ir.Layouts)),
		},
	})
	if requireOutputDir && strings.TrimSpace(outputDir) == "" {
		return reporter, buildPlan{}, reporter.fail("validate", fmt.Errorf("build output directory is required"))
	}
	if validate {
		if err := compiler.ValidateProgram(config, ir); err != nil {
			return reporter, buildPlan{}, reporter.fail("validate", err)
		}
	} else if err := gwdkir.CheckInvariants(ir); err != nil {
		return reporter, buildPlan{}, reporter.fail("validate", fmt.Errorf("internal compiler error: %w", err))
	}
	reporter.info("validate", "ir_valid", "compiler IR validation completed", BuildEvent{})
	reportBackendBindings(reporter, backendBindings)
	reportContractReferences(reporter, ir.ContractRefs)
	reportRealtimeSubscriptions(reporter, ir.RealtimeSubscriptions)
	reportQueryInvalidations(reporter, ir.QueryInvalidations)
	if err := compiler.ValidateBackendBindingPolicyIR(config, ir); err != nil {
		return reporter, buildPlan{}, reporter.fail("bind", err)
	}

	planned, err := planFromIR(config, ir, outputDir)
	if err != nil {
		return reporter, buildPlan{}, reporter.fail("plan", err)
	}
	reporter.info("plan", "artifacts_planned", "app artifacts planned", BuildEvent{
		Data: map[string]string{
			"pages":  fmt.Sprint(len(planned.pages)),
			"css":    fmt.Sprint(len(planned.css)),
			"assets": fmt.Sprint(len(planned.assets)),
		},
	})
	reportAssetObfuscation(reporter, config.Build.ObfuscateAssets, planned.obfuscations)
	reportSkippedPrerenderPages(reporter, config, ir)
	reportStructuredData(reporter, ir)
	return reporter, planned, nil
}

func reportBackendBindings(reporter *buildReporter, bindings []source.BackendBinding) {
	for _, binding := range bindings {
		data := map[string]string{
			"kind":     binding.Kind,
			"block":    binding.BlockName,
			"method":   binding.Method,
			"status":   string(binding.Status),
			"function": binding.FunctionName,
		}
		if binding.PackageName != "" {
			data["package"] = binding.PackageName
		}
		if binding.ImportPath != "" {
			data["import"] = binding.ImportPath
		}
		if binding.Signature != "" {
			data["signature"] = string(binding.Signature)
		}
		if binding.InputType != "" {
			data["inputType"] = binding.InputType
		}
		if binding.Message != "" {
			data["message"] = binding.Message
		}
		reporter.info("bind", "backend_binding", "backend binding resolved", BuildEvent{
			PageID: binding.PageID,
			Route:  binding.Route,
			Data:   data,
		})
	}
}

func reportContractReferences(reporter *buildReporter, refs []gwdkir.ContractReference) {
	for _, ref := range refs {
		status := ref.Status
		if status == "" {
			status = gwdkir.ContractBindingUnknown
		}
		data := map[string]string{
			"kind":      string(ref.Kind),
			"name":      ref.Name,
			"status":    string(status),
			"ownerKind": string(ref.OwnerKind),
			"owner":     ref.OwnerID,
		}
		if ref.Span.Start.Line > 0 {
			data["line"] = fmt.Sprint(ref.Span.Start.Line)
			data["column"] = fmt.Sprint(ref.Span.Start.Column)
		}
		if ref.Handler != "" {
			data["handler"] = ref.Handler
		}
		if ref.Register != "" {
			data["register"] = ref.Register
		}
		if ref.ImportAlias != "" {
			data["importAlias"] = ref.ImportAlias
		}
		if ref.ImportPath != "" {
			data["importPath"] = ref.ImportPath
		}
		if ref.Type != "" {
			data["type"] = ref.Type
		}
		if ref.Result != "" {
			data["result"] = ref.Result
		}
		if len(ref.Roles) > 0 {
			data["roles"] = strings.Join(ref.Roles, ",")
		}
		if len(ref.Guards) > 0 {
			data["guards"] = strings.Join(ref.Guards, ",")
		}
		if len(ref.InputFields) > 0 {
			var fields []string
			for _, field := range ref.InputFields {
				fields = append(fields, field.FieldName+":"+field.FormName+":"+field.Type)
			}
			data["inputFields"] = strings.Join(fields, ",")
		}
		if ref.Method != "" {
			data["method"] = ref.Method
		}
		if ref.Path != "" {
			data["path"] = ref.Path
		}
		if ref.Message != "" {
			data["message"] = ref.Message
		}
		if ref.Package != "" {
			data["package"] = ref.Package
		}
		reporter.info("bind", "contract_reference", "contract reference discovered", BuildEvent{
			PageID: ref.OwnerID,
			Path:   ref.Source,
			Data:   data,
		})
	}
}

func reportRealtimeSubscriptions(reporter *buildReporter, subscriptions []gwdkir.RealtimeSubscription) {
	for _, subscription := range subscriptions {
		status := subscription.Status
		if status == "" {
			status = gwdkir.ContractBindingUnknown
		}
		data := map[string]string{
			"query":     subscription.Query,
			"event":     subscription.Event,
			"status":    string(status),
			"ownerKind": string(subscription.OwnerKind),
			"owner":     subscription.OwnerID,
		}
		if subscription.Span.Start.Line > 0 {
			data["line"] = fmt.Sprint(subscription.Span.Start.Line)
			data["column"] = fmt.Sprint(subscription.Span.Start.Column)
		}
		if subscription.QueryImportAlias != "" {
			data["queryImportAlias"] = subscription.QueryImportAlias
		}
		if subscription.QueryImportPath != "" {
			data["queryImportPath"] = subscription.QueryImportPath
		}
		if subscription.QueryType != "" {
			data["queryType"] = subscription.QueryType
		}
		if subscription.EventImportAlias != "" {
			data["eventImportAlias"] = subscription.EventImportAlias
		}
		if subscription.EventImportPath != "" {
			data["eventImportPath"] = subscription.EventImportPath
		}
		if subscription.EventType != "" {
			data["eventType"] = subscription.EventType
		}
		if subscription.EventCategory != "" {
			data["eventCategory"] = subscription.EventCategory
		}
		if subscription.Handler != "" {
			data["handler"] = subscription.Handler
		}
		if subscription.Register != "" {
			data["register"] = subscription.Register
		}
		if len(subscription.Roles) > 0 {
			data["roles"] = strings.Join(subscription.Roles, ",")
		}
		if len(subscription.Guards) > 0 {
			data["guards"] = strings.Join(subscription.Guards, ",")
		}
		if subscription.Message != "" {
			data["message"] = subscription.Message
		}
		if subscription.Package != "" {
			data["package"] = subscription.Package
		}
		reporter.info("bind", "realtime_subscription", "realtime subscription discovered", BuildEvent{
			PageID: subscription.OwnerID,
			Path:   subscription.Source,
			Data:   data,
		})
	}
}

func reportQueryInvalidations(reporter *buildReporter, invalidations []gwdkir.QueryInvalidation) {
	for _, invalidation := range invalidations {
		status := invalidation.Status
		if status == "" {
			status = gwdkir.ContractBindingUnknown
		}
		data := map[string]string{
			"query":         invalidation.Query,
			"queryType":     invalidation.QueryType,
			"event":         invalidation.Event,
			"eventType":     invalidation.EventType,
			"eventCategory": invalidation.EventCategory,
			"status":        string(status),
			"ownerKind":     string(invalidation.OwnerKind),
			"owner":         invalidation.OwnerID,
		}
		if invalidation.Span.Start.Line > 0 {
			data["line"] = fmt.Sprint(invalidation.Span.Start.Line)
			data["column"] = fmt.Sprint(invalidation.Span.Start.Column)
		}
		if invalidation.QueryImportAlias != "" {
			data["queryImportAlias"] = invalidation.QueryImportAlias
		}
		if invalidation.QueryImportPath != "" {
			data["queryImportPath"] = invalidation.QueryImportPath
		}
		if invalidation.EventImportPath != "" {
			data["eventImportPath"] = invalidation.EventImportPath
		}
		if len(invalidation.Guards) > 0 {
			data["guards"] = strings.Join(invalidation.Guards, ",")
		}
		if invalidation.Message != "" {
			data["message"] = invalidation.Message
		}
		if invalidation.Package != "" {
			data["package"] = invalidation.Package
		}
		reporter.info("bind", "query_invalidation", "query invalidation discovered", BuildEvent{
			PageID: invalidation.OwnerID,
			Path:   invalidation.Source,
			Data:   data,
		})
	}
}

func reportStructuredData(reporter *buildReporter, ir gwdkir.Program) {
	for _, page := range ir.Pages {
		if len(page.Metadata.Structured) == 0 {
			continue
		}
		var kinds []string
		for _, structured := range page.Metadata.Structured {
			if kind := strings.TrimSpace(structured.Kind); kind != "" {
				kinds = append(kinds, kind)
			}
		}
		reporter.info("seo", "structured_data", "structured data metadata planned", BuildEvent{
			PageID: page.ID,
			Route:  page.Route,
			Data: map[string]string{
				"kinds": strings.Join(kinds, ","),
				"count": fmt.Sprint(len(kinds)),
			},
		})
	}
}

func BuildIncremental(config gowdk.Config, sources gwdkanalysis.Sources, outputDir string, changedPageSources []string) (Result, error) {
	ir, bindings, err := compiler.AssembleProgram(config, sources)
	if err != nil {
		return Result{}, err
	}
	return buildIncrementalFromIR(config, ir, bindings, outputDir, changedPageSources)
}

// BuildIncrementalFromIR incrementally renders changed SPA page outputs from
// normalized compiler IR.
func BuildIncrementalFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string, changedPageSources []string) (Result, error) {
	return buildIncrementalFromIR(config, ir, compiler.BackendBindingsFromIR(ir), outputDir, changedPageSources)
}

func buildIncrementalFromIR(config gowdk.Config, ir gwdkir.Program, backendBindings []source.BackendBinding, outputDir string, changedPageSources []string) (Result, error) {
	reporter := newBuildReporter("incremental", outputDir)
	reporter.info("start", "build_started", "incremental SPA build started", BuildEvent{
		Data: map[string]string{
			"pages":          fmt.Sprint(len(ir.Pages)),
			"changedSources": fmt.Sprint(len(changedPageSources)),
		},
	})
	if strings.TrimSpace(outputDir) == "" {
		return Result{}, reporter.fail("validate", fmt.Errorf("build output directory is required"))
	}
	if err := compiler.ValidateProgram(config, ir); err != nil {
		return Result{}, reporter.fail("validate", err)
	}
	reporter.info("validate", "ir_valid", "compiler IR validation completed", BuildEvent{})
	reportBackendBindings(reporter, backendBindings)
	reportContractReferences(reporter, ir.ContractRefs)
	reportRealtimeSubscriptions(reporter, ir.RealtimeSubscriptions)
	reportStructuredData(reporter, ir)
	if err := compiler.ValidateBackendBindingPolicyIR(config, ir); err != nil {
		return Result{}, reporter.fail("bind", err)
	}

	changedPages := sourcePathSet(changedPageSources)
	components, componentFailures := buildComponents(ir.Components)
	layouts, layoutFailures := buildLayouts(ir.Layouts)
	css, cssFailures := planCSS(config, ir, outputDir, components, layouts)
	componentAssets, componentAssetFailures := planComponentFileAssets(ir.Assets, outputDir)
	scopedJS, scopedJSFailures := planScopedJSAssets(ir.Assets, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)
	actionFields := pageActionInputFields(ir)
	realtimeEventTypeNames := realtimeSubscriptionEventTypeNames(ir.RealtimeSubscriptions)
	queryTypeNames := queryInvalidationTypeNames(ir.QueryInvalidations)

	var failures []string
	failures = append(failures, componentFailures...)
	failures = append(failures, layoutFailures...)
	failures = append(failures, cssFailures...)
	failures = append(failures, componentAssetFailures...)
	failures = append(failures, scopedJSFailures...)
	if len(failures) > 0 {
		return Result{}, reporter.fail("plan", errors.New(strings.Join(failures, "\n")))
	}
	runtime, err := runtimeArtifacts(config, ir, outputDir, layouts, components)
	if err != nil {
		return Result{}, reporter.fail("plan", err)
	}
	runtime = append(componentAssets, append(scopedJS, runtime...)...)
	var obfuscations []assetObfuscationRecord
	runtime, obfuscations, err = applyAssetObfuscation(config, outputDir, runtime)
	if err != nil {
		return Result{}, reporter.fail("plan", err)
	}
	reporter.info("plan", "artifacts_planned", "incremental artifacts planned", BuildEvent{
		Data: map[string]string{
			"css":    fmt.Sprint(len(css.assets)),
			"assets": fmt.Sprint(len(runtime)),
		},
	})
	reportAssetObfuscation(reporter, config.Build.ObfuscateAssets, obfuscations)

	result := Result{
		Artifacts:      make([]Artifact, 0, len(ir.Pages)),
		CSSArtifacts:   make([]CSSArtifact, 0, len(css.assets)),
		AssetArtifacts: make([]AssetArtifact, 0, 1),
	}
	previousRoutes, err := readRouteManifestIfExists(outputDir)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	reporter.debug("manifest", "previous_route_manifest_read", "previous route manifest read", BuildEvent{
		Data: map[string]string{"routes": fmt.Sprint(len(previousRoutes.Routes))},
	})
	changedPageIDs := map[string]bool{}
	for _, artifact := range css.assets {
		wrote, err := writeFileIfChangedStatus(artifact.Path, artifact.contents)
		if err != nil {
			return Result{}, reporter.fail("write", err)
		}
		recordWriteStat(&result, wrote)
		reporter.debug("write", "css_written", "CSS artifact written", BuildEvent{Path: eventPath(outputDir, artifact.Path)})
		finalizeCSSArtifact(&artifact)
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
	}
	for _, artifact := range runtime {
		wrote, err := writeFileIfChangedStatus(artifact.Path, artifact.contents)
		if err != nil {
			return Result{}, reporter.fail("write", err)
		}
		recordWriteStat(&result, wrote)
		reporter.debug("write", "asset_written", "runtime asset written", BuildEvent{Path: eventPath(outputDir, artifact.Path)})
		finalizeAssetArtifact(&artifact)
		result.AssetArtifacts = append(result.AssetArtifacts, artifact.AssetArtifact)
	}

	seenOutputPaths := map[string]string{}
	for _, page := range ir.Pages {
		if isRequestTimePage(config, page) {
			continue
		}
		routeArtifacts, err := pageRouteArtifacts(config, outputDir, page)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range routeArtifacts {
			if _, err := relativeOutputPath(outputDir, artifact.Path); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", page.ID, err))
				continue
			}
			if previousPage, ok := seenOutputPaths[artifact.Path]; ok {
				failures = append(failures, pageOutputCollisionError(page, artifact.Route, previousPage))
				continue
			}
			seenOutputPaths[artifact.Path] = page.ID
			result.Artifacts = append(result.Artifacts, artifact)
		}

		if !sourcePathChanged(changedPages, page.Source) {
			continue
		}
		changedPageIDs[page.ID] = true
		stylesheets := append([]gowdk.Stylesheet{}, baseStylesheets...)
		stylesheets = append(stylesheets, css.pageStylesheets[page.ID]...)
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, layouts, stylesheets, actionFields[page.ID], realtimeEventTypeNames, queryTypeNames)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range pageArtifacts {
			wrote, err := writeFileIfChangedStatus(artifact.Path, artifact.contents)
			if err != nil {
				return Result{}, reporter.fail("write", err)
			}
			recordWriteStat(&result, wrote)
			reporter.debug("write", "page_written", "page artifact written", BuildEvent{
				PageID: artifact.PageID,
				Route:  artifact.Route,
				Path:   eventPath(outputDir, artifact.Path),
			})
		}
	}
	if len(failures) > 0 {
		return Result{}, reporter.fail("plan", errors.New(strings.Join(failures, "\n")))
	}
	reportSkippedPrerenderPages(reporter, config, ir)
	if err := removeStaleChangedPageArtifacts(outputDir, previousRoutes, result.Artifacts, changedPageIDs); err != nil {
		return Result{}, reporter.fail("cleanup", err)
	}
	reporter.info("cleanup", "stale_artifacts_removed", "stale changed-page artifacts removed", BuildEvent{
		Data: map[string]string{"changedPages": fmt.Sprint(len(changedPageIDs))},
	})

	endpoints := compiler.BuildRouteMetadataFromIR(config, ir).Endpoints
	manifestPath, err := writeRouteManifest(outputDir, result.Artifacts, endpoints)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.RouteManifestPath = manifestPath
	reporter.info("manifest", "route_manifest_written", "route manifest written", BuildEvent{Path: eventPath(outputDir, manifestPath)})
	seoPlan, err := planSEOArtifacts(config, ir, result.Artifacts)
	if err != nil {
		return Result{}, reporter.fail("seo", err)
	}
	reportSEOExclusions(reporter, seoPlan.Exclusions)
	sitemapPath, robotsPath, sitemapWrote, robotsWrote, err := writeSEOArtifacts(outputDir, seoPlan)
	if err != nil {
		return Result{}, reporter.fail("seo", err)
	}
	if sitemapPath != "" {
		recordWriteStat(&result, sitemapWrote)
		result.SitemapPath = sitemapPath
		reporter.info("seo", "sitemap_written", "sitemap written", BuildEvent{
			Path: eventPath(outputDir, sitemapPath),
			Data: map[string]string{"urls": fmt.Sprint(len(seoPlan.URLs))},
		})
	}
	if robotsPath != "" {
		recordWriteStat(&result, robotsWrote)
		result.RobotsPath = robotsPath
		reporter.info("seo", "robots_written", "robots.txt written", BuildEvent{Path: eventPath(outputDir, robotsPath)})
	}
	assetManifestPath, err := writeAssetManifest(outputDir, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.AssetManifestPath = assetManifestPath
	reporter.info("manifest", "asset_manifest_written", "asset manifest written", BuildEvent{Path: eventPath(outputDir, assetManifestPath)})
	reportCachePolicies(reporter, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	reportAssetSizes(reporter, outputDir, result.AssetArtifacts)
	openAPIPath, err := writeOpenAPI(outputDir, config, ir)
	if err != nil {
		return Result{}, reporter.fail("report", err)
	}
	result.OpenAPIPath = openAPIPath
	reporter.info("report", "openapi_written", "OpenAPI report written", BuildEvent{Path: eventPath(outputDir, openAPIPath)})
	securityManifestPath, err := writeSecurityManifest(outputDir, config, ir)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.SecurityManifestPath = securityManifestPath
	reporter.info("manifest", "security_manifest_written", "security manifest written", BuildEvent{Path: eventPath(outputDir, securityManifestPath)})
	reporter.info("complete", "build_complete", "incremental SPA build completed", BuildEvent{
		Data: map[string]string{
			"pages":        fmt.Sprint(len(result.Artifacts)),
			"changedPages": fmt.Sprint(len(changedPageIDs)),
			"css":          fmt.Sprint(len(result.CSSArtifacts)),
			"assets":       fmt.Sprint(len(result.AssetArtifacts)),
		},
	})
	result.Report = reporter.result()
	buildReportPath, err := writeBuildReport(outputDir, result.Report)
	if err != nil {
		return Result{}, reporter.fail("report", err)
	}
	result.BuildReportPath = buildReportPath
	return result, nil
}

func plan(config gowdk.Config, sources gwdkanalysis.Sources, outputDir string) (buildPlan, error) {
	ir, _, err := compiler.AssembleProgram(config, sources)
	if err != nil {
		return buildPlan{}, err
	}
	return planFromIR(config, ir, outputDir)
}

// reportSkippedPrerenderPages records a build-report event for every
// request-time page (SSR/Hybrid) that build-output prerendering intentionally
// does not emit as static HTML. The skip itself happens silently in
// planFromIR/BuildIncrementalFromIR; this surfaces it as a clear diagnostic so
// the build report makes the unsupported-for-prerender slice explicit.
func reportSkippedPrerenderPages(reporter *buildReporter, config gowdk.Config, ir gwdkir.Program) {
	for _, page := range ir.Pages {
		if !isRequestTimePage(config, page) {
			continue
		}
		reporter.info("plan", "request_time_page_skipped", "request-time page skipped from prerender output", BuildEvent{
			PageID: page.ID,
			Route:  page.Route,
			Data:   map[string]string{"mode": string(page.RenderMode(config.Render.DefaultMode()))},
		})
	}
}

func reportCachePolicies(reporter *buildReporter, pages []Artifact, css []CSSArtifact, assets []AssetArtifact) {
	data := map[string]string{
		"pageHtml":           fmt.Sprint(len(pages)),
		"css":                fmt.Sprint(len(css)),
		"assets":             fmt.Sprint(len(assets)),
		"defaultPageHTML":    noCacheAssetCachePolicy,
		"defaultRequestTime": "no-store",
	}
	if policies := cachePolicyCounts(pageCachePolicies(pages)); policies != "" {
		data["pageHTMLPolicies"] = policies
	}
	if policies := cachePolicyCounts(cssCachePolicies(css)); policies != "" {
		data["cssPolicies"] = policies
	}
	if policies := cachePolicyCounts(assetCachePolicies(assets)); policies != "" {
		data["assetPolicies"] = policies
	}
	reporter.info("report", "cache_policy", "cache policies summarized", BuildEvent{Data: data})
}

func reportAssetSizes(reporter *buildReporter, outputDir string, assets []AssetArtifact) {
	for _, artifact := range assets {
		rel := eventPath(outputDir, artifact.Path)
		logical := artifactLogicalPath(artifact.LogicalPath, rel)
		data := map[string]string{
			"bytes": fmt.Sprint(artifact.SizeBytes),
			"kind":  assetReportKind(logical),
		}
		if logical != "" && logical != rel {
			data["logicalPath"] = logical
		}
		if artifact.Hash != "" {
			data["hash"] = artifact.Hash
		}
		if artifact.CachePolicy != "" {
			data["cache"] = artifact.CachePolicy
		}
		if logical == islandWASMExecAssetPath() {
			data["wasmExecGoVersion"] = islandWASMExecGoVersion()
		}
		reporter.info("report", "asset_size", "generated asset size recorded", BuildEvent{
			Path: rel,
			Data: data,
		})
	}
}

func assetReportKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js":
		return "javascript"
	case ".wasm":
		return "wasm"
	case ".map":
		return "sourcemap"
	case ".css":
		return "css"
	default:
		return "asset"
	}
}

func pageCachePolicies(artifacts []Artifact) []string {
	policies := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		policy := artifact.CachePolicy
		if policy == "" {
			policy = noCacheAssetCachePolicy
		}
		policies = append(policies, policy)
	}
	return policies
}

func cssCachePolicies(artifacts []CSSArtifact) []string {
	policies := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		policy := artifact.CachePolicy
		if policy == "" {
			policy = immutableAssetCachePolicy
		}
		policies = append(policies, policy)
	}
	return policies
}

func assetCachePolicies(artifacts []AssetArtifact) []string {
	policies := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		policy := artifact.CachePolicy
		if policy == "" {
			policy = noCacheAssetCachePolicy
		}
		policies = append(policies, policy)
	}
	return policies
}

func cachePolicyCounts(policies []string) string {
	if len(policies) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, policy := range policies {
		counts[policy]++
	}
	keys := make([]string, 0, len(counts))
	for policy := range counts {
		keys = append(keys, policy)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, policy := range keys {
		key, err := json.Marshal(policy)
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d", key, counts[policy]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func planFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) (buildPlan, error) {
	components, componentFailures := buildComponents(ir.Components)
	layouts, layoutFailures := buildLayouts(ir.Layouts)
	css, cssFailures := planCSS(config, ir, outputDir, components, layouts)
	componentAssets, componentAssetFailures := planComponentFileAssets(ir.Assets, outputDir)
	scopedJS, scopedJSFailures := planScopedJSAssets(ir.Assets, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)
	actionFields := pageActionInputFields(ir)
	realtimeEventTypeNames := realtimeSubscriptionEventTypeNames(ir.RealtimeSubscriptions)
	queryTypeNames := queryInvalidationTypeNames(ir.QueryInvalidations)
	var planned []plannedArtifact
	var failures []string
	seenOutputPaths := map[string]string{}
	failures = append(failures, componentFailures...)
	failures = append(failures, layoutFailures...)
	failures = append(failures, cssFailures...)
	failures = append(failures, componentAssetFailures...)
	failures = append(failures, scopedJSFailures...)
	for _, page := range ir.Pages {
		if isRequestTimePage(config, page) {
			continue
		}
		stylesheets := append([]gowdk.Stylesheet{}, baseStylesheets...)
		stylesheets = append(stylesheets, css.pageStylesheets[page.ID]...)
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, layouts, stylesheets, actionFields[page.ID], realtimeEventTypeNames, queryTypeNames)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range pageArtifacts {
			if _, err := relativeOutputPath(outputDir, artifact.Path); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", page.ID, err))
				continue
			}
			if previousPage, ok := seenOutputPaths[artifact.Path]; ok {
				failures = append(failures, pageOutputCollisionError(page, artifact.Route, previousPage))
				continue
			}
			seenOutputPaths[artifact.Path] = page.ID
			planned = append(planned, artifact)
		}
	}
	if len(failures) > 0 {
		return buildPlan{}, errors.New(strings.Join(failures, "\n"))
	}
	runtime, err := runtimeArtifacts(config, ir, outputDir, layouts, components)
	if err != nil {
		return buildPlan{}, err
	}
	assets := append(componentAssets, scopedJS...)
	assets = append(assets, runtime...)
	assets, obfuscations, err := applyAssetObfuscation(config, outputDir, assets)
	if err != nil {
		return buildPlan{}, err
	}
	return buildPlan{pages: planned, css: css.assets, assets: assets, obfuscations: obfuscations}, nil
}

func pageBuildErrorPrefix(page gwdkir.Page) string {
	if sourcePath := strings.TrimSpace(page.Source); sourcePath != "" {
		return fmt.Sprintf("%s (%s)", page.ID, sourcePath)
	}
	return page.ID
}

func pageOutputCollisionError(page gwdkir.Page, route string, previousPage string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		route = strings.TrimSpace(page.Route)
	}
	if route != "" {
		return fmt.Sprintf("%s: page route %q duplicates page %s", pageBuildErrorPrefix(page), route, previousPage)
	}
	return fmt.Sprintf("%s: generated page output duplicates page %s", pageBuildErrorPrefix(page), previousPage)
}
