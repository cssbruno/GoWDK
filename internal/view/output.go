package view

import gowdkrender "github.com/cssbruno/gowdk/runtime/render"

type renderOutput struct {
	builder gowdkrender.Builder
}

func (out *renderOutput) write(value string) {
	out.builder.Markup(value)
}

func (out *renderOutput) writeByte(value byte) {
	out.builder.Markup(string(value))
}

func (out *renderOutput) string() string {
	return out.builder.String()
}
