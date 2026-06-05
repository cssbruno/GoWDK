package staticgen

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"

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

func runtimeArtifacts(app manifest.Manifest, outputDir string, layouts map[string]manifest.Layout) []plannedAssetArtifact {
	var artifacts []plannedAssetArtifact
	artifacts = append(artifacts, clientRuntimeArtifacts(app.Pages, outputDir)...)
	artifacts = append(artifacts, islandRuntimeArtifacts(app, outputDir, layouts)...)
	return dedupeAssetArtifacts(artifacts)
}

func islandRuntimeArtifacts(app manifest.Manifest, outputDir string, layouts map[string]manifest.Layout) []plannedAssetArtifact {
	stateful := statefulComponentNames(app.Components)
	componentBodies := componentViewBodies(app.Components)
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
				addAsset(planned, islandWASMArtifact(outputDir, usage.Component))
				addAsset(planned, islandWASMLoaderArtifact(outputDir, usage.Component))
			case "":
				if stateful[usage.Component] || usage.ReactiveProps {
					addAsset(planned, islandJSArtifact(outputDir, usage.Component))
				}
			}
		}
	}
	if len(planned) == 0 {
		return nil
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
	return artifacts
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

func islandJSArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandJSAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(islandJSSource(componentName)),
	}
}

func islandWASMArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandWASMAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00},
	}
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

func componentAssetName(componentName string) string {
	name := exportedSafe(componentName)
	if name == "" {
		return "component"
	}
	return name
}

func islandJSSource(componentName string) string {
	component := strconv.Quote(componentName)
	return fmt.Sprintf(`(() => {
  const component = %s;
  const selector = "gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"js\"]";
  const booleanAttrs = new Set(["allowfullscreen", "async", "autofocus", "autoplay", "checked", "controls", "default", "defer", "disabled", "formnovalidate", "hidden", "inert", "ismap", "loop", "multiple", "muted", "nomodule", "novalidate", "open", "readonly", "required", "reversed", "selected"]);
  const staleAsyncResult = Symbol("gowdk stale async result");
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

  function collectBindings(root) {
    const bindings = { text: [], value: [], checked: [], classes: [], styles: [], attrs: [], conditionals: [], lists: [] };
    matchingNodes(root, "[data-gowdk-binding-text]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      bindings.text.push({ id: node.getAttribute("data-gowdk-binding-text"), node, field: node.getAttribute("data-gowdk-bind") });
    });
    matchingNodes(root, "[data-gowdk-binding-value]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      bindings.value.push({ id: node.getAttribute("data-gowdk-binding-value"), node, field: node.getAttribute("data-gowdk-bind-value") });
    });
    matchingNodes(root, "[data-gowdk-binding-checked]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      bindings.checked.push({ id: node.getAttribute("data-gowdk-binding-checked"), node, field: node.getAttribute("data-gowdk-bind-checked") });
    });
    matchingNodes(root, "[data-gowdk-binding-if]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      bindings.conditionals.push({ id: node.getAttribute("data-gowdk-binding-if"), node });
    });
    matchingNodes(root, "[data-gowdk-binding-list]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      bindings.lists.push({ id: node.getAttribute("data-gowdk-binding-list"), node });
    });
    root.querySelectorAll("*").forEach((node) => {
      if (!ownsNode(root, node)) return;
      Array.from(node.attributes).forEach((attr) => {
        if (attr.name.startsWith("data-gowdk-binding-class-")) {
          const name = attr.name.slice("data-gowdk-binding-class-".length);
          bindings.classes.push({ id: attr.value, node, name, expression: node.getAttribute("data-gowdk-class-" + name) });
        } else if (attr.name.startsWith("data-gowdk-binding-style-")) {
          const name = attr.name.slice("data-gowdk-binding-style-".length);
          bindings.styles.push({ id: attr.value, node, name, expression: node.getAttribute("data-gowdk-style-" + name), unit: node.getAttribute("data-gowdk-style-unit-" + name) || "" });
        } else if (attr.name.startsWith("data-gowdk-binding-attr-")) {
          const name = attr.name.slice("data-gowdk-binding-attr-".length);
          bindings.attrs.push({ id: attr.value, node, name, expression: node.getAttribute("data-gowdk-attr-" + name) });
        }
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

  async function mountComponent(scope) {
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
    const destroyIsland = async () => {
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

  registry.components[component] = mountComponent;
  mountComponent(document);
})();
`, component)
}

func islandWASMLoaderSource(componentName string) string {
	component := strconv.Quote(componentName)
	wasmPath := strconv.Quote("/" + islandWASMAssetPath(componentName))
	return fmt.Sprintf(`(() => {
  const component = %s;
  const wasmPath = %s;
  const roots = document.querySelectorAll("gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"wasm\"]");
  if (roots.length === 0 || typeof WebAssembly === "undefined") return;
  WebAssembly.instantiateStreaming(fetch(wasmPath), {}).catch(() => {});
})();
`, component, wasmPath)
}
