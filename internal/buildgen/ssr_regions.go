package buildgen

import (
	"html"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

// SSRQueryRegion is the build-time render recipe for one g:query region. It is
// lowered by the app generator into a runtime region renderer.
type SSRQueryRegion = source.SSRQueryRegion

// ssrQueryRegions extracts the standalone render recipe for every g:query region
// in the rendered page HTML that carries a resolved query type.
//
// A region is only emitted when it can be re-rendered without route context the
// command adapter lacks: the page must declare no dynamic route params (so its
// load needs none) and the region template must contain no route-param
// placeholder. Purely static regions (no g:for/g:if and no scalar load field) are
// skipped because there is nothing to re-render reactively. Regions left out here
// fall back to the client refetch path, which fetches the live page with its real
// route context.
func ssrQueryRegions(htmlSource string, lists []SSRListSpec, conds []SSRCondSpec, loadReplacements []SSRLoadReplacement, paramReplacements []SSRReplacement, hasDynamicParams bool) []SSRQueryRegion {
	if hasDynamicParams {
		return nil
	}
	raw := extractQueryRegionTemplates(htmlSource)
	if len(raw) == 0 {
		return nil
	}
	var regions []SSRQueryRegion
	for _, region := range raw {
		if region.queryType == "" {
			continue
		}
		if regionUsesParam(region.template, paramReplacements) {
			continue
		}
		listSpecs := specsInTemplate(region.template, lists)
		condSpecs := condsInTemplate(region.template, conds)
		regionLoad := loadReplacementsInTemplate(region.template, loadReplacements)
		if len(listSpecs) == 0 && len(condSpecs) == 0 && len(regionLoad) == 0 {
			continue
		}
		regions = append(regions, SSRQueryRegion{
			QueryType:        region.queryType,
			Template:         region.template,
			ListSpecs:        listSpecs,
			CondSpecs:        condSpecs,
			LoadReplacements: regionLoad,
		})
	}
	return regions
}

type rawQueryRegion struct {
	queryType string
	template  string
}

const queryTypeMarker = `data-gowdk-query-type="`

// extractQueryRegionTemplates scans rendered page HTML for every element that
// carries a data-gowdk-query-type attribute and returns its outer HTML. Generated
// HTML is well-formed and HTML-escapes every attribute value and text node, so a
// raw '<' always opens a tag and a raw '>' always closes one, which makes the
// element boundary unambiguous.
func extractQueryRegionTemplates(htmlSource string) []rawQueryRegion {
	var regions []rawQueryRegion
	search := 0
	for {
		offset := strings.Index(htmlSource[search:], queryTypeMarker)
		if offset < 0 {
			break
		}
		attrPos := search + offset
		valueStart := attrPos + len(queryTypeMarker)
		valueLen := strings.IndexByte(htmlSource[valueStart:], '"')
		if valueLen < 0 {
			break
		}
		queryType := html.UnescapeString(htmlSource[valueStart : valueStart+valueLen])
		tagStart := strings.LastIndexByte(htmlSource[:attrPos], '<')
		if tagStart < 0 {
			search = valueStart + valueLen
			continue
		}
		outer, ok := elementOuterHTML(htmlSource, tagStart)
		if !ok {
			search = valueStart + valueLen
			continue
		}
		regions = append(regions, rawQueryRegion{queryType: queryType, template: outer})
		search = tagStart + len(outer)
	}
	return regions
}

// elementOuterHTML returns the outer HTML of the element whose opening tag begins
// at the '<' at tagStart, balancing nested same-name tags.
func elementOuterHTML(htmlSource string, tagStart int) (string, bool) {
	name, _, ok := readTagName(htmlSource, tagStart+1)
	if !ok {
		return "", false
	}
	openEnd := strings.IndexByte(htmlSource[tagStart:], '>')
	if openEnd < 0 {
		return "", false
	}
	openEnd += tagStart
	if htmlSource[openEnd-1] == '/' {
		return htmlSource[tagStart : openEnd+1], true
	}
	lower := strings.ToLower(name)
	depth := 1
	pos := openEnd + 1
	for pos < len(htmlSource) {
		next := strings.IndexByte(htmlSource[pos:], '<')
		if next < 0 {
			return "", false
		}
		next += pos
		if next+1 < len(htmlSource) && htmlSource[next+1] == '/' {
			closeName, closeNameEnd, ok := readTagName(htmlSource, next+2)
			if ok && strings.ToLower(closeName) == lower {
				depth--
				if depth == 0 {
					gt := strings.IndexByte(htmlSource[closeNameEnd:], '>')
					if gt < 0 {
						return "", false
					}
					return htmlSource[tagStart : closeNameEnd+gt+1], true
				}
			}
			pos = next + 2
			continue
		}
		openName, _, ok := readTagName(htmlSource, next+1)
		if ok && strings.ToLower(openName) == lower {
			gt := strings.IndexByte(htmlSource[next:], '>')
			if gt < 0 {
				return "", false
			}
			gt += next
			if htmlSource[gt-1] != '/' {
				depth++
			}
		}
		pos = next + 1
	}
	return "", false
}

// readTagName reads an HTML tag name starting at start and returns the name, the
// index just past it, and whether a name was present.
func readTagName(htmlSource string, start int) (string, int, bool) {
	end := start
	for end < len(htmlSource) {
		char := htmlSource[end]
		if char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '>' || char == '/' {
			break
		}
		end++
	}
	if end == start {
		return "", start, false
	}
	return htmlSource[start:end], end, true
}

func regionUsesParam(template string, replacements []SSRReplacement) bool {
	for _, replacement := range replacements {
		if strings.Contains(template, replacement.Placeholder) {
			return true
		}
	}
	return false
}

func specsInTemplate(template string, specs []SSRListSpec) []SSRListSpec {
	var out []SSRListSpec
	for _, spec := range specs {
		if strings.Contains(template, spec.Placeholder) {
			out = append(out, spec)
		}
	}
	return out
}

func condsInTemplate(template string, specs []SSRCondSpec) []SSRCondSpec {
	var out []SSRCondSpec
	for _, spec := range specs {
		if strings.Contains(template, spec.Placeholder) {
			out = append(out, spec)
		}
	}
	return out
}

func loadReplacementsInTemplate(template string, replacements []SSRLoadReplacement) []SSRLoadReplacement {
	var out []SSRLoadReplacement
	for _, replacement := range replacements {
		if strings.Contains(template, replacement.Placeholder) {
			out = append(out, replacement)
		}
	}
	return out
}
