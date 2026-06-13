package buildgen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

// TestPersistedStoreSurvivesReloadInBrowser builds a page whose store opts into
// persist "local", drives a real browser to mutate it, reloads, and asserts the
// value is restored from localStorage. Skips unless node + chromium + playwright
// are available (same gating as the other browser harness tests).
func TestPersistedStoreSurvivesReloadInBrowser(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	chromium, err := lookupChromium()
	if err != nil {
		t.Skip(err)
	}
	requireNodePlaywright(t, node)

	outputDir := t.TempDir()
	component := counterComponent()
	component.Blocks.Client = true
	component.Blocks.ClientBody = `use cart`
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "counter",
			Route:   "/counter",
			Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			Stores: []gwdkir.Store{{
				Name:    "cart",
				Type:    gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
				Init:    gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
				Persist: "local",
			}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<main><Counter /></main>`},
		}},
		Components: []gwdkir.Component{component},
	}
	if _, err := Build(gowdk.Config{}, app, outputDir); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.FileServer(http.Dir(outputDir)))
	defer server.Close()

	script := filepath.Join(t.TempDir(), "gowdk-persist-browser-test.cjs")
	if err := os.WriteFile(script, []byte(persistedStoreBrowserHarness()), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, script, server.URL, chromium)
	command.Dir = mustWorkingDir(t)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("persisted store browser test timed out:\n%s", output)
	}
	if err != nil {
		t.Fatalf("persisted store browser test failed: %v\n%s", err, output)
	}
}

func persistedStoreBrowserHarness() string {
	return `
"use strict";

const assert = require("node:assert/strict");
const nodeModule = require("node:module");

const baseURL = process.argv[2];
const executablePath = process.argv[3];
const { chromium } = nodeModule.createRequire(process.cwd() + "/gowdk-test.js")("playwright");

async function waitForButtonText(page, expected) {
  await page.waitForFunction((expected) => {
    return document.querySelector("gowdk-island button")?.textContent === expected;
  }, expected);
}

(async () => {
  const browser = await chromium.launch({ executablePath, headless: true, args: ["--no-sandbox"] });
  const context = await browser.newContext();
  const page = await context.newPage();
  const consoleErrors = [];
  page.on("console", (message) => {
    if (message.type() === "error" && message.text().includes("GOWDK")) consoleErrors.push(message.text());
  });

  // Initial load: seed is Count 1, nothing persisted yet.
  await page.goto(baseURL + "/counter/", { waitUntil: "networkidle" });
  await waitForButtonText(page, "1");

  // Mutate to 3; each click syncs to the persisted cart store.
  await page.evaluate(() => document.querySelector("gowdk-island button").click());
  await page.evaluate(() => document.querySelector("gowdk-island button").click());
  await waitForButtonText(page, "3");

  // localStorage carries the versioned, field-projected value.
  const stored = await page.evaluate(() => window.localStorage.getItem("gowdk:store:cart"));
  const blob = JSON.parse(stored);
  assert.equal(blob.s.Count, 3, "store value persisted to localStorage");
  assert.ok(typeof blob.v === "string" && blob.v.length > 0, "persisted blob carries a version");

  // Reload: the island must hydrate Count 3 from localStorage, not the seed.
  await page.goto(baseURL + "/counter/", { waitUntil: "networkidle" });
  await waitForButtonText(page, "3");
  assert.equal(await page.evaluate(() => window.__gowdkStores.get("cart").Count), 3, "store rehydrated after reload");

  // clear() drops the persisted copy; a fresh reload returns to the seed.
  await page.evaluate(() => window.__gowdkStores.clear("cart"));
  assert.equal(await page.evaluate(() => window.localStorage.getItem("gowdk:store:cart")), null, "clear removed storage");
  await page.goto(baseURL + "/counter/", { waitUntil: "networkidle" });
  await waitForButtonText(page, "1");

  assert.deepEqual(consoleErrors, []);
  await browser.close();
})().catch(async (error) => {
  console.error(error && error.stack || error);
  process.exit(1);
});
`
}
