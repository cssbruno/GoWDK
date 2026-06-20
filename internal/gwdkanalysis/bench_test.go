package gwdkanalysis

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func BenchmarkIRAssembly(b *testing.B) {
	sources := benchmarkSources()
	config := gowdk.Config{}
	for b.Loop() {
		program := BuildProgram(config, sources)
		if len(program.Pages) != len(sources.Pages) {
			b.Fatalf("assembled %d pages, want %d", len(program.Pages), len(sources.Pages))
		}
	}
}

func benchmarkSources() Sources {
	return Sources{
		Pages: []gwdkir.Page{
			benchmarkPage("home", "/", `<main><h1>Home</h1><Card title="Welcome" /></main>`),
			benchmarkPage("docs", "/docs", `<main><h1>Docs</h1><Card title="Read" /></main>`),
			benchmarkPage("account", "/account", `<main><h1>Account</h1><Card title="Secure" /></main>`),
		},
		Components: []gwdkir.Component{{
			Source:  "components/card.cmp.gwdk",
			Package: "app",
			Name:    "Card",
			Props:   []gwdkir.Prop{{Name: "title", Type: "string"}},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<article class="card"><h2>{title}</h2><slot /></article>`,
			},
		}},
	}
}

func benchmarkPage(id, route, view string) gwdkir.Page {
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
