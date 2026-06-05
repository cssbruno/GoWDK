package appgen

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// Result describes generated app artifacts.
type Result struct {
	AppDir      string
	MainPath    string
	PackagePath string
	ModulePath  string
	OutputDir   string
	Files       []string
	BinaryPath  string
}

// Options configures generated app output.
type Options struct {
	Actions    []ActionRoute
	SSR        []SSRRoute
	AutoRoutes bool
	Config     gowdk.Config
	Manifest   *manifest.Manifest
}

// ActionRoute describes a generated action handler.
type ActionRoute struct {
	PageID         string
	ActionName     string
	Route          string
	InputName      string
	InputType      string
	InputFields    []string
	RequiredFields []string
	ValidatesInput bool
	Redirect       string
	Fragments      []ActionFragment
}

// ActionFragment describes a generated partial response fragment.
type ActionFragment struct {
	Target string
	HTML   string
}

// SSRRoute describes a generated request-time page handler.
type SSRRoute struct {
	PageID       string
	Route        string
	HTML         string
	Replacements []SSRReplacement
}

// SSRReplacement maps a generated placeholder back to a request route param.
type SSRReplacement struct {
	Param       string
	Placeholder string
}
