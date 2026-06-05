package staticgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/clientrt"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

func clientRuntimeArtifacts(pages []manifest.Page, outputDir string) []plannedAssetArtifact {
	for _, page := range pages {
		if pageUsesPartialRuntime(page, page.Blocks.ViewBody) {
			return []plannedAssetArtifact{{
				AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath))},
				contents:      clientrt.Source(),
			}}
		}
	}
	return nil
}

var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d}

func runtimeArtifacts(config gowdk.Config, app manifest.Manifest, outputDir string, layouts map[string]manifest.Layout) ([]plannedAssetArtifact, error) {
	var artifacts []plannedAssetArtifact
	artifacts = append(artifacts, clientRuntimeArtifacts(app.Pages, outputDir)...)
	islands, err := islandRuntimeArtifacts(config, app, outputDir, layouts)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, islands...)
	return dedupeAssetArtifacts(artifacts), nil
}

func islandRuntimeArtifacts(config gowdk.Config, app manifest.Manifest, outputDir string, layouts map[string]manifest.Layout) ([]plannedAssetArtifact, error) {
	stateful := statefulComponentNames(app.Components)
	componentBodies := componentViewBodies(app.Components)
	components := componentsByName(app.Components)
	includeSourceMaps := config.Build.DebugAssets()
	planned := map[string]plannedAssetArtifact{}
	for _, page := range app.Pages {
		source, err := composePageViewSource(page, layouts)
		if err != nil {
			source = page.Blocks.ViewBody
		}
		usages, err := recursiveComponentCallUsages(source, func(name string) (string, bool) {
			body, ok := componentBodies[name]
			return body, ok
		})
		if err != nil {
			continue
		}
		for _, usage := range usages {
			switch usage.Island {
			case "wasm":
				if _, exists := planned[filepath.Join(outputDir, filepath.FromSlash(islandWASMAssetPath(usage.Component)))]; !exists {
					component := components[usage.Component]
					artifact, err := islandWASMArtifact(outputDir, component)
					if err != nil {
						return nil, err
					}
					addAsset(planned, artifact)
				}
				addAsset(planned, islandWASMLoaderArtifact(outputDir, usage.Component))
			case "":
				if stateful[usage.Component] || usage.ReactiveProps {
					addAsset(planned, islandJSArtifact(outputDir, usage.Component, includeSourceMaps))
					if includeSourceMaps {
						component, ok := components[usage.Component]
						if !ok {
							continue
						}
						addAsset(planned, islandJSSourceMapArtifact(outputDir, component))
					}
				}
			}
		}
	}
	if len(planned) == 0 {
		return nil, nil
	}
	paths := make([]string, 0, len(planned))
	for path := range planned {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	artifacts := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		artifacts = append(artifacts, planned[path])
	}
	return artifacts, nil
}

func islandScriptHrefs(source string, components map[string]view.Component) []string {
	usages, err := recursiveComponentCallUsages(source, func(name string) (string, bool) {
		component, ok := components[name]
		if !ok {
			return "", false
		}
		return component.Body, true
	})
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var scripts []string
	for _, usage := range usages {
		href := ""
		switch usage.Island {
		case "wasm":
			href = "/" + islandWASMLoaderAssetPath(usage.Component)
		case "":
			component, ok := components[usage.Component]
			if ok && (component.StateJSON != "" || component.HandlersJSON != "" || len(component.Emits) > 0 || usage.ReactiveProps) {
				href = "/" + islandJSAssetPath(usage.Component)
			}
		}
		if href == "" || seen[href] {
			continue
		}
		seen[href] = true
		scripts = append(scripts, href)
	}
	sort.Strings(scripts)
	return scripts
}

func recursiveComponentCallUsages(source string, componentBody func(string) (string, bool)) ([]view.ComponentCallUsage, error) {
	var usages []view.ComponentCallUsage
	visiting := map[string]bool{}
	var walk func(string) error
	walk = func(source string) error {
		direct, err := view.ComponentCallUsages(source)
		if err != nil {
			return err
		}
		for _, usage := range direct {
			usages = append(usages, usage)
			if visiting[usage.Component] {
				continue
			}
			body, ok := componentBody(usage.Component)
			if !ok {
				continue
			}
			visiting[usage.Component] = true
			if err := walk(body); err != nil {
				return err
			}
			delete(visiting, usage.Component)
		}
		return nil
	}
	if err := walk(source); err != nil {
		return nil, err
	}
	return usages, nil
}

func statefulComponentNames(components []manifest.Component) map[string]bool {
	out := map[string]bool{}
	for _, component := range components {
		if component.State.Type.Name != "" || component.Blocks.Client || len(component.Emits) > 0 {
			out[component.Name] = true
		}
	}
	return out
}

func componentViewBodies(components []manifest.Component) map[string]string {
	out := map[string]string{}
	for _, component := range components {
		out[component.Name] = component.Blocks.ViewBody
	}
	return out
}

func componentsByName(components []manifest.Component) map[string]manifest.Component {
	out := map[string]manifest.Component{}
	for _, component := range components {
		out[component.Name] = component
	}
	return out
}

func addAsset(artifacts map[string]plannedAssetArtifact, artifact plannedAssetArtifact) {
	artifacts[artifact.Path] = artifact
}

func dedupeAssetArtifacts(artifacts []plannedAssetArtifact) []plannedAssetArtifact {
	if len(artifacts) < 2 {
		return artifacts
	}
	seen := map[string]plannedAssetArtifact{}
	for _, artifact := range artifacts {
		seen[artifact.Path] = artifact
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		out = append(out, seen[path])
	}
	return out
}

func islandJSArtifact(outputDir, componentName string, includeSourceMap bool) plannedAssetArtifact {
	assetPath := islandJSAssetPath(componentName)
	source := islandJSSource(componentName, includeSourceMap)
	if !includeSourceMap {
		source = compactGeneratedJSSource(source)
	}
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(source),
	}
}

func compactGeneratedJSSource(source string) string {
	var lines []string
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
}

func islandJSSourceMapArtifact(outputDir string, component manifest.Component) plannedAssetArtifact {
	assetPath := islandJSSourceMapAssetPath(component.Name)
	source := islandJSSource(component.Name, true)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      islandJSSourceMap(component, source),
	}
}

func islandWASMArtifact(outputDir string, component manifest.Component) (plannedAssetArtifact, error) {
	assetPath := islandWASMAssetPath(component.Name)
	contents := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	if strings.TrimSpace(component.WASM.Package) != "" {
		wasm, err := buildWASMIslandPackage(component)
		if err != nil {
			return plannedAssetArtifact{}, err
		}
		contents = wasm
	}
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      contents,
	}, nil
}

