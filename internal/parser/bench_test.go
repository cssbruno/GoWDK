package parser

import "testing"

var benchmarkPageSource = []byte(`package app

import interop "github.com/acme/app/interop"

page dashboard
route "/dashboard"
guard public

build {
  => { title: "Dashboard", count: "12" }
}

act Save POST "/dashboard/save"
api Session GET "/api/session"

view {
  <main>
    <h1>{title}</h1>
    <form g:post={Save}>
      <input name="name" aria-label="Name" />
      <button type="submit">Save</button>
    </form>
  </main>
}
`)

func BenchmarkParseLowerPage(b *testing.B) {
	for b.Loop() {
		if _, err := ParsePage(benchmarkPageSource); err != nil {
			b.Fatal(err)
		}
	}
}
