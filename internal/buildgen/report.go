package buildgen

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

type BuildEventLevel string

const (
	BuildEventDebug BuildEventLevel = "debug"
	BuildEventInfo  BuildEventLevel = "info"
	BuildEventError BuildEventLevel = "error"
)

type BuildEvent struct {
	Level   BuildEventLevel   `json:"level"`
	Stage   string            `json:"stage"`
	Kind    string            `json:"kind,omitempty"`
	Message string            `json:"message"`
	PageID  string            `json:"pageId,omitempty"`
	Route   string            `json:"route,omitempty"`
	Path    string            `json:"path,omitempty"`
	Data    map[string]string `json:"data,omitempty"`
}

type BuildReport struct {
	Version   int          `json:"version"`
	Mode      string       `json:"mode"`
	OutputDir string       `json:"outputDir"`
	Events    []BuildEvent `json:"events"`
}

// BuildDiagnostic is a structured diagnostic produced during SPA planning
// or output generation after parser/compiler validation has already completed.
type BuildDiagnostic struct {
	Code          string              `json:"code"`
	ComponentName string              `json:"componentName,omitempty"`
	Source        string              `json:"source,omitempty"`
	Span          manifest.SourceSpan `json:"span,omitempty"`
	Message       string              `json:"message"`
}

type BuildError struct {
	Err         error
	Report      BuildReport
	Diagnostics []BuildDiagnostic
}

func (err *BuildError) Error() string {
	if err == nil || err.Err == nil {
		return ""
	}
	return err.Err.Error()
}

func (err *BuildError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

type buildReporter struct {
	report BuildReport
}

type buildDiagnosticError interface {
	BuildDiagnostics() []BuildDiagnostic
}

func newBuildReporter(mode string, outputDir string) *buildReporter {
	return &buildReporter{
		report: BuildReport{
			Version:   1,
			Mode:      mode,
			OutputDir: outputDir,
		},
	}
}

func (reporter *buildReporter) debug(stage string, kind string, message string, event BuildEvent) {
	reporter.add(BuildEventDebug, stage, kind, message, event)
}

func (reporter *buildReporter) info(stage string, kind string, message string, event BuildEvent) {
	reporter.add(BuildEventInfo, stage, kind, message, event)
}

func (reporter *buildReporter) add(level BuildEventLevel, stage string, kind string, message string, event BuildEvent) {
	event.Level = level
	event.Stage = stage
	event.Kind = kind
	event.Message = message
	reporter.report.Events = append(reporter.report.Events, event)
}

func (reporter *buildReporter) fail(stage string, err error) error {
	if err == nil {
		return nil
	}
	reporter.add(BuildEventError, stage, "failed", err.Error(), BuildEvent{})
	var diagnostics []BuildDiagnostic
	if typed, ok := err.(buildDiagnosticError); ok {
		diagnostics = typed.BuildDiagnostics()
	}
	return &BuildError{
		Err:         err,
		Report:      reporter.result(),
		Diagnostics: diagnostics,
	}
}

func (reporter *buildReporter) result() BuildReport {
	report := reporter.report
	report.Events = make([]BuildEvent, len(reporter.report.Events))
	for i, event := range reporter.report.Events {
		report.Events[i] = event
		if event.Data != nil {
			report.Events[i].Data = map[string]string{}
			for key, value := range event.Data {
				report.Events[i].Data[key] = value
			}
		}
	}
	return report
}

func writeBuildReport(outputDir string, report BuildReport) (string, error) {
	reportPath := filepath.Join(outputDir, buildReportFile)
	payload, err := buildReportPayload(report)
	if err != nil {
		return "", err
	}
	if err := writeFileIfChanged(reportPath, payload); err != nil {
		return "", err
	}
	return reportPath, nil
}

func buildReportPayload(report BuildReport) ([]byte, error) {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')
	return payload, nil
}

func buildReportPath(outputDir string) string {
	return filepath.Join(outputDir, buildReportFile)
}

func eventPath(outputDir string, path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	rel, err := relativeOutputPath(outputDir, path)
	if err == nil {
		return rel
	}
	return path
}
