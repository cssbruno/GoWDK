// Package gwdkanalysis lowers GOWDK AST files into normalized manifest and IR
// metadata.
package gwdkanalysis

import (
	"fmt"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

type SourceKind = gwdkir.SourceKind

const (
	SourcePage      = gwdkir.SourcePage
	SourceComponent = gwdkir.SourceComponent
	SourceLayout    = gwdkir.SourceLayout
)

// SourceFile is one parsed GOWDK AST file ready for analysis.
type SourceFile struct {
	Path string
	Kind SourceKind
	AST  gwdkast.File
}

// Result contains compatibility manifest records plus the stable IR produced
// from them.
type Result struct {
	Manifest manifest.Manifest
	IR       gwdkir.Program
}

// Analyze lowers parsed AST files into normalized compiler metadata.
func Analyze(config gowdk.Config, files []SourceFile) (Result, error) {
	var result Result
	result.IR.Version = gwdkir.Version

	for _, file := range files {
		switch file.Kind {
		case SourcePage:
			page, err := LowerPage(file.Path, file.AST)
			if err != nil {
				return Result{}, err
			}
			result.Manifest.Pages = append(result.Manifest.Pages, page)
		case SourceComponent:
			component, err := LowerComponent(file.Path, file.AST)
			if err != nil {
				return Result{}, err
			}
			result.Manifest.Components = append(result.Manifest.Components, component)
		case SourceLayout:
			layout, err := LowerLayout(file.Path, file.AST)
			if err != nil {
				return Result{}, err
			}
			result.Manifest.Layouts = append(result.Manifest.Layouts, layout)
		default:
			return Result{}, fmt.Errorf("unsupported GOWDK source kind %q for %s", file.Kind, file.Path)
		}
	}

	result.IR = BuildIR(config, result.Manifest)
	return result, nil
}
