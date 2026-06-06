package buildgen

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func Build(config gowdk.Config, app manifest.Manifest, outputDir string) (Result, error) {
	return buildFromIR(config, app, gwdkanalysis.BuildIR(config, app), outputDir)
}

// BuildFromIR writes SPA build artifacts from normalized compiler IR.
func BuildFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) (Result, error) {
	return buildFromIR(config, buildModelFromIR(ir), ir, outputDir)
}

func buildFromIR(config gowdk.Config, app manifest.Manifest, ir gwdkir.Program, outputDir string) (Result, error) {
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
	if err := compiler.ValidateManifest(config, app); err != nil {
		return Result{}, reporter.fail("validate", err)
	}
	reporter.info("validate", "manifest_valid", "manifest validation completed", BuildEvent{})
	reportBackendBindings(reporter, app.BackendBindings)
	if err := compiler.ValidateBackendBindingPolicy(config, app); err != nil {
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

	result := Result{
		Artifacts:      make([]Artifact, 0, len(planned.pages)),
		CSSArtifacts:   make([]CSSArtifact, 0, len(planned.css)),
		AssetArtifacts: make([]AssetArtifact, 0, len(planned.assets)),
	}
	for _, artifact := range planned.css {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, reporter.fail("write", err)
		}
		reporter.debug("write", "css_written", "CSS artifact written", BuildEvent{Path: eventPath(outputDir, artifact.Path)})
		artifact.CSSArtifact.Hash = contentHash(artifact.contents)
		artifact.CSSArtifact.CachePolicy = immutableAssetCachePolicy
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
	}
	for _, artifact := range planned.assets {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, reporter.fail("write", err)
		}
		reporter.debug("write", "asset_written", "runtime asset written", BuildEvent{Path: eventPath(outputDir, artifact.Path)})
		artifact.AssetArtifact.Hash = contentHash(artifact.contents)
		artifact.AssetArtifact.CachePolicy = immutableAssetCachePolicy
		result.AssetArtifacts = append(result.AssetArtifacts, artifact.AssetArtifact)
	}
	for _, artifact := range planned.pages {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, reporter.fail("write", err)
		}
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

func BuildMemory(config gowdk.Config, app manifest.Manifest, outputDir string) (MemoryResult, error) {
	return buildMemoryFromIR(config, app, gwdkanalysis.BuildIR(config, app), outputDir)
}

// BuildMemoryFromIR plans SPA build artifacts from normalized compiler IR
// without writing them to disk.
func BuildMemoryFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) (MemoryResult, error) {
	return buildMemoryFromIR(config, buildModelFromIR(ir), ir, outputDir)
}

