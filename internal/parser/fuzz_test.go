package parser

import "testing"

func FuzzParseSyntax(f *testing.F) {
	seeds := []string{
		`package pages

page home
route "/"

build {
  => { title: "Home" }
}

view {
  <main><h1>{title}</h1></main>
}
`,
		`package pages

page docs
route "/docs/{path...}"

paths {
  => { path: "intro/getting-started" }
}

view {
  <main><a href="/docs/intro">Docs</a></main>
}
`,
		`package components

component Badge

props {
  Label string = "New"
}

asset "./badge.svg"
js {
  console.log("badge")
}

style {
  .badge { display: inline-flex; }
}

view {
  <span class="badge">{Label}</span>
}
`,
		`package layouts

layout root

view {
  <html><body><slot /></body></html>
}
`,
		`package broken

page bad
route "/bad"

build {
  title = "Bad"
}

view {
  <main>
`,
		`package api

page newsletter
route "/newsletter"

act Subscribe POST "/newsletter"
api Health GET "/api/health"
fragment List GET "/newsletter/list" "#items" {
  <ul><li>One</li></ul>
}

view {
  <form g:post={Subscribe}><input name="email" /></form>
}
`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source string) {
		_, _ = ParseSyntax([]byte(source))
	})
}
