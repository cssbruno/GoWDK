package lang

import (
	"fmt"
	"sort"
	"strconv"
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
	facts := collectAccessibilityFacts(nodes)
	check := accessibilityCheck{
		file:      file,
		body:      body,
		bodyStart: bodyStart,
		labels:    facts.labels,
	}
	check.reportGlobalFacts(facts)
	check.walk(nodes, 0)
	return check.diagnostics
}

type accessibilityFacts struct {
	ids    map[string][]viewmodel.Element
	labels map[string]bool
	refs   []accessibilityReference
}

type accessibilityReference struct {
	attr   viewmodel.Attr
	target string
	kind   string
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
		case viewmodel.AwaitBlock:
			check.walk(typed.Pending, labelDepth)
			check.walk(typed.Then, labelDepth)
			check.walk(typed.Catch, labelDepth)
		}
	}
}

func (check *accessibilityCheck) reportGlobalFacts(facts accessibilityFacts) {
	for _, id := range sortedElementIDs(facts.ids) {
		elements := facts.ids[id]
		if len(elements) < 2 {
			continue
		}
		for _, element := range elements[1:] {
			check.add(element, "duplicate_literal_id", fmt.Sprintf("literal id %q is declared more than once in this view", id), `Use unique literal id values within one page, component, or layout view.`)
		}
	}
	for _, ref := range facts.refs {
		if strings.TrimSpace(ref.target) == "" {
			continue
		}
		if len(facts.ids[ref.target]) == 0 {
			check.addAttr(ref.attr, "unresolved_accessibility_reference", fmt.Sprintf("%s reference %q does not match a literal id in this view", ref.kind, ref.target), `Point the reference at an existing literal id, or keep dynamic relationships out of static accessibility lint.`)
		}
	}
}

func (check *accessibilityCheck) visitElement(element viewmodel.Element, labelDepth int) {
	name := strings.ToLower(element.Name)
	role := effectiveRole(element)
	check.validateARIA(element, role)
	check.validateInteraction(element, role)
	check.validateFocus(element)
	check.validateAccessibleName(element, role)

	switch name {
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
	check.addOffset(element.Start, element.End, code, message, suggestion)
}

func (check *accessibilityCheck) addAttr(attr viewmodel.Attr, code, message, suggestion string) {
	check.addOffset(attr.Start, attr.End, code, message, suggestion)
}

func (check *accessibilityCheck) addOffset(startOffset, endOffset int, code, message, suggestion string) {
	start := viewBodyOffsetPosition(check.body, check.bodyStart, startOffset)
	end := viewBodyOffsetPosition(check.body, check.bodyStart, endOffset)
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

func collectAccessibilityFacts(nodes []viewmodel.Node) accessibilityFacts {
	facts := accessibilityFacts{
		ids:    map[string][]viewmodel.Element{},
		labels: map[string]bool{},
	}
	walkViewNodes(nodes, func(element viewmodel.Element) {
		if id, ok := literalAttr(element.Attrs, "id"); ok && strings.TrimSpace(id) != "" {
			facts.ids[strings.TrimSpace(id)] = append(facts.ids[strings.TrimSpace(id)], element)
		}
		for _, attr := range element.Attrs {
			if attr.Expression || attr.Boolean || attr.Spread {
				continue
			}
			name := strings.ToLower(attr.Name)
			if strings.EqualFold(element.Name, "label") && name == "for" && strings.TrimSpace(attr.Value) != "" {
				id := strings.TrimSpace(attr.Value)
				facts.labels[id] = true
				facts.refs = append(facts.refs, accessibilityReference{attr: attr, target: id, kind: "<label for>"})
				continue
			}
			if !ariaReferenceAttrs[name] {
				continue
			}
			for _, target := range strings.Fields(attr.Value) {
				facts.refs = append(facts.refs, accessibilityReference{attr: attr, target: target, kind: name})
			}
		}
	})
	return facts
}

func walkViewNodes(nodes []viewmodel.Node, visit func(viewmodel.Element)) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Element:
			visit(typed)
			walkViewNodes(typed.Children, visit)
		case viewmodel.ComponentCall:
			walkViewNodes(typed.Children, visit)
		case viewmodel.AwaitBlock:
			walkViewNodes(typed.Pending, visit)
			walkViewNodes(typed.Then, visit)
			walkViewNodes(typed.Catch, visit)
		}
	}
}

