package compiler

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func BenchmarkValidateProgram(b *testing.B) {
	config := gowdk.Config{}
	program := gwdkanalysis.BuildProgram(config, benchmarkValidationSources())
	b.ResetTimer()
	for b.Loop() {
		if err := ValidateProgram(config, program); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkValidationSources() gwdkanalysis.Sources {
	return gwdkanalysis.Sources{
		Pages: []gwdkir.Page{
			benchmarkValidationPage("home", "/", `<main><h1>Home</h1></main>`),
			benchmarkValidationPage("docs", "/docs", `<main><h1>Docs</h1></main>`),
			benchmarkValidationPage("settings", "/settings", `<main><h1>Settings</h1></main>`),
		},
	}
}

func benchmarkValidationPage(id, route, view string) gwdkir.Page {
	return gwdkir.Page{
		Source:  "pages/" + id + ".page.gwdk",
		Package: "app",
		ID:      id,
		Route:   route,
		Render:  gowdk.SPA,
		Guards:  []string{"public"},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: view,
		},
	}
}
