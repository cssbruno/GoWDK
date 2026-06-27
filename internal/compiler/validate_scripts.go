package compiler

import (
	"fmt"
	"go/parser"
	"go/token"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func validateGoBlocks(config gowdk.Config, app gwdkir.Program) []ValidationError {
	var diagnostics []ValidationError
	enabledAddons := addonsByName(config)
	for _, page := range app.Pages {
		mode := page.RenderMode(config.Render.DefaultMode())
		for _, block := range page.Blocks.GoBlocks {
			diagnostics = append(diagnostics, validateGoBlockSyntax(page.Package, page.Source, page.ID, "", block)...)
			diagnostics = append(diagnostics, validateGoBlockTarget(enabledAddons, page.ID, "", page.Source, page.Package, mode, block)...)
		}
	}
	for _, component := range app.Components {
		for _, block := range component.Blocks.GoBlocks {
			diagnostics = append(diagnostics, validateGoBlockSyntax(component.Package, component.Source, "", component.Name, block)...)
			diagnostics = append(diagnostics, validateNonPageGoBlockTarget(config, enabledAddons, "", component.Name, component.Source, component.Package, block)...)
		}
	}
	for _, layout := range app.Layouts {
		for _, block := range layout.Blocks.GoBlocks {
			diagnostics = append(diagnostics, validateGoBlockSyntax(layout.Package, layout.Source, "", "", block)...)
			diagnostics = append(diagnostics, validateNonPageGoBlockTarget(config, enabledAddons, "", "", layout.Source, layout.Package, block)...)
		}
	}
	return diagnostics
}

func validateGoBlockSyntax(packageName string, sourcePath string, pageID string, componentName string, block gwdkir.GoBlock) []ValidationError {
	if strings.TrimSpace(block.Body) == "" {
		return nil
	}
	if strings.TrimSpace(packageName) == "" {
		packageName = "gowdkgoblock"
	}
	_, err := parser.ParseFile(token.NewFileSet(), "go-block.gwdk.go", "package "+packageName+"\n"+block.Body, parser.AllErrors)
	if err == nil {
		return nil
	}
	return []ValidationError{{
		Code:          "invalid_go_block",
		PageID:        pageID,
		ComponentName: componentName,
		Source:        sourcePath,
		Span:          block.Span,
		Message:       fmt.Sprintf("go %s contains invalid Go: %v", goBlockLabel(block.Target), err),
	}}
}

func validateGoBlockTarget(enabledAddons map[string]gowdk.Addon, pageID string, componentName string, sourcePath string, packageName string, mode gowdk.RenderMode, block gwdkir.GoBlock) []ValidationError {
	target := strings.TrimSpace(block.Target)
	switch {
	case target == "" || target == "client":
		return nil
	case target == "build":
		return nil
	case target == "server":
		if mode == gowdk.SSR || mode == gowdk.Hybrid {
			return nil
		}
		return []ValidationError{{
			Code:          "go_server_requires_request_render",
			PageID:        pageID,
			ComponentName: componentName,
			Source:        sourcePath,
			Span:          block.Span,
			Message:       fmt.Sprintf("%s declares go server {}, but server Go code is request-time behavior and requires the SSR addon", pageID),
		}}
	case target == "ssr":
		return []ValidationError{renamedGoServerTarget(pageID, componentName, sourcePath, block)}
	case strings.HasPrefix(target, "addon."):
		return validateAddonGoBlockTarget(enabledAddons, pageID, componentName, sourcePath, packageName, mode, block)
	default:
		return []ValidationError{{
			Code:          "unknown_go_block_target",
			PageID:        pageID,
			ComponentName: componentName,
			Source:        sourcePath,
			Span:          block.Span,
			Message:       fmt.Sprintf("unknown go block target %q; use go {}, go build {}, go client {}, go server {}, or go addon.<name> {}", target),
		}}
	}
}

// renamedGoServerTarget reports the breaking rename of the request-time Go block
// target from go ssr {} to go server {} so existing pages get a precise nudge
// rather than an opaque "unknown go block target" error.
func renamedGoServerTarget(pageID, componentName, sourcePath string, block gwdkir.GoBlock) ValidationError {
	return ValidationError{
		Code:          "go_ssr_renamed_to_server",
		PageID:        pageID,
		ComponentName: componentName,
		Source:        sourcePath,
		Span:          block.Span,
		Message:       "go ssr {} was renamed to go server {}; the server lane (server {} + go server {}) is GoWDK's request-time lane",
	}
}

func validateNonPageGoBlockTarget(config gowdk.Config, enabledAddons map[string]gowdk.Addon, pageID string, componentName string, sourcePath string, packageName string, block gwdkir.GoBlock) []ValidationError {
	target := strings.TrimSpace(block.Target)
	switch {
	case target == "":
		return nil
	case target == "client":
		return []ValidationError{{
			Code:          "go_client_requires_page",
			PageID:        pageID,
			ComponentName: componentName,
			Source:        sourcePath,
			Span:          block.Span,
			Message:       "go client {} is page-level client-side behavior; use a page go client {} block or a component wasm package",
		}}
	case target == "build":
		return nil
	case target == "server":
		if config.HasFeature(gowdk.FeatureSSR) {
			return nil
		}
		return []ValidationError{{
			Code:          "missing_ssr_addon",
			PageID:        pageID,
			ComponentName: componentName,
			Source:        sourcePath,
			Span:          block.Span,
			Message:       "go server {} requires the SSR addon before request-time Go code can be validated or generated",
		}}
	case target == "ssr":
		return []ValidationError{renamedGoServerTarget(pageID, componentName, sourcePath, block)}
	case strings.HasPrefix(target, "addon."):
		return validateAddonGoBlockTarget(enabledAddons, pageID, componentName, sourcePath, packageName, gowdk.SPA, block)
	default:
		return []ValidationError{{
			Code:          "unknown_go_block_target",
			PageID:        pageID,
			ComponentName: componentName,
			Source:        sourcePath,
			Span:          block.Span,
			Message:       fmt.Sprintf("unknown go block target %q; use go {}, go build {}, go client {}, go server {}, or go addon.<name> {}", target),
		}}
	}
}

func validateAddonGoBlockTarget(enabledAddons map[string]gowdk.Addon, pageID string, componentName string, sourcePath string, packageName string, render gowdk.RenderMode, block gwdkir.GoBlock) []ValidationError {
	name := strings.TrimPrefix(strings.TrimSpace(block.Target), "addon.")
	addon, ok := enabledAddons[name]
	if name != "" && ok {
		consumer, ok := addon.(gowdk.GoBlockConsumer)
		if !ok {
			return []ValidationError{{
				Code:          "unsupported_addon_go_block_target",
				PageID:        pageID,
				ComponentName: componentName,
				Source:        sourcePath,
				Span:          block.Span,
				Message:       fmt.Sprintf("addon %q is enabled but does not implement gowdk.GoBlockConsumer for go block target %q", name, block.Target),
			}}
		}
		if !goBlockConsumerSupportsTarget(consumer, block.Target) {
			return []ValidationError{{
				Code:          "unsupported_addon_go_block_target",
				PageID:        pageID,
				ComponentName: componentName,
				Source:        sourcePath,
				Span:          block.Span,
				Message:       fmt.Sprintf("addon %q does not consume go block target %q", name, block.Target),
			}}
		}
		return addonGoBlockDiagnostics(consumer, pageID, componentName, sourcePath, packageName, render, block)
	}
	return []ValidationError{{
		Code:          "unknown_addon_go_block_target",
		PageID:        pageID,
		ComponentName: componentName,
		Source:        sourcePath,
		Span:          block.Span,
		Message:       fmt.Sprintf("go addon.%s {} requires an enabled addon named %q", name, name),
	}}
}

func goBlockConsumerSupportsTarget(consumer gowdk.GoBlockConsumer, target string) bool {
	for _, supported := range consumer.GoBlockTargets() {
		if supported == target {
			return true
		}
	}
	return false
}

func addonGoBlockDiagnostics(consumer gowdk.GoBlockConsumer, pageID string, componentName string, sourcePath string, packageName string, render gowdk.RenderMode, block gwdkir.GoBlock) []ValidationError {
	target := gowdkGoBlockTarget(pageID, componentName, sourcePath, packageName, block)
	context := gowdk.GoBlockContext{Render: render}
	var diagnostics []ValidationError
	for _, diagnostic := range consumer.ValidateGoBlock(target, context) {
		span := block.Span
		if diagnostic.Span.Start.Line != 0 || diagnostic.Span.End.Line != 0 {
			span = manifestSpan(diagnostic.Span)
		}
		code := strings.TrimSpace(diagnostic.Code)
		if code == "" {
			code = "addon_go_block_diagnostic"
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:          code,
			PageID:        pageID,
			ComponentName: componentName,
			Source:        sourcePath,
			Span:          span,
			Message:       diagnostic.Message,
		})
	}
	return diagnostics
}