func buildWASMIslandPackage(component manifest.Component) ([]byte, error) {
	packagePath := strings.TrimSpace(component.WASM.Package)
	temp, err := os.CreateTemp("", "gowdk-"+componentAssetName(component.Name)+"-*.wasm")
	if err != nil {
		return nil, fmt.Errorf("component %s wasm package %q: create temp output: %w", component.Name, packagePath, err)
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("component %s wasm package %q: close temp output: %w", component.Name, packagePath, err)
	}
	defer os.Remove(tempPath)

	dir, buildPackage, err := wasmIslandBuildContext(packagePath)
	if err != nil {
		return nil, fmt.Errorf("component %s wasm package %q: %w", component.Name, packagePath, err)
	}
	if err := validateWASMIslandPackageImports(component, dir, buildPackage, packagePath); err != nil {
		return nil, err
	}
	command := exec.Command("go", "build", "-buildvcs=false", "-o", tempPath, buildPackage)
	if dir != "" {
		command.Dir = dir
	}
	command.Env = append(envWithout(os.Environ(), "GOOS", "GOARCH"), "GOOS=js", "GOARCH=wasm")
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("component %s wasm package %q failed to build with GOOS=js GOARCH=wasm: %w\n%s", component.Name, packagePath, err, strings.TrimSpace(string(output)))
	}
	contents, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, fmt.Errorf("component %s wasm package %q: read built artifact: %w", component.Name, packagePath, err)
	}
	if !bytes.HasPrefix(contents, wasmMagic) {
		return nil, fmt.Errorf("component %s wasm package %q did not produce a browser WASM module; declare a package main with a main function", component.Name, packagePath)
	}
	return contents, nil
}

var forbiddenWASMIslandImports = map[string]string{
	"net":          "network listeners belong in server api {} or act {} handlers",
	"net/http":     "HTTP clients and servers belong in server api {} or act {} handlers",
	"os/exec":      "process execution is not available in browser WASM islands",
	"plugin":       "Go plugins are not available in browser WASM islands",
	"syscall":      "use syscall/js for browser interop instead of syscall",
	"unsafe":       "unsafe is not allowed in browser WASM island packages",
	"database/sql": "database access belongs in server api {} or act {} handlers",
}

func validateWASMIslandPackageImports(component manifest.Component, dir, buildPackage, packagePath string) error {
	sourceDir, ok := wasmIslandLocalSourceDir(dir, buildPackage, packagePath)
	if !ok {
		return nil
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("component %s wasm package %q: read package source: %w", component.Name, packagePath, err)
	}
	fileset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filePath := filepath.Join(sourceDir, entry.Name())
		file, err := parser.ParseFile(fileset, filePath, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("component %s wasm package %q: parse imports in %s: %w", component.Name, packagePath, entry.Name(), err)
		}
		for _, item := range file.Imports {
			importPath, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return fmt.Errorf("component %s wasm package %q: parse import path in %s: %w", component.Name, packagePath, entry.Name(), err)
			}
			reason, forbidden := forbiddenWASMIslandImports[importPath]
			if !forbidden {
				continue
			}
			position := fileset.Position(item.Pos())
			return fmt.Errorf("component %s wasm package %q imports unsupported browser package %q at %s:%d: %s", component.Name, packagePath, importPath, filepath.ToSlash(filePath), position.Line, reason)
		}
	}
	return nil
}

func wasmIslandLocalSourceDir(dir, buildPackage, packagePath string) (string, bool) {
	if filepath.IsAbs(packagePath) {
		return packagePath, true
	}
	if strings.HasPrefix(packagePath, ".") {
		abs, err := filepath.Abs(packagePath)
		if err != nil {
			return "", false
		}
		return abs, true
	}
	if dir != "" && strings.HasPrefix(buildPackage, "./") {
		return filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(buildPackage, "./"))), true
	}
	return "", false
}

func wasmIslandBuildContext(packagePath string) (string, string, error) {
	if !filepath.IsAbs(packagePath) {
		return "", packagePath, nil
	}
	info, err := os.Stat(packagePath)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("package path is not a directory")
	}
	moduleRoot := packagePath
	for {
		if _, err := os.Stat(filepath.Join(moduleRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(moduleRoot)
		if parent == moduleRoot {
			return "", "", fmt.Errorf("no go.mod found for local package")
		}
		moduleRoot = parent
	}
	rel, err := filepath.Rel(moduleRoot, packagePath)
	if err != nil {
		return "", "", err
	}
	return moduleRoot, "./" + filepath.ToSlash(rel), nil
}

func envWithout(env []string, names ...string) []string {
	blocked := map[string]bool{}
	for _, name := range names {
		blocked[name+"="] = true
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		skip := false
		for prefix := range blocked {
			if strings.HasPrefix(entry, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, entry)
		}
	}
	return out
}

func islandWASMLoaderArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandWASMLoaderAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(islandWASMLoaderSource(componentName)),
	}
}

func islandJSAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".js")
}

func islandWASMAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".wasm")
}

func islandWASMLoaderAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".wasm.js")
}

func islandJSSourceMapAssetPath(componentName string) string {
	return islandJSAssetPath(componentName) + ".map"
}

func componentAssetName(componentName string) string {
	name := exportedSafe(componentName)
	if name == "" {
		return "component"
	}
	return name
}