func sortedElementIDs(ids map[string][]viewmodel.Element) []string {
	keys := make([]string, 0, len(ids))
	for id := range ids {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	return keys
}

func (check *accessibilityCheck) validateARIA(element viewmodel.Element, role string) {
	if value, attr, ok := literalAttrWithSource(element.Attrs, "role"); ok && strings.TrimSpace(value) != "" {
		for _, token := range strings.Fields(value) {
			token = strings.ToLower(strings.TrimSpace(token))
			if token != "" && !supportedARIARoles[token] {
				check.addAttr(attr, "invalid_aria_role", fmt.Sprintf("ARIA role %q is not in the supported role list", token), `Use a supported ARIA role or a native semantic element.`)
				break
			}
		}
	}
	for _, attr := range element.Attrs {
		name := strings.ToLower(attr.Name)
		if !strings.HasPrefix(name, "aria-") {
			continue
		}
		if !supportedARIAAttrs[name] {
			check.addAttr(attr, "invalid_aria_attribute", fmt.Sprintf("ARIA attribute %q is not supported", name), `Use a supported aria-* attribute name or remove the attribute.`)
			continue
		}
		if allowedRoles, ok := roleSpecificARIAAttrs[name]; ok && role != "" && !allowedRoles[role] {
			check.addAttr(attr, "aria_role_attribute_mismatch", fmt.Sprintf("%s is not valid for role %q", name, role), `Use an ARIA attribute that matches the element role, or use a native control.`)
		}
	}
}

func (check *accessibilityCheck) validateInteraction(element viewmodel.Element, role string) {
	if !hasEventHandler(element.Attrs, "click") || isNativeInteractive(element) {
		return
	}
	if interactiveRoles[role] && isFocusable(element) && hasKeyboardHandler(element.Attrs) {
		return
	}
	check.add(element, "interactive_semantics_missing", "<"+element.Name+"> has a click handler without complete interactive semantics", `Use a native control, or add an interactive role, focusability, and keyboard event handling.`)
}

func (check *accessibilityCheck) validateFocus(element viewmodel.Element) {
	value, attr, ok := literalAttrWithSource(element.Attrs, "tabindex")
	if ok {
		tabIndex, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			check.addAttr(attr, "invalid_tabindex", fmt.Sprintf("tabindex %q is not an integer", value), `Use tabindex="0" only for custom keyboard targets, tabindex="-1" for programmatic focus, or remove tabindex.`)
		} else if tabIndex > 0 {
			check.addAttr(attr, "positive_tabindex", fmt.Sprintf("positive tabindex %d changes the natural focus order", tabIndex), `Use the document order with tabindex="0", or remove tabindex.`)
		}
	}
	if value, attr, ok := literalAttrWithSource(element.Attrs, "aria-hidden"); ok && strings.EqualFold(strings.TrimSpace(value), "true") && isFocusable(element) {
		check.addAttr(attr, "aria_hidden_focusable", "focusable element is hidden from assistive technology", `Remove aria-hidden from focusable controls, or remove the element from the keyboard focus order.`)
	}
}

func (check *accessibilityCheck) validateAccessibleName(element viewmodel.Element, role string) {
	if controlNeedsAccessibleName(element, role) && strings.TrimSpace(accessibleText(element)) == "" {
		check.add(element, "missing_accessible_name", "<"+element.Name+"> has no accessible name", `Add visible text, aria-label, aria-labelledby, title, alt text, or a form label as appropriate.`)
	}
	if landmarkNeedsAccessibleName(element, role) && !hasExplicitAccessibleName(element.Attrs) {
		check.add(element, "missing_landmark_name", "<"+element.Name+"> landmark has no accessible name", `Add aria-label or aria-labelledby to distinguish this landmark.`)
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
	if strings.EqualFold(element.Name, "input") {
		if value, ok := literalAttr(element.Attrs, "value"); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
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
			if value, ok := literalAttr(typed.Attrs, "aria-hidden"); ok && strings.EqualFold(strings.TrimSpace(value), "true") {
				continue
			}
			if strings.EqualFold(typed.Name, "img") {
				if alt, ok := literalAttr(typed.Attrs, "alt"); ok {
					text = append(text, alt)
				}
			}
			text = append(text, textFromNodes(typed.Children))
		case viewmodel.ComponentCall:
			text = append(text, textFromNodes(typed.Children))
		case viewmodel.AwaitBlock:
			text = append(text, textFromNodes(typed.Pending), textFromNodes(typed.Then), textFromNodes(typed.Catch))
		}
	}
	return strings.Join(text, " ")
}

