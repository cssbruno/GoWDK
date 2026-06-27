package cssexample

import (
	"fmt"

	"github.com/cssbruno/gowdk"
)

type brandCSS struct{}

func (brandCSS) Name() string {
	return "brand-css"
}

func (brandCSS) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureCSS}
}

func (brandCSS) ProcessCSS(gowdk.CSSContext) (gowdk.CSSResult, error) {
	return gowdk.CSSResult{
		Assets: []gowdk.CSSAsset{
			{Path: "assets/site.css", Contents: []byte("body { font-family: system-ui; }\n")},
		},
		Stylesheets: []gowdk.Stylesheet{
			{Href: "/assets/site.css"},
		},
	}, nil
}

func ExampleCSSProcessor() {
	var processor gowdk.CSSProcessor = brandCSS{}
	result, _ := processor.ProcessCSS(gowdk.CSSContext{OutputDir: "dist"})

	fmt.Println(processor.Name())
	fmt.Println(result.Assets[0].Path)
	fmt.Println(result.Stylesheets[0].Href)

	// Output:
	// brand-css
	// assets/site.css
	// /assets/site.css
}