func islandJSSource(componentName string, includeSourceMap bool) string {
	component := strconv.Quote(componentName)
	name := componentAssetName(componentName)
	mountFunction := "mount" + name + "Island"
	destroyFunction := "destroy" + name + "Island"
	source := fmt.Sprintf(`(() => {
  const component = %s;
  const selector = "gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"js\"]";
  const booleanAttrs = new Set(["allowfullscreen", "async", "autofocus", "autoplay", "checked", "controls", "default", "defer", "disabled", "formnovalidate", "hidden", "inert", "ismap", "loop", "multiple", "muted", "nomodule", "novalidate", "open", "readonly", "required", "reversed", "selected"]);
  const staleAsyncResult = Symbol("gowdk stale async result");
  const bindingTable = Object.freeze([
    { kind: "text", selector: "[data-gowdk-binding-text]", id: "data-gowdk-binding-text", field: "data-gowdk-bind" },
    { kind: "value", selector: "[data-gowdk-binding-value]", id: "data-gowdk-binding-value", field: "data-gowdk-bind-value" },
    { kind: "checked", selector: "[data-gowdk-binding-checked]", id: "data-gowdk-binding-checked", field: "data-gowdk-bind-checked" },
    { kind: "conditional", selector: "[data-gowdk-binding-if]", id: "data-gowdk-binding-if" },
    { kind: "list", selector: "[data-gowdk-binding-list]", id: "data-gowdk-binding-list" },
    { kind: "class", attrPrefix: "data-gowdk-binding-class-", valuePrefix: "data-gowdk-class-" },
    { kind: "style", attrPrefix: "data-gowdk-binding-style-", valuePrefix: "data-gowdk-style-", unitPrefix: "data-gowdk-style-unit-" },
    { kind: "attr", attrPrefix: "data-gowdk-binding-attr-", valuePrefix: "data-gowdk-attr-" }
  ]);
  const registry = window.__gowdkIslandRegistry || (window.__gowdkIslandRegistry = { components: Object.create(null), roots: new WeakMap() });
  window.__gowdkMountIslands = () => {
    Object.keys(registry.components).forEach((name) => registry.components[name](document));
  };
  window.__gowdkDestroyIslands = (scope, includeRoot) => {
    scope = scope || document;
    const roots = [];
    if (includeRoot && scope.matches && scope.matches("gowdk-island")) roots.push(scope);
    if (scope.querySelectorAll) scope.querySelectorAll("gowdk-island").forEach((root) => roots.push(root));
    roots.forEach((root) => {
      const destroy = registry.roots.get(root);
      if (destroy) destroy();
    });
  };

  function matchingBrace(source, openIndex) {
    let depth = 0;
    let inString = false;
    let escaped = false;
    for (let i = openIndex; i < source.length; i++) {
      const char = source[i];
      if (escaped) {
        escaped = false;
        continue;
      }
      if (inString) {
        if (char === "\\") escaped = true;
        else if (char === "\"") inString = false;
        continue;
      }
      if (char === "\"") inString = true;
      else if (char === "{") depth++;
      else if (char === "}") {
        depth--;
        if (depth === 0) return i;
      }
    }
    return -1;
  }

  function expressionSource(source) {
    source = source.trim();
    if (!source.startsWith("if ")) return source.replace(/\bnil\b/g, "null");
    const thenOpen = source.indexOf("{");
    if (thenOpen < 0) return source.replace(/\bnil\b/g, "null");
    const thenClose = matchingBrace(source, thenOpen);
    if (thenClose < 0) return source.replace(/\bnil\b/g, "null");
    const tail = source.slice(thenClose + 1).trim();
    if (!tail.startsWith("else")) return source.replace(/\bnil\b/g, "null");
    const elseOpen = source.indexOf("{", thenClose + 1);
    if (elseOpen < 0) return source.replace(/\bnil\b/g, "null");
    const elseClose = matchingBrace(source, elseOpen);
    if (elseClose < 0) return source.replace(/\bnil\b/g, "null");
    const cond = source.slice(2, thenOpen).trim();
    const thenExpr = source.slice(thenOpen + 1, thenClose).trim();
    const elseExpr = source.slice(elseOpen + 1, elseClose).trim();
    return "(" + expressionSource(cond) + " ? " + expressionSource(thenExpr) + " : " + expressionSource(elseExpr) + ")";
  }

  function callHelper(name, args, state, helpers, stack) {
    const helper = helpers && helpers[name];
    if (!helper) return null;
    stack = stack || [];
    if (stack.indexOf(name) >= 0) throw new Error("recursive GOWDK helper " + name);
    const nextScope = Object.create(null);
    (helper.params || []).forEach((param, index) => {
      nextScope[param] = args[index];
    });
    return valueOf(helper.return || "", state, nextScope, helpers, stack.concat([name]));
  }

  const builtins = Object.freeze({
    len(value) {
      if (value == null) return 0;
      if (typeof value === "string" || Array.isArray(value)) return value.length;
      return 0;
    },
    string(value) {
      if (value == null) return "";
      return String(value);
    },
    lower(value) {
      return String(value == null ? "" : value).toLowerCase();
    },
    upper(value) {
      return String(value == null ? "" : value).toUpperCase();
    },
    contains(value, query) {
      return String(value == null ? "" : value).includes(String(query == null ? "" : query));
    },
    int(value) {
      const next = Number.parseInt(value, 10);
      return Number.isNaN(next) ? 0 : next;
    },
    float(value) {
      const next = Number.parseFloat(value);
      return Number.isNaN(next) ? 0 : next;
    }
  });

  async function fetchJSON(url, signal) {
    const response = await fetch(String(url), { headers: { "Accept": "application/json" }, signal });
    if (!response.ok) throw new Error("GOWDK fetchJSON failed with HTTP " + response.status);
    const contentType = response.headers.get("content-type") || "";
    if (!/\bapplication\/json\b|\+json\b/i.test(contentType)) {
      throw new Error("GOWDK fetchJSON expected JSON response");
    }
    const text = await response.text();
    if (text.trim() === "") return null;
    try {
      return JSON.parse(text);
    } catch (_error) {
      throw new Error("GOWDK fetchJSON received invalid JSON");
    }
  }

  function clearAsyncError(state) {
    if (Object.prototype.hasOwnProperty.call(state, "Error")) state.Error = "";
  }

  function recordAsyncError(state, error) {
    if (Object.prototype.hasOwnProperty.call(state, "Error")) {
      state.Error = error && error.message ? error.message : String(error || "async error");
    }
    if (Object.prototype.hasOwnProperty.call(state, "Loading")) {
      state.Loading = false;
    }
  }

  function valueOf(token, state, scope, helpers, stack) {
    token = token.trim();
    if (token === "true") return true;
    if (token === "false") return false;
    if (token === "null" || token === "nil") return null;
    if (/^-?[0-9]+(?:\.[0-9]+)?$/.test(token)) return Number(token);
    if (token[0] === "\"") return JSON.parse(token);
    if (scope && Object.prototype.hasOwnProperty.call(scope, token)) return scope[token];
    if (Object.prototype.hasOwnProperty.call(state, token)) return state[token];
    const env = Object.assign(Object.create(null), builtins, state, scope || {});
    Object.keys(helpers || {}).forEach((name) => {
      env[name] = (...args) => callHelper(name, args, state, helpers, stack || []);
    });
    return Function("env", "with (env) { return (" + expressionSource(token) + "); }")(env);
  }

  function recomputeComputed(state, computeds, helpers) {
    (computeds || []).forEach((computed) => {
      state[computed.name] = valueOf(computed.expr, state, null, helpers);
    });
  }

  function splitArgs(source) {
    source = source.trim();
    if (!source) return [];
    const args = [];
    let start = 0;
    let depth = 0;
    let inString = false;
    let escaped = false;
    for (let i = 0; i < source.length; i++) {
      const char = source[i];
      if (escaped) {
        escaped = false;
        continue;
      }
      if (inString) {
        if (char === "\\") escaped = true;
        else if (char === "\"") inString = false;
        continue;
      }
      if (char === "\"") inString = true;
      else if (char === "(" || char === "[" || char === "{") depth++;
      else if (char === ")" || char === "]" || char === "}") depth--;
      else if (char === ",") {
        if (depth > 0) continue;
        args.push(source.slice(start, i).trim());
        start = i + 1;
      }
    }
    args.push(source.slice(start).trim());
    return args;
  }

  function emitComponentEvent(root, emitEvents, name, args, state, scope, helpers) {
    const event = emitEvents && emitEvents[name];
    if (!event) return;
    const payload = Object.create(null);
    (event.params || []).forEach((param, index) => {
      payload[param] = valueOf(args[index] || "", state, scope, helpers);
    });
    root.dispatchEvent(new CustomEvent(name, { detail: payload, bubbles: true }));
  }

  async function applyExpression(expr, state, handlers, helpers, scope, refs, computeds, asyncTokens, root, emitEvents) {
    expr = expr.trim();
    let local = expr.match(/^let\s+([A-Za-z_][A-Za-z0-9_]*)\s+[A-Za-z_][A-Za-z0-9_]*\s*=\s*(.+)$/);
    if (local) {
      if (!scope) return;
      scope[local[1]] = valueOf(local[2], state, scope, helpers);
      return;
    }
    let awaitedFetch = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*await\s+fetchJSON\[(.*)\]\((.*)\)$/);
    if (awaitedFetch) {
      const target = awaitedFetch[1];
      const token = (asyncTokens[target] || 0) + 1;
      const controllers = asyncTokens.__controllers || (asyncTokens.__controllers = Object.create(null));
      if (controllers[target] && typeof controllers[target].abort === "function") controllers[target].abort();
      const controller = typeof AbortController === "undefined" ? null : new AbortController();
      controllers[target] = controller;
      asyncTokens[target] = token;
      clearAsyncError(state);
      let next;
      try {
        next = await fetchJSON(valueOf(awaitedFetch[3], state, scope, helpers), controller ? controller.signal : undefined);
      } catch (error) {
        if (asyncTokens[target] !== token || (error && error.name === "AbortError")) throw staleAsyncResult;
        delete asyncTokens[target];
        if (controllers[target] === controller) delete controllers[target];
        throw error;
      }
      if (asyncTokens[target] !== token) throw staleAsyncResult;
      state[target] = next;
      delete asyncTokens[target];
      if (controllers[target] === controller) delete controllers[target];
      return;
    }
    let call = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\((.*)\)$/);
    if (call) {
      if (call[1] === "append" || call[1] === "remove" || call[1] === "move") {
        const args = splitArgs(call[2]);
        const field = (args[0] || "").trim();
        if (!Array.isArray(state[field])) return;
        if (call[1] === "append" && args.length === 2) {
          state[field] = state[field].concat([valueOf(args[1], state, scope, helpers)]);
          return;
        }
        if (call[1] === "remove" && args.length === 2) {
          const index = Number(valueOf(args[1], state, scope, helpers));
          if (!Number.isInteger(index) || index < 0 || index >= state[field].length) return;
          state[field] = state[field].slice(0, index).concat(state[field].slice(index + 1));
          return;
        }
        if (call[1] === "move" && args.length === 3) {
          const from = Number(valueOf(args[1], state, scope, helpers));
          const to = Number(valueOf(args[2], state, scope, helpers));
          if (!Number.isInteger(from) || !Number.isInteger(to) || from < 0 || from >= state[field].length || to < 0 || to >= state[field].length || from === to) return;
          const next = state[field].slice();
          const item = next.splice(from, 1)[0];
          next.splice(to, 0, item);
          state[field] = next;
          return;
        }
        return;
      }
      const handler = handlers[call[1]];
      if (!handler) return;
      const params = handler.params || [];
      const args = splitArgs(call[2]);
      const nextScope = Object.create(null);
      params.forEach((param, index) => {
        nextScope[param] = valueOf(args[index] || "", state, scope, helpers);
      });
      for (const statement of (handler.statements || handler)) {
        await applyExpression(statement, state, handlers, helpers, nextScope, refs, computeds, asyncTokens, root, emitEvents);
        recomputeComputed(state, computeds, helpers);
      }
      return;
    }
    let emit = expr.match(/^emit\s+([A-Za-z_][A-Za-z0-9_]*)\((.*)\)$/);
    if (emit) {
      emitComponentEvent(root, emitEvents, emit[1], splitArgs(emit[2]), state, scope, helpers);
      return;
    }
    let refCall = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\.(Focus|Blur|ScrollIntoView)\(\)$/);
    if (refCall) {
      const node = refs && refs[refCall[1]];
      if (!node) return;
      if (refCall[2] === "Focus" && typeof node.focus === "function") node.focus();
      else if (refCall[2] === "Blur" && typeof node.blur === "function") node.blur();
      else if (refCall[2] === "ScrollIntoView" && typeof node.scrollIntoView === "function") node.scrollIntoView();
      return;
    }
    let match = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)(\+\+|--)$/);
    if (match) {
      const current = Number(state[match[1]] || 0);
      state[match[1]] = match[2] === "++" ? current + 1 : current - 1;
      return;
    }
    match = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$/);
    if (match) {
      const target = match[1];
      const value = match[2].trim();
      const toggle = value.match(/^!\s*([A-Za-z_][A-Za-z0-9_]*)$/);
      state[target] = toggle ? !Boolean(valueOf(toggle[1], state, scope, helpers)) : valueOf(value, state, scope, helpers);
    }
  }

  async function applyStatements(statements, state, handlers, helpers, scope, refs, computeds, asyncTokens, root, emitEvents) {
    for (const statement of (statements || [])) {
      await applyExpression(statement, state, handlers, helpers, scope || null, refs, computeds, asyncTokens, root, emitEvents);
      recomputeComputed(state, computeds, helpers);
    }
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"']/g, (char) => {
      if (char === "&") return "&amp;";
      if (char === "<") return "&lt;";
      if (char === ">") return "&gt;";
      if (char === "\"") return "&quot;";
      return "&#39;";
    });
  }

  function interpolateTemplate(template, state, scope, helpers) {
    return template.replace(/\{\{([^{}]+)\}\}/g, (_match, expr) => {
      const value = valueOf(expr, state, scope, helpers);
      return value == null ? "" : escapeHTML(value);
    });
  }

  function firstTemplateElement(html) {
    const holder = document.createElement("template");
    holder.innerHTML = html.trim();
    return holder.content.firstElementChild;
  }

  function syncElement(target, source) {
    Array.from(target.attributes).forEach((attr) => {
      if (!source.hasAttribute(attr.name)) target.removeAttribute(attr.name);
    });
    Array.from(source.attributes).forEach((attr) => {
      if (target.getAttribute(attr.name) !== attr.value) target.setAttribute(attr.name, attr.value);
    });
    if (target.innerHTML !== source.innerHTML) target.innerHTML = source.innerHTML;
  }

  function matchingNodes(container, selector) {
    const nodes = [];
    if (container.matches && container.matches(selector)) nodes.push(container);
    container.querySelectorAll(selector).forEach((node) => nodes.push(node));
    return nodes;
  }

  function emptyBindings() {
    return { text: [], value: [], checked: [], classes: [], styles: [], attrs: [], conditionals: [], lists: [] };
  }

  function collectDirectBinding(bindings, spec, root) {
    matchingNodes(root, spec.selector).forEach((node) => {
      if (!ownsNode(root, node)) return;
      const id = node.getAttribute(spec.id);
      if (spec.kind === "text") bindings.text.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "value") bindings.value.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "checked") bindings.checked.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "conditional") bindings.conditionals.push({ id, node });
      else if (spec.kind === "list") bindings.lists.push({ id, node });
    });
  }

  function collectPrefixBinding(bindings, spec, node, attr) {
    if (!attr.name.startsWith(spec.attrPrefix)) return;
    const name = attr.name.slice(spec.attrPrefix.length);
    if (spec.kind === "class") {
      bindings.classes.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name) });
    } else if (spec.kind === "style") {
      bindings.styles.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name), unit: node.getAttribute(spec.unitPrefix + name) || "" });
    } else if (spec.kind === "attr") {
      bindings.attrs.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name) });
    }
  }

  function collectBindings(root) {
    const bindings = emptyBindings();
    bindingTable.filter((spec) => spec.selector).forEach((spec) => collectDirectBinding(bindings, spec, root));
    const prefixSpecs = bindingTable.filter((spec) => spec.attrPrefix);
    root.querySelectorAll("*").forEach((node) => {
      if (!ownsNode(root, node)) return;
      Array.from(node.attributes).forEach((attr) => {
        prefixSpecs.forEach((spec) => collectPrefixBinding(bindings, spec, node, attr));
      });
    });
    return bindings;
  }

  function renderConditionals(container, state, scope, helpers, options) {
    options = options || {};
    const shouldSkip = (node) => {
      if (options.owner && !ownsNode(options.owner, node)) return true;
      if (options.skipLoopItems && node.closest("[data-gowdk-for-item]")) return true;
      return false;
    };
    const conditionalNodes = options.bindings ? options.bindings.conditionals.map((binding) => binding.node).filter((node) => node.isConnected) : matchingNodes(container, "[data-gowdk-if]:not([data-gowdk-if-group]), [data-gowdk-if-group]");
    conditionalNodes.forEach((node) => {
      if (shouldSkip(node)) return;
      if (node.hasAttribute("data-gowdk-if-group")) return;
      if (!node.hasAttribute("data-gowdk-if")) return;
      node.hidden = !Boolean(valueOf(node.getAttribute("data-gowdk-if"), state, scope, helpers));
    });
    const conditionalGroups = new Map();
    conditionalNodes.forEach((node) => {
      if (shouldSkip(node)) return;
      if (!node.hasAttribute("data-gowdk-if-group")) return;
      const group = node.getAttribute("data-gowdk-if-group");
      if (!conditionalGroups.has(group)) conditionalGroups.set(group, []);
      conditionalGroups.get(group).push(node);
    });
    conditionalGroups.forEach((nodes) => {
      nodes.sort((left, right) => Number(left.getAttribute("data-gowdk-if-index")) - Number(right.getAttribute("data-gowdk-if-index")));
      let matched = false;
      nodes.forEach((node) => {
        const condition = node.getAttribute("data-gowdk-if");
        const visible = !matched && (condition == null || Boolean(valueOf(condition, state, scope, helpers)));
        node.hidden = !visible;
        if (visible) matched = true;
      });
    });
  }

  function renderListLoops(root, state, helpers, bindings) {
    const markers = bindings ? bindings.lists.map((binding) => binding.node).filter((node) => node.isConnected) : Array.from(root.querySelectorAll("template[data-gowdk-for]"));
    markers.forEach((marker) => {
      if (!ownsNode(root, marker)) return;
      const group = marker.getAttribute("data-gowdk-for");
      const itemName = marker.getAttribute("data-gowdk-for-var");
      const indexName = marker.getAttribute("data-gowdk-for-index-var");
      const source = marker.getAttribute("data-gowdk-for-source");
      const keyExpr = marker.getAttribute("data-gowdk-for-key");
      const template = marker.getAttribute("data-gowdk-for-template") || "";
      const items = valueOf(source, state, null, helpers);
      const existing = new Map();
      let cursor = marker.nextSibling;
      while (cursor) {
        const next = cursor.nextSibling;
        if (cursor.nodeType !== 1 || cursor.getAttribute("data-gowdk-for-item") !== group) break;
        existing.set(cursor.getAttribute("data-gowdk-key-value") || "", cursor);
        cursor = next;
      }
      if (!Array.isArray(items)) return;
      const fragment = document.createDocumentFragment();
      const used = new Set();
      items.forEach((item, index) => {
        const scope = Object.create(null);
        scope[itemName] = item;
        scope.index = index;
        if (indexName) scope[indexName] = index;
        const key = String(valueOf(keyExpr, state, scope, helpers) ?? "");
        const fresh = firstTemplateElement(interpolateTemplate(template, state, scope, helpers));
        if (!fresh) return;
        renderConditionals(fresh, state, scope, helpers);
        const reused = existing.get(key);
        if (reused && !used.has(key)) {
          syncElement(reused, fresh);
          fragment.appendChild(reused);
          used.add(key);
          return;
        }
        fragment.appendChild(fresh);
        used.add(key);
      });
      marker.parentNode.insertBefore(fragment, marker.nextSibling);
      existing.forEach((node, key) => {
        if (!used.has(key) && node.parentNode) node.parentNode.removeChild(node);
      });
    });
  }

  function eventModifiers(source) {
    const modifiers = { prevent: false, stop: false, once: false, capture: false, debounce: 0, throttle: 0 };
    (source || "").split(/\s+/).filter(Boolean).forEach((item) => {
      if (item === "prevent") modifiers.prevent = true;
      else if (item === "stop") modifiers.stop = true;
      else if (item === "once") modifiers.once = true;
      else if (item === "capture") modifiers.capture = true;
      else if (item.startsWith("debounce:")) modifiers.debounce = Number(item.slice("debounce:".length)) || 0;
      else if (item.startsWith("throttle:")) modifiers.throttle = Number(item.slice("throttle:".length)) || 0;
    });
    return modifiers;
  }

  function ownsNode(root, node) {
    return node.closest("gowdk-island") === root;
  }

  function syncChildProps(root, state, helpers) {
    root.querySelectorAll("gowdk-island[data-gowdk-props]").forEach((node) => {
      const props = JSON.parse(node.getAttribute("data-gowdk-props") || "{}");
      const payload = Object.create(null);
      Object.keys(props).forEach((name) => {
        payload[name] = valueOf(props[name], state, null, helpers);
      });
      node.dispatchEvent(new CustomEvent("gowdk:props", { detail: payload }));
    });
  }

  function updateTextBindings(bindings, state) {
    bindings.text.forEach(({ node, field }) => {
      node.textContent = state[field] == null ? "" : String(state[field]);
    });
  }

  function updateValueBindings(bindings, state) {
    bindings.value.forEach(({ node, field }) => {
      if (node.type === "radio") {
        node.checked = String(state[field] == null ? "" : state[field]) === node.value;
        return;
      }
      const value = state[field] == null ? "" : String(state[field]);
      if (document.activeElement !== node && node.value !== value) node.value = value;
    });
  }

  function updateCheckedBindings(bindings, state) {
    bindings.checked.forEach(({ node, field }) => {
      const checked = Boolean(state[field]);
      if (node.checked !== checked) node.checked = checked;
    });
  }

  function updateClassBindings(bindings, state, helpers) {
    bindings.classes.forEach(({ node, name, expression }) => {
      node.classList.toggle(name, Boolean(valueOf(expression, state, null, helpers)));
    });
  }

  function updateStyleBindings(bindings, state, helpers) {
    bindings.styles.forEach(({ node, name, expression, unit }) => {
      const value = valueOf(expression, state, null, helpers);
      if (value == null || value === false || value === "") node.style.removeProperty(name);
      else node.style.setProperty(name, String(value) + unit);
    });
  }

  function updateAttrBindings(bindings, state, helpers) {
    bindings.attrs.forEach(({ node, name, expression }) => {
      const value = valueOf(expression, state, null, helpers);
      if (booleanAttrs.has(name)) {
        if (Boolean(value)) node.setAttribute(name, "");
        else node.removeAttribute(name);
        return;
      }
      if (value == null || value === false) node.removeAttribute(name);
      else node.setAttribute(name, String(value));
    });
  }

  function updateBindings(root, state, helpers, bindings) {
    updateTextBindings(bindings, state);
    renderConditionals(root, state, null, helpers, { owner: root, skipLoopItems: true, bindings });
    updateValueBindings(bindings, state);
    updateCheckedBindings(bindings, state);
    updateClassBindings(bindings, state, helpers);
    updateStyleBindings(bindings, state, helpers);
    updateAttrBindings(bindings, state, helpers);
  }

  function render(root, state, helpers, bindings) {
    renderListLoops(root, state, helpers, bindings);
    bindings = collectBindings(root);
    updateBindings(root, state, helpers, bindings);
    root.setAttribute("data-gowdk-state", JSON.stringify(state));
    return bindings;
  }

  async function %s(scope) {
    scope = scope || document;
    scope.querySelectorAll(selector).forEach(async (root) => {
    if (root.getAttribute("data-gowdk-mounted") === "js") return;
    root.setAttribute("data-gowdk-mounted", "js");
    const state = JSON.parse(root.getAttribute("data-gowdk-state") || "{}");
    const client = JSON.parse(root.getAttribute("data-gowdk-client") || "{}");
    const hasEnvelope = Boolean(client.handlers || client.helpers || client.emits || client.mount || client.destroy || client.effects || client.computed);
    const handlers = hasEnvelope ? (client.handlers || {}) : client;
    const helpers = client.helpers || {};
    const emitEvents = client.emits || {};
    const mountStatements = client.mount || [];
    const destroyStatements = client.destroy || [];
    const effects = client.effects || [];
    const computeds = client.computed || [];
    const refs = Object.create(null);
    const asyncTokens = Object.create(null);
    recomputeComputed(state, computeds, helpers);
    root.querySelectorAll("[data-gowdk-ref]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      refs[node.getAttribute("data-gowdk-ref")] = node;
    });
    const effectValues = Object.create(null);
    const effectCleanups = Object.create(null);
    effects.forEach((effect) => {
      effectValues[effect.field] = state[effect.field];
    });
    let bindings = collectBindings(root);
    const runEffectCleanup = async (effect) => {
      const cleanup = effectCleanups[effect.field];
      if (!cleanup || cleanup.length === 0) return;
      effectCleanups[effect.field] = null;
      await applyStatements(cleanup, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
    };
    const runAllEffectCleanups = async () => {
      for (const effect of effects) {
        await runEffectCleanup(effect);
      }
    };
    const settleEffects = async () => {
      for (let pass = 0; pass < 10; pass++) {
        let ran = false;
        for (const effect of effects) {
          const current = state[effect.field];
          if (Object.is(effectValues[effect.field], current)) continue;
          await runEffectCleanup(effect);
          effectValues[effect.field] = current;
          await applyStatements(effect.statements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
          effectCleanups[effect.field] = effect.cleanup || null;
          ran = true;
        }
        if (!ran) return;
      }
    };
    const rerender = () => {
      bindings = render(root, state, helpers, bindings);
      bindInteractiveNodes();
      syncChildProps(root, state, helpers);
    };
    let renderScheduled = false;
    const scheduleRender = () => {
      if (renderScheduled) return;
      renderScheduled = true;
      const flush = () => {
        renderScheduled = false;
        rerender();
      };
      if (typeof queueMicrotask === "function") queueMicrotask(flush);
      else Promise.resolve().then(flush);
    };
    const bindInteractiveNodes = () => {
      root.querySelectorAll("*").forEach((node) => {
        const owned = ownsNode(root, node);
        if (owned && node.hasAttribute("data-gowdk-bind-value") && !node.hasAttribute("data-gowdk-bound-value")) {
          node.setAttribute("data-gowdk-bound-value", "");
          const field = node.getAttribute("data-gowdk-bind-value");
          const type = node.getAttribute("data-gowdk-bind-type") || "string";
          const event = node.tagName === "SELECT" || node.type === "radio" ? "change" : "input";
          node.addEventListener(event, async () => {
            if (node.type === "radio") {
              if (!node.checked) return;
              state[field] = node.value;
            } else if (type === "int") {
              const next = parseInt(node.value, 10);
              state[field] = Number.isNaN(next) ? 0 : next;
            } else if (type === "float") {
              const next = parseFloat(node.value);
              state[field] = Number.isNaN(next) ? 0 : next;
            } else {
              state[field] = node.value;
            }
            recomputeComputed(state, computeds, helpers);
            await settleEffects();
            recomputeComputed(state, computeds, helpers);
            scheduleRender();
          });
        }
        if (owned && node.hasAttribute("data-gowdk-bind-checked") && !node.hasAttribute("data-gowdk-bound-checked")) {
          node.setAttribute("data-gowdk-bound-checked", "");
          const field = node.getAttribute("data-gowdk-bind-checked");
          node.addEventListener("change", async () => {
            state[field] = node.checked;
            recomputeComputed(state, computeds, helpers);
            await settleEffects();
            recomputeComputed(state, computeds, helpers);
            scheduleRender();
          });
        }
        Array.from(node.attributes).forEach((attr) => {
          if (attr.name.startsWith("data-gowdk-parent-on-")) {
            const event = attr.name.slice("data-gowdk-parent-on-".length);
            const boundAttr = "data-gowdk-bound-parent-on-" + event;
            if (node.hasAttribute(boundAttr)) return;
            node.setAttribute(boundAttr, "");
            const modifiers = eventModifiers(node.getAttribute("data-gowdk-parent-event-" + event));
            let debounceTimer = 0;
            let throttleUntil = 0;
            const invoke = async (customEvent) => {
              const eventScope = Object.create(null);
              eventScope.event = customEvent.detail || {};
              try {
                await applyExpression(attr.value, state, handlers, helpers, eventScope, refs, computeds, asyncTokens, root, emitEvents);
              } catch (error) {
                if (error !== staleAsyncResult) recordAsyncError(state, error);
              } finally {
                recomputeComputed(state, computeds, helpers);
                await settleEffects();
                recomputeComputed(state, computeds, helpers);
                scheduleRender();
              }
            };
            const listener = (customEvent) => {
              if (modifiers.prevent) customEvent.preventDefault();
              if (modifiers.stop) customEvent.stopPropagation();
              if (modifiers.debounce > 0) {
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(() => invoke(customEvent), modifiers.debounce);
                return;
              }
              if (modifiers.throttle > 0) {
                const now = Date.now();
                if (now < throttleUntil) return;
                throttleUntil = now + modifiers.throttle;
              }
              invoke(customEvent);
            };
            node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });
            return;
          }
          if (!owned) return;
          if (!attr.name.startsWith("data-gowdk-on-")) return;
          const event = attr.name.slice("data-gowdk-on-".length);
          const boundAttr = "data-gowdk-bound-on-" + event;
          if (node.hasAttribute(boundAttr)) return;
          node.setAttribute(boundAttr, "");
          const modifiers = eventModifiers(node.getAttribute("data-gowdk-event-" + event));
          let debounceTimer = 0;
          let throttleUntil = 0;
          const invoke = async () => {
            try {
              await applyExpression(attr.value, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
            } catch (error) {
              if (error !== staleAsyncResult) recordAsyncError(state, error);
            } finally {
              recomputeComputed(state, computeds, helpers);
              await settleEffects();
              recomputeComputed(state, computeds, helpers);
              scheduleRender();
            }
          };
          const listener = (domEvent) => {
            if (modifiers.prevent) domEvent.preventDefault();
            if (modifiers.stop) domEvent.stopPropagation();
            if (modifiers.debounce > 0) {
              clearTimeout(debounceTimer);
              debounceTimer = setTimeout(invoke, modifiers.debounce);
              return;
            }
            if (modifiers.throttle > 0) {
              const now = Date.now();
              if (now < throttleUntil) return;
              throttleUntil = now + modifiers.throttle;
            }
            invoke();
          };
          node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });
        });
      });
    };
    root.addEventListener("gowdk:props", async (event) => {
      Object.assign(state, event.detail || {});
      recomputeComputed(state, computeds, helpers);
      await settleEffects();
      recomputeComputed(state, computeds, helpers);
      scheduleRender();
    });
    await applyStatements(mountStatements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
    await settleEffects();
    recomputeComputed(state, computeds, helpers);
    const destroyIsland = async function %s() {
      if (root.getAttribute("data-gowdk-mounted") !== "js") return;
      root.removeAttribute("data-gowdk-mounted");
      registry.roots.delete(root);
      if (destroyStatements.length > 0) {
        await runAllEffectCleanups();
        await applyStatements(destroyStatements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
      } else if (effects.length > 0) {
        await runAllEffectCleanups();
      }
    };
    registry.roots.set(root, destroyIsland);
    window.addEventListener("pagehide", destroyIsland, { once: true });
    rerender();
  });
  }

  registry.components[component] = %s;
  %s(document);
})();
`, component, mountFunction, destroyFunction, mountFunction, mountFunction)
	if includeSourceMap {
		source += "//# sourceMappingURL=" + path.Base(islandJSSourceMapAssetPath(componentName)) + "\n"
	}
	return source
}

