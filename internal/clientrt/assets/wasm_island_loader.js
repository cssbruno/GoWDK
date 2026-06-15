(() => {
  const component = "__GOWDK_COMPONENT__";
  const componentID = "__GOWDK_COMPONENT_ID__";
  const abiVersion = "__GOWDK_ABI_VERSION__";
  const wasmPath = "__GOWDK_WASM_PATH__";
  const wasmExecPath = "__GOWDK_WASM_EXEC_PATH__";
  const mountExport = "GOWDKMount" + component;
  const handleExport = "GOWDKHandle" + component;
  const destroyExport = "GOWDKDestroy" + component;
  const bindingTargetAttributes = ["data-gowdk-binding-text", "data-gowdk-binding-if", "data-gowdk-binding-list", "data-gowdk-binding-value", "data-gowdk-binding-checked"];
  const roots = document.querySelectorAll("gowdk-island[data-gowdk-component-id=\"" + componentID + "\"][data-gowdk-runtime=\"wasm\"],gowdk-island:not([data-gowdk-component-id])[data-gowdk-component=\"" + componentID + "\"][data-gowdk-runtime=\"wasm\"]");
  if (roots.length === 0 || typeof WebAssembly === "undefined") return;

  function tracedFetch(url, options, name) {
    if (window.__gowdkTrace && window.__gowdkTrace.fetch) {
      return window.__gowdkTrace.fetch(url, options || {}, { name: name || "wasm island fetch", lane: "island" });
    }
    return fetch(url, options);
  }

  function traceStart(name) {
    if (window.__gowdkTrace && window.__gowdkTrace.enabled && window.__gowdkTrace.enabled()) {
      return window.__gowdkTrace.start(name, "island");
    }
    return null;
  }

  function traceEnd(span, status, message) {
    if (window.__gowdkTrace && window.__gowdkTrace.end) window.__gowdkTrace.end(span, status || "ok", message || "");
  }

  function currentTraceparent() {
    if (window.__gowdkTrace && window.__gowdkTrace.traceparent) return window.__gowdkTrace.traceparent();
    return "";
  }

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

  function storeNamesFor(root) {
    const client = parseJSON(root.getAttribute("data-gowdk-client"), {});
    return Array.isArray(client.stores) ? client.stores : [];
  }

  // mergedState reads the island's seed state and merges in the current value of
  // every page store it `use`s, so a WASM island mounts and handles events with
  // the shared (and persisted) store value rather than its build-time seed. This
  // mirrors the JS island read path.
  function mergedState(root) {
    const state = parseJSON(root.getAttribute("data-gowdk-state"), {});
    const registry = window.__gowdkStores;
    if (registry) {
      storeNamesFor(root).forEach((name) => Object.assign(state, registry.get(name)));
    }
    return state;
  }

  function bootstrap(root) {
    const client = parseJSON(root.getAttribute("data-gowdk-client"), {});
    return {
      abiVersion,
      component,
      state: mergedState(root),
      stores: storeNamesFor(root),
      props: parseJSON(root.getAttribute("data-gowdk-props"), {}),
      emits: client.emits || {},
      refs: collectRefs(root),
      bindings: collectBindings(root)
    };
  }

  function targetByBinding(root, id) {
    if (!id) return null;
    const expected = String(id);
    for (const attr of bindingTargetAttributes) {
      const nodes = matchingNodes(root, "[" + attr + "]");
      for (const node of nodes) {
        if (node.getAttribute(attr) === expected) return node;
      }
    }
    return null;
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

  // normalizeResult accepts either the legacy bare patch array (or its JSON
  // string) or an extended { patches, stores } object. The stores map lets a
  // WASM island surface its current store-bound state so the loader can write it
  // back to window.__gowdkStores, completing store participation (read + write).
  function normalizeResult(result) {
    const value = typeof result === "string" ? parseJSON(result, null) : result;
    if (Array.isArray(value)) return { patches: value, stores: null };
    if (value && typeof value === "object") {
      return {
        patches: Array.isArray(value.patches) ? value.patches : [],
        stores: value.stores && typeof value.stores === "object" ? value.stores : null
      };
    }
    return { patches: [], stores: null };
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
        return await WebAssembly.instantiateStreaming(tracedFetch(wasmPath, {}, "wasm island module"), imports);
      } catch (_error) {
        // Fall through for servers that do not serve application/wasm yet.
      }
    }
    const response = await tracedFetch(wasmPath, {}, "wasm island module");
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
    const islandRegistry = window.__gowdkIslandRegistry || (window.__gowdkIslandRegistry = { components: Object.create(null), roots: new WeakMap() });
    roots.forEach((root) => {
      const registry = window.__gowdkStores;
      const names = storeNamesFor(root);
      // applyingStoreUpdate guards the write-back path while this island is being
      // re-rendered in response to an external store change, so mirroring another
      // island's update does not echo straight back into the registry.
      let applyingStoreUpdate = false;
      // publishingStores guards this island's own subscription while it writes a
      // store, so its own write-back does not re-invoke mount on itself (other
      // islands subscribed to the same store are still notified).
      let publishingStores = false;
      const processResult = (result) => {
        const normalized = normalizeResult(result);
        if (normalized.stores && registry && !applyingStoreUpdate) {
          publishingStores = true;
          try {
            names.forEach((name) => {
              if (Object.prototype.hasOwnProperty.call(normalized.stores, name)) {
                registry.set(name, normalized.stores[name]);
              }
            });
          } finally {
            publishingStores = false;
          }
        }
        applyPatches(root, normalized.patches);
      };

      processResult(callExport(exports, mountExport, bootstrap(root)));
      root.querySelectorAll("*").forEach((node) => {
        if (!ownsNode(root, node)) return;
        Array.from(node.attributes).forEach((attr) => {
          if (!attr.name.startsWith("data-gowdk-binding-on-")) return;
          const event = attr.name.slice("data-gowdk-binding-on-".length);
          node.addEventListener(event, (domEvent) => {
            const span = traceStart("wasm island " + event);
            try {
              processResult(callExport(exports, handleExport, {
                abiVersion,
                component,
                event,
                binding: attr.value,
                traceparent: currentTraceparent(),
                detail: { value: domEvent && domEvent.target ? domEvent.target.value : undefined },
                state: mergedState(root),
                stores: names
              }));
              traceEnd(span, "ok");
            } catch (error) {
              traceEnd(span, "error", error && error.message || String(error || "wasm island event failed"));
              throw error;
            }
          });
        });
      });
      const unsubscribes = [];
      if (registry && names.length > 0) {
        names.forEach((name) => {
          unsubscribes.push(registry.subscribe(name, () => {
            // Skip our own write-back; only re-render for external changes.
            if (publishingStores) return;
            applyingStoreUpdate = true;
            try {
              processResult(callExport(exports, mountExport, bootstrap(root)));
            } finally {
              applyingStoreUpdate = false;
            }
          }));
        });
      }
      // cleanup runs once on page unload (pagehide) and on SPA navigation teardown
      // (__gowdkDestroyIslands looks the root up in the shared island registry). It
      // unsubscribes the store listeners so a detached root no longer re-renders on
      // later store changes, then runs the destroy export.
      let destroyed = false;
      const cleanup = () => {
        if (destroyed) return;
        destroyed = true;
        unsubscribes.forEach((unsubscribe) => {
          if (typeof unsubscribe === "function") unsubscribe();
        });
        processResult(callExport(exports, destroyExport, { abiVersion, component, state: mergedState(root), stores: names }));
      };
      islandRegistry.roots.set(root, cleanup);
      window.addEventListener("pagehide", cleanup, { once: true });
    });
  }).catch((error) => {
    if (typeof console !== "undefined") console.error("GOWDK WASM island failed to start", component, error);
  });
})();
