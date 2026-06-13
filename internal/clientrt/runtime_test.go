package clientrt

import (
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
		`window.addEventListener('popstate'`,
		`event.target.closest && event.target.closest('a[href]')`,
		`X-GOWDK-Navigate`,
		`new DOMParser().parseFromString(html, 'text/html')`,
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

func TestFilename(t *testing.T) {
	if Filename != "gowdk.js" {
		t.Fatalf("unexpected runtime filename %q", Filename)
	}
}
