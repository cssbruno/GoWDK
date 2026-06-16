package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/diagnosticfix"
	"github.com/cssbruno/gowdk/internal/diagnostics"
	"github.com/cssbruno/gowdk/internal/lang"
)

type fixOptions struct {
	DryRun bool
	Code   string
	Args   []string
}

func fixCommand(args []string) error {
	fixOptions, err := parseFixOptions(args)
	if err != nil {
		return err
	}
	options, paths, err := loadCommandInputs(fixOptions.Args, "fix", false)
	if err != nil {
		return err
	}
	_, foundDiagnostics := lang.CheckFilesWithOptions(options.Config, paths, lang.CheckOptions{ProjectRoot: options.ProjectRoot})
	summary, err := collectFixes(foundDiagnostics, fixOptions.Code)
	if err != nil {
		return err
	}
	if len(summary.files) == 0 {
		fmt.Println("no fixes available")
		return nil
	}
	if fixOptions.DryRun {
		for _, path := range summary.paths() {
			fmt.Printf("%s: %d fix(es) available\n", path, len(summary.files[path].edits))
		}
		return nil
	}
	for _, path := range summary.paths() {
		if err := writeFixedFile(path, summary.files[path]); err != nil {
			return err
		}
		fmt.Printf("%s: applied %d fix(es)\n", path, len(summary.files[path].edits))
	}
	return nil
}

func parseFixOptions(args []string) (fixOptions, error) {
	var options fixOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			options.DryRun = true
		case arg == "--code":
			i++
			if i >= len(args) {
				return fixOptions{}, fmt.Errorf(fixUsage)
			}
			options.Code = args[i]
		case strings.HasPrefix(arg, "--code="):
			options.Code = strings.TrimPrefix(arg, "--code=")
		case strings.HasPrefix(arg, "-"):
			options.Args = append(options.Args, arg)
		default:
			options.Args = append(options.Args, arg)
		}
	}
	if strings.TrimSpace(options.Code) != "" {
		if _, ok := diagnostics.Lookup(options.Code); !ok {
			return fixOptions{}, fmt.Errorf("unknown diagnostic code %q", options.Code)
		}
		if _, ok := diagnostics.FixFor(options.Code); !ok {
			return fixOptions{}, fmt.Errorf("diagnostic code %q has no registered fix", options.Code)
		}
	}
	return options, nil
}

const fixUsage = "usage: gowdk fix [--dry-run] [--code <diagnostic-code>] [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...]"

type fileFixes struct {
	source string
	edits  []diagnosticfix.TextEdit
}

type fixSummary struct {
	files map[string]fileFixes
}

