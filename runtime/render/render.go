package render

import (
	"context"
	"strings"

	"github.com/cssbruno/gowdk/runtime/component"
	gowdkhtml "github.com/cssbruno/gowdk/runtime/html"
)

// Renderer is the core HTML renderer used by app, action, partial, and SSR
// output.
type Renderer struct{}

// Builder is the low-level generated render target. Markup writes are trusted
// compiler output; expression writes escape by default.
type Builder struct {
	parts []string
}

// Markup writes compiler-owned markup.
func (builder *Builder) Markup(value string) {
	builder.parts = append(builder.parts, value)
}

// Text writes expression output escaped for HTML text context.
func (builder *Builder) Text(value string) {
	builder.parts = append(builder.parts, gowdkhtml.Escape(value))
}

// String returns the rendered HTML.
func (builder *Builder) String() string {
	return strings.Join(builder.parts, "")
}

// Render joins generated component output into one HTML response body.
func (renderer Renderer) Render(ctx context.Context, components ...component.Component) (string, error) {
	htmlParts := make([]string, 0, len(components))
	for _, cmp := range components {
		html, err := cmp.Render(ctx)
		if err != nil {
			return "", err
		}
		htmlParts = append(htmlParts, html)
	}
	return strings.Join(htmlParts, ""), nil
}