type jsSourceMap struct {
	Version        int      `json:"version"`
	File           string   `json:"file"`
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent,omitempty"`
	Names          []string `json:"names"`
	Mappings       string   `json:"mappings"`
}

func islandJSSourceMap(component manifest.Component, generatedSource string) []byte {
	source := component.Source
	if source == "" {
		source = "components/" + component.Name + ".cmp.gwdk"
	}
	content := componentSourceMapContent(component)
	payload, err := json.MarshalIndent(jsSourceMap{
		Version:        3,
		File:           path.Base(islandJSAssetPath(component.Name)),
		Sources:        []string{source},
		SourcesContent: []string{content},
		Names:          []string{},
		Mappings:       sourceMapMappings(component, generatedSource),
	}, "", "  ")
	if err != nil {
		return []byte(`{"version":3,"file":"` + path.Base(islandJSAssetPath(component.Name)) + `","sources":[],"names":[],"mappings":""}`)
	}
	return append(payload, '\n')
}

type sourceMapAnchor struct {
	generatedLine int
	sourceLine    int
	sourceColumn  int
}

func sourceMapMappings(component manifest.Component, generatedSource string) string {
	anchors := sourceMapAnchors(component, generatedSource)
	if len(anchors) == 0 {
		return ""
	}
	byLine := map[int]sourceMapAnchor{}
	maxLine := 0
	for _, anchor := range anchors {
		if anchor.generatedLine <= 0 || anchor.sourceLine <= 0 || anchor.sourceColumn <= 0 {
			continue
		}
		if _, exists := byLine[anchor.generatedLine]; exists {
			continue
		}
		byLine[anchor.generatedLine] = anchor
		if anchor.generatedLine > maxLine {
			maxLine = anchor.generatedLine
		}
	}
	if maxLine == 0 {
		return ""
	}

	var builder strings.Builder
	previousGeneratedColumn := 0
	previousSourceIndex := 0
	previousSourceLine := 0
	previousSourceColumn := 0
	for line := 1; line <= maxLine; line++ {
		if line > 1 {
			builder.WriteByte(';')
			previousGeneratedColumn = 0
		}
		anchor, ok := byLine[line]
		if !ok {
			continue
		}
		sourceLine := anchor.sourceLine - 1
		sourceColumn := anchor.sourceColumn - 1
		builder.WriteString(sourceMapVLQ(0 - previousGeneratedColumn))
		builder.WriteString(sourceMapVLQ(0 - previousSourceIndex))
		builder.WriteString(sourceMapVLQ(sourceLine - previousSourceLine))
		builder.WriteString(sourceMapVLQ(sourceColumn - previousSourceColumn))
		previousGeneratedColumn = 0
		previousSourceIndex = 0
		previousSourceLine = sourceLine
		previousSourceColumn = sourceColumn
	}
	return builder.String()
}

