package buildgen

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func BenchmarkGeneratedOutputFromValidatedIR(b *testing.B) {
	config := gowdk.Config{}
	sources := benchmarkBuildSources(`<main><h1>Home</h1></main>`)
	analyzed, err := compiler.AnalyzeProgram(config, sources)
	if err != nil {
		b.Fatal(err)
	}
	validated, err := compiler.ValidateAnalyzedProgram(config, analyzed)
	if err != nil {
		b.Fatal(err)
	}
	outputDir := filepath.Join(b.TempDir(), "dist")

	b.ResetTimer()
	for b.Loop() {
		if _, err := BuildFromValidatedProgram(config, validated, outputDir); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIncrementalRebuildChangedPage(b *testing.B) {
	config := gowdk.Config{}
	outputDir := filepath.Join(b.TempDir(), "dist")
	homeSource := "pages/home.page.gwdk"

	initial := benchmarkBuildSources(`<main><h1>Home before</h1></main>`)
	if _, err := Build(config, initial, outputDir); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		changed := benchmarkBuildSources(fmt.Sprintf(`<main><h1>Home %d</h1></main>`, i))
		if _, err := BuildIncremental(config, changed, outputDir, []string{homeSource}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkBuildSources(homeView string) gwdkanalysis.Sources {
	return gwdkanalysis.Sources{Pages: []gwdkir.Page{
		benchmarkBuildPage("home", "/", homeView),
		benchmarkBuildPage("docs", "/docs", `<main><h1>Docs</h1></main>`),
		benchmarkBuildPage("about", "/about", `<main><h1>About</h1></main>`),
	}}
}

func benchmarkBuildPage(id, route, view string) gwdkir.Page {
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
