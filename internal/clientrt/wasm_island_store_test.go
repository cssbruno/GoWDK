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
const { TextEncoder } = require("node:util");

const loaderSrc = fs.readFileSync(process.argv[2], "utf8");

const mountPayloads = [];
const handlePayloads = [];
const memory = { buffer: new ArrayBuffer(65536) };
const encoder = new TextEncoder();
let resultOffset = 1024;

function currentPayload() {
  return JSON.parse(global.window.__gowdkWASMIslandPayload || "{}");
}

function writeResult(value) {
  const bytes = encoder.encode(JSON.stringify(value) + "\0");
  const offset = resultOffset;
  new Uint8Array(memory.buffer).set(bytes, offset);
  resultOffset += bytes.length + 16;
  return offset;
}

// A WASM island whose exports echo the mount/handle state into the cart store
// (write-back) after reading it from the payload (read).
const exportsObj = {
  mem: memory,
  GOWDKMountCounter() {
    const payload = currentPayload();
    mountPayloads.push(payload);
    return writeResult({ patches: [], stores: { cart: { Count: payload.state.Count, Open: payload.state.Open } } });
  },
  GOWDKHandleCounter() {
    const payload = currentPayload();
    handlePayloads.push(payload);
    return writeResult({ patches: [], stores: { cart: { Count: payload.state.Count + 1, Open: payload.state.Open } } });
  },
  GOWDKDestroyCounter() { destroyPayloads.push(currentPayload()); return writeResult([]); }
};
const destroyPayloads = [];

const registry = {
  store: { cart: { Count: 5, Open: false } },
  listeners: { cart: [] },
  sets: [],
  get(name) { return Object.assign({}, this.store[name] || {}); },
  set(name, value) { this.store[name] = Object.assign({}, this.store[name] || {}, value || {}); this.sets.push(name); this.notify(name); },
  subscribe(name, fn) { (this.listeners[name] = this.listeners[name] || []).push(fn); return () => { this.listeners[name] = (this.listeners[name] || []).filter((item) => item !== fn); }; },
  notify(name) { (this.listeners[name] || []).slice().forEach((fn) => fn(this.get(name))); }
};

const eventHandlers = {};
const clickNode = {
  attributes: [{ name: "data-gowdk-binding-on-click", value: "b1" }],
  getAttribute() { return null; },
  closest() { return root; },
  addEventListener(event, fn) { eventHandlers[event] = fn; }
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
  querySelectorAll(selector) { return selector === "*" ? [clickNode] : []; },
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
  assert.equal(Object.prototype.hasOwnProperty.call(global.window, "__gowdkWASMIslandPayload"), false, "loader clears the payload global after mount");

  // WRITE: the mount result's stores map was written back to the registry.
  assert.ok(registry.sets.includes("cart"), "returned store state was written back (write)");

  // SELF-TRIGGER GUARD: a handler that writes its own store must not re-invoke
  // mount on itself via its own subscription, even though the write notifies.
  const mountsBeforeClick = mountPayloads.length;
  assert.equal(typeof eventHandlers.click, "function", "the loader wired the click handler");
  eventHandlers.click({ target: { value: undefined } });
  for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));
  assert.equal(handlePayloads.length, 1, "the handle export ran for the click");
  assert.equal(registry.store.cart.Count, 6, "the handler wrote its store value back");
  assert.equal(mountPayloads.length, mountsBeforeClick, "the island's own store write does not re-invoke its mount");
  assert.equal(Object.prototype.hasOwnProperty.call(global.window, "__gowdkWASMIslandPayload"), false, "loader clears the payload global after handle");

  // SYNC: an external store change re-invokes the island, and the re-render does
  // not echo back into the registry (guarded write-back).
  const setsBefore = registry.sets.length;
  registry.store.cart = { Count: 9, Open: true };
  registry.notify("cart");
  for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));
  assert.equal(mountPayloads.length, 2, "external store change re-invoked mount (sync)");
  assert.equal(mountPayloads[1].state.Count, 9, "re-render saw the updated store value");
  assert.equal(registry.sets.length, setsBefore, "store-driven re-render does not echo back into the registry");

  // TEARDOWN: the loader registers a per-root cleanup in the shared island
  // registry (used by SPA navigation teardown). Running it unsubscribes the store
  // listener so later store changes no longer re-invoke the detached island.
  const cleanup = global.window.__gowdkIslandRegistry.roots.get(root);
  assert.equal(typeof cleanup, "function", "loader registered a cleanup in the island registry");
  cleanup();
  assert.equal(destroyPayloads.length, 1, "cleanup runs the destroy export");
  assert.equal(Object.prototype.hasOwnProperty.call(global.window, "__gowdkWASMIslandPayload"), false, "loader clears the payload global after destroy");
  const mountsAfterCleanup = mountPayloads.length;
  registry.store.cart = { Count: 11, Open: false };
  registry.notify("cart");
  for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));
  assert.equal(mountPayloads.length, mountsAfterCleanup, "after teardown a store change no longer re-invokes the island");

  console.log("OK");
})().catch((error) => { console.error(error && error.stack || error); process.exit(1); });
`
}
