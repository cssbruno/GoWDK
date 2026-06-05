package components

import gowdkrender "github.com/cssbruno/gowdk/runtime/render"

type HeroProps struct {
	Title   string
	Tagline string
}

func RenderHero(props HeroProps) (string, error) {
	var out gowdkrender.Builder
	out.Markup("<section><h1>")
	out.Text(props.Title)
	out.Markup("</h1><p>")
	out.Text(props.Tagline)
	out.Markup("</p></section>")
	return out.String(), nil
}