func sourceMapAnchors(component manifest.Component, generatedSource string) []sourceMapAnchor {
	name := componentAssetName(component.Name)
	componentSpan := firstSourceSpan(component.Span, component.Blocks.Spans.Client, component.Blocks.Spans.View)
	clientSpan := firstSourceSpan(component.Blocks.Spans.Client, componentSpan)
	viewSpan := firstSourceSpan(component.Blocks.Spans.View, componentSpan)
	var anchors []sourceMapAnchor
	for index, line := range strings.Split(generatedSource, "\n") {
		lineNumber := index + 1
		switch {
		case strings.Contains(line, "const component = "):
			anchors = appendSourceMapAnchor(anchors, lineNumber, componentSpan)
		case sourceMapLineBelongsToClient(line, name):
			anchors = appendSourceMapAnchor(anchors, lineNumber, clientSpan)
		case sourceMapLineBelongsToView(line):
			anchors = appendSourceMapAnchor(anchors, lineNumber, viewSpan)
		}
	}
	return anchors
}

func sourceMapLineBelongsToClient(line, componentAssetName string) bool {
	return strings.Contains(line, "async function mount"+componentAssetName+"Island(scope)") ||
		strings.Contains(line, "async function applyExpression(") ||
		strings.Contains(line, "async function applyStatements(") ||
		strings.Contains(line, "function recomputeComputed(")
}

