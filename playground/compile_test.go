package playground

import (
	"strings"
	"testing"
)

func TestCompileReturnsSPAPreviewFiles(t *testing.T) {
	result := Compile(Project{Files: map[string]string{
		"src/pages/home.page.gwdk": `@page home
@route "/"

view {
  <main><h1>Hello playground</h1></main>
}`,
	}})

	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", result.Diagnostics)
	}
	if len(result.Routes) != 1 || result.Routes[0].Route != "/" || result.Routes[0].Path != "index.html" {
		t.Fatalf("unexpected routes: %#v", result.Routes)
	}
	html := result.HTML["index.html"]
	if !strings.Contains(html, "Hello playground") {
		t.Fatalf("expected rendered HTML, got %q", html)
	}
	if !strings.Contains(result.Files["gowdk-routes.json"], `"route": "/"`) {
		t.Fatalf("expected route manifest, got %q", result.Files["gowdk-routes.json"])
	}
	if !strings.Contains(result.Files["gowdk-assets.json"], `"version": 1`) {
		t.Fatalf("expected asset manifest, got %q", result.Files["gowdk-assets.json"])
	}
}

func TestCompileReturnsStyleBlockCSS(t *testing.T) {
	result := Compile(Project{Files: map[string]string{
		"src/pages/styled.page.gwdk": `@page styled
@route "/styled"

view {
  <main class="hero">Styled playground</main>
}

style {
  .hero {
    color: red;
  }
}`,
	}})

	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", result.Diagnostics)
	}
	html := result.HTML["styled/index.html"]
	if !strings.Contains(html, `rel="stylesheet"`) || !strings.Contains(html, `/assets/`) {
		t.Fatalf("expected rendered HTML to link generated CSS, got %q", html)
	}
	var cssPath, cssBody string
	for path, body := range result.CSS {
		if strings.Contains(body, ".hero") {
			cssPath = path
			cssBody = body
			break
		}
	}
	if cssPath == "" {
		t.Fatalf("expected generated CSS for style block, got %#v", result.CSS)
	}
	if !strings.Contains(result.Files[cssPath], ".hero") || !strings.Contains(cssBody, "color:red") {
		t.Fatalf("expected generated style CSS, path=%q css=%q file=%q", cssPath, cssBody, result.Files[cssPath])
	}
}

func TestCompileReportsDiagnostics(t *testing.T) {
	result := Compile(Project{Files: map[string]string{
		"src/pages/broken.page.gwdk": `@page broken
@route "/broken"

view {
  <main>
}`,
	}})

	if len(result.Diagnostics) == 0 {
		t.Fatal("expected diagnostics")
	}
	if result.Diagnostics[0].Severity != "error" {
		t.Fatalf("expected error diagnostic, got %#v", result.Diagnostics[0])
	}
}

func TestCompileRequiresPage(t *testing.T) {
	result := Compile(Project{Files: map[string]string{
		"src/components/button.cmp.gwdk": `@component button

view {
  <button>Click</button>
}`,
	}})

	if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "at least one page") {
		t.Fatalf("expected missing page diagnostic, got %#v", result.Diagnostics)
	}
}
