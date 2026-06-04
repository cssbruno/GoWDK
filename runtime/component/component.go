package component

import "context"

// Component is the runtime contract for generated components.
type Component interface {
	Render(context.Context) (string, error)
}

// Func adapts a render function into a Component.
type Func func(context.Context) (string, error)

// Render calls the wrapped component function.
func (fn Func) Render(ctx context.Context) (string, error) {
	return fn(ctx)
}
