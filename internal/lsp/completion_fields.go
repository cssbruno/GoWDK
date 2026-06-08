package lsp

func inferredComponentFields(viewBody, clientBody string) []string {
	fields := map[string]bool{}
	for _, match := range simpleInterpolationCompletionPattern.FindAllStringSubmatch(viewBody, -1) {
		fields[match[1]] = true
	}
	for _, match := range bindingCompletionPattern.FindAllStringSubmatch(viewBody, -1) {
		fields[match[1]] = true
	}
	for _, match := range assignmentCompletionPattern.FindAllStringSubmatch(clientBody, -1) {
		fields[match[1]] = true
	}
	out := make([]string, 0, len(fields))
	for field := range fields {
		out = append(out, field)
	}
	return out
}
