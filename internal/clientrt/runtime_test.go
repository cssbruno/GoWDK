package clientrt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceEmitsPartialUpdateRuntime(t *testing.T) {
	source := string(Source())
	for _, expected := range []string{
		`form[data-gowdk-target]`,
		`gowdk:before-request`,
		`gowdk:after-swap`,
		`gowdk:request-error`,
		`validateFormBeforePartialSubmit(form, target)`,
		`form.checkValidity`,
		`form.reportValidity`,
		`gowdk:validation-blocked`,
		`partialRequestError(response)`,
		`X-GOWDK-Reload`,
		`reloadPage()`,
		`status: error && error.status || 0`,
		`body: error && error.body || ''`,
		`response: error && error.response || null`,
		`X-GOWDK-Fragment-Swap`,
		`X-GOWDK-Target`,
		`X-GOWDK-Swap`,
		`target.outerHTML = html`,
		`target.innerHTML = html`,
		`window.__gowdkDestroyIslands(target, swap === 'outerHTML')`,
		`typeof window !== 'undefined' && window.__gowdkMountIslands`,
		`aria-busy`,
		`focusTarget(document.activeElement)`,
		`restoreFocus(focused)`,
		`document.getElementById(target.id)`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected runtime source to contain %q:\n%s", expected, source)
		}
	}
}

func TestSourceEmitsSPANavigationRuntime(t *testing.T) {
	source := string(Source())
	for _, expected := range []string{
		`document.addEventListener('click', navigateLink)`,
		`document.addEventListener('mouseover', prefetchLink)`,
		`document.addEventListener('mouseout', cancelHoverPrefetch)`,
		`document.addEventListener('focusin', prefetchLink)`,
		`document.addEventListener('touchstart', prefetchLink, { passive: true })`,
		// Hover prefetch waits a beat (cancelable) and the cache stays bounded.
		`hoverPrefetchTimer = setTimeout(`,
		`function cancelHoverPrefetch()`,
		`function rememberPrefetched(url)`,
		`while (prefetchOrder.length > prefetchLimit)`,
		`window.addEventListener('popstate'`,
		`event.target.closest && event.target.closest('a[href]')`,
		`X-GOWDK-Navigate`,
		`X-GOWDK-Prefetch`,
		`prefetchedDocuments[url]`,
		`credentials: 'same-origin'`,
		`data-gowdk-navigating`,
		`gowdk:navigate-start`,
		`gowdk:navigate-end`,
		`new DOMParser().parseFromString(fetched.html, 'text/html')`,
		`document.head.innerHTML = next.head ? next.head.innerHTML : ''`,
		`document.body.innerHTML = next.body.innerHTML`,
		`window.history.pushState({}, document.title, url)`,
		`gowdk:before-navigate`,
		`gowdk:after-navigate`,
		`gowdk:navigate-error`,
		`window.location.href = url.href`,
		`activateNewScripts(previousScripts)`,
		// The store runtime runs (and hydrates) before island bundles, which
		// auto-mount on execution and read the store registry during mount.
		`script.hasAttribute('data-gowdk-store-runtime')`,
		`return runScripts(storeScripts).then(`,
		`window.__gowdkStores.hydrate()`,
		`return runScripts(otherScripts)`,
		// Ordered (non-async) execution so a dependency cannot lose the race.
		`active.async = false`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected runtime source to contain %q:\n%s", expected, source)
		}
	}
}

func TestSourceEmitsRealtimePatchRuntime(t *testing.T) {
	source := string(Source())
	for _, expected := range []string{
		`new window.EventSource(realtimeEventsURL())`,
		`realtimeEventsPath + (realtimeEventsPath.indexOf('?') >= 0 ? '&' : '?') + 'path=' + encodeURIComponent(path)`,
		`var realtimeQueryRefreshPath = '/_gowdk/realtime/query-refresh'`,
		`traceFetch(realtimeQueryRefreshURL(queries)`,
		`url.searchParams.append('query', query)`,
		`addEventListener('gowdk-presentation', handleRealtimeEvent)`,
		`data-gowdk-subscribe`,
		`data-gowdk-subscribe-type`,
		`normalizeRealtimePatches(envelope.value || envelope.Value)`,
		`assertRealtimePayloadVersion(value)`,
		`patch.op !== 'replaceHTML'`,
		`region.innerHTML = patch.html`,
		`region.outerHTML = patch.html`,
		`gowdk:realtime-patch`,
		`gowdk:realtime-error`,
		`closeRealtime()`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected runtime source to contain %q:\n%s", expected, source)
		}
	}
}

func TestSourceTraceBridgeKeepsDisabledTraceparentEmpty(t *testing.T) {
	source := string(Source())
	for _, expected := range []string{
		`if (!traceEnabled()) {
            return '';
          }`,
		`return '00-' + traceHex(16) + '-' + traceHex(8) + '-01';`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected runtime source to contain %q:\n%s", expected, source)
		}
	}
}

func TestSourceTraceFetchNormalizesRequestInputs(t *testing.T) {
	source := string(Source())
	for _, expected := range []string{
		`function traceInputURL(url)`,
		`typeof Request !== 'undefined' && url instanceof Request`,
		`return url.url || '';`,
		`return new URL(traceInputURL(url), window.location.href).origin === window.location.origin;`,
		`function traceInputHeaders(url, options)`,
		`url && typeof url === 'object' && url.headers`,
		`var headers = new Headers(traceInputHeaders(url, traced));`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected runtime source to contain %q:\n%s", expected, source)
		}
	}
}

