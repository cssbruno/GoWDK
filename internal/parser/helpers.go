package parser

import "strings"

func trimQuotes(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}

// renamedTopLevelBlockNudge reports a migration hint when a top-level block uses
// a keyword that was renamed in the lane-model redesign, so existing pages get a
// precise pointer instead of a generic "unsupported top-level block" error.
func renamedTopLevelBlockNudge(line string) (string, bool) {
	if !strings.HasSuffix(line, "{") {
		return "", false
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false
	}
	switch fields[0] {
	case "load":
		return "load {} was renamed to server {}; it declares the page's request-time server lane data (rendered with go server {})", true
	default:
		return "", false
	}
}

func unsupportedTopLevelBlockName(line string) string {
	if !strings.HasSuffix(line, "{") {
		return ""
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	name := fields[0]
	if !isBlockName(name) {
		return ""
	}
	return name
}

func isMalformedPackage(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "package"
}

func isMalformedImport(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "import"
}

func isMalformedUse(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "use"
}

func isMalformedJS(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && fields[0] == "js"
}

func isBlockName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if !isIdentStart(r) {
				return false
			}
			continue
		}
		if !isBlockNamePart(r) {
			return false
		}
	}
	return true
}

func isIdentStart(r rune) bool {
	return r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isBlockNamePart(r rune) bool {
	return isIdentStart(r) || (r >= '0' && r <= '9') || r == '.' || r == '-'
}

func isExportedIdentifier(value string) bool {
	if !identifierPattern.MatchString(value) {
		return false
	}
	for _, r := range value {
		return r >= 'A' && r <= 'Z'
	}
	return false
}

func exportedIdentifierSuggestion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Handler"
	}
	out := make([]rune, 0, len(value))
	upperNext := true
	for _, r := range value {
		if !isIdentStart(r) && (r < '0' || r > '9') {
			upperNext = true
			continue
		}
		if len(out) == 0 && r >= '0' && r <= '9' {
			out = append(out, 'X')
		}
		if upperNext {
			if r >= 'a' && r <= 'z' {
				r = r - 'a' + 'A'
			}
			upperNext = false
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "Handler"
	}
	return string(out)
}
