(() => {
  const pageID = "__GOWDK_PAGE_ID__";
  const loaderPath = "__GOWDK_LOADER_PATH__";
  const wasmPath = "__GOWDK_WASM_PATH__";
  const wasmExecPath = "__GOWDK_WASM_EXEC_PATH__";
  const mountExport = "__GOWDK_MOUNT_EXPORT__";
  const registry = window.__gowdkClientGoBlockRegistry || (window.__gowdkClientGoBlockRegistry = { entries: Object.create(null) });
  window.__gowdkMountClientGoBlocks = () => {
    Object.keys(registry.entries).forEach((key) => registry.entries[key].mount());
  };
  if (typeof WebAssembly === "undefined") return;

  function tracedFetch(url, options, name) {
    if (window.__gowdkTrace && window.__gowdkTrace.fetch) {
      return window.__gowdkTrace.fetch(url, options || {}, { name: name || "client go wasm fetch", lane: "island" });
    }
    return fetch(url, options);
  }

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
        return await WebAssembly.instantiateStreaming(tracedFetch(wasmPath, {}, "client go wasm module"), go.importObject);
      } catch (_error) {}
    }
    const response = await tracedFetch(wasmPath, {}, "client go wasm module");
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