func buildMemoryFromIR(config gowdk.Config, app manifest.Manifest, ir gwdkir.Program, outputDir string) (MemoryResult, error) {
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
	if err := compiler.ValidateManifest(config, app); err != nil {
		return MemoryResult{}, reporter.fail("validate", err)
	}
	reporter.info("validate", "manifest_valid", "manifest validation completed", BuildEvent{})
	reportBackendBindings(reporter, app.BackendBindings)
	if err := compiler.ValidateBackendBindingPolicy(config, app); err != nil {
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

	result := MemoryResult{
		Result: Result{
			Artifacts:         make([]Artifact, 0, len(planned.pages)),
			CSSArtifacts:      make([]CSSArtifact, 0, len(planned.css)),
			AssetArtifacts:    make([]AssetArtifact, 0, len(planned.assets)),
			RouteManifestPath: filepath.Join(outputDir, routeManifestFile),
			AssetManifestPath: filepath.Join(outputDir, assetManifestFile),
			BuildReportPath:   buildReportPath(outputDir),
		},
		Files: map[string][]byte{},
	}
	for _, artifact := range planned.css {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return MemoryResult{}, reporter.fail("memory", err)
		}
		artifact.CSSArtifact.Hash = contentHash(artifact.contents)
		artifact.CSSArtifact.CachePolicy = immutableAssetCachePolicy
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
		result.Files[rel] = append([]byte(nil), artifact.contents...)
		reporter.debug("memory", "css_collected", "CSS artifact collected", BuildEvent{Path: rel})
	}
	for _, artifact := range planned.assets {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return MemoryResult{}, reporter.fail("memory", err)
		}
		artifact.AssetArtifact.Hash = contentHash(artifact.contents)
		artifact.AssetArtifact.CachePolicy = immutableAssetCachePolicy
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

func reportBackendBindings(reporter *buildReporter, bindings []manifest.BackendBinding) {
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

func BuildIncremental(config gowdk.Config, app manifest.Manifest, outputDir string, changedPageSources []string) (Result, error) {
	return buildIncrementalFromIR(config, app, gwdkanalysis.BuildIR(config, app), outputDir, changedPageSources)
}

// BuildIncrementalFromIR incrementally renders changed SPA page outputs from
// normalized compiler IR.
func BuildIncrementalFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string, changedPageSources []string) (Result, error) {
	return buildIncrementalFromIR(config, buildModelFromIR(ir), ir, outputDir, changedPageSources)
}

func buildIncrementalFromIR(config gowdk.Config, app manifest.Manifest, ir gwdkir.Program, outputDir string, changedPageSources []string) (Result, error) {
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
	if err := compiler.ValidateManifest(config, app); err != nil {
		return Result{}, reporter.fail("validate", err)
	}
	reporter.info("validate", "manifest_valid", "manifest validation completed", BuildEvent{})
	if err := compiler.ValidateBackendBindingPolicy(config, app); err != nil {
		return Result{}, reporter.fail("bind", err)
	}

	changedPages := sourcePathSet(changedPageSources)
	components, componentFailures := buildComponents(app.Components)
	layouts, layoutFailures := buildLayouts(app.Layouts)
	css, cssFailures := planCSS(config, app, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)

	var failures []string
	failures = append(failures, componentFailures...)
	failures = append(failures, layoutFailures...)
	failures = append(failures, cssFailures...)
	if len(failures) > 0 {
		return Result{}, reporter.fail("plan", errors.New(strings.Join(failures, "\n")))
	}
	runtime, err := runtimeArtifacts(config, app, outputDir, layouts, components)
	if err != nil {
		return Result{}, reporter.fail("plan", err)
	}
	reporter.info("plan", "artifacts_planned", "incremental artifacts planned", BuildEvent{
		Data: map[string]string{
			"css":    fmt.Sprint(len(css.assets)),
			"assets": fmt.Sprint(len(runtime)),
		},
	})

	result := Result{
		Artifacts:      make([]Artifact, 0, len(app.Pages)),
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
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, reporter.fail("write", err)
		}
		reporter.debug("write", "css_written", "CSS artifact written", BuildEvent{Path: eventPath(outputDir, artifact.Path)})
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
	}
	for _, artifact := range runtime {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, reporter.fail("write", err)
		}
		reporter.debug("write", "asset_written", "runtime asset written", BuildEvent{Path: eventPath(outputDir, artifact.Path)})
		result.AssetArtifacts = append(result.AssetArtifacts, artifact.AssetArtifact)
	}

	seenOutputPaths := map[string]string{}
	for _, page := range app.Pages {
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
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, layouts, stylesheets)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range pageArtifacts {
			if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
				return Result{}, reporter.fail("write", err)
			}
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

func plan(config gowdk.Config, app manifest.Manifest, outputDir string) (buildPlan, error) {
	return planFromIR(config, gwdkanalysis.BuildIR(config, app), outputDir)
}

func planFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) (buildPlan, error) {
	app := buildModelFromIR(ir)
	components, componentFailures := buildComponents(app.Components)
	layouts, layoutFailures := buildLayouts(app.Layouts)
	css, cssFailures := planCSS(config, app, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)
	var planned []plannedArtifact
	var failures []string
	seenOutputPaths := map[string]string{}
	failures = append(failures, componentFailures...)
	failures = append(failures, layoutFailures...)
	failures = append(failures, cssFailures...)
	for _, page := range app.Pages {
		if isRequestTimePage(config, page) {
			continue
		}
		stylesheets := append([]gowdk.Stylesheet{}, baseStylesheets...)
		stylesheets = append(stylesheets, css.pageStylesheets[page.ID]...)
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, layouts, stylesheets)
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
	runtime, err := runtimeArtifacts(config, app, outputDir, layouts, components)
	if err != nil {
		return buildPlan{}, err
	}
	return buildPlan{pages: planned, css: css.assets, assets: runtime}, nil
}