func sourceMapLineBelongsToView(line string) bool {
	return strings.Contains(line, "const bindingTable = Object.freeze(") ||
		strings.Contains(line, "function collectBindings(") ||
		strings.Contains(line, "function renderConditionals(") ||
		strings.Contains(line, "function renderListLoops(") ||
		strings.Contains(line, "function updateTextBindings(") ||
		strings.Contains(line, "function updateValueBindings(") ||
		strings.Contains(line, "function updateCheckedBindings(") ||
		strings.Contains(line, "function updateClassBindings(") ||
		strings.Contains(line, "function updateStyleBindings(") ||
		strings.Contains(line, "function updateAttrBindings(") ||
		strings.Contains(line, "function updateBindings(") ||
		strings.Contains(line, "function render(root, state, helpers, bindings)")
}

func appendSourceMapAnchor(anchors []sourceMapAnchor, generatedLine int, span manifest.SourceSpan) []sourceMapAnchor {
	if span.Start.Line <= 0 || span.Start.Column <= 0 {
		return anchors
	}
	return append(anchors, sourceMapAnchor{
		generatedLine: generatedLine,
		sourceLine:    span.Start.Line,
		sourceColumn:  span.Start.Column,
	})
}

func firstSourceSpan(spans ...manifest.SourceSpan) manifest.SourceSpan {
	for _, span := range spans {
		if span.Start.Line > 0 && span.Start.Column > 0 {
			return span
		}
	}
	return manifest.SourceSpan{}
}