func addonsByName(config gowdk.Config) map[string]gowdk.Addon {
	names := map[string]gowdk.Addon{}
	for _, addon := range config.Addons {
		names[addon.Name()] = addon
	}
	return names
}

func gowdkGoBlockTarget(pageID string, componentName string, sourcePath string, packageName string, block gwdkir.GoBlock) gowdk.GoBlockTarget {
	ownerKind := "layout"
	ownerID := ""
	if pageID != "" {
		ownerKind = "page"
		ownerID = pageID
	} else if componentName != "" {
		ownerKind = "component"
		ownerID = componentName
	}
	return gowdk.GoBlockTarget{
		Target:       block.Target,
		OwnerKind:    ownerKind,
		OwnerID:      ownerID,
		OwnerPackage: packageName,
		SourcePath:   sourcePath,
		Body:         block.Body,
		Span:         gowdkSpan(block.Span),
	}
}

func gowdkSpan(span source.SourceSpan) gowdk.SourceSpan {
	return gowdk.SourceSpan{
		Start: gowdk.SourcePosition{Line: span.Start.Line, Column: span.Start.Column},
		End:   gowdk.SourcePosition{Line: span.End.Line, Column: span.End.Column},
	}
}

func manifestSpan(span gowdk.SourceSpan) source.SourceSpan {
	return source.SourceSpan{
		Start: source.SourcePosition{Line: span.Start.Line, Column: span.Start.Column},
		End:   source.SourcePosition{Line: span.End.Line, Column: span.End.Column},
	}
}

func goBlockLabel(target string) string {
	if strings.TrimSpace(target) == "" {
		return "{}"
	}
	return strings.TrimSpace(target) + " {}"
}
