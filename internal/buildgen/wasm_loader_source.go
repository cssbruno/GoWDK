package buildgen

import (
	"fmt"
	"strconv"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func clientGoBlockWASMLoaderSource(page gwdkir.Page) string {
	pageID := strconv.Quote(page.ID)
	loaderPath := strconv.Quote("/" + clientGoBlockWASMLoaderAssetPath(page))
	wasmPath := strconv.Quote("/" + clientGoBlockWASMAssetPath(page))
	wasmExecPath := strconv.Quote("/" + islandWASMExecAssetPath())
	mountExport := strconv.Quote(clientGoBlockMountExportName(page))
	return fmt.Sprintf(`(() => {
  const pageID = %s;
  const loaderPath = %s;
  const wasmPath = %s;
  const wasmExecPath = %s;
  const mountExport = %s;
  const registry = window.__gowdkClientGoBlockRegistry || (window.__gowdkClientGoBlockRegistry = { entries: Object.create(null) });
  window.__gowdkMountClientGoBlocks = () => {
    Object.keys(registry.entries).forEach((key) => registry.entries[key].mount());
  };
  if (typeof WebAssembly === "undefined") return;

  function currentPageUsesScript() {
    const expected = new URL(loaderPath, window.location.href).href;
    return Array.prototype.some.call(document.querySelectorAll("script[src]"), (script) => script.src === expected);
  }

  function loadGoRuntime() {
    if (window.Go) return Promise.resolve();
    if (window.__gowdkGoWASMLoading) return window.__gowdkGoWASMLoading;
    window.__gowdkGoWASMLoading = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = wasmExecPath;
      script.onload = resolve;
      script.onerror = () => reject(new Error("failed to load Go WASM runtime"));
      document.head.appendChild(script);
    });
    return window.__gowdkGoWASMLoading;
  }

  async function instantiate(go) {
    if (WebAssembly.instantiateStreaming) {
      try {
        return await WebAssembly.instantiateStreaming(fetch(wasmPath), go.importObject);
      } catch (_error) {}
    }
    const response = await fetch(wasmPath);
    const bytes = await response.arrayBuffer();
    return WebAssembly.instantiate(bytes, go.importObject);
  }

  loadGoRuntime().then(async () => {
    const go = new Go();
    const result = await instantiate(go);
    const exports = result.instance && result.instance.exports || {};
    if (typeof exports[mountExport] !== "function") {
      if (typeof console !== "undefined") console.error("GOWDK client go block missing export", mountExport);
      return;
    }
    const mountedBodies = new WeakSet();
    registry.entries[loaderPath] = {
      mount() {
        if (!currentPageUsesScript() || mountedBodies.has(document.body)) return;
        mountedBodies.add(document.body);
        try {
          exports[mountExport]();
        } catch (error) {
          if (typeof console !== "undefined") console.error("GOWDK client go block mount failed", pageID, error);
        }
      }
    };
    const run = go.run(result.instance);
    if (run && typeof run.catch === "function") {
      run.catch((error) => {
        if (typeof console !== "undefined") console.error("GOWDK client go block Go runtime failed", pageID, error);
      });
    }
    window.__gowdkMountClientGoBlocks();
  }).catch((error) => {
    if (typeof console !== "undefined") console.error("GOWDK client go block failed to start", pageID, error);
  });
})();
`, pageID, loaderPath, wasmPath, wasmExecPath, mountExport)
}

func islandWASMLoaderSource(componentName string) string {
	component := strconv.Quote(componentName)
	wasmPath := strconv.Quote("/" + islandWASMAssetPath(componentName))
	wasmExecPath := strconv.Quote("/" + islandWASMExecAssetPath())
	return fmt.Sprintf(`(() => {
  const component = %s;
  const wasmPath = %s;
  const wasmExecPath = %s;
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
    else if (patch.type === "replaceList" && node) node.innerHTML = Array.isArray(patch.html) ? patch.html.join("") : String(patch.html || "");
    else if (patch.type === "emit" && patch.name) root.dispatchEvent(new CustomEvent(patch.name, { detail: patch.detail || {}, bubbles: true }));
    else if (patch.type && typeof console !== "undefined") console.error("GOWDK WASM island rejected patch", patch.type, patch);
  }

  function applyPatches(root, result) {
    const patches = typeof result === "string" ? parseJSON(result, []) : result;
    if (!Array.isArray(patches)) return;
    patches.forEach((patch) => applyPatch(root, patch));
  }

  function callExport(exports, name, payload) {
    const fn = exports && exports[name];
    if (typeof fn !== "function") {
      if (typeof console !== "undefined") console.error("GOWDK WASM island missing export", name);
      return undefined;
    }
    return fn(payload);
  }

  function missingExports(exports) {
    return [mountExport, handleExport, destroyExport].filter((name) => typeof exports[name] !== "function");
  }

  function loadScript(src) {
    return new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = src;
      script.async = true;
      script.onload = resolve;
      script.onerror = () => reject(new Error("failed to load " + src));
      document.head.appendChild(script);
    });
  }

  async function loadGoRuntime() {
    if (typeof Go !== "function") {
      await loadScript(wasmExecPath);
    }
    if (typeof Go !== "function") return null;
    return new Go();
  }

  async function instantiateWithImports(imports) {
    if (WebAssembly.instantiateStreaming) {
      try {
        return await WebAssembly.instantiateStreaming(fetch(wasmPath), imports);
      } catch (_error) {
        // Fall through for servers that do not serve application/wasm yet.
      }
    }
    const response = await fetch(wasmPath);
    const bytes = await response.arrayBuffer();
    return WebAssembly.instantiate(bytes, imports);
  }

  async function instantiate() {
    try {
      return await instantiateWithImports({});
    } catch (directError) {
      const go = await loadGoRuntime();
      if (!go) throw directError;
      const result = await instantiateWithImports(go.importObject);
      const run = go.run(result.instance);
      if (run && typeof run.catch === "function") {
        run.catch((error) => {
          if (typeof console !== "undefined") console.error("GOWDK WASM island Go runtime failed", error);
        });
      }
      return result;
    }
  }

  instantiate().then((result) => {
    const exports = result.instance && result.instance.exports || {};
    const missing = missingExports(exports);
    if (missing.length > 0) {
      if (typeof console !== "undefined") console.error("GOWDK WASM island missing exports", missing.join(", "));
      return;
    }
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
  }).catch((error) => {
    if (typeof console !== "undefined") console.error("GOWDK WASM island failed to start", component, error);
  });
})();
`, component, wasmPath, wasmExecPath)
}
