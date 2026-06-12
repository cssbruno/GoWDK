package parser

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func lowerComponentSyntax(src []byte, ast gwdkast.File) (gwdkir.Component, error) {
	var component gwdkir.Component
	if ast.Package != nil {
		component.Package = ast.Package.Name
		component.PackageSpan = ast.Package.Span
	}
	component.Imports = lowerSyntaxImports(ast.Imports)
	component.Uses = lowerSyntaxUses(ast.Uses)
	if err := lowerComponentSyntaxMetadata(ast, &component); err != nil {
		return gwdkir.Component{}, err
	}
	if ast.PropsType != nil {
		component.PropsType = lowerSyntaxGoTypeRef(*ast.PropsType)
	}
	if ast.State != nil {
		component.State = gwdkir.StateContract{
			Type: lowerSyntaxGoTypeRef(ast.State.Type),
			Init: lowerSyntaxGoFuncRef(ast.State.Init),
			Span: ast.State.Span,
		}
	}
	if ast.WASM != nil {
		component.WASM = gwdkir.WASMContract{Package: trimQuotes(ast.WASM.Package), Span: ast.WASM.Span}
	}
	for _, block := range ast.Blocks {
		if err := applyComponentSyntaxBlock(src, &component, block); err != nil {
			return gwdkir.Component{}, err
		}
	}
	if component.Name == "" {
		return gwdkir.Component{}, fmt.Errorf("missing component")
	}
	return component, nil
}

func lowerComponentSyntaxMetadata(ast gwdkast.File, component *gwdkir.Component) error {
	for _, metadata := range ast.Metadata {
		lineNumber := metadata.Span.Start.Line
		value := strings.TrimSpace(metadata.Value)
		switch metadata.Name {
		case "component":
			if value == "" {
				return fmt.Errorf("line %d: component requires a value", lineNumber)
			}
			if ast.Component != nil {
				component.Name = ast.Component.Name
				component.Span = ast.Component.Span
			}
		case "wasm":
			if value == "" {
				return fmt.Errorf("line %d: wasm requires a package path", lineNumber)
			}
		case "css":
			if value == "" {
				return fmt.Errorf("line %d: css requires a value", lineNumber)
			}
		case "asset":
			if value == "" {
				return fmt.Errorf("line %d: asset requires a value", lineNumber)
			}
		default:
			return withLine(lineNumber, fmt.Errorf("unsupported metadata %s", metadata.Name))
		}
	}
	component.CSS, component.Spans.CSS = lowerSyntaxAssetRefs(ast.CSS)
	component.Assets, component.Spans.Assets = lowerSyntaxAssetRefs(ast.Assets)
	component.JS, component.InlineJS, component.Spans.JS, component.Spans.InlineJS = lowerSyntaxScripts(ast.JS)
	return nil
}

func applyComponentSyntaxBlock(src []byte, component *gwdkir.Component, block gwdkast.Block) error {
	switch block.Kind {
	case "props":
		if component.PropsType.Name != "" || len(component.Props) > 0 {
			return fmt.Errorf("line %d: component declares multiple props contracts", block.Span.Start.Line)
		}
		component.Props = lowerSyntaxProps(block.Props)
	case "exports":
		if len(component.Exports) > 0 {
			return fmt.Errorf("line %d: component declares multiple exports blocks", block.Span.Start.Line)
		}
		component.Exports = lowerSyntaxExports(block.Exports)
		component.Blocks.Spans.Exports = block.Span
	case "emits":
		if len(component.Emits) > 0 {
			return fmt.Errorf("line %d: component declares multiple emits blocks", block.Span.Start.Line)
		}
		component.Emits = lowerSyntaxEmits(block.Emits)
		component.Blocks.Spans.Emits = block.Span
	case "client":
		if component.Blocks.Client {
			return fmt.Errorf("line %d: component declares multiple client blocks", block.Span.Start.Line)
		}
		component.Blocks.Client = true
		component.Blocks.ClientBody = block.Body
		component.Blocks.Spans.Client = block.Span
	case "go":
		component.Blocks.GoBlocks = append(component.Blocks.GoBlocks, gwdkir.GoBlock{
			Target: block.Name,
			Body:   block.Body,
			Span:   block.Span,
		})
		component.Blocks.Spans.GoBlocks = append(component.Blocks.Spans.GoBlocks, source.NamedSpan{Name: block.Name, Span: block.Span})
	case "view":
		component.Blocks.View = true
		component.Blocks.ViewBody = block.Body
		component.Blocks.Spans.View = block.Span
		component.Blocks.Spans.ViewBodyStart = block.BodyStart
	case "style":
		if component.Blocks.Style {
			return fmt.Errorf("line %d: component declares multiple style blocks", block.Span.Start.Line)
		}
		component.Blocks.StyleBody = block.StyleBody
		component.Blocks.Style = strings.TrimSpace(block.StyleBody) != ""
	default:
		lineNumber := block.Span.Start.Line
		return lineDiagnosticError(DiagnosticUnsupportedTopLevelBlock, lineNumber, sourceLineText(src, lineNumber), "unsupported top-level block %q", block.Kind)
	}
	return nil
}

func lowerSyntaxGoTypeRef(ref gwdkast.GoTypeRef) gwdkir.GoRef {
	return gwdkir.GoRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

func lowerSyntaxGoFuncRef(ref gwdkast.GoFuncRef) gwdkir.GoRef {
	return gwdkir.GoRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

func lowerSyntaxProps(in []gwdkast.Prop) []gwdkir.Prop {
	out := make([]gwdkir.Prop, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Prop{Name: item.Name, Type: item.Type, Span: item.Span})
	}
	return out
}

func lowerSyntaxExports(in []gwdkast.Export) []gwdkir.Export {
	out := make([]gwdkir.Export, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Export{Name: item.Name, Type: item.Type, Span: item.Span})
	}
	return out
}

func lowerSyntaxEmits(in []gwdkast.Emit) []gwdkir.Emit {
	out := make([]gwdkir.Emit, 0, len(in))
	for _, item := range in {
		params := make([]gwdkir.EmitParam, 0, len(item.Params))
		for _, param := range item.Params {
			params = append(params, gwdkir.EmitParam{Name: param.Name, Type: param.Type, Span: param.Span})
		}
		out = append(out, gwdkir.Emit{Name: item.Name, Params: params, Span: item.Span})
	}
	return out
}

func lowerSyntaxAssetRefs(in []gwdkast.AssetRef) ([]string, []source.NamedSpan) {
	values := make([]string, 0, len(in))
	spans := make([]source.NamedSpan, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		values = append(values, item.Path)
		spans = append(spans, source.NamedSpan{Name: item.Path, Span: item.Span})
	}
	return values, spans
}

func lowerSyntaxScripts(in []gwdkast.AssetRef) ([]string, []source.InlineScript, []source.NamedSpan, []source.NamedSpan) {
	var paths []string
	var inline []source.InlineScript
	var pathSpans []source.NamedSpan
	var inlineSpans []source.NamedSpan
	for _, item := range in {
		if strings.TrimSpace(item.Path) != "" {
			paths = append(paths, item.Path)
			pathSpans = append(pathSpans, source.NamedSpan{Name: item.Path, Span: item.Span})
			continue
		}
		name := source.InlineScriptName(len(inline))
		inline = append(inline, source.InlineScript{Name: name, Body: item.Inline, Span: item.Span})
		inlineSpans = append(inlineSpans, source.NamedSpan{Name: name, Span: item.Span})
	}
	return paths, inline, pathSpans, inlineSpans
}
