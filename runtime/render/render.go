package render

import (
	"context"
	"strings"

	"github.com/gowdk/gowdk/runtime/component"
)

// Renderer is the core HTML renderer used by static, action, partial, and SSR
// addons.
type Renderer struct{}

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
