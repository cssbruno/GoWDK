package parser

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// layoutIDFromPath derives a layout's identity from its file name:
// `layouts/root.layout.gwdk` -> `root`.
func layoutIDFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".layout.gwdk")
}

// ParseLayout extracts layout metadata and top-level block declarations.
// Layout identity is derived from the file name (`root.layout.gwdk` -> `root`);
// any `layout` metadata declaration declares parent layouts this layout nests within.
func ParseLayout(path string, src []byte) (gwdkir.Layout, error) {
	ast, err := ParseSyntax(src)
	if err != nil {
		return gwdkir.Layout{}, err
	}
	return lowerLayoutSyntax(path, src, ast)
}

func lowerLayoutSyntax(path string, src []byte, ast gwdkast.File) (gwdkir.Layout, error) {
	layout := gwdkir.Layout{ID: layoutIDFromPath(path)}
	if ast.Package != nil {
		layout.Package = ast.Package.Name
		layout.PackageSpan = ast.Package.Span
	}
	layout.Uses = lowerSyntaxUses(ast.Uses)
	for _, metadata := range ast.Metadata {
		lineNumber := metadata.Span.Start.Line
		value := strings.TrimSpace(metadata.Value)
		switch metadata.Name {
		case "layout":
			if value == "" {
				return gwdkir.Layout{}, fmt.Errorf("line %d: layout requires a value", lineNumber)
			}
			if (layout.Span == source.SourceSpan{}) {
				layout.Span = metadata.Span
			}
		case "error":
		default:
			return gwdkir.Layout{}, lineDiagnosticError(DiagnosticUnsupportedLayoutMetadata, lineNumber, sourceLineText(src, lineNumber), "unsupported metadata %s", metadata.Name)
		}
	}
	for _, ref := range ast.Layouts {
		layout.Layouts = append(layout.Layouts, ref.ID)
		layout.LayoutSpans = append(layout.LayoutSpans, source.NamedSpan{Name: ref.ID, Span: ref.Span})
	}
	if ast.ErrorPage != nil {
		layout.ErrorPage = ast.ErrorPage.Path
		layout.ErrorPageSpan = ast.ErrorPage.Span
	}
	for _, block := range ast.Blocks {
		if err := applyLayoutSyntaxBlock(src, &layout, block); err != nil {
			return gwdkir.Layout{}, err
		}
	}
	if layout.ID == "" {
		return gwdkir.Layout{}, fmt.Errorf("layout file name must be <name>.layout.gwdk")
	}
	return layout, nil
}

func applyLayoutSyntaxBlock(src []byte, layout *gwdkir.Layout, block gwdkast.Block) error {
	switch block.Kind {
	case "view":
		layout.Blocks.View = true
		layout.Blocks.ViewBody = block.Body
		layout.Blocks.ViewNodes = append(layout.Blocks.ViewNodes[:0], block.View...)
		layout.Blocks.Spans.View = block.Span
		layout.Blocks.Spans.ViewBodyStart = block.BodyStart
	case "style":
		if layout.Blocks.Style {
			return fmt.Errorf("line %d: layout declares multiple style blocks", block.Span.Start.Line)
		}
		layout.Blocks.StyleBody = block.StyleBody
		layout.Blocks.Style = strings.TrimSpace(block.StyleBody) != ""
	case "go":
		layout.Blocks.GoBlocks = append(layout.Blocks.GoBlocks, gwdkir.GoBlock{
			Target: block.Name,
			Body:   block.Body,
			Span:   block.Span,
		})
		layout.Blocks.Spans.GoBlocks = append(layout.Blocks.Spans.GoBlocks, source.NamedSpan{Name: block.Name, Span: block.Span})
	default:
		lineNumber := block.Span.Start.Line
		return lineDiagnosticError(DiagnosticUnsupportedTopLevelBlock, lineNumber, sourceLineText(src, lineNumber), "unsupported top-level block %q", block.Kind)
	}
	return nil
}
