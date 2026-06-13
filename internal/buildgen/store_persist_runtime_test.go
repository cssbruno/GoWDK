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

// 9. Adopting persistence across SPA navigation: a store first declared WITHOUT
// persistence must pick up a later route's persist config (and restore the saved
// value) instead of silently staying unpersisted, so persistence does not depend
// on which route loaded first.
localStorage.setItem("gowdk:store:wishlist", JSON.stringify({ v: "w1", s: { Items: 2 } }));
// First navigation declares wishlist without persistence (no data-gowdk-persist).
global.document.querySelectorAll = () => [{
  getAttribute(name) { return name === "data-gowdk-store" ? "wishlist" : null; },
  get textContent() { return '{"Items":0}'; }
}];
r.hydrate();
assert.equal(r.get("wishlist").Items, 0, "unpersisted first declaration uses the seed, not storage");
// Second navigation declares the same store as persist "local".
let adopted = null;
r.subscribe("wishlist", (next) => { adopted = next; });
global.document.querySelectorAll = () => [{
  getAttribute(name) {
    if (name === "data-gowdk-store") return "wishlist";
    if (name === "data-gowdk-persist") return "local";
    if (name === "data-gowdk-persist-key") return "gowdk:store:wishlist";
    if (name === "data-gowdk-persist-version") return "w1";
    return null;
  },
  get textContent() { return '{"Items":0}'; }
}];
r.hydrate();
assert.equal(r.get("wishlist").Items, 2, "re-hydrate adopts persistence and restores the saved value");
assert.ok(adopted && adopted.Items === 2, "adopting persistence notifies subscribers");
r.set("wishlist", { Items: 7 });
assert.equal(JSON.parse(localStorage.getItem("gowdk:store:wishlist")).s.Items, 7, "after adoption, set() writes through to storage");

// 10. A later route re-declares an existing persisted store with a different
// shape and version. The runtime must re-seed to the new field set and discard
// storage written under the old version, so the current route's islands read the
// fields they declared instead of the prior route's (build-time
// page_store_persist_key_conflict warns on the divergence).
localStorage.setItem("gowdk:store:profile", JSON.stringify({ v: "a1", s: { Name: "ada", Theme: "dark" } }));
global.document.querySelectorAll = () => [{
  getAttribute(name) {
    if (name === "data-gowdk-store") return "profile";
    if (name === "data-gowdk-persist") return "local";
    if (name === "data-gowdk-persist-key") return "gowdk:store:profile";
    if (name === "data-gowdk-persist-version") return "a1";
    return null;
  },
  get textContent() { return '{"Name":"","Theme":"light"}'; }
}];
r.hydrate();
assert.equal(r.get("profile").Theme, "dark", "precondition: restored saved value under version a1");
let reseeded = null;
r.subscribe("profile", (next) => { reseeded = next; });
global.document.querySelectorAll = () => [{
  getAttribute(name) {
    if (name === "data-gowdk-store") return "profile";
    if (name === "data-gowdk-persist") return "local";
    if (name === "data-gowdk-persist-key") return "gowdk:store:profile";
    if (name === "data-gowdk-persist-version") return "a2";
    return null;
  },
  get textContent() { return '{"Name":"","Density":"cozy"}'; }
}];
r.hydrate();
assert.equal(r.get("profile").Density, "cozy", "shape/version change re-seeds to the new field set");
assert.ok(!("Theme" in r.get("profile")), "the dropped field is gone after a re-seed");
assert.equal(JSON.parse(localStorage.getItem("gowdk:store:profile") || "{}").v, "a1", "stale storage stays under the old version until the next write");
assert.ok(reseeded && reseeded.Density === "cozy", "a shape/version change notifies subscribers");
r.set("profile", { Name: "ada", Density: "compact" });
assert.equal(JSON.parse(localStorage.getItem("gowdk:store:profile")).v, "a2", "after a re-seed, writes persist under the new version");

// 11. A storage event from a different storage area must be ignored so local and
// session stores that share a key cannot cross-contaminate.
global.document.querySelectorAll = () => [makeNode()];
r = boot();
r.set("cart", { Count: 1, Open: false });
storageListeners[0]({ key: "gowdk:store:cart", newValue: JSON.stringify({ v: "v1", s: { Count: 99, Open: true } }), storageArea: sessionStorage });
assert.equal(r.get("cart").Count, 1, "a storage event from a different area (session) is ignored by a local store");
storageListeners[0]({ key: "gowdk:store:cart", newValue: JSON.stringify({ v: "v1", s: { Count: 99, Open: true } }), storageArea: localStorage });
assert.equal(r.get("cart").Count, 99, "a storage event from the matching area is applied");

// 12. Adopting persistence on a FRESH storage slot must re-seed to the later
// route's declared seed, not silently keep the earlier unpersisted route's seed.
// Two routes can share a top-level field name yet declare a different default;
// with nothing to restore, the current route's islands must read THEIR seed.
global.document.querySelectorAll = () => [{
  getAttribute(name) { return name === "data-gowdk-store" ? "banner" : null; },
  get textContent() { return '{"Dismissed":true}'; }
}];
r.hydrate();
assert.equal(r.get("banner").Dismissed, true, "precondition: unpersisted route uses its own seed");
global.document.querySelectorAll = () => [{
  getAttribute(name) {
    if (name === "data-gowdk-store") return "banner";
    if (name === "data-gowdk-persist") return "local";
    if (name === "data-gowdk-persist-key") return "gowdk:store:banner";
    if (name === "data-gowdk-persist-version") return "b1";
    return null;
  },
  get textContent() { return '{"Dismissed":false}'; }
}];
r.hydrate();
assert.equal(r.get("banner").Dismissed, false, "adopting persistence on empty storage re-seeds to the persisted route's declared seed");

// 13. Session-scoped stores must NOT mirror cross-tab storage events:
// sessionStorage is partitioned per top-level tab, so a "storage" event from
// another tab cannot belong to this tab's session store.
seedJSON = '{"Count":0,"Open":false}';
scope = "session";
version = "v1";
global.document.querySelectorAll = () => [makeNode()];
r = boot();
r.set("cart", { Count: 3, Open: false });
storageListeners[0]({ key: "gowdk:store:cart", newValue: JSON.stringify({ v: "v1", s: { Count: 77, Open: true } }), storageArea: sessionStorage });
assert.equal(r.get("cart").Count, 3, "a session-scoped store ignores cross-tab storage events");

// 14. Re-executing the runtime against the SAME window (as SPA navigation does
// when stores.js is re-activated) must NOT register a second storage listener;
// a duplicate would notify islands twice per cross-tab write.
scope = "local";
global.document.querySelectorAll = () => [makeNode()];
r = boot();
assert.equal(storageListeners.length, 1, "one storage listener after boot");
new Function(runtimeSrc)();
assert.equal(storageListeners.length, 1, "re-executing the runtime does not add a second storage listener");

console.log("OK");
`
}
