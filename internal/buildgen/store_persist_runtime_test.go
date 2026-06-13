package buildgen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestStoreRuntimePersistenceUnderNode executes the generated store runtime in
// node against mocked window/localStorage and asserts the real behavior of the
// persistence path: hydrate-on-load, field projection, version invalidation,
// clear, quota-failure tolerance, and cross-tab storage-event sync. It needs
// only node (no chromium/playwright), so it runs in more environments than the
// full browser harness.
func TestStoreRuntimePersistenceUnderNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "stores.js")
	if err := os.WriteFile(runtimePath, []byte(storeRuntimeSource()), 0o600); err != nil {
		t.Fatal(err)
	}
	harnessPath := filepath.Join(dir, "harness.cjs")
	if err := os.WriteFile(harnessPath, []byte(storeRuntimeNodeHarness()), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(node, harnessPath, runtimePath).CombinedOutput()
	if err != nil {
		t.Fatalf("store runtime behavior test failed: %v\n%s", err, output)
	}
}

func storeRuntimeNodeHarness() string {
	return `
"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");

const runtimeSrc = fs.readFileSync(process.argv[2], "utf8");

function makeStorage() {
  const map = new Map();
  return {
    failNext: false,
    getItem(k) { return map.has(k) ? map.get(k) : null; },
    setItem(k, v) { if (this.failNext) { this.failNext = false; throw new Error("QuotaExceededError"); } map.set(k, String(v)); },
    removeItem(k) { map.delete(k); }
  };
}

const localStorage = makeStorage();
const sessionStorage = makeStorage();
let storageListeners = [];
let warnings = [];

// Harness-controlled seed and persist attributes, read fresh on every boot.
let seedJSON = '{"Count":0,"Open":false}';
let scope = "local";
let version = "v1";

function makeNode() {
  return {
    getAttribute(name) {
      if (name === "data-gowdk-store") return "cart";
      if (name === "data-gowdk-persist") return scope;
      if (name === "data-gowdk-persist-key") return "gowdk:store:cart";
      if (name === "data-gowdk-persist-version") return version;
      return null;
    },
    get textContent() { return seedJSON; }
  };
}

global.console = { warn: (m) => warnings.push(m), error: (m) => warnings.push(m), log: console.log };
global.document = { querySelectorAll: () => [makeNode()] };

function boot() {
  // Fresh registry each boot simulates a page load; storage persists across boots.
  global.window = {
    localStorage,
    sessionStorage,
    addEventListener(type, fn) { if (type === "storage") storageListeners.push(fn); }
  };
  storageListeners = [];
  new Function(runtimeSrc)();
  return global.window.__gowdkStores;
}

// 1. Empty storage -> seed.
let r = boot();
assert.deepEqual(r.get("cart"), { Count: 0, Open: false }, "fresh load should equal seed");

// 2. set persists only declared fields (Extra is dropped).
r.set("cart", { Count: 5, Open: true, Extra: "x" });
const stored = JSON.parse(localStorage.getItem("gowdk:store:cart"));
assert.equal(stored.v, "v1", "stored version");
assert.deepEqual(stored.s, { Count: 5, Open: true }, "only seed fields persist, not Extra");

// 3. Reload hydrates from storage.
r = boot();
assert.equal(r.get("cart").Count, 5, "reload should restore persisted Count");

// 4. Version change discards stale storage.
version = "v2";
r = boot();
assert.equal(r.get("cart").Count, 0, "shape/version change should discard stale data");

// 5. clear() removes storage and resets to seed, notifying subscribers.
version = "v1";
r = boot();
r.set("cart", { Count: 9, Open: false });
r = boot();
assert.equal(r.get("cart").Count, 9, "precondition: persisted 9");
let notified = null;
r.subscribe("cart", (next) => { notified = next; });
r.clear("cart");
assert.equal(localStorage.getItem("gowdk:store:cart"), null, "clear removes the storage key");
assert.equal(r.get("cart").Count, 0, "clear resets to seed");
assert.equal(notified.Count, 0, "clear notifies subscribers");

// 6. Quota failure on write must not throw and must warn once.
r = boot();
warnings = [];
localStorage.failNext = true;
assert.doesNotThrow(() => r.set("cart", { Count: 3 }), "write failure must not throw");
assert.equal(r.get("cart").Count, 3, "in-memory state still updates on write failure");
assert.ok(warnings.some((m) => m.includes("GOWDK")), "a one-time GOWDK warning is logged");

// 7. Cross-tab storage event mirrors the value and notifies, without writing back.
r = boot();
let crossTab = null;
r.subscribe("cart", (next) => { crossTab = next; });
assert.ok(storageListeners.length > 0, "a storage listener is registered");
storageListeners[0]({ key: "gowdk:store:cart", newValue: JSON.stringify({ v: "v1", s: { Count: 42, Open: false } }) });
assert.equal(r.get("cart").Count, 42, "cross-tab write is mirrored");
assert.equal(crossTab.Count, 42, "cross-tab write notifies subscribers");
// A cleared key in another tab resets to seed.
storageListeners[0]({ key: "gowdk:store:cart", newValue: null });
assert.equal(r.get("cart").Count, 0, "cross-tab clear resets to seed");

// 8. SPA-navigation re-hydration: a store first declared on a later route is
// picked up by hydrate() without re-running the runtime, and existing stores
// are left untouched (init bails on stores already in the registry).
global.document.querySelectorAll = () => [
  makeNode(),
  {
    getAttribute(name) {
      if (name === "data-gowdk-store") return "prefs";
      if (name === "data-gowdk-persist") return "local";
      if (name === "data-gowdk-persist-key") return "gowdk:store:prefs";
      if (name === "data-gowdk-persist-version") return "p1";
      return null;
    },
    get textContent() { return '{"Theme":"dark"}'; }
  }
];
r.hydrate();
assert.equal(r.get("prefs").Theme, "dark", "hydrate() picks up a store first seen on a later route");
assert.equal(r.get("cart").Count, 0, "re-hydrate leaves an existing store untouched");

console.log("OK");
`
}