func effectiveRole(element viewmodel.Element) string {
	if value, ok := literalAttr(element.Attrs, "role"); ok {
		fields := strings.Fields(value)
		if len(fields) > 0 {
			return strings.ToLower(fields[0])
		}
	}
	return nativeRole(element)
}

func nativeRole(element viewmodel.Element) string {
	switch strings.ToLower(element.Name) {
	case "a":
		if linkHasHref(element.Attrs) {
			return "link"
		}
	case "button":
		return "button"
	case "input":
		typ := inputType(element)
		switch typ {
		case "button", "submit", "reset", "image":
			return "button"
		case "checkbox":
			return "checkbox"
		case "radio":
			return "radio"
		case "range":
			return "slider"
		default:
			return "textbox"
		}
	case "select":
		return "combobox"
	case "textarea":
		return "textbox"
	case "summary":
		return "button"
	}
	return ""
}

func inputType(element viewmodel.Element) string {
	typ, ok := literalAttr(element.Attrs, "type")
	if !ok || strings.TrimSpace(typ) == "" {
		return "text"
	}
	return strings.ToLower(strings.TrimSpace(typ))
}

func hasEventHandler(attrs []viewmodel.Attr, event string) bool {
	prefix := "g:on:" + strings.ToLower(event)
	for _, attr := range attrs {
		name := strings.ToLower(attr.Name)
		if name == prefix || strings.HasPrefix(name, prefix+".") || strings.HasPrefix(name, prefix+"(") {
			return true
		}
	}
	return false
}

func hasKeyboardHandler(attrs []viewmodel.Attr) bool {
	for _, event := range []string{"keydown", "keyup", "keypress"} {
		if hasEventHandler(attrs, event) {
			return true
		}
	}
	return false
}

func isNativeInteractive(element viewmodel.Element) bool {
	switch strings.ToLower(element.Name) {
	case "button", "select", "textarea", "summary":
		return true
	case "input":
		return !strings.EqualFold(inputType(element), "hidden")
	case "a":
		return linkHasHref(element.Attrs)
	}
	return false
}

func isFocusable(element viewmodel.Element) bool {
	if hasAttr(element.Attrs, "disabled") {
		return false
	}
	if value, ok := literalAttr(element.Attrs, "tabindex"); ok {
		tabIndex, err := strconv.Atoi(strings.TrimSpace(value))
		return err == nil && tabIndex >= 0
	}
	return isNativeInteractive(element)
}

func controlNeedsAccessibleName(element viewmodel.Element, role string) bool {
	name := strings.ToLower(element.Name)
	if name == "button" {
		return true
	}
	if name == "a" && linkHasHref(element.Attrs) {
		return true
	}
	if name == "input" {
		switch inputType(element) {
		case "button", "submit", "reset", "image":
			return true
		}
	}
	return interactiveRoles[role] && !strings.EqualFold(role, nativeRole(element))
}

func landmarkNeedsAccessibleName(element viewmodel.Element, role string) bool {
	if role == "" {
		return false
	}
	return namedLandmarkRoles[role]
}

func hasExplicitAccessibleName(attrs []viewmodel.Attr) bool {
	return hasNonEmptyLiteralAttr(attrs, "aria-label") || hasNonEmptyLiteralAttr(attrs, "aria-labelledby")
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

func hasAttr(attrs []viewmodel.Attr, name string) bool {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name, name) {
			return true
		}
	}
	return false
}

func hasNonEmptyLiteralAttr(attrs []viewmodel.Attr, name string) bool {
	value, ok := literalAttr(attrs, name)
	return ok && strings.TrimSpace(value) != ""
}

func literalAttr(attrs []viewmodel.Attr, name string) (string, bool) {
	value, _, ok := literalAttrWithSource(attrs, name)
	return value, ok
}

