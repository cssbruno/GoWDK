// Package contractscan discovers runtime contract registrations in normal Go
// source using the standard Go AST.
package contractscan

import (
	"encoding/json"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

const (
	RuntimeImportPath      = "github.com/cssbruno/gowdk/runtime/contracts"
	generatedAppModulePath = "gowdk-generated-app"
)

// Contract describes one discovered registration call.
type Contract struct {
	Kind             runtimecontracts.Kind          `json:"kind"`
	EventCategory    runtimecontracts.EventCategory `json:"eventCategory,omitempty"`
	Package          string                         `json:"package,omitempty"`
	Type             string                         `json:"type"`
	TypeImportPath   string                         `json:"typeImportPath,omitempty"`
	Result           string                         `json:"result,omitempty"`
	ResultImportPath string                         `json:"resultImportPath,omitempty"`
	Handler          string                         `json:"handler,omitempty"`
	Register         string                         `json:"register,omitempty"`
	InputFields      []source.BackendInputField     `json:"inputFields,omitempty"`
	ResultFields     []source.BackendInputField     `json:"resultFields,omitempty"`
	PayloadFields    []source.BackendInputField     `json:"payloadFields,omitempty"`
	Emits            []EventRef                     `json:"emits,omitempty"`
	Roles            []string                       `json:"roles,omitempty"`
	Source           string                         `json:"source"`
	Line             int                            `json:"line"`
	Column           int                            `json:"column"`
}

// Invalidation describes one explicit event-to-query invalidation edge.
type Invalidation struct {
	EventCategory       runtimecontracts.EventCategory `json:"eventCategory"`
	EventType           string                         `json:"eventType"`
	EventTypeImportPath string                         `json:"eventTypeImportPath,omitempty"`
	QueryType           string                         `json:"queryType"`
	QueryTypeImportPath string                         `json:"queryTypeImportPath,omitempty"`
	Register            string                         `json:"register,omitempty"`
	Source              string                         `json:"source"`
	Line                int                            `json:"line"`
	Column              int                            `json:"column"`
}

// Diagnostic describes a validation issue found while scanning contracts.
type Diagnostic struct {
	Severity       string                `json:"severity"`
	Code           string                `json:"code,omitempty"`
	Kind           runtimecontracts.Kind `json:"kind,omitempty"`
	Package        string                `json:"package,omitempty"`
	Type           string                `json:"type,omitempty"`
	TypeImportPath string                `json:"typeImportPath,omitempty"`
	Handler        string                `json:"handler,omitempty"`
	Source         string                `json:"source"`
	Line           int                   `json:"line"`
	Column         int                   `json:"column"`
	Message        string                `json:"message"`
}

// EventRef describes one event a command handler can emit.
type EventRef struct {
	Category       runtimecontracts.EventCategory `json:"category"`
	Type           string                         `json:"type"`
	TypeImportPath string                         `json:"typeImportPath,omitempty"`
}

// Report is the full discovery output.
type Report struct {
	Version       int            `json:"version"`
	Root          string         `json:"root"`
	Contracts     []Contract     `json:"contracts"`
	Invalidations []Invalidation `json:"invalidations,omitempty"`
	Diagnostics   []Diagnostic   `json:"diagnostics,omitempty"`
}

// Scan walks root and reports registrations that call runtime/contracts helpers.
func Scan(root string) (Report, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	var files []string
	if err := filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) && path != absRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return Report{}, err
	}
	sort.Strings(files)
	fset := token.NewFileSet()
	var contracts []Contract
	var invalidations []Invalidation
	var diagnostics []Diagnostic
	packages, err := parseScanPackages(fset, absRoot, files)
	if err != nil {
		return Report{}, err
	}
	inspectionCache := newPackageInspectionCache()
	for _, pkg := range packages {
		discovered := scanPackage(fset, pkg, inspectionCache)
		contracts = append(contracts, discovered.Contracts...)
		invalidations = append(invalidations, discovered.Invalidations...)
		diagnostics = append(diagnostics, discovered.Diagnostics...)
	}
	diagnostics = append(diagnostics, duplicateCommandDiagnostics(contracts)...)
	diagnostics = append(diagnostics, emittedEventCategoryDiagnostics(contracts)...)
	diagnostics = append(diagnostics, invalidationDiagnostics(invalidations, contracts)...)
	sort.Slice(contracts, func(i, j int) bool {
		if contracts[i].Kind != contracts[j].Kind {
			return contracts[i].Kind < contracts[j].Kind
		}
		if contracts[i].EventCategory != contracts[j].EventCategory {
			return contracts[i].EventCategory < contracts[j].EventCategory
		}
		if contracts[i].Package != contracts[j].Package {
			return contracts[i].Package < contracts[j].Package
		}
		if contracts[i].Type != contracts[j].Type {
			return contracts[i].Type < contracts[j].Type
		}
		if contracts[i].Source != contracts[j].Source {
			return contracts[i].Source < contracts[j].Source
		}
		return contracts[i].Line < contracts[j].Line
	})
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Source != diagnostics[j].Source {
			return diagnostics[i].Source < diagnostics[j].Source
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Column < diagnostics[j].Column
	})
	sort.Slice(invalidations, func(i, j int) bool {
		if invalidations[i].EventCategory != invalidations[j].EventCategory {
			return invalidations[i].EventCategory < invalidations[j].EventCategory
		}
		if invalidations[i].EventType != invalidations[j].EventType {
			return invalidations[i].EventType < invalidations[j].EventType
		}
		if invalidations[i].QueryType != invalidations[j].QueryType {
			return invalidations[i].QueryType < invalidations[j].QueryType
		}
		if invalidations[i].Source != invalidations[j].Source {
			return invalidations[i].Source < invalidations[j].Source
		}
		return invalidations[i].Line < invalidations[j].Line
	})
	return Report{Version: 1, Root: absRoot, Contracts: contracts, Invalidations: invalidations, Diagnostics: diagnostics}, nil
}

// Filter returns contracts of kind. Empty kind returns a copy of all contracts.
func (report Report) Filter(kind runtimecontracts.Kind) []Contract {
	out := make([]Contract, 0, len(report.Contracts))
	for _, contract := range report.Contracts {
		if kind == "" || contract.Kind == kind {
			out = append(out, contract)
		}
	}
	return out
}

// JSON returns deterministic indented JSON.
func (report Report) JSON(kind runtimecontracts.Kind) ([]byte, error) {
	out := struct {
		Version       int            `json:"version"`
		Root          string         `json:"root"`
		Contracts     []Contract     `json:"contracts"`
		Invalidations []Invalidation `json:"invalidations,omitempty"`
		Diagnostics   []Diagnostic   `json:"diagnostics,omitempty"`
	}{
		Version:       report.Version,
		Root:          report.Root,
		Contracts:     report.Filter(kind),
		Invalidations: report.Invalidations,
		Diagnostics:   report.Diagnostics,
	}
	return json.MarshalIndent(out, "", "  ")
}

// LinkReferences resolves GOWDK IR contract references against scanned Go
// runtime contract registrations.
