package buildgen

import (
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

func markPageURLPlaceholders(html string, replacements []SSRReplacement, loadReplacements []SSRLoadReplacement) (string, []SSRReplacement, []SSRLoadReplacement) {
	next := 0
	html, replacements, loadReplacements, _ = markURLPlaceholdersInHTML(html, replacements, loadReplacements, nil, &next)
	return html, replacements, loadReplacements
}

func markRegionURLPlaceholders(regions ssrRegions) ssrRegions {
	next := 0
	regions.Lists = markSSRListSpecsURLPlaceholders(regions.Lists, &next)
	regions.Conds = markSSRCondSpecsURLPlaceholders(regions.Conds, &next)
	return regions
}

func markSSRListSpecsURLPlaceholders(specs []source.SSRListSpec, next *int) []source.SSRListSpec {
	if len(specs) == 0 {
		return specs
	}
	out := make([]source.SSRListSpec, 0, len(specs))
	for _, spec := range specs {
		spec.RowTemplate, _, _, spec.Fields = markURLPlaceholdersInHTML(spec.RowTemplate, nil, nil, spec.Fields, next)
		spec.Fields = usedSSRListFields(spec.RowTemplate, spec.Fields)
		spec.Lists = markSSRListSpecsURLPlaceholders(spec.Lists, next)
		spec.Conds = markSSRCondSpecsURLPlaceholders(spec.Conds, next)
		out = append(out, spec)
	}
	return out
}

func markSSRCondSpecsURLPlaceholders(specs []source.SSRCondSpec, next *int) []source.SSRCondSpec {
	if len(specs) == 0 {
		return specs
	}
	out := make([]source.SSRCondSpec, 0, len(specs))
	for _, spec := range specs {
		spec.Template, _, _, spec.Fields = markURLPlaceholdersInHTML(spec.Template, nil, nil, spec.Fields, next)
		spec.Fields = usedSSRListFields(spec.Template, spec.Fields)
		spec.Lists = markSSRListSpecsURLPlaceholders(spec.Lists, next)
		spec.Conds = markSSRCondSpecsURLPlaceholders(spec.Conds, next)
		out = append(out, spec)
	}
	return out
}

func markURLPlaceholdersInHTML(html string, replacements []SSRReplacement, loadReplacements []SSRLoadReplacement, fields []source.SSRListField, next *int) (string, []SSRReplacement, []SSRLoadReplacement, []source.SSRListField) {
	if html == "" {
		return html, replacements, loadReplacements, fields
	}
	ranges := urlAttrValueRanges(html)
	if len(ranges) == 0 {
		return html, replacements, loadReplacements, fields
	}

	routeURLs := map[string]string{}
	loadURLs := map[string]string{}
	fieldURLs := map[string]string{}
	var out strings.Builder
	last := 0
	for _, valueRange := range ranges {
		if valueRange.start < last || valueRange.end > len(html) {
			continue
		}
		out.WriteString(html[last:valueRange.start])
		value := html[valueRange.start:valueRange.end]
		value, replacements, loadReplacements, fields = markURLPlaceholdersInValue(value, replacements, loadReplacements, fields, routeURLs, loadURLs, fieldURLs, next)
		out.WriteString(value)
		last = valueRange.end
	}
	out.WriteString(html[last:])
	return out.String(), replacements, loadReplacements, fields
}

func markURLPlaceholdersInValue(value string, replacements []SSRReplacement, loadReplacements []SSRLoadReplacement, fields []source.SSRListField, routeURLs, loadURLs, fieldURLs map[string]string, next *int) (string, []SSRReplacement, []SSRLoadReplacement, []source.SSRListField) {
	routeCount := len(replacements)
	for index := 0; index < routeCount; index++ {
		replacement := replacements[index]
		if replacement.URL || !strings.Contains(value, replacement.Placeholder) {
			continue
		}
		placeholder, ok := routeURLs[replacement.Placeholder]
		if !ok {
			placeholder = urlPlaceholder(replacement.Placeholder, next)
			routeURLs[replacement.Placeholder] = placeholder
			replacement.Placeholder = placeholder
			replacement.URL = true
			replacements = append(replacements, replacement)
		}
		value = strings.ReplaceAll(value, replacements[index].Placeholder, placeholder)
	}

	loadCount := len(loadReplacements)
	for index := 0; index < loadCount; index++ {
		replacement := loadReplacements[index]
		if replacement.URL || !strings.Contains(value, replacement.Placeholder) {
			continue
		}
		placeholder, ok := loadURLs[replacement.Placeholder]
		if !ok {
			placeholder = urlPlaceholder(replacement.Placeholder, next)
			loadURLs[replacement.Placeholder] = placeholder
			replacement.Placeholder = placeholder
			replacement.URL = true
			loadReplacements = append(loadReplacements, replacement)
		}
		value = strings.ReplaceAll(value, loadReplacements[index].Placeholder, placeholder)
	}

	fieldCount := len(fields)
	for index := 0; index < fieldCount; index++ {
		field := fields[index]
		if field.URL || !strings.Contains(value, field.Placeholder) {
			continue
		}
		placeholder, ok := fieldURLs[field.Placeholder]
		if !ok {
			placeholder = urlPlaceholder(field.Placeholder, next)
			fieldURLs[field.Placeholder] = placeholder
			field.Placeholder = placeholder
			field.URL = true
			fields = append(fields, field)
		}
		value = strings.ReplaceAll(value, fields[index].Placeholder, placeholder)
	}
	return value, replacements, loadReplacements, fields
}

func urlPlaceholder(base string, next *int) string {
	*next++
	suffix := "_URL_" + strconv.Itoa(*next)
	if strings.HasSuffix(base, "__") {
		return strings.TrimSuffix(base, "__") + suffix + "__"
	}
	return base + suffix
}

func usedSSRReplacements(html string, replacements []SSRReplacement) []SSRReplacement {
	if len(replacements) == 0 {
		return replacements
	}
	used := make([]SSRReplacement, 0, len(replacements))
	for _, replacement := range replacements {
		if strings.Contains(html, replacement.Placeholder) {
			used = append(used, replacement)
		}
	}
	return used
}

func usedSSRListFields(template string, fields []source.SSRListField) []source.SSRListField {
	if len(fields) == 0 {
		return fields
	}
	used := make([]source.SSRListField, 0, len(fields))
	for _, field := range fields {
		if strings.Contains(template, field.Placeholder) {
			used = append(used, field)
		}
	}
	return used
}

type attrValueRange struct {
	start int
	end   int
}

func urlAttrValueRanges(html string) []attrValueRange {
	var ranges []attrValueRange
	for index := 0; index < len(html); index++ {
		if html[index] != '<' {
			continue
		}
		end := htmlTagEnd(html, index+1)
		if end < 0 {
			break
		}
		ranges = append(ranges, tagURLAttrValueRanges(html, index+1, end)...)
		index = end
	}
	return ranges
}

func htmlTagEnd(html string, cursor int) int {
	var quote byte
	for cursor < len(html) {
		if quote != 0 {
			if html[cursor] == quote {
				quote = 0
			}
			cursor++
			continue
		}
		switch html[cursor] {
		case '\'', '"':
			quote = html[cursor]
		case '>':
			return cursor
		}
		cursor++
	}
	return -1
}

func tagURLAttrValueRanges(html string, cursor, end int) []attrValueRange {
	var ranges []attrValueRange
	if cursor >= end || html[cursor] == '/' || html[cursor] == '!' || html[cursor] == '?' {
		return nil
	}
	for cursor < end && !isHTMLSpace(html[cursor]) && html[cursor] != '/' {
		cursor++
	}
	for cursor < end {
		for cursor < end && isHTMLSpace(html[cursor]) {
			cursor++
		}
		if cursor >= end || html[cursor] == '/' {
			return ranges
		}
		nameStart := cursor
		for cursor < end && !isHTMLSpace(html[cursor]) && html[cursor] != '=' && html[cursor] != '/' {
			cursor++
		}
		name := html[nameStart:cursor]
		for cursor < end && isHTMLSpace(html[cursor]) {
			cursor++
		}
		if cursor >= end || html[cursor] != '=' {
			continue
		}
		cursor++
		for cursor < end && isHTMLSpace(html[cursor]) {
			cursor++
		}
		valueStart, valueEnd, next := htmlAttrValueRange(html, cursor, end)
		cursor = next
		if urlBearingSSRAttr(name) {
			ranges = append(ranges, attrValueRange{start: valueStart, end: valueEnd})
		}
	}
	return ranges
}

func htmlAttrValueRange(html string, cursor, end int) (int, int, int) {
	if cursor >= end {
		return cursor, cursor, cursor
	}
	if html[cursor] == '\'' || html[cursor] == '"' {
		quote := html[cursor]
		cursor++
		start := cursor
		for cursor < end && html[cursor] != quote {
			cursor++
		}
		if cursor < end {
			return start, cursor, cursor + 1
		}
		return start, cursor, cursor
	}
	start := cursor
	for cursor < end && !isHTMLSpace(html[cursor]) && html[cursor] != '/' {
		cursor++
	}
	return start, cursor, cursor
}

func isHTMLSpace(char byte) bool {
	switch char {
	case ' ', '\n', '\r', '\t', '\f':
		return true
	default:
		return false
	}
}

func urlBearingSSRAttr(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "href", "src", "srcset", "action", "formaction", "poster", "cite", "data", "longdesc", "manifest", "xlink:href":
		return true
	default:
		return false
	}
}
