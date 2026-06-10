package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// ParseLayout extracts layout metadata and top-level block declarations.
func ParseLayout(src []byte) (gwdkir.Layout, error) {
	var layout gwdkir.Layout
	var viewBody []string
	inView := false
	var styleBody []string
	inStyle := false
	styleDepth := 0
	var goBlockBody []string
	inGoBlock := false
	goBlockDepth := 0
	goBlockTarget := ""
	seenGoBlocks := map[string]source.SourceSpan{}
	seenDeclaration := false
	var blockScanner braceScanner

	scanner := bufio.NewScanner(bytes.NewReader(src))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if inGoBlock {
			if line == "}" && !blockScanner.inMultiline() {
				goBlockDepth--
				if goBlockDepth == 0 {
					layout.Blocks.GoBlocks = append(layout.Blocks.GoBlocks, gwdkir.GoBlock{
						Target: goBlockTarget,
						Body:   strings.TrimSpace(strings.Join(goBlockBody, "\n")),
						Span:   seenGoBlocks[goBlockTarget],
					})
					layout.Blocks.Spans.GoBlocks = append(layout.Blocks.Spans.GoBlocks, source.NamedSpan{Name: goBlockTarget, Span: seenGoBlocks[goBlockTarget]})
					inGoBlock = false
					goBlockBody = nil
					goBlockDepth = 0
					goBlockTarget = ""
					continue
				}
				goBlockBody = append(goBlockBody, rawLine)
				continue
			}
			goBlockDepth += blockScanner.delta(rawLine)
			if goBlockDepth < 1 {
				return gwdkir.Layout{}, fmt.Errorf("line %d: go block closed unexpectedly", lineNumber)
			}
			goBlockBody = append(goBlockBody, rawLine)
			continue
		}
		if inStyle {
			styleDepth += blockScanner.delta(rawLine)
			if styleDepth < 0 {
				return gwdkir.Layout{}, fmt.Errorf("line %d: style block closed unexpectedly", lineNumber)
			}
			if styleDepth == 0 {
				layout.Blocks.StyleBody = strings.TrimSpace(strings.Join(styleBody, "\n"))
				layout.Blocks.Style = layout.Blocks.StyleBody != ""
				inStyle = false
				styleBody = nil
				styleDepth = 0
				continue
			}
			styleBody = append(styleBody, rawLine)
			continue
		}
		if inView {
			if line == "style {" {
				return gwdkir.Layout{}, fmt.Errorf("line %d: style block must be outside view {}", lineNumber)
			}
			if line == "}" {
				layout.Blocks.View = true
				layout.Blocks.ViewBody = strings.TrimSpace(strings.Join(viewBody, "\n"))
				layout.Blocks.Spans.ViewBodyStart = sourceBodyStart(viewBody, lineNumber-len(viewBody))
				inView = false
				viewBody = nil
				continue
			}
			viewBody = append(viewBody, rawLine)
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if match := packagePattern.FindStringSubmatch(line); match != nil {
			if seenDeclaration {
				return gwdkir.Layout{}, fmt.Errorf("line %d: package declaration must be the first non-comment declaration", lineNumber)
			}
			layout.Package = match[1]
			layout.PackageSpan = sourceLineSpan(lineNumber, rawLine)
			seenDeclaration = true
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return gwdkir.Layout{}, fmt.Errorf("line %d: malformed package declaration %q", lineNumber, line)
		}
		seenDeclaration = true

		if strings.HasPrefix(line, "@") {
			match := annotationPattern.FindStringSubmatch(line)
			if match == nil {
				return gwdkir.Layout{}, fmt.Errorf("line %d: malformed annotation %q", lineNumber, line)
			}
			if err := applyLayoutAnnotation(&layout, match[1], match[2], lineNumber, rawLine); err != nil {
				return gwdkir.Layout{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			continue
		}

		if match := usePattern.FindStringSubmatch(line); match != nil {
			layout.Uses = append(layout.Uses, gwdkir.Use{
				Alias:   match[1],
				Package: match[2],
				Span:    sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedUse(line) {
			return gwdkir.Layout{}, fmt.Errorf("line %d: malformed use %q", lineNumber, line)
		}

		switch line {
		case "view {":
			layout.Blocks.Spans.View = sourceLineSpan(lineNumber, rawLine)
			inView = true
			continue
		case "style {":
			if layout.Blocks.Style {
				return gwdkir.Layout{}, fmt.Errorf("line %d: layout declares multiple style blocks", lineNumber)
			}
			layout.Blocks.Style = true
			inStyle = true
			styleDepth = 1
			blockScanner = braceScanner{lang: braceLangCSS}
			continue
		case "go {":
			span := sourceLineSpan(lineNumber, rawLine)
			if first, exists := seenGoBlocks[""]; exists {
				return gwdkir.Layout{}, fmt.Errorf("line %d: duplicate go block; first declared on line %d", lineNumber, first.Start.Line)
			}
			seenGoBlocks[""] = span
			inGoBlock = true
			goBlockDepth = 1
			goBlockTarget = ""
			blockScanner = braceScanner{lang: braceLangGo}
			continue
		}
		if match := goBlockPattern.FindStringSubmatch(line); match != nil {
			target := strings.TrimSpace(match[1])
			span := sourceLineSpan(lineNumber, rawLine)
			if first, exists := seenGoBlocks[target]; exists {
				label := "go"
				if target != "" {
					label = "go " + target
				}
				return gwdkir.Layout{}, fmt.Errorf("line %d: duplicate %s block; first declared on line %d", lineNumber, label, first.Start.Line)
			}
			seenGoBlocks[target] = span
			inGoBlock = true
			goBlockDepth = 1
			goBlockTarget = target
			blockScanner = braceScanner{lang: braceLangGo}
			continue
		}

		if name := unsupportedTopLevelBlockName(line); name != "" {
			return gwdkir.Layout{}, fmt.Errorf("line %d: unsupported top-level block %q", lineNumber, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return gwdkir.Layout{}, err
	}
	if inView {
		return gwdkir.Layout{}, fmt.Errorf("view block missing closing }")
	}
	if inStyle {
		return gwdkir.Layout{}, fmt.Errorf("style block missing closing }")
	}
	if inGoBlock {
		return gwdkir.Layout{}, fmt.Errorf("go block missing closing }")
	}
	if layout.ID == "" {
		return gwdkir.Layout{}, fmt.Errorf("missing @layout")
	}
	return layout, nil
}