func TestFilename(t *testing.T) {
	if Filename != "gowdk.js" {
		t.Fatalf("unexpected runtime filename %q", Filename)
	}
}

func TestEmbeddedRuntimeSourceFilesParseWithNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	entries, err := runtimeAssets.ReadDir("assets")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected embedded JavaScript runtime source files")
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			source, err := runtimeAssets.ReadFile("assets/" + entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), entry.Name())
			if err := os.WriteFile(file, source, 0o600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command(node, "--check", file).CombinedOutput(); err != nil {
				t.Fatalf("embedded JavaScript source must parse: %v\n%s", err, output)
			}
		})
	}
}

func TestRuntimeTemplatesReplacePlaceholders(t *testing.T) {
	rendered := []string{
		IslandJSSource(IslandJSOptions{
			Component:       "Counter",
			MountFunction:   "mountCounterIsland",
			DestroyFunction: "destroyCounterIsland",
		}),
		ClientGoBlockWASMLoaderSource(ClientGoBlockWASMLoaderOptions{
			PageID:       "home",
			LoaderPath:   "/assets/gowdk/islands/pages/Home.wasm.js",
			WASMPath:     "/assets/gowdk/islands/pages/Home.wasm",
			WASMExecPath: "/assets/gowdk/islands/wasm_exec.js",
			MountExport:  "GOWDKMountHome",
		}),
		WASMIslandLoaderSource(WASMIslandLoaderOptions{
			Component:    "Counter",
			ABIVersion:   "gowdk-wasm-island-v1",
			WASMPath:     "/assets/gowdk/islands/Counter.wasm",
			WASMExecPath: "/assets/gowdk/islands/wasm_exec.js",
		}),
	}
	for _, source := range rendered {
		if strings.Contains(source, "__GOWDK_") {
			t.Fatalf("rendered runtime template still contains a placeholder:\n%s", source)
		}
	}
	if !strings.Contains(rendered[0], `const component = "Counter";`) ||
		!strings.Contains(rendered[0], `window.__gowdkRegisterJSIsland`) ||
		!strings.Contains(rendered[0], `registry.components[component] = true`) {
		t.Fatalf("rendered island stub did not include component registration:\n%s", rendered[0])
	}
	if !strings.Contains(rendered[1], `const mountExport = "GOWDKMountHome";`) {
		t.Fatalf("rendered page WASM loader did not include mount export:\n%s", rendered[1])
	}
	if !strings.Contains(rendered[2], `const abiVersion = "gowdk-wasm-island-v1";`) {
		t.Fatalf("rendered WASM island loader did not include ABI version:\n%s", rendered[2])
	}
}

func TestIslandRuntimeClonesTemplateDOM(t *testing.T) {
	source := IslandRuntimeSource()
	if strings.Contains(source, "__GOWDK_EXPRESSION_SPEC__") {
		t.Fatalf("rendered island runtime still contains expression spec placeholder:\n%s", source)
	}
	for _, forbidden := range []string{
		`data-gowdk-for-template`,
		`holder.innerHTML`,
		`firstTemplateElement`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("island runtime must not reparse list template text via %q:\n%s", forbidden, source)
		}
	}
	for _, expected := range []string{
		`const expressionSpec = Object.freeze({`,
		`"builtins":[{"name":"len","args":1}`,
		`{"name":"fixed","args":2}`,
		`{"name":"formatTime","args":2}`,
		`const builtinSpecByName = Object.freeze(Object.fromEntries`,
		`expressionOperators.equality.has`,
		`function cloneListTemplate(marker, state, scope, helpers)`,
		`const source = marker.content && marker.content.firstElementChild;`,
		`if (node.content) Array.from(node.content.childNodes).forEach`,
		`interpolateTemplateNode(fresh, state, scope, helpers);`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected island runtime to contain %q:\n%s", expected, source)
		}
	}
}

func TestWASMIslandLoaderFindsBindingsWithoutDynamicSelectors(t *testing.T) {
	source := WASMIslandLoaderSource(WASMIslandLoaderOptions{
		Component:    "Counter",
		ABIVersion:   "gowdk-wasm-island-v1",
		WASMPath:     "/assets/gowdk/islands/Counter.wasm",
		WASMExecPath: "/assets/gowdk/islands/wasm_exec.js",
	})
	for _, forbidden := range []string{
		`CSS.escape`,
		`querySelector("[data-gowdk-binding-text=\"`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("WASM island loader must not build binding selectors from patch ids via %q:\n%s", forbidden, source)
		}
	}
	for _, expected := range []string{
		`const bindingTargetAttributes = ["data-gowdk-binding-text", "data-gowdk-binding-if", "data-gowdk-binding-list", "data-gowdk-binding-value", "data-gowdk-binding-checked"];`,
		`const nodes = matchingNodes(root, "[" + attr + "]");`,
		`if (node.getAttribute(attr) === expected) return node;`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("expected WASM island loader to find binding targets without dynamic selector escaping:\n%s", source)
		}
	}
}
