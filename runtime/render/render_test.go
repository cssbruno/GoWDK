package render

import (
	"context"
	"errors"
	"testing"

	"github.com/gowdk/gowdk/runtime/component"
)

func TestRendererJoinsComponentOutput(t *testing.T) {
	html, err := (Renderer{}).Render(context.Background(),
		component.Func(func(context.Context) (string, error) { return "<header></header>", nil }),
		component.Func(func(context.Context) (string, error) { return "<main></main>", nil }),
	)
	if err != nil {
		t.Fatal(err)
	}
	if html != "<header></header><main></main>" {
		t.Fatalf("unexpected html: %q", html)
	}
}

func TestRendererPropagatesComponentError(t *testing.T) {
	expected := errors.New("component failed")
	_, err := (Renderer{}).Render(context.Background(),
		component.Func(func(context.Context) (string, error) { return "", expected }),
	)
	if !errors.Is(err, expected) {
		t.Fatalf("expected component error, got %v", err)
	}
}

func TestBuilderEscapesTextExpressions(t *testing.T) {
	var builder Builder
	builder.Static("<h1>")
	builder.Text(`<script>alert("x")</script>`)
	builder.Static("</h1>")

	if got := builder.String(); got != `<h1>&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;</h1>` {
		t.Fatalf("unexpected rendered html: %q", got)
	}
}
