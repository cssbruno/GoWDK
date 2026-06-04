package render

import (
	"context"
	"strings"

	"github.com/gowdk/gowdk/runtime/component"
	gowdkhtml "github.com/gowdk/gowdk/runtime/html"
)

// Renderer is the core HTML renderer used by static, action, partial, and SSR
// addons.
type Renderer struct{}

// Builder is the low-level generated render target. Static writes are trusted
// compiler output; expression writes escape by default.
type Builder struct {
	out strings.Builder
}

// Static writes compiler-owned markup.
func (builder *Builder) Static(value string) {
	builder.out.WriteString(value)
}

// Text writes expression output escaped for HTML text context.
func (builder *Builder) Text(value string) {
	builder.out.WriteString(gowdkhtml.Escape(value))
}

// String returns the rendered HTML.
func (builder *Builder) String() string {
	return builder.out.String()
}

// Render joins generated component output into one HTML response body.
func (renderer Renderer) Render(ctx context.Context, components ...component.Component) (string, error) {
	var out strings.Builder
	for _, cmp := range components {
		html, err := cmp.Render(ctx)
		if err != nil {
			return "", err
		}
		out.WriteString(html)
	}
	return out.String(), nil
}
