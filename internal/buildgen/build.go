package buildgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func Build(config gowdk.Config, sources gwdkanalysis.Sources, outputDir string) (Result, error) {
	ir := gwdkanalysis.BuildProgram(config, sources)
	return buildFromIR(config, ir, compiler.BackendBindingsFromIR(ir), outputDir, true)
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
	reporter := newBuildReporter("build", outputDir)
	reporter.info("start", "build_started", "SPA build started", BuildEvent{
		Data: map[string]string{
			"pages":      fmt.Sprint(len(ir.Pages)),
			"components": fmt.Sprint(len(ir.Components)),
			"layouts":    fmt.Sprint(len(ir.Layouts)),
		},
	})
	if strings.TrimSpace(outputDir) == "" {
		return Result{}, reporter.fail("validate", fmt.Errorf("build output directory is required"))
	}
	if validate {
		if err := compiler.ValidateProgram(config, ir); err != nil {
			return Result{}, reporter.fail("validate", err)
		}
	} else if err := gwdkir.CheckInvariants(ir); err != nil {
		return Result{}, reporter.fail("validate", fmt.Errorf("internal compiler error: %w", err))
	}
	reporter.info("validate", "ir_valid", "compiler IR validation completed", BuildEvent{})
	reportBackendBindings(reporter, backendBindings)
	reportContractReferences(reporter, ir.ContractRefs)
	if err := compiler.ValidateBackendBindingPolicyIR(config, ir); err != nil {
		return Result{}, reporter.fail("bind", err)
	}

	planned, err := planFromIR(config, ir, outputDir)
	if err != nil {
		return Result{}, reporter.fail("plan", err)
	}
	reporter.info("plan", "artifacts_planned", "app artifacts planned", BuildEvent{
		Data: map[string]string{
			"pages":  fmt.Sprint(len(planned.pages)),
			"css":    fmt.Sprint(len(planned.css)),
			"assets": fmt.Sprint(len(planned.assets)),
		},
	})
	reportSkippedPrerenderPages(reporter, config, ir)

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
	manifestPath, err := writeRouteManifest(outputDir, result.Artifacts)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.RouteManifestPath = manifestPath
	reporter.info("manifest", "route_manifest_written", "route manifest written", BuildEvent{Path: eventPath(outputDir, manifestPath)})
	assetManifestPath, err := writeAssetManifest(outputDir, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.AssetManifestPath = assetManifestPath
	reporter.info("manifest", "asset_manifest_written", "asset manifest written", BuildEvent{Path: eventPath(outputDir, assetManifestPath)})
	reportCachePolicies(reporter, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	reportAssetSizes(reporter, outputDir, result.AssetArtifacts)
	openAPIPath, err := writeOpenAPI(outputDir, ir)
	if err != nil {
		return Result{}, reporter.fail("report", err)
	}
	result.OpenAPIPath = openAPIPath
	reporter.info("report", "openapi_written", "OpenAPI report written", BuildEvent{Path: eventPath(outputDir, openAPIPath)})
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
	ir := gwdkanalysis.BuildProgram(config, sources)
	return buildMemoryFromIR(config, ir, compiler.BackendBindingsFromIR(ir), outputDir)
}

// BuildMemoryFromIR plans SPA build artifacts from normalized compiler IR
// without writing them to disk.
func BuildMemoryFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) (MemoryResult, error) {
	return buildMemoryFromIR(config, ir, compiler.BackendBindingsFromIR(ir), outputDir)
}

func buildMemoryFromIR(config gowdk.Config, ir gwdkir.Program, backendBindings []source.BackendBinding, outputDir string) (MemoryResult, error) {
	reporter := newBuildReporter("memory", outputDir)
	reporter.info("start", "build_started", "in-memory SPA build started", BuildEvent{
		Data: map[string]string{
			"pages":      fmt.Sprint(len(ir.Pages)),
			"components": fmt.Sprint(len(ir.Components)),
			"layouts":    fmt.Sprint(len(ir.Layouts)),
		},
	})
	if strings.TrimSpace(outputDir) == "" {
		return MemoryResult{}, reporter.fail("validate", fmt.Errorf("build output directory is required"))
	}
	if err := compiler.ValidateProgram(config, ir); err != nil {
		return MemoryResult{}, reporter.fail("validate", err)
	}
	reporter.info("validate", "ir_valid", "compiler IR validation completed", BuildEvent{})
	reportBackendBindings(reporter, backendBindings)
	reportContractReferences(reporter, ir.ContractRefs)
	if err := compiler.ValidateBackendBindingPolicyIR(config, ir); err != nil {
		return MemoryResult{}, reporter.fail("bind", err)
	}

	planned, err := planFromIR(config, ir, outputDir)
	if err != nil {
		return MemoryResult{}, reporter.fail("plan", err)
	}
	reporter.info("plan", "artifacts_planned", "app artifacts planned", BuildEvent{
		Data: map[string]string{
			"pages":  fmt.Sprint(len(planned.pages)),
			"css":    fmt.Sprint(len(planned.css)),
			"assets": fmt.Sprint(len(planned.assets)),
		},
	})
	reportSkippedPrerenderPages(reporter, config, ir)

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

	routeManifest, err := routeManifestPayload(outputDir, result.Artifacts)
	if err != nil {
		return MemoryResult{}, reporter.fail("manifest", err)
	}
	result.Files[routeManifestFile] = routeManifest
	reporter.info("manifest", "route_manifest_collected", "route manifest collected", BuildEvent{Path: routeManifestFile})
	assetManifest, err := assetManifestPayload(outputDir, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	if err != nil {
		return MemoryResult{}, reporter.fail("manifest", err)
	}
	result.Files[assetManifestFile] = assetManifest
	reporter.info("manifest", "asset_manifest_collected", "asset manifest collected", BuildEvent{Path: assetManifestFile})
	reportCachePolicies(reporter, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	reportAssetSizes(reporter, outputDir, result.AssetArtifacts)
	openAPI, err := openAPIPayload(ir)
	if err != nil {
		return MemoryResult{}, reporter.fail("report", err)
	}
	result.Files[openAPIFile] = openAPI
	reporter.info("report", "openapi_collected", "OpenAPI report collected", BuildEvent{Path: openAPIFile})
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

func BuildIncremental(config gowdk.Config, sources gwdkanalysis.Sources, outputDir string, changedPageSources []string) (Result, error) {
	return buildIncrementalFromIR(config, gwdkanalysis.BuildProgram(config, sources), outputDir, changedPageSources)
}

// BuildIncrementalFromIR incrementally renders changed SPA page outputs from
// normalized compiler IR.
func BuildIncrementalFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string, changedPageSources []string) (Result, error) {
	return buildIncrementalFromIR(config, ir, outputDir, changedPageSources)
}

func buildIncrementalFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string, changedPageSources []string) (Result, error) {
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
	reportContractReferences(reporter, ir.ContractRefs)
	if err := compiler.ValidateBackendBindingPolicyIR(config, ir); err != nil {
		return Result{}, reporter.fail("bind", err)
	}

	changedPages := sourcePathSet(changedPageSources)
	components, componentFailures := buildComponents(ir.Components)
	layouts, layoutFailures := buildLayouts(ir.Layouts)
	css, cssFailures := planCSS(config, ir, outputDir)
	componentAssets, componentAssetFailures := planComponentFileAssets(ir.Assets, outputDir)
	scopedJS, scopedJSFailures := planScopedJSAssets(ir.Assets, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)
	actionFields := pageActionInputFields(ir)

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
	reporter.info("plan", "artifacts_planned", "incremental artifacts planned", BuildEvent{
		Data: map[string]string{
			"css":    fmt.Sprint(len(css.assets)),
			"assets": fmt.Sprint(len(runtime)),
		},
	})

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
		routeArtifacts, err := pageRouteArtifacts(outputDir, page)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range routeArtifacts {
			rel, err := relativeOutputPath(outputDir, artifact.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", page.ID, err))
				continue
			}
			if previousPage, ok := seenOutputPaths[artifact.Path]; ok {
				failures = append(failures, fmt.Sprintf("%s: generated output path %q duplicates page %s", page.ID, rel, previousPage))
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
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, layouts, stylesheets, actionFields[page.ID])
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

	manifestPath, err := writeRouteManifest(outputDir, result.Artifacts)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.RouteManifestPath = manifestPath
	reporter.info("manifest", "route_manifest_written", "route manifest written", BuildEvent{Path: eventPath(outputDir, manifestPath)})
	assetManifestPath, err := writeAssetManifest(outputDir, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	if err != nil {
		return Result{}, reporter.fail("manifest", err)
	}
	result.AssetManifestPath = assetManifestPath
	reporter.info("manifest", "asset_manifest_written", "asset manifest written", BuildEvent{Path: eventPath(outputDir, assetManifestPath)})
	reportCachePolicies(reporter, result.Artifacts, result.CSSArtifacts, result.AssetArtifacts)
	reportAssetSizes(reporter, outputDir, result.AssetArtifacts)
	openAPIPath, err := writeOpenAPI(outputDir, ir)
	if err != nil {
		return Result{}, reporter.fail("report", err)
	}
	result.OpenAPIPath = openAPIPath
	reporter.info("report", "openapi_written", "OpenAPI report written", BuildEvent{Path: eventPath(outputDir, openAPIPath)})
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
	return planFromIR(config, gwdkanalysis.BuildProgram(config, sources), outputDir)
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
			data["wasmExecGoVersion"] = goruntime.Version()
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
	css, cssFailures := planCSS(config, ir, outputDir)
	componentAssets, componentAssetFailures := planComponentFileAssets(ir.Assets, outputDir)
	scopedJS, scopedJSFailures := planScopedJSAssets(ir.Assets, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)
	actionFields := pageActionInputFields(ir)
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
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, layouts, stylesheets, actionFields[page.ID])
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range pageArtifacts {
			rel, err := relativeOutputPath(outputDir, artifact.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", page.ID, err))
				continue
			}
			if previousPage, ok := seenOutputPaths[artifact.Path]; ok {
				failures = append(failures, fmt.Sprintf("%s: generated output path %q duplicates page %s", page.ID, rel, previousPage))
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
	return buildPlan{pages: planned, css: css.assets, assets: assets}, nil
}
