package components

import gowdkrender "github.com/cssbruno/gowdk/runtime/render"

type HeroProps struct {
	Title   string
	Tagline string
}

func RenderHero(props HeroProps) (string, error) {
	var out gowdkrender.Builder
	out.Static("<section><h1>")
	out.Text(props.Title)
	out.Static("</h1><p>")
	out.Text(props.Tagline)
	out.Static("</p></section>")
	return out.String(), nil
}
