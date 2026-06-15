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

// TestClearStatementResetsStoreInBrowser builds a component whose client handler
// runs the bounded `clear <store>` statement, drives a real browser to mutate the
// persisted store and then clear it, and asserts the store resets to its seed.
// This exercises the runtime lowering of `clear cart` to __gowdkStores.clear,
// not the registry method directly. Skips unless node + chromium + playwright are
// available (same gating as the other browser harness tests).
func TestClearStatementResetsStoreInBrowser(t *testing.T) {
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
	component.Blocks.ClientBody = `use cart

fn Add() {
  Count++
}

fn Reset() {
  clear cart
}`
	component.Blocks.ViewBody = `<div>` +
		`<button data-role="add" g:on:click={Add()}>{Count}</button>` +
		`<button data-role="reset" g:on:click={Reset()}>reset</button>` +
		`</div>`
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

	script := filepath.Join(t.TempDir(), "gowdk-clear-browser-test.cjs")
	if err := os.WriteFile(script, []byte(clearStatementBrowserHarness()), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, script, server.URL, chromium)
	command.Dir = mustWorkingDir(t)
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("clear statement browser test timed out:\n%s", output)
	}
	if err != nil {
		t.Fatalf("clear statement browser test failed: %v\n%s", err, output)
	}
}

func clearStatementBrowserHarness() string {
	return `
"use strict";

const assert = require("node:assert/strict");
const nodeModule = require("node:module");

const baseURL = process.argv[2];
const executablePath = process.argv[3];
const { chromium } = nodeModule.createRequire(process.cwd() + "/gowdk-test.js")("playwright");

async function waitForAddText(page, expected) {
  await page.waitForFunction((expected) => {
    return document.querySelector('gowdk-island button[data-role="add"]')?.textContent === expected;
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

  await page.goto(baseURL + "/counter/", { waitUntil: "networkidle" });
  await waitForAddText(page, "1");

  // Mutate to 3; clicks sync into the persisted cart store.
  await page.evaluate(() => document.querySelector('gowdk-island button[data-role="add"]').click());
  await page.evaluate(() => document.querySelector('gowdk-island button[data-role="add"]').click());
  await waitForAddText(page, "3");
  assert.equal(await page.evaluate(() => window.__gowdkStores.get("cart").Count), 3, "store mutated to 3");

  // The bounded clear-store statement (run by the Reset handler) must reset the
  // store to its build-time seed and update the island, with no JS escape hatch.
  await page.evaluate(() => document.querySelector('gowdk-island button[data-role="reset"]').click());
  await waitForAddText(page, "1");
  assert.equal(await page.evaluate(() => window.__gowdkStores.get("cart").Count), 1, "clear statement reset the store to seed");

  assert.deepEqual(consoleErrors, []);
  await browser.close();
})().catch(async (error) => {
  console.error(error && error.stack || error);
  process.exit(1);
});
`
}
