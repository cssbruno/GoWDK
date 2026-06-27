package buildgen

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

type jsSourceMap struct {
	Version        int      `json:"version"`
	File           string   `json:"file"`
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent,omitempty"`
	Names          []string `json:"names"`
	Mappings       string   `json:"mappings"`
}

func islandJSSourceMap(component gwdkir.Component, generatedSource string) []byte {
	source := component.Source
	if source == "" {
		source = "components/" + component.Name + ".cmp.gwdk"
	}
	content := componentSourceMapContent(component)
	payload, err := json.MarshalIndent(jsSourceMap{
		Version:        3,
		File:           path.Base(islandJSAssetPath(component.Package, component.Name)),
		Sources:        []string{source},
		SourcesContent: []string{content},
		Names:          []string{},
		Mappings:       sourceMapMappings(component, generatedSource),
	}, "", "  ")
	if err != nil {
		return []byte(`{"version":3,"file":"` + path.Base(islandJSAssetPath(component.Package, component.Name)) + `","sources":[],"names":[],"mappings":""}`)
	}
	return append(payload, '\n')
}

type sourceMapAnchor struct {
	generatedLine int
	sourceLine    int
	sourceColumn  int
}

func sourceMapMappings(component gwdkir.Component, generatedSource string) string {
	anchors := sourceMapAnchors(component, generatedSource)
	if len(anchors) == 0 {
		return ""
	}
	byLine := map[int]sourceMapAnchor{}
	maxLine := 0
	for _, anchor := range anchors {
		if anchor.generatedLine <= 0 || anchor.sourceLine <= 0 || anchor.sourceColumn <= 0 {
			continue
		}
		if _, exists := byLine[anchor.generatedLine]; exists {
			continue
		}
		byLine[anchor.generatedLine] = anchor
		if anchor.generatedLine > maxLine {
			maxLine = anchor.generatedLine
		}
	}
	if maxLine == 0 {
		return ""
	}

	mappings := make([]string, 0, maxLine)
	previousSourceIndex := 0
	previousSourceLine := 0
	previousSourceColumn := 0
	for line := 1; line <= maxLine; line++ {
		anchor, ok := byLine[line]
		if !ok {
			mappings = append(mappings, "")
			continue
		}
		sourceLine := anchor.sourceLine - 1
		sourceColumn := anchor.sourceColumn - 1
		mappings = append(mappings,
			sourceMapVLQ(0)+
				sourceMapVLQ(0-previousSourceIndex)+
				sourceMapVLQ(sourceLine-previousSourceLine)+
				sourceMapVLQ(sourceColumn-previousSourceColumn),
		)
		previousSourceIndex = 0
		previousSourceLine = sourceLine
		previousSourceColumn = sourceColumn
	}
	return strings.Join(mappings, ";")
}

func sourceMapAnchors(component gwdkir.Component, generatedSource string) []sourceMapAnchor {
	name := componentAssetName(component.Name)
	componentSpan := firstSourceSpan(component.Span, component.Blocks.Spans.Client, component.Blocks.Spans.View)
	clientSpan := firstSourceSpan(component.Blocks.Spans.Client, componentSpan)
	viewSpan := firstSourceSpan(component.Blocks.Spans.View, componentSpan)
	var anchors []sourceMapAnchor
	for index, line := range strings.Split(generatedSource, "\n") {
		lineNumber := index + 1
		switch {
		case strings.Contains(line, "const component = "):
			anchors = appendSourceMapAnchor(anchors, lineNumber, componentSpan)
		case sourceMapLineBelongsToClient(line, name):
			anchors = appendSourceMapAnchor(anchors, lineNumber, clientSpan)
		case sourceMapLineBelongsToView(line):
			anchors = appendSourceMapAnchor(anchors, lineNumber, viewSpan)
		}
	}
	return anchors
}

func sourceMapLineBelongsToClient(line, componentAssetName string) bool {
	return strings.Contains(line, "async function mount"+componentAssetName+"Island(scope)") ||
		strings.Contains(line, "async function applyExpression(") ||
		strings.Contains(line, "async function applyStatements(") ||
		strings.Contains(line, "function recomputeComputed(")
}

func sourceMapLineBelongsToView(line string) bool {
	return strings.Contains(line, "const bindingTable = Object.freeze(") ||
		strings.Contains(line, "function collectBindings(") ||
		strings.Contains(line, "function renderConditionals(") ||
		strings.Contains(line, "function renderListLoops(") ||
		strings.Contains(line, "function updateTextBindings(") ||
		strings.Contains(line, "function updateValueBindings(") ||
		strings.Contains(line, "function updateCheckedBindings(") ||
		strings.Contains(line, "function updateClassBindings(") ||
		strings.Contains(line, "function updateStyleBindings(") ||
		strings.Contains(line, "function updateAttrBindings(") ||
		strings.Contains(line, "function updateBindings(") ||
		strings.Contains(line, "function render(root, state, helpers, bindings)")
}

func appendSourceMapAnchor(anchors []sourceMapAnchor, generatedLine int, span source.SourceSpan) []sourceMapAnchor {
	if span.Start.Line <= 0 || span.Start.Column <= 0 {
		return anchors
	}
	return append(anchors, sourceMapAnchor{
		generatedLine: generatedLine,
		sourceLine:    span.Start.Line,
		sourceColumn:  span.Start.Column,
	})
}

func firstSourceSpan(spans ...source.SourceSpan) source.SourceSpan {
	for _, span := range spans {
		if span.Start.Line > 0 && span.Start.Column > 0 {
			return span
		}
	}
	return source.SourceSpan{}
}

const sourceMapBase64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func sourceMapVLQ(value int) string {
	vlq := value << 1
	if value < 0 {
		vlq = ((-value) << 1) + 1
	}
	out := make([]byte, 0, 2)
	for {
		digit := vlq & 31
		vlq >>= 5
		if vlq > 0 {
			digit |= 32
		}
		out = append(out, sourceMapBase64[digit])
		if vlq == 0 {
			break
		}
	}
	return string(out)
}

func componentSourceMapContent(component gwdkir.Component) string {
	if component.Blocks.ClientBody == "" {
		return "view {\n" + component.Blocks.ViewBody + "\n}\n"
	}
	return "client {\n" + component.Blocks.ClientBody + "\n}\n\nview {\n" + component.Blocks.ViewBody + "\n}\n"
}
