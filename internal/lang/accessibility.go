package lang

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

func accessibilityDiagnostics(app manifest.Manifest) Diagnostics {
	var diagnostics Diagnostics
	for _, page := range app.Pages {
		diagnostics = append(diagnostics, viewAccessibilityDiagnostics(page.Source, page.Blocks)...)
	}
	for _, component := range app.Components {
		diagnostics = append(diagnostics, viewAccessibilityDiagnostics(component.Source, component.Blocks)...)
	}
	for _, layout := range app.Layouts {
		diagnostics = append(diagnostics, viewAccessibilityDiagnostics(layout.Source, layout.Blocks)...)
	}
	return diagnostics
}

func viewAccessibilityDiagnostics(file string, blocks manifest.Blocks) Diagnostics {
	if !blocks.View || blocks.ViewBody == "" || blocks.Spans.ViewBodyStart.Line <= 0 {
		return nil
	}
	if !strings.Contains(blocks.ViewBody, "<img") {
		return nil
	}
	nodes, err := view.Parse(blocks.ViewBody)
	if err != nil {
		return nil
	}
	return imageAltDiagnostics(file, blocks.ViewBody, blocks.Spans.ViewBodyStart, nodes)
}

func imageAltDiagnostics(file string, body string, bodyStart manifest.SourcePosition, nodes []view.Node) Diagnostics {
	var diagnostics Diagnostics
	walkViewNodes(nodes, func(element view.Element) {
		if element.Name != "img" || imageHasAlt(element.Attrs) {
			return
		}
		start := viewBodyOffsetPosition(body, bodyStart, element.Start)
		end := viewBodyOffsetPosition(body, bodyStart, element.End)
		diagnostics = append(diagnostics, Diagnostic{
			File:       file,
			Code:       "missing_img_alt",
			Pos:        start,
			Range:      sourceRange(start, end),
			Severity:   "warning",
			Message:    "<img> is missing explicit alt text",
			Suggestion: `Add alt="..." for informative images or alt="" for decorative images.`,
		})
	})
	return diagnostics
}

func walkViewNodes(nodes []view.Node, visit func(view.Element)) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Element:
			visit(typed)
			walkViewNodes(typed.Children, visit)
		case view.ComponentCall:
			walkViewNodes(typed.Children, visit)
		}
	}
}

func imageHasAlt(attrs []view.Attr) bool {
	for _, attr := range attrs {
		if attr.Name == "alt" && !attr.Boolean {
			return true
		}
	}
	return false
}

func viewBodyOffsetPosition(body string, start manifest.SourcePosition, offset int) Position {
	if start.Column <= 0 {
		start.Column = 1
	}
	if start.Line <= 0 {
		start.Line = 1
	}
	if offset < 0 {
		offset = 0
	}
	runes := []rune(body)
	if offset > len(runes) {
		offset = len(runes)
	}
	line := start.Line
	column := start.Column
	for _, char := range runes[:offset] {
		if char == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return Position{Line: line, Column: column}
}
