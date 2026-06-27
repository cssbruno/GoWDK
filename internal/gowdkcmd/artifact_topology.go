package gowdkcmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
)

const appOutputDirName = "gowdkapp/app"

type artifactDestination struct {
	Target    string
	Role      string
	Kind      string
	RawPath   string
	AbsPath   string
	Canonical string
	PathType  string
}

func validateAdHocBuildTopology(root string, request buildRequest, timings bool) error {
	return validateArtifactTopology(root, destinationsForBuildRequest("adhoc", request, timings))
}

func validateConfiguredBuildTopology(root string, targets []gowdk.BuildTargetConfig, timings bool, timingsPath string) error {
	var destinations []artifactDestination
	for _, target := range targets {
		destinations = append(destinations, destinationsForBuildRequest(target.Name, buildRequest{
			OutputDir:         target.Output,
			AppDir:            target.App,
			BinaryPath:        target.Binary,
			WASMPath:          target.WASM,
			BackendAppDir:     target.BackendApp,
			BackendBinaryPath: target.BackendBinary,
			WorkerAppDir:      target.WorkerApp,
			WorkerBinaryPath:  target.WorkerBinary,
			CronAppDir:        target.CronApp,
			CronBinaryPath:    target.CronBinary,
			DeployRecipes:     target.DeployRecipes,
			TimingsPath:       timingsPath,
		}, timings)...)
	}
	return validateArtifactTopology(root, destinations)
}

func validateCleanArtifactOwnership(root string, config gowdk.Config, selectedNames []string, candidates []string) error {
	if len(config.Build.Targets) == 0 || len(candidates) == 0 {
		return nil
	}
	allTargets, err := selectBuildTargets(config.Build.Targets, nil)
	if err != nil {
		return err
	}
	selectedTargets, err := selectBuildTargets(config.Build.Targets, selectedNames)
	if err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, target := range selectedTargets {
		selected[target.Name] = true
	}
	if len(selectedNames) == 0 {
		for _, target := range allTargets {
			selected[target.Name] = true
		}
	}
	cleanRoots, err := normalizeArtifactDestinations(root, cleanCandidateDestinations(candidates))
	if err != nil {
		return err
	}
	var unselected []artifactDestination
	for _, target := range allTargets {
		if selected[target.Name] {
			continue
		}
		unselected = append(unselected, destinationsForBuildRequest(target.Name, buildRequest{
			OutputDir:         target.Output,
			AppDir:            target.App,
			BinaryPath:        target.Binary,
			WASMPath:          target.WASM,
			BackendAppDir:     target.BackendApp,
			BackendBinaryPath: target.BackendBinary,
			WorkerAppDir:      target.WorkerApp,
			WorkerBinaryPath:  target.WorkerBinary,
			CronAppDir:        target.CronApp,
			CronBinaryPath:    target.CronBinary,
			DeployRecipes:     target.DeployRecipes,
		}, false)...)
	}
	owned, err := normalizeArtifactDestinations(root, unselected)
	if err != nil {
		return err
	}
	for _, cleanRoot := range cleanRoots {
		for _, destination := range owned {
			if cleanRoot.Canonical == destination.Canonical || (cleanRoot.PathType == "dir" && pathContains(cleanRoot.Canonical, destination.Canonical)) {
				return fmt.Errorf("clean_output_overlap: clean target %q contains target %q %s %q; select both targets or clean a narrower path", cleanRoot.RawPath, destination.Target, destination.Kind, destination.RawPath)
			}
		}
	}
	return nil
}

func cleanCandidateDestinations(candidates []string) []artifactDestination {
	destinations := make([]artifactDestination, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		destinations = append(destinations, artifactDestination{
			Target:   "clean",
			Role:     "clean",
			Kind:     "clean-root",
			RawPath:  candidate,
			PathType: "dir",
		})
	}
	return destinations
}

