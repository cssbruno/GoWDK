package playground

import (
	"strings"
	"testing"
)

func TestUIHTMLIncludesCompilerPreviewAndDiagnostics(t *testing.T) {
	html := UIHTML()
	for _, expected := range []string{
		`<textarea id="source">`,
		`<iframe id="preview"`,
		`id="diagnostics"`,
		`id="files"`,
		`wasm_exec.js`,
		`gowdk.wasm`,
		`window.gowdkCompile`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in playground UI:\n%s", expected, html)
		}
	}
}
