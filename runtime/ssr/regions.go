package ssr

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"sync"

	gowdkhtml "github.com/cssbruno/gowdk/runtime/html"
)

// RegionPatch is the rendered HTML for one invalidated g:query region, returned
// inline in a g:command response so the submitting client applies it without a
// second page fetch (true single-flight).
type RegionPatch struct {
	Query string `json:"query"`
	HTML  string `json:"html"`
}

// CommandEnvelope wraps a command result with its single-flight region patches.
// A g:command adapter only emits this shape (signalled by the X-GOWDK-Patches
// response header) when it rendered at least one region; otherwise the raw
// command result body is returned unchanged, so non-browser callers are
// unaffected.
type CommandEnvelope struct {
	Result  any           `json:"result"`
	Patches []RegionPatch `json:"patches"`
}

// RegionLoadField is one scalar load {} substitution scoped to a region. It
// mirrors the page-level SSRLoadReplacement but is applied to a single region's
// rendered HTML.
type RegionLoadField struct {
	Path        string
	Placeholder string
}

// RegionRenderer renders one request-time g:query region standalone from its
// page's load data. The app generator lowers each eligible region into a
// RegionRenderer and registers it by fully-qualified query type, so a g:command
// can render exactly the regions it invalidated inline in its response.
//
// A region is only registered when it is renderable without route context the
// command request lacks: its page has no dynamic route params and its template
// carries no route-param placeholder. Everything else falls back to the client
// refetch path.
type RegionRenderer struct {
	// QueryType is the fully-qualified query type that names this region, matching
	// the X-GOWDK-Queries header and the region's data-gowdk-query-type attribute.
	QueryType string
	// Template is the region element's outer HTML with its region and scalar
	// placeholders intact, ready for RenderRegions.
	Template string
	// Lists and Conds are the page's top-level g:for/g:if specs that fall inside
	// this region's template.
	Lists []ListSpec
	Conds []CondSpec
	// LoadFields are the scalar load {} substitutions that appear in the template.
	LoadFields []RegionLoadField
	// Load runs the region's page load {} against the command request, yielding
	// the same data a page GET would resolve the region from.
	Load func(*http.Request) (map[string]any, error)
}

func (renderer RegionRenderer) render(request *http.Request) (string, bool) {
	if renderer.Load == nil {
		return "", false
	}
	data, err := renderer.Load(request)
	if err != nil || data == nil {
		return "", false
	}
	html := RenderRegions(renderer.Template, renderer.Lists, renderer.Conds, data)
	for _, field := range renderer.LoadFields {
		value, ok := LoadPath(data, field.Path)
		if !ok {
			return "", false
		}
		html = strings.ReplaceAll(html, field.Placeholder, gowdkhtml.Escape(fmt.Sprint(value)))
	}
	return html, true
}

type regionEntry struct {
	renderer  RegionRenderer
	ambiguous bool
}

var (
	regionMu        sync.RWMutex
	regionRenderers = map[string]regionEntry{}
)

// RegisterRegion records a region renderer by query type. When two renderers
// register the same query type (the same query backs regions on more than one
// parameterless page), the type is marked ambiguous and single-flight falls back
// to the client refetch for it, since the command request cannot tell which
// page's region the submitter is viewing.
func RegisterRegion(renderer RegionRenderer) {
	if renderer.QueryType == "" || renderer.Load == nil {
		return
	}
	regionMu.Lock()
	defer regionMu.Unlock()
	if existing, ok := regionRenderers[renderer.QueryType]; ok {
		existing.ambiguous = true
		regionRenderers[renderer.QueryType] = existing
		return
	}
	regionRenderers[renderer.QueryType] = regionEntry{renderer: renderer}
}

// RenderInvalidatedRegions renders the registered g:query regions for the given
// invalidated query types, using request for load context. Query types without
// an eligible, unambiguous renderer (or whose load fails) are skipped so the
// client refetches them. The returned patches follow the order of queries.
func RenderInvalidatedRegions(request *http.Request, queries []string) []RegionPatch {
	if len(queries) == 0 {
		return nil
	}
	var patches []RegionPatch
	for _, query := range queries {
		regionMu.RLock()
		entry, ok := regionRenderers[query]
		regionMu.RUnlock()
		if !ok || entry.ambiguous {
			continue
		}
		html, rendered := entry.renderer.render(request)
		if !rendered {
			continue
		}
		if htmlContainsPostForm(html) {
			continue
		}
		patches = append(patches, RegionPatch{Query: query, HTML: html})
	}
	return patches
}

func htmlContainsPostForm(html string) bool {
	payload := []byte(html)
	for _, match := range formStartTagRanges(payload) {
		if formStartTagHasPostMethod(payload[match[0]:match[1]]) {
			return true
		}
	}
	// A nominally GET form can still POST through a submit control that
	// overrides the method, e.g. <button formmethod="post">. Scan every start
	// tag for that override so such a form is not embedded in a patch without a
	// freshly generated CSRF token.
	for _, match := range startTagRanges(payload) {
		if tagHasPostFormmethod(payload[match[0]:match[1]]) {
			return true
		}
	}
	return false
}

