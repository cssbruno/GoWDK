package lang

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

func accessibilityDiagnostics(ir gwdkir.Program) Diagnostics {
	var diagnostics Diagnostics
	for _, page := range ir.Pages {
		diagnostics = append(diagnostics, viewAccessibilityDiagnostics(page.Source, page.Blocks)...)
	}
	for _, component := range ir.Components {
		diagnostics = append(diagnostics, viewAccessibilityDiagnostics(component.Source, component.Blocks)...)
	}
	for _, layout := range ir.Layouts {
		diagnostics = append(diagnostics, viewAccessibilityDiagnostics(layout.Source, layout.Blocks)...)
	}
	return diagnostics
}

func viewAccessibilityDiagnostics(file string, blocks gwdkir.Blocks) Diagnostics {
	if !blocks.View || blocks.ViewBody == "" || blocks.Spans.ViewBodyStart.Line <= 0 {
		return nil
	}
	nodes := blocks.ViewNodes
	if len(nodes) == 0 {
		var err error
		nodes, err = viewparse.Parse(blocks.ViewBody)
		if err != nil {
			return nil
		}
	}
	return accessibilityDiagnosticsForNodes(file, blocks.ViewBody, blocks.Spans.ViewBodyStart, nodes)
}

func accessibilityDiagnosticsForNodes(file string, body string, bodyStart source.SourcePosition, nodes []viewmodel.Node) Diagnostics {
	labels := labelForIDs(nodes)
	check := accessibilityCheck{
		file:      file,
		body:      body,
		bodyStart: bodyStart,
		labels:    labels,
	}
	check.walk(nodes, 0)
	return check.diagnostics
}

type accessibilityCheck struct {
	file        string
	body        string
	bodyStart   source.SourcePosition
	labels      map[string]bool
	lastHeading int
	diagnostics Diagnostics
}

func (check *accessibilityCheck) walk(nodes []viewmodel.Node, labelDepth int) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Element:
			nextLabelDepth := labelDepth
			if strings.EqualFold(typed.Name, "label") {
				nextLabelDepth++
			}
			check.visitElement(typed, labelDepth)
			check.walk(typed.Children, nextLabelDepth)
		case viewmodel.ComponentCall:
			check.walk(typed.Children, labelDepth)
		}
	}
}

func (check *accessibilityCheck) visitElement(element viewmodel.Element, labelDepth int) {
	switch strings.ToLower(element.Name) {
	case "img":
		if !imageHasAlt(element.Attrs) {
			check.add(element, "missing_img_alt", "<img> is missing explicit alt text", `Add alt="..." for informative images or alt="" for decorative images.`)
		}
	case "input", "select", "textarea":
		if !formControlHasLabel(element, labelDepth, check.labels) {
			check.add(element, "missing_form_label", "<"+element.Name+"> is missing an accessible label", `Add a <label for="...">, wrap the control in <label>, or add aria-label/aria-labelledby.`)
		}
	case "a":
		if linkHasHref(element.Attrs) && strings.TrimSpace(accessibleText(element)) == "" {
			check.add(element, "empty_link_text", "<a> with href has no accessible text", `Add link text or an accessible label.`)
		}
	case "button":
		if !hasLiteralAttr(element.Attrs, "type") {
			check.add(element, "missing_button_type", "<button> is missing an explicit type", `Add type="button" for ordinary buttons or type="submit" for form submit buttons.`)
		}
	}
	if level := headingLevel(element.Name); level > 0 {
		if check.lastHeading > 0 && level > check.lastHeading+1 {
			check.add(element, "heading_order_skip", fmt.Sprintf("<h%d> skips from previous <h%d>", level, check.lastHeading), `Use sequential heading levels so assistive technology can follow the document outline.`)
		}
		check.lastHeading = level
	}
}

func (check *accessibilityCheck) add(element viewmodel.Element, code, message, suggestion string) {
	start := viewBodyOffsetPosition(check.body, check.bodyStart, element.Start)
	end := viewBodyOffsetPosition(check.body, check.bodyStart, element.End)
	check.diagnostics = append(check.diagnostics, Diagnostic{
		File:       check.file,
		Code:       code,
		Pos:        start,
		Range:      sourceRange(start, end),
		Severity:   "warning",
		Message:    message,
		Suggestion: suggestion,
	})
}

func labelForIDs(nodes []viewmodel.Node) map[string]bool {
	labels := map[string]bool{}
	walkViewNodes(nodes, func(element viewmodel.Element) {
		if !strings.EqualFold(element.Name, "label") {
			return
		}
		id, ok := literalAttr(element.Attrs, "for")
		if ok && strings.TrimSpace(id) != "" {
			labels[strings.TrimSpace(id)] = true
		}
	})
	return labels
}

func walkViewNodes(nodes []viewmodel.Node, visit func(viewmodel.Element)) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Element:
			visit(typed)
			walkViewNodes(typed.Children, visit)
		case viewmodel.ComponentCall:
			walkViewNodes(typed.Children, visit)
		}
	}
}

func imageHasAlt(attrs []viewmodel.Attr) bool {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name, "alt") && !attr.Boolean {
			return true
		}
	}
	return false
}

func formControlHasLabel(element viewmodel.Element, labelDepth int, labels map[string]bool) bool {
	if strings.EqualFold(element.Name, "input") {
		typ, ok := literalAttr(element.Attrs, "type")
		if ok && strings.EqualFold(strings.TrimSpace(typ), "hidden") {
			return true
		}
	}
	if labelDepth > 0 || hasNonEmptyLiteralAttr(element.Attrs, "aria-label") || hasNonEmptyLiteralAttr(element.Attrs, "aria-labelledby") {
		return true
	}
	id, ok := literalAttr(element.Attrs, "id")
	return ok && labels[strings.TrimSpace(id)]
}

func linkHasHref(attrs []viewmodel.Attr) bool {
	_, ok := literalAttr(attrs, "href")
	return ok
}

func accessibleText(element viewmodel.Element) string {
	for _, attrName := range []string{"aria-label", "title"} {
		if value, ok := literalAttr(element.Attrs, attrName); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return textFromNodes(element.Children)
}

func textFromNodes(nodes []viewmodel.Node) string {
	var text []string
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Text:
			text = append(text, typed.Value)
		case viewmodel.Element:
			if strings.EqualFold(typed.Name, "img") {
				if alt, ok := literalAttr(typed.Attrs, "alt"); ok {
					text = append(text, alt)
				}
			}
			text = append(text, textFromNodes(typed.Children))
		case viewmodel.ComponentCall:
			text = append(text, textFromNodes(typed.Children))
		}
	}
	return strings.Join(text, " ")
}

func headingLevel(name string) int {
	if len(name) != 2 || name[0] != 'h' || name[1] < '1' || name[1] > '6' {
		return 0
	}
	return int(name[1] - '0')
}

func hasLiteralAttr(attrs []viewmodel.Attr, name string) bool {
	_, ok := literalAttr(attrs, name)
	return ok
}

func hasNonEmptyLiteralAttr(attrs []viewmodel.Attr, name string) bool {
	value, ok := literalAttr(attrs, name)
	return ok && strings.TrimSpace(value) != ""
}

func literalAttr(attrs []viewmodel.Attr, name string) (string, bool) {
	for _, attr := range attrs {
		if !strings.EqualFold(attr.Name, name) || attr.Expression || attr.Boolean {
			continue
		}
		return attr.Value, true
	}
	return "", false
}

func viewBodyOffsetPosition(body string, start source.SourcePosition, offset int) Position {
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
