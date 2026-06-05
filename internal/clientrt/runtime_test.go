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

func TestFilename(t *testing.T) {
	if Filename != "gowdk.js" {
		t.Fatalf("unexpected runtime filename %q", Filename)
	}
}
