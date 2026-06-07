package playground

import (
	"strings"
	"testing"
)

func TestUIHTMLIncludesCompilerPreviewAndDiagnostics(t *testing.T) {
	html := UIHTML()
	for _, expected := range []string{
		`<textarea id="source"`,
		`<iframe id="preview"`,
		`id="diagnostics"`,
		`id="project-tree"`,
		`id="generated-html"`,
		`id="generated-css"`,
		`id="generated-js"`,
		`id="export-project"`,
		`id="share-project"`,
		`#project=`,
		`starter`,
		`wasm_exec.js`,
		`gowdk.wasm`,
		`window.gowdkCompile`,
		`preview.srcdoc = preparePreviewHTML(result, result.html[first]);`,
		`URL.createObjectURL`,
		`rewritePreviewAssetURL`,
		`text/css`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in playground UI:\n%s", expected, html)
		}
	}
}
