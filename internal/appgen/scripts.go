package appgen

import (
	"fmt"
	"go/format"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/goblockgen"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

type inlineGoBlockGroup struct {
	imports  []gwdkir.Import
	goBlocks []gwdkir.GoBlock
}

type addonGoBlockTarget struct {
	target gowdk.GoBlockTarget
	render gowdk.RenderMode
}

func writeInlineGoBlockFiles(appDir string, options Options) ([]string, error) {
	if options.IR == nil {
		return nil, nil
	}
	groups := inlineGoBlockGroups(*options.IR)
	if len(groups) == 0 {
		return nil, nil
	}
	var files []string
	for packageName, group := range groups {
		generated, err := goblockgen.Source(packageName, group.imports, group.goBlocks)
		if err != nil {
			return nil, fmt.Errorf("generate inline go block package %s: %w", packageName, err)
		}
		if len(generated) == 0 {
			continue
		}
		relPath := goblockgen.GeneratedRelPath(packageName)
		if err := writeFileIfChanged(filepath.Join(appDir, relPath), generated); err != nil {
			return nil, err
		}
		files = append(files, relPath)
	}
	return files, nil
}

func inlineGoBlockGroups(ir gwdkir.Program) map[string]inlineGoBlockGroup {
	groups := map[string]inlineGoBlockGroup{}
	for _, page := range ir.Pages {
		goBlocks := generatedInlineGoBlocks(page.Blocks.GoBlocks)
		if len(goBlocks) == 0 {
			continue
		}
		group := groups[page.Package]
		group.imports = mergeGoBlockImports(group.imports, page.Imports)
		group.goBlocks = append(group.goBlocks, goBlocks...)
		groups[page.Package] = group
	}
	for _, component := range ir.Components {
		goBlocks := generatedInlineGoBlocks(component.Blocks.GoBlocks)
		if len(goBlocks) == 0 {
			continue
		}
		group := groups[component.Package]
		group.imports = mergeGoBlockImports(group.imports, component.Imports)
		group.goBlocks = append(group.goBlocks, goBlocks...)
		groups[component.Package] = group
	}
	for _, layout := range ir.Layouts {
		goBlocks := generatedInlineGoBlocks(layout.Blocks.GoBlocks)
		if len(goBlocks) == 0 {
			continue
		}
		group := groups[layout.Package]
		group.goBlocks = append(group.goBlocks, goBlocks...)
		groups[layout.Package] = group
	}
	return groups
}

func isGeneratedInlineGoBlockTarget(target string) bool {
	switch strings.TrimSpace(target) {
	case "", "ssr":
		return true
	default:
		return false
	}
}

func generatedInlineGoBlocks(blocks []gwdkir.GoBlock) []gwdkir.GoBlock {
	var out []gwdkir.GoBlock
	for _, script := range blocks {
		if !isGeneratedInlineGoBlockTarget(script.Target) {
			continue
		}
		out = append(out, script)
	}
	return out
}

func mergeGoBlockImports(left []gwdkir.Import, right []gwdkir.Import) []gwdkir.Import {
	seen := map[string]bool{}
	out := make([]gwdkir.Import, 0, len(left)+len(right))
	for _, item := range append(append([]gwdkir.Import(nil), left...), right...) {
		key := item.Alias + "\x00" + item.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func writeAddonGoBlockFiles(appDir string, options Options) ([]string, error) {
	if options.IR == nil {
		return nil, nil
	}
	var files []string
	for _, target := range addonGoBlockTargets(*options.IR, options.Config) {
		consumer, ok := addonGoBlockConsumer(options.Config, target.target.Target)
		if !ok {
			return nil, fmt.Errorf("go block target %s requires an enabled addon implementing gowdk.GoBlockConsumer", target.target.Target)
		}
		generated, err := consumer.GeneratedGo(target.target, gowdk.GoBlockContext{Render: target.render})
		if err != nil {
			return nil, fmt.Errorf("generate addon go block target %s: %w", target.target.Target, err)
		}
		for _, file := range generated {
			relPath, err := safeAddonGoBlockFilePath(file.Path)
			if err != nil {
				return nil, err
			}
			contents := []byte(file.Source)
			if strings.HasSuffix(relPath, ".go") {
				formatted, err := format.Source(contents)
				if err != nil {
					return nil, fmt.Errorf("format addon go block file %s: %w", relPath, err)
				}
				contents = formatted
			}
			if err := writeFileIfChanged(filepath.Join(appDir, relPath), contents); err != nil {
				return nil, err
			}
			files = append(files, relPath)
		}
	}
	return files, nil
}

func addonGoBlockConsumer(config gowdk.Config, target string) (gowdk.GoBlockConsumer, bool) {
	name := strings.TrimPrefix(strings.TrimSpace(target), "addon.")
	if name == target || name == "" {
		return nil, false
	}
	for _, addon := range config.Addons {
		if addon.Name() != name {
			continue
		}
		consumer, ok := addon.(gowdk.GoBlockConsumer)
		if !ok {
			return nil, false
		}
		for _, supported := range consumer.GoBlockTargets() {
			if supported == target {
				return consumer, true
			}
		}
	}
	return nil, false
}

func addonGoBlockTargets(ir gwdkir.Program, config gowdk.Config) []addonGoBlockTarget {
	var targets []addonGoBlockTarget
	for _, page := range ir.Pages {
		render := page.Render
		if render == "" {
			render = config.Render.DefaultMode()
		}
		for _, script := range page.Blocks.GoBlocks {
			if strings.HasPrefix(strings.TrimSpace(script.Target), "addon.") {
				targets = append(targets, addonGoBlockTarget{
					target: gowdkGoBlockTarget("page", page.ID, page.Package, page.Source, script.Target, script.Body, script.Span),
					render: render,
				})
			}
		}
	}
	for _, component := range ir.Components {
		for _, script := range component.Blocks.GoBlocks {
			if strings.HasPrefix(strings.TrimSpace(script.Target), "addon.") {
				targets = append(targets, addonGoBlockTarget{
					target: gowdkGoBlockTarget("component", component.Name, component.Package, component.Source, script.Target, script.Body, script.Span),
					render: config.Render.DefaultMode(),
				})
			}
		}
	}
	for _, layout := range ir.Layouts {
		for _, script := range layout.Blocks.GoBlocks {
			if strings.HasPrefix(strings.TrimSpace(script.Target), "addon.") {
				targets = append(targets, addonGoBlockTarget{
					target: gowdkGoBlockTarget("layout", layout.ID, layout.Package, layout.Source, script.Target, script.Body, script.Span),
					render: config.Render.DefaultMode(),
				})
			}
		}
	}
	return targets
}

func gowdkGoBlockTarget(ownerKind string, ownerID string, packageName string, sourcePath string, target string, body string, span source.SourceSpan) gowdk.GoBlockTarget {
	return gowdk.GoBlockTarget{
		Target:       target,
		OwnerKind:    ownerKind,
		OwnerID:      ownerID,
		OwnerPackage: packageName,
		SourcePath:   sourcePath,
		Body:         body,
		Span: gowdk.SourceSpan{
			Start: gowdk.SourcePosition{Line: span.Start.Line, Column: span.Start.Column},
			End:   gowdk.SourcePosition{Line: span.End.Line, Column: span.End.Column},
		},
	}
}

func safeAddonGoBlockFilePath(value string) (string, error) {
	cleaned := filepath.Clean(strings.TrimSpace(value))
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("addon go block file path is required")
	}
	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", fmt.Errorf("addon go block file path %q must stay inside the generated app directory", value)
	}
	return cleaned, nil
}