func (summary fixSummary) paths() []string {
	paths := make([]string, 0, len(summary.files))
	for path := range summary.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func collectFixes(foundDiagnostics lang.Diagnostics, codeFilter string) (fixSummary, error) {
	summary := fixSummary{files: map[string]fileFixes{}}
	for _, item := range foundDiagnostics {
		if codeFilter != "" && item.Code != codeFilter {
			continue
		}
		fix, ok := diagnostics.FixFor(item.Code)
		if !ok {
			continue
		}
		if strings.TrimSpace(item.File) == "" {
			return fixSummary{}, fmt.Errorf("%s fix is ambiguous: diagnostic has no file", item.Code)
		}
		fileFixes, err := fixesForFile(summary.files, item.File)
		if err != nil {
			return fixSummary{}, err
		}
		edits, err := diagnosticfix.Edits(fix, fileFixes.source, diagnosticfix.Diagnostic{
			Code:    item.Code,
			Message: item.Message,
			Range:   diagnosticRange(item),
		})
		if err != nil {
			return fixSummary{}, err
		}
		fileFixes.edits = append(fileFixes.edits, edits...)
		summary.files[item.File] = fileFixes
	}
	for path, fileFixes := range summary.files {
		edits, err := normalizeEdits(fileFixes.source, fileFixes.edits)
		if err != nil {
			return fixSummary{}, fmt.Errorf("%s: %w", path, err)
		}
		fileFixes.edits = edits
		summary.files[path] = fileFixes
	}
	return summary, nil
}

func fixesForFile(files map[string]fileFixes, path string) (fileFixes, error) {
	if fileFixes, ok := files[path]; ok {
		return fileFixes, nil
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return fileFixes{}, err
	}
	return fileFixes{source: string(source)}, nil
}

func diagnosticRange(item lang.Diagnostic) diagnosticfix.Range {
	if item.Range != nil {
		return diagnosticfix.Range{
			Start: diagnosticfix.Position{Line: item.Range.Start.Line, Column: item.Range.Start.Column},
			End:   diagnosticfix.Position{Line: item.Range.End.Line, Column: item.Range.End.Column},
		}
	}
	endColumn := item.Pos.Column + 1
	if endColumn <= 1 {
		endColumn = 2
	}
	return diagnosticfix.Range{
		Start: diagnosticfix.Position{Line: item.Pos.Line, Column: item.Pos.Column},
		End:   diagnosticfix.Position{Line: item.Pos.Line, Column: endColumn},
	}
}

func normalizeEdits(source string, edits []diagnosticfix.TextEdit) ([]diagnosticfix.TextEdit, error) {
	if len(edits) == 0 {
		return nil, nil
	}
	type indexedEdit struct {
		edit  diagnosticfix.TextEdit
		start int
		end   int
	}
	byKey := map[string]indexedEdit{}
	for _, edit := range edits {
		start, err := offsetForPosition(source, edit.Range.Start)
		if err != nil {
			return nil, err
		}
		end, err := offsetForPosition(source, edit.Range.End)
		if err != nil {
			return nil, err
		}
		if end < start {
			return nil, fmt.Errorf("fix edit has an invalid range")
		}
		key := fmt.Sprintf("%d:%d:%s", start, end, edit.NewText)
		byKey[key] = indexedEdit{edit: edit, start: start, end: end}
	}
	indexed := make([]indexedEdit, 0, len(byKey))
	for _, edit := range byKey {
		indexed = append(indexed, edit)
	}
	sort.Slice(indexed, func(i, j int) bool {
		if indexed[i].start == indexed[j].start {
			return indexed[i].end < indexed[j].end
		}
		return indexed[i].start < indexed[j].start
	})
	for i := 1; i < len(indexed); i++ {
		if indexed[i].start < indexed[i-1].end {
			return nil, fmt.Errorf("fix edits overlap")
		}
	}
	out := make([]diagnosticfix.TextEdit, 0, len(indexed))
	for _, edit := range indexed {
		out = append(out, edit.edit)
	}
	return out, nil
}

func writeFixedFile(path string, fileFixes fileFixes) error {
	source := fileFixes.source
	indexed := make([]struct {
		edit  diagnosticfix.TextEdit
		start int
		end   int
	}, 0, len(fileFixes.edits))
	for _, edit := range fileFixes.edits {
		start, err := offsetForPosition(source, edit.Range.Start)
		if err != nil {
			return err
		}
		end, err := offsetForPosition(source, edit.Range.End)
		if err != nil {
			return err
		}
		indexed = append(indexed, struct {
			edit  diagnosticfix.TextEdit
			start int
			end   int
		}{edit: edit, start: start, end: end})
	}
	sort.Slice(indexed, func(i, j int) bool {
		return indexed[i].start > indexed[j].start
	})
	for _, item := range indexed {
		source = source[:item.start] + item.edit.NewText + source[item.end:]
	}
	return os.WriteFile(path, []byte(source), 0o644)
}

func offsetForPosition(source string, position diagnosticfix.Position) (int, error) {
	if position.Line <= 0 || position.Column <= 0 {
		return 0, fmt.Errorf("fix edit has an invalid position")
	}
	line := 1
	column := 1
	for offset, char := range source {
		if line == position.Line && column == position.Column {
			return offset, nil
		}
		if char == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	if line == position.Line && column == position.Column {
		return len(source), nil
	}
	return 0, fmt.Errorf("fix edit position %d:%d is outside the file", position.Line, position.Column)
}
