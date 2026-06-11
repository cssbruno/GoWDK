package source

import "strings"

// ExportedIdentifier returns a PascalCase Go identifier fragment derived from
// value. Underscores are preserved because they are valid Go identifier
// characters and can distinguish otherwise identical generated names.
// fallback is returned when value contains no identifier characters.
func ExportedIdentifier(value string, fallback string) string {
	out := make([]rune, 0, len(value))
	uppercaseNext := true
	for _, char := range strings.TrimSpace(value) {
		if char >= 'a' && char <= 'z' {
			if uppercaseNext {
				char = char - 'a' + 'A'
			}
			out = append(out, char)
			uppercaseNext = false
			continue
		}
		if char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' {
			if len(out) == 0 && char >= '0' && char <= '9' {
				out = append(out, 'P')
			}
			out = append(out, char)
			uppercaseNext = false
			continue
		}
		if char == '_' {
			out = append(out, char)
			uppercaseNext = false
			continue
		}
		uppercaseNext = true
	}
	if len(out) == 0 {
		return fallback
	}
	return string(out)
}