const sourceMapBase64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func sourceMapVLQ(value int) string {
	vlq := value << 1
	if value < 0 {
		vlq = ((-value) << 1) + 1
	}
	var out strings.Builder
	for {
		digit := vlq & 31
		vlq >>= 5
		if vlq > 0 {
			digit |= 32
		}
		out.WriteByte(sourceMapBase64[digit])
		if vlq == 0 {
			break
		}
	}
	return out.String()
}

func componentSourceMapContent(component manifest.Component) string {
	if component.Blocks.ClientBody == "" {
		return "view {\n" + component.Blocks.ViewBody + "\n}\n"
	}
	return "client {\n" + component.Blocks.ClientBody + "\n}\n\nview {\n" + component.Blocks.ViewBody + "\n}\n"
}

func islandWASMLoaderSource(componentName string) string {
	component := strconv.Quote(componentName)
	wasmPath := strconv.Quote("/" + islandWASMAssetPath(componentName))
	return fmt.Sprintf(`(() => {
  const component = %s;
  const wasmPath = %s;
  const mountExport = "GOWDKMount" + component;
  const handleExport = "GOWDKHandle" + component;
  const destroyExport = "GOWDKDestroy" + component;
  const roots = document.querySelectorAll("gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"wasm\"]");
  if (roots.length === 0 || typeof WebAssembly === "undefined") return;

  function parseJSON(value, fallback) {
    try {
      return JSON.parse(value || "");
    } catch (_error) {
      return fallback;
    }
  }

  function ownsNode(root, node) {
    return node.closest("gowdk-island") === root;
  }

  function matchingNodes(root, selector) {
    const nodes = [];
    if (root.matches && root.matches(selector)) nodes.push(root);
    root.querySelectorAll(selector).forEach((node) => nodes.push(node));
    return nodes.filter((node) => ownsNode(root, node));
  }

  function collectRefs(root) {
    const refs = Object.create(null);
    root.querySelectorAll("[data-gowdk-ref]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      refs[node.getAttribute("data-gowdk-ref")] = node.getAttribute("data-gowdk-binding-ref") || "";
    });
    return refs;
  }

  function collectBindings(root) {
    const bindings = { text: [], attrs: [], classes: [], styles: [], conditionals: [], lists: [], events: [] };
    matchingNodes(root, "[data-gowdk-binding-text]").forEach((node) => {
      bindings.text.push({ id: node.getAttribute("data-gowdk-binding-text"), field: node.getAttribute("data-gowdk-bind") });
    });
    matchingNodes(root, "[data-gowdk-binding-if]").forEach((node) => {
      bindings.conditionals.push({ id: node.getAttribute("data-gowdk-binding-if"), expr: node.getAttribute("data-gowdk-if") || "" });
    });
    matchingNodes(root, "[data-gowdk-binding-list]").forEach((node) => {
      bindings.lists.push({ id: node.getAttribute("data-gowdk-binding-list"), source: node.getAttribute("data-gowdk-for-source") || "", key: node.getAttribute("data-gowdk-for-key") || "" });
    });
    root.querySelectorAll("*").forEach((node) => {
      if (!ownsNode(root, node)) return;
      Array.from(node.attributes).forEach((attr) => {
        if (attr.name.startsWith("data-gowdk-binding-on-")) {
          bindings.events.push({ id: attr.value, event: attr.name.slice("data-gowdk-binding-on-".length), expr: node.getAttribute("data-gowdk-on-" + attr.name.slice("data-gowdk-binding-on-".length)) || "" });
        } else if (attr.name.startsWith("data-gowdk-binding-attr-")) {
          const name = attr.name.slice("data-gowdk-binding-attr-".length);
          bindings.attrs.push({ id: attr.value, name, expr: node.getAttribute("data-gowdk-attr-" + name) || "" });
        } else if (attr.name.startsWith("data-gowdk-binding-class-")) {
          const name = attr.name.slice("data-gowdk-binding-class-".length);
          bindings.classes.push({ id: attr.value, name, expr: node.getAttribute("data-gowdk-class-" + name) || "" });
        } else if (attr.name.startsWith("data-gowdk-binding-style-")) {
          const name = attr.name.slice("data-gowdk-binding-style-".length);
          bindings.styles.push({ id: attr.value, name, expr: node.getAttribute("data-gowdk-style-" + name) || "", unit: node.getAttribute("data-gowdk-style-unit-" + name) || "" });
        }
      });
    });
    return bindings;
  }

  function bootstrap(root) {
    const client = parseJSON(root.getAttribute("data-gowdk-client"), {});
    return {
      component,
      state: parseJSON(root.getAttribute("data-gowdk-state"), {}),
      props: parseJSON(root.getAttribute("data-gowdk-props"), {}),
      emits: client.emits || {},
      refs: collectRefs(root),
      bindings: collectBindings(root)
    };
  }

  function targetByBinding(root, id) {
    if (!id) return null;
    const escaped = typeof CSS !== "undefined" && CSS.escape ? CSS.escape(id) : String(id).replace(/"/g, "\\\"");
    return root.querySelector("[data-gowdk-binding-text=\"" + escaped + "\"], [data-gowdk-binding-if=\"" + escaped + "\"], [data-gowdk-binding-list=\"" + escaped + "\"], [data-gowdk-binding-value=\"" + escaped + "\"], [data-gowdk-binding-checked=\"" + escaped + "\"]");
  }

  function applyPatch(root, patch) {
    if (!patch || typeof patch !== "object") return;
    const node = targetByBinding(root, patch.target || patch.binding);
    if (patch.type === "setText" && node) node.textContent = patch.value == null ? "" : String(patch.value);
    else if (patch.type === "setHidden" && node) node.hidden = Boolean(patch.value);
    else if (patch.type === "setAttr" && node && patch.name) node.setAttribute(patch.name, String(patch.value == null ? "" : patch.value));
    else if (patch.type === "removeAttr" && node && patch.name) node.removeAttribute(patch.name);
    else if (patch.type === "toggleClass" && node && patch.name) node.classList.toggle(patch.name, Boolean(patch.value));
    else if (patch.type === "setStyle" && node && patch.name) node.style.setProperty(patch.name, String(patch.value == null ? "" : patch.value));
    else if (patch.type === "emit" && patch.name) root.dispatchEvent(new CustomEvent(patch.name, { detail: patch.detail || {}, bubbles: true }));
  }

  function applyPatches(root, result) {
    const patches = typeof result === "string" ? parseJSON(result, []) : result;
    if (!Array.isArray(patches)) return;
    patches.forEach((patch) => applyPatch(root, patch));
  }

  function callExport(exports, name, payload) {
    const fn = exports && exports[name];
    if (typeof fn !== "function") return undefined;
    return fn(payload);
  }

  async function instantiate() {
    if (WebAssembly.instantiateStreaming) {
      try {
        return await WebAssembly.instantiateStreaming(fetch(wasmPath), {});
      } catch (_error) {
        // Fall through for servers that do not serve application/wasm yet.
      }
    }
    const response = await fetch(wasmPath);
    const bytes = await response.arrayBuffer();
    return WebAssembly.instantiate(bytes, {});
  }

  instantiate().then((result) => {
    const exports = result.instance && result.instance.exports || {};
    roots.forEach((root) => {
      const mountPayload = bootstrap(root);
      applyPatches(root, callExport(exports, mountExport, mountPayload));
      root.querySelectorAll("*").forEach((node) => {
        if (!ownsNode(root, node)) return;
        Array.from(node.attributes).forEach((attr) => {
          if (!attr.name.startsWith("data-gowdk-binding-on-")) return;
          const event = attr.name.slice("data-gowdk-binding-on-".length);
          node.addEventListener(event, (domEvent) => {
            applyPatches(root, callExport(exports, handleExport, {
              event,
              binding: attr.value,
              detail: { value: domEvent && domEvent.target ? domEvent.target.value : undefined }
            }));
          });
        });
      });
      window.addEventListener("pagehide", () => {
        applyPatches(root, callExport(exports, destroyExport, { component, state: parseJSON(root.getAttribute("data-gowdk-state"), {}) }));
      }, { once: true });
    });
  }).catch(() => {});
})();
`, component, wasmPath)
}
