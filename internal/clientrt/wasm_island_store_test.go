package clientrt

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestWASMIslandStoreParticipationUnderNode runs the generated WASM island loader
// in node against a mocked DOM, a mocked WebAssembly instance whose exports read
// the mount payload and return an extended { patches, stores } result, and a
// mock store registry. It asserts the three halves of store participation:
// READ (store values reach the mount payload state), WRITE (returned store state
// is written back to the registry), and SYNC (an external store change re-invokes
// the island without echoing back into the registry).
func TestWASMIslandStoreParticipationUnderNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	dir := t.TempDir()
	loaderPath := filepath.Join(dir, "wasm_island_loader.js")
	source := WASMIslandLoaderSource(WASMIslandLoaderOptions{
		Component:    "Counter",
		ABIVersion:   "gowdk-wasm-island-v1",
		WASMPath:     "/counter.wasm",
		WASMExecPath: "/wasm_exec.js",
	})
	if err := os.WriteFile(loaderPath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	harnessPath := filepath.Join(dir, "harness.cjs")
	if err := os.WriteFile(harnessPath, []byte(wasmIslandStoreHarness()), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(node, harnessPath, loaderPath).CombinedOutput()
	if err != nil {
		t.Fatalf("WASM island store participation test failed: %v\n%s", err, output)
	}
}

func wasmIslandStoreHarness() string {
	return `
"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");

const loaderSrc = fs.readFileSync(process.argv[2], "utf8");

const mountPayloads = [];
const handlePayloads = [];

// A WASM island whose exports echo the mount/handle state into the cart store
// (write-back) after reading it from the payload (read).
const exportsObj = {
  GOWDKMountCounter(payload) {
    mountPayloads.push(payload);
    return { patches: [], stores: { cart: { Count: payload.state.Count, Open: payload.state.Open } } };
  },
  GOWDKHandleCounter(payload) {
    handlePayloads.push(payload);
    return { patches: [], stores: { cart: { Count: payload.state.Count + 1, Open: payload.state.Open } } };
  },
  GOWDKDestroyCounter() { return []; }
};

const registry = {
  store: { cart: { Count: 5, Open: false } },
  listeners: { cart: [] },
  sets: [],
  get(name) { return Object.assign({}, this.store[name] || {}); },
  set(name, value) { this.store[name] = Object.assign({}, this.store[name] || {}, value || {}); this.sets.push(name); this.notify(name); },
  subscribe(name, fn) { (this.listeners[name] = this.listeners[name] || []).push(fn); return () => {}; },
  notify(name) { (this.listeners[name] || []).slice().forEach((fn) => fn(this.get(name))); }
};

const root = {
  attrs: {
    "data-gowdk-component-id": "Counter",
    "data-gowdk-runtime": "wasm",
    "data-gowdk-client": JSON.stringify({ stores: ["cart"] }),
    "data-gowdk-state": JSON.stringify({ Count: 0, Open: false }),
    "data-gowdk-props": "{}"
  },
  getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; },
  matches() { return false; },
  closest() { return root; },
  querySelectorAll() { return []; },
  addEventListener() {},
  dispatchEvent() {}
};

global.console = { error: (...a) => { throw new Error("loader logged error: " + a.join(" ")); }, warn() {}, log: console.log };
global.window = { __gowdkStores: registry, addEventListener() {} };
global.document = {
  querySelectorAll() { return [root]; },
  head: { appendChild() {} },
  createElement() { return {}; }
};
global.fetch = async () => ({ arrayBuffer: async () => new ArrayBuffer(0) });
global.WebAssembly = {
  instantiate: async () => ({ instance: { exports: exportsObj } })
  // No instantiateStreaming: the loader falls back to fetch + instantiate.
};

new Function(loaderSrc)();

(async () => {
  // Let the loader's async instantiate().then(...) chain settle.
  for (let i = 0; i < 10; i++) await new Promise((r) => setTimeout(r, 0));

  // READ: the mount payload carries the store's current value (Count 5), not the
  // island's build-time seed (Count 0), and lists the used store.
  assert.equal(mountPayloads.length, 1, "island mounted once");
  assert.equal(mountPayloads[0].state.Count, 5, "store value reached the mount payload (read)");
  assert.deepEqual(mountPayloads[0].stores, ["cart"], "mount payload lists the used store");

  // WRITE: the mount result's stores map was written back to the registry.
  assert.ok(registry.sets.includes("cart"), "returned store state was written back (write)");

  // SYNC: an external store change re-invokes the island, and the re-render does
  // not echo back into the registry (guarded write-back).
  const setsBefore = registry.sets.length;
  registry.store.cart = { Count: 9, Open: true };
  registry.notify("cart");
  for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));
  assert.equal(mountPayloads.length, 2, "external store change re-invoked mount (sync)");
  assert.equal(mountPayloads[1].state.Count, 9, "re-render saw the updated store value");
  assert.equal(registry.sets.length, setsBefore, "store-driven re-render does not echo back into the registry");

  console.log("OK");
})().catch((error) => { console.error(error && error.stack || error); process.exit(1); });
`
}