func destinationsForBuildRequest(target string, request buildRequest, timings bool) []artifactDestination {
	var destinations []artifactDestination
	add := func(role, kind, pathType, raw string) {
		if strings.TrimSpace(raw) == "" {
			return
		}
		destinations = append(destinations, artifactDestination{
			Target:   target,
			Role:     role,
			Kind:     kind,
			RawPath:  strings.TrimSpace(raw),
			PathType: pathType,
		})
	}

	add("frontend", "static-output", "dir", request.OutputDir)
	add("frontend", "frontend-app", "dir", request.AppDir)
	if strings.TrimSpace(request.AppDir) != "" {
		add("frontend", "frontend-embedded-output", "dir", filepath.Join(request.AppDir, appOutputDirName))
	}
	add("frontend", "frontend-binary", "file", request.BinaryPath)
	add("frontend", "frontend-wasm", "file", request.WASMPath)
	add("backend", "backend-app", "dir", request.BackendAppDir)
	add("backend", "backend-binary", "file", request.BackendBinaryPath)
	add("worker", "worker-app", "dir", request.WorkerAppDir)
	add("worker", "worker-binary", "file", request.WorkerBinaryPath)
	add("cron", "cron-app", "dir", request.CronAppDir)
	add("cron", "cron-binary", "file", request.CronBinaryPath)

	if strings.TrimSpace(request.OutputDir) != "" {
		add("compiler", "build-report", "file", filepath.Join(request.OutputDir, "gowdk-build-report.json"))
		add("compiler", "route-manifest", "file", filepath.Join(request.OutputDir, "gowdk-routes.json"))
		add("compiler", "asset-manifest", "file", filepath.Join(request.OutputDir, "gowdk-assets.json"))
		if timings && strings.TrimSpace(request.TimingsPath) == "" {
			add("compiler", "timings-report", "file", filepath.Join(request.OutputDir, buildTimingsFile))
		}
	}
	if timings && strings.TrimSpace(request.TimingsPath) != "" {
		add("compiler", "timings-report", "file", request.TimingsPath)
	}
	if request.Docker && strings.TrimSpace(request.BinaryPath) != "" {
		dir := filepath.Dir(request.BinaryPath)
		add("deploy", "dockerfile", "file", filepath.Join(dir, "Dockerfile"))
		add("deploy", "dockerignore", "file", filepath.Join(dir, ".dockerignore"))
	}
	for _, recipe := range request.DeployRecipes {
		switch recipe {
		case deployRecipeStatic:
			add("deploy", "deployment-recipe", "file", filepath.Join(request.OutputDir, "deploy", "static-host.md"))
		case deployRecipeSplit:
			add("deploy", "deployment-recipe", "file", filepath.Join(request.OutputDir, "deploy", "split-frontend-backend.md"))
		case deployRecipeSystem:
			if binary := deploymentRecipeBinaryPath(deploymentRecipeRequest{
				BinaryPath:        request.BinaryPath,
				BackendBinaryPath: request.BackendBinaryPath,
				WorkerBinaryPath:  request.WorkerBinaryPath,
				CronBinaryPath:    request.CronBinaryPath,
			}); strings.TrimSpace(binary) != "" {
				add("deploy", "deployment-recipe", "file", filepath.Join(filepath.Dir(binary), deploymentServiceName(binary)+".service"))
			}
		case deployRecipeCaddy:
			if binary := deploymentRecipeBinaryPath(deploymentRecipeRequest{BinaryPath: request.BinaryPath, BackendBinaryPath: request.BackendBinaryPath}); strings.TrimSpace(binary) != "" {
				add("deploy", "deployment-recipe", "file", filepath.Join(filepath.Dir(binary), "Caddyfile"))
			}
		case deployRecipeNginx:
			if binary := deploymentRecipeBinaryPath(deploymentRecipeRequest{BinaryPath: request.BinaryPath, BackendBinaryPath: request.BackendBinaryPath}); strings.TrimSpace(binary) != "" {
				add("deploy", "deployment-recipe", "file", filepath.Join(filepath.Dir(binary), "nginx.gowdk.conf"))
			}
		}
	}
	return destinations
}