func literalAttrWithSource(attrs []viewmodel.Attr, name string) (string, viewmodel.Attr, bool) {
	for _, attr := range attrs {
		if !strings.EqualFold(attr.Name, name) || attr.Expression || attr.Boolean || attr.Spread {
			continue
		}
		return attr.Value, attr, true
	}
	return "", viewmodel.Attr{}, false
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

var ariaReferenceAttrs = map[string]bool{
	"aria-activedescendant": true,
	"aria-controls":         true,
	"aria-describedby":      true,
	"aria-details":          true,
	"aria-errormessage":     true,
	"aria-flowto":           true,
	"aria-labelledby":       true,
	"aria-owns":             true,
}

var supportedARIAAttrs = map[string]bool{
	"aria-activedescendant":       true,
	"aria-atomic":                 true,
	"aria-autocomplete":           true,
	"aria-braillelabel":           true,
	"aria-brailleroledescription": true,
	"aria-busy":                   true,
	"aria-checked":                true,
	"aria-colcount":               true,
	"aria-colindex":               true,
	"aria-colindextext":           true,
	"aria-colspan":                true,
	"aria-controls":               true,
	"aria-current":                true,
	"aria-describedby":            true,
	"aria-description":            true,
	"aria-details":                true,
	"aria-disabled":               true,
	"aria-dropeffect":             true,
	"aria-errormessage":           true,
	"aria-expanded":               true,
	"aria-flowto":                 true,
	"aria-grabbed":                true,
	"aria-haspopup":               true,
	"aria-hidden":                 true,
	"aria-invalid":                true,
	"aria-keyshortcuts":           true,
	"aria-label":                  true,
	"aria-labelledby":             true,
	"aria-level":                  true,
	"aria-live":                   true,
	"aria-modal":                  true,
	"aria-multiline":              true,
	"aria-multiselectable":        true,
	"aria-orientation":            true,
	"aria-owns":                   true,
	"aria-placeholder":            true,
	"aria-posinset":               true,
	"aria-pressed":                true,
	"aria-readonly":               true,
	"aria-relevant":               true,
	"aria-required":               true,
	"aria-roledescription":        true,
	"aria-rowcount":               true,
	"aria-rowindex":               true,
	"aria-rowindextext":           true,
	"aria-rowspan":                true,
	"aria-selected":               true,
	"aria-setsize":                true,
	"aria-sort":                   true,
	"aria-valuemax":               true,
	"aria-valuemin":               true,
	"aria-valuenow":               true,
	"aria-valuetext":              true,
}

var supportedARIARoles = map[string]bool{
	"alert": true, "alertdialog": true, "application": true, "article": true,
	"banner": true, "blockquote": true, "button": true, "caption": true,
	"cell": true, "checkbox": true, "code": true, "columnheader": true,
	"combobox": true, "complementary": true, "contentinfo": true,
	"definition": true, "deletion": true, "dialog": true, "directory": true,
	"document": true, "emphasis": true, "feed": true, "figure": true,
	"form": true, "generic": true, "grid": true, "gridcell": true,
	"group": true, "heading": true, "img": true, "insertion": true,
	"link": true, "list": true, "listbox": true, "listitem": true,
	"log": true, "main": true, "marquee": true, "math": true,
	"menu": true, "menubar": true, "menuitem": true,
	"menuitemcheckbox": true, "menuitemradio": true, "meter": true,
	"navigation": true, "none": true, "note": true, "option": true,
	"paragraph": true, "presentation": true, "progressbar": true,
	"radio": true, "radiogroup": true, "region": true, "row": true,
	"rowgroup": true, "rowheader": true, "scrollbar": true, "search": true,
	"searchbox": true, "separator": true, "slider": true, "spinbutton": true,
	"status": true, "strong": true, "subscript": true, "superscript": true,
	"switch": true, "tab": true, "table": true, "tablist": true,
	"tabpanel": true, "term": true, "textbox": true, "time": true,
	"timer": true, "toolbar": true, "tooltip": true, "tree": true,
	"treegrid": true, "treeitem": true,
}

var roleSpecificARIAAttrs = map[string]map[string]bool{
	"aria-checked": {
		"checkbox": true, "menuitemcheckbox": true, "menuitemradio": true,
		"radio": true, "switch": true,
	},
	"aria-pressed": {
		"button": true,
	},
	"aria-selected": {
		"gridcell": true, "option": true, "row": true, "tab": true,
	},
}

var interactiveRoles = map[string]bool{
	"button": true, "checkbox": true, "combobox": true, "link": true,
	"menuitem": true, "menuitemcheckbox": true, "menuitemradio": true,
	"option": true, "radio": true, "searchbox": true, "slider": true,
	"spinbutton": true, "switch": true, "tab": true, "textbox": true,
	"treeitem": true,
}

var namedLandmarkRoles = map[string]bool{
	"complementary": true,
	"form":          true,
	"navigation":    true,
	"region":        true,
	"search":        true,
}
