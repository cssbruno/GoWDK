package parser

import "testing"

func TestParseSyntaxBuildsTypedASTForCurrentSubset(t *testing.T) {
	file, err := ParseSyntax([]byte(`
@page newsletter
@route "/newsletter/{slug}"
@guard auth.required

paths {
  => { slug: "hello" }
}

build {
  => { title: "Newsletter" }
}

act subscribe {
  input := form SubscribeInput
  valid(input)?
  fragment "#status" {
    <p>Saved</p>
  }
  -> "/newsletter?ok=1"
}

api health {
  GET "/api/health"
}

view {
  <main><Panel><h1>{title}</h1></Panel></main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Annotations) != 3 || file.Annotations[1].Name != "route" || file.Annotations[1].Span.Start.Line != 3 {
		t.Fatalf("unexpected annotations: %#v", file.Annotations)
	}
	if len(file.Blocks) != 5 {
		t.Fatalf("expected five blocks, got %#v", file.Blocks)
	}
	paths := file.Blocks[0]
	if paths.Kind != "paths" || len(paths.Records) != 1 || paths.Records[0].Fields["slug"] != "hello" {
		t.Fatalf("unexpected paths AST: %#v", paths)
	}
	build := file.Blocks[1]
	if build.Kind != "build" || len(build.Records) != 1 || build.Records[0].Fields["title"] != "Newsletter" {
		t.Fatalf("unexpected build AST: %#v", build)
	}
	action := file.Blocks[2]
	if action.Kind != "act" || action.Name != "subscribe" || len(action.Actions) != 4 {
		t.Fatalf("unexpected action AST: %#v", action)
	}
	if action.Actions[0].Kind != "input" || action.Actions[0].InputType != "SubscribeInput" {
		t.Fatalf("unexpected input action statement: %#v", action.Actions[0])
	}
	if action.Actions[2].Kind != "fragment" || action.Actions[2].Target != "#status" || action.Actions[2].Body != "<p>Saved</p>" {
		t.Fatalf("unexpected fragment action statement: %#v", action.Actions[2])
	}
	api := file.Blocks[3]
	if api.Kind != "api" || api.Name != "health" || len(api.APIs) != 1 || api.APIs[0].Route != "/api/health" {
		t.Fatalf("unexpected api AST: %#v", api)
	}
	view := file.Blocks[4]
	if view.Kind != "view" || len(view.View) != 1 {
		t.Fatalf("expected parsed view AST, got %#v", view)
	}
}

func TestParseSyntaxReportsBodySyntaxLine(t *testing.T) {
	_, err := ParseSyntax([]byte(`@page bad
@route "/bad"

build {
  title = "Bad"
}
`))
	if err == nil {
		t.Fatal("expected literal record error")
	}
	if err.Error() != `line 5: unsupported literal record syntax "title = \"Bad\""` {
		t.Fatalf("unexpected error: %v", err)
	}
}