// startTagRanges returns the byte ranges of every HTML start tag in payload.
func startTagRanges(payload []byte) [][2]int {
	var matches [][2]int
	for index := 0; index < len(payload); index++ {
		if payload[index] != '<' {
			continue
		}
		nameStart := index + 1
		if nameStart >= len(payload) || !isHTMLNameChar(payload[nameStart]) {
			// Not a start tag: a closing tag, comment, or stray '<'.
			continue
		}
		end := htmlTagEnd(payload, nameStart)
		if end < 0 {
			break
		}
		matches = append(matches, [2]int{index, end + 1})
		index = end
	}
	return matches
}

// tagHasPostFormmethod reports whether the start tag carries formmethod="post".
func tagHasPostFormmethod(tag []byte) bool {
	return tagAttrMatch(tag, func(name, value []byte) bool {
		return bytes.EqualFold(name, []byte("formmethod")) && strings.EqualFold(string(value), http.MethodPost)
	})
}

// tagAttrMatch walks the attributes of an HTML start tag (the slice must begin
// at '<') and reports whether want returns true for any name/value pair.
func tagAttrMatch(tag []byte, want func(name, value []byte) bool) bool {
	cursor := 1
	for cursor < len(tag) && !isHTMLSpace(tag[cursor]) && tag[cursor] != '>' && tag[cursor] != '/' {
		cursor++
	}
	for cursor < len(tag) {
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		if cursor >= len(tag) || tag[cursor] == '>' || tag[cursor] == '/' {
			return false
		}
		nameStart := cursor
		for cursor < len(tag) && !isHTMLSpace(tag[cursor]) && tag[cursor] != '=' && tag[cursor] != '/' && tag[cursor] != '>' {
			cursor++
		}
		name := tag[nameStart:cursor]
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		if cursor >= len(tag) || tag[cursor] != '=' {
			continue
		}
		cursor++
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		value, next := htmlAttrValue(tag, cursor)
		cursor = next
		if want(name, value) {
			return true
		}
	}
	return false
}

func formStartTagRanges(payload []byte) [][2]int {
	var matches [][2]int
	for index := 0; index < len(payload); index++ {
		if payload[index] != '<' || !bytesHasFoldPrefix(payload[index+1:], "form") {
			continue
		}
		afterName := index + len("<form")
		if afterName < len(payload) && isHTMLNameChar(payload[afterName]) {
			continue
		}
		end := htmlTagEnd(payload, afterName)
		if end < 0 {
			break
		}
		matches = append(matches, [2]int{index, end + 1})
		index = end
	}
	return matches
}

func formStartTagHasPostMethod(tag []byte) bool {
	cursor := len("<form")
	for cursor < len(tag) {
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		if cursor >= len(tag) || tag[cursor] == '>' || tag[cursor] == '/' {
			return false
		}
		nameStart := cursor
		for cursor < len(tag) && !isHTMLSpace(tag[cursor]) && tag[cursor] != '=' && tag[cursor] != '/' && tag[cursor] != '>' {
			cursor++
		}
		name := tag[nameStart:cursor]
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		if cursor >= len(tag) || tag[cursor] != '=' {
			continue
		}
		cursor++
		for cursor < len(tag) && isHTMLSpace(tag[cursor]) {
			cursor++
		}
		value, next := htmlAttrValue(tag, cursor)
		cursor = next
		if bytes.EqualFold(name, []byte("method")) && strings.EqualFold(string(value), http.MethodPost) {
			return true
		}
	}
	return false
}

func htmlTagEnd(payload []byte, cursor int) int {
	var quote byte
	for cursor < len(payload) {
		if quote != 0 {
			if payload[cursor] == '\\' {
				cursor += 2
				continue
			}
			if payload[cursor] == quote {
				quote = 0
			}
			cursor++
			continue
		}
		switch payload[cursor] {
		case '\'', '"':
			quote = payload[cursor]
		case '>':
			return cursor
		}
		cursor++
	}
	return -1
}

func htmlAttrValue(tag []byte, cursor int) ([]byte, int) {
	if cursor >= len(tag) {
		return nil, cursor
	}
	if tag[cursor] == '\'' || tag[cursor] == '"' {
		quote := tag[cursor]
		cursor++
		start := cursor
		for cursor < len(tag) && tag[cursor] != quote {
			cursor++
		}
		if cursor < len(tag) {
			return tag[start:cursor], cursor + 1
		}
		return tag[start:], cursor
	}
	start := cursor
	for cursor < len(tag) && !isHTMLSpace(tag[cursor]) && tag[cursor] != '/' && tag[cursor] != '>' {
		cursor++
	}
	return tag[start:cursor], cursor
}

func bytesHasFoldPrefix(value []byte, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	for index := 0; index < len(prefix); index++ {
		if asciiLower(value[index]) != prefix[index] {
			return false
		}
	}
	return true
}

func asciiLower(char byte) byte {
	if char >= 'A' && char <= 'Z' {
		return char + ('a' - 'A')
	}
	return char
}

func isHTMLNameChar(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-'
}

func isHTMLSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

// resetRegions clears the registry. It exists for tests that register renderers
// into the process-global registry.
func resetRegions() {
	regionMu.Lock()
	defer regionMu.Unlock()
	regionRenderers = map[string]regionEntry{}
}
