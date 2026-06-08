package view

import "strings"

type renderOutput struct {
	parts []string
}

func (out *renderOutput) write(value string) {
	out.parts = append(out.parts, value)
}

func (out *renderOutput) writeByte(value byte) {
	out.parts = append(out.parts, string(value))
}

func (out *renderOutput) string() string {
	return strings.Join(out.parts, "")
}
