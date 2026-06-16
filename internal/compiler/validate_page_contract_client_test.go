package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func contractCommandPage(id string, render gowdk.RenderMode) gwdkir.Page {
	return gwdkir.Page{
		ID:     id,
		Route:  "/" + id,
		Source: "pages/" + id + ".page.gwdk",
		Render: render,
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><form g:command="issues.CreateIssue"><input name="title" /></form></main>`,
		},
	}
}

func ssrConfig() gowdk.Config {
	return gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("ssr", gowdk.FeatureSSR)}}
}

func TestValidatePageWarnsCommandWriteFormOnRequestTimePage(t *testing.T) {
	for _, render := range []gowdk.RenderMode{gowdk.SSR, gowdk.Hybrid} {
		t.Run(string(render), func(t *testing.T) {
			page := contractCommandPage("board", render)
			diagnostics := ValidatePage(ssrConfig(), irPage(page))
			diagnostic := firstDiagnostic(diagnostics, "ssr_command_no_client")
			if diagnostic == nil {
				t.Fatalf("missing ssr_command_no_client diagnostic: %#v", diagnostics)
			}
			if diagnostic.Severity != SeverityWarning {
				t.Fatalf("expected warning severity, got %q", diagnostic.Severity)
			}
			if !strings.Contains(diagnostic.Message, "issues.CreateIssue") || !strings.Contains(diagnostic.Message, "response.Response") {
				t.Fatalf("unexpected diagnostic message: %q", diagnostic.Message)
			}
			if ValidationErrors(diagnostics).HasErrors() {
				t.Fatalf("ssr_command_no_client must not fail the build: %#v", diagnostics)
			}
		})
	}
}

func TestValidatePageAllowsCommandWriteFormOnBuildTimePage(t *testing.T) {
	page := contractCommandPage("board", gowdk.SPA)
	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if firstDiagnostic(diagnostics, "ssr_command_no_client") != nil {
		t.Fatalf("build-time pages ship the client runtime; did not expect ssr_command_no_client: %#v", diagnostics)
	}
}

func TestValidatePageIgnoresQueryOnlyRequestTimePage(t *testing.T) {
	page := gwdkir.Page{
		ID:     "board",
		Route:  "/board",
		Source: "pages/board.page.gwdk",
		Render: gowdk.SSR,
		Guards: []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main><section g:query="issues.GetBoard">Board</section></main>`,
		},
	}
	diagnostics := ValidatePage(ssrConfig(), irPage(page))
	if firstDiagnostic(diagnostics, "ssr_command_no_client") != nil {
		t.Fatalf("a read-only g:query region is not a broken write path: %#v", diagnostics)
	}
}