func validateArtifactTopology(root string, destinations []artifactDestination) error {
	normalized, err := normalizeArtifactDestinations(root, destinations)
	if err != nil {
		return err
	}
	var conflicts []string
	byPath := map[string]artifactDestination{}
	for _, destination := range normalized {
		if previous, exists := byPath[destination.Canonical]; exists {
			if !sameDestinationOwner(previous, destination) {
				conflicts = append(conflicts, artifactCollision("build_output_collision", previous, destination, "share the same destination"))
			}
			continue
		}
		byPath[destination.Canonical] = destination
	}
	for i := range normalized {
		for j := i + 1; j < len(normalized); j++ {
			left, right := normalized[i], normalized[j]
			if left.Canonical == right.Canonical {
				continue
			}
			if conflict := containmentConflict(left, right); conflict != "" {
				conflicts = append(conflicts, conflict)
			}
		}
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return errors.New(strings.Join(conflicts, "\n"))
	}
	return nil
}

func normalizeArtifactDestinations(root string, destinations []artifactDestination) ([]artifactDestination, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	normalized := make([]artifactDestination, 0, len(destinations))
	for _, destination := range destinations {
		raw := strings.TrimSpace(destination.RawPath)
		if raw == "" {
			continue
		}
		abs := raw
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(rootAbs, raw)
		}
		abs = filepath.Clean(abs)
		destination.AbsPath = abs
		destination.Canonical = canonicalArtifactPath(abs)
		normalized = append(normalized, destination)
	}
	return normalized, nil
}

func canonicalArtifactPath(abs string) string {
	probe := abs
	var suffix []string
	for {
		if resolved, err := filepath.EvalSymlinks(probe); err == nil {
			parts := append([]string{resolved}, reverseStrings(suffix)...)
			path := filepath.Join(parts...)
			if caseFoldArtifactPaths() {
				return strings.ToLower(path)
			}
			return path
		}
		next := filepath.Dir(probe)
		if next == probe {
			break
		}
		suffix = append(suffix, filepath.Base(probe))
		probe = next
	}
	clean := filepath.Clean(abs)
	if caseFoldArtifactPaths() {
		return strings.ToLower(clean)
	}
	return clean
}

func reverseStrings(values []string) []string {
	out := make([]string, len(values))
	for i := range values {
		out[len(values)-1-i] = values[i]
	}
	return out
}

func caseFoldArtifactPaths() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}

func containmentConflict(left, right artifactDestination) string {
	if left.PathType == "dir" && pathContains(left.Canonical, right.Canonical) && !allowedContainment(left, right) {
		return artifactCollision("build_output_overlap", left, right, "contains an unsupported generated artifact destination")
	}
	if right.PathType == "dir" && pathContains(right.Canonical, left.Canonical) && !allowedContainment(right, left) {
		return artifactCollision("build_output_overlap", right, left, "contains an unsupported generated artifact destination")
	}
	return ""
}

func allowedContainment(parent, child artifactDestination) bool {
	if parent.Target == child.Target && parent.Kind == "frontend-app" && child.Kind == "frontend-embedded-output" {
		return true
	}
	if parent.Target == child.Target && parent.Kind == "static-output" && (child.Kind == "build-report" || child.Kind == "route-manifest" || child.Kind == "asset-manifest" || child.Kind == "timings-report" || child.Kind == "deployment-recipe") {
		return true
	}
	if parent.Kind == "static-output" && (strings.HasSuffix(child.Kind, "-binary") || child.Kind == "frontend-wasm" || strings.HasSuffix(child.Kind, "-app") || child.Kind == "frontend-embedded-output") {
		return false
	}
	if strings.HasSuffix(parent.Kind, "-app") && (strings.HasSuffix(child.Kind, "-binary") || child.Kind == "frontend-wasm" || strings.HasSuffix(child.Kind, "-app") || child.Kind == "static-output") {
		return false
	}
	if parent.Kind == "static-output" && child.Kind == "static-output" {
		return false
	}
	if strings.HasSuffix(parent.Kind, "-app") && strings.HasSuffix(child.Kind, "-app") {
		return false
	}
	return false
}

func sameDestinationOwner(left, right artifactDestination) bool {
	return left.Target == right.Target && left.Role == right.Role && left.Kind == right.Kind
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(filepath.ToSlash(rel), "../")
}

func artifactCollision(code string, left, right artifactDestination, reason string) string {
	return fmt.Sprintf("%s: target %q %s %q and target %q %s %q resolve to %q: %s",
		code,
		left.Target,
		left.Kind,
		left.RawPath,
		right.Target,
		right.Kind,
		right.RawPath,
		left.AbsPath,
		reason,
	)
}
