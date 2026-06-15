package addonregistry

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const CurrentSchemaVersion = 1

//go:embed registry.json
var registryFS embed.FS

type Registry struct {
	SchemaVersion int     `json:"schemaVersion"`
	Addons        []Entry `json:"addons"`
}

type Entry struct {
	Name                  string      `json:"name"`
	Summary               string      `json:"summary"`
	Description           string      `json:"description"`
	Kind                  string      `json:"kind"`
	Lifecycle             string      `json:"lifecycle"`
	Compatibility         string      `json:"compatibility"`
	MinGOWDK              string      `json:"minGOWDK,omitempty"`
	MaxGOWDK              string      `json:"maxGOWDK,omitempty"`
	ModulePath            string      `json:"modulePath"`
	PackagePath           string      `json:"packagePath"`
	ImportPath            string      `json:"importPath"`
	Owner                 string      `json:"owner"`
	SourceRepository      string      `json:"sourceRepository"`
	License               string      `json:"license"`
	Documentation         string      `json:"documentation"`
	Features              []string    `json:"features,omitempty"`
	PublicInterfaces      []string    `json:"publicInterfaces,omitempty"`
	RequiredExternalTools []string    `json:"requiredExternalTools,omitempty"`
	NetworkBehavior       []string    `json:"networkBehavior,omitempty"`
	ProcessBehavior       []string    `json:"processBehavior,omitempty"`
	SecurityNotes         []string    `json:"securityNotes,omitempty"`
	Trust                 Trust       `json:"trust"`
	Constructor           Constructor `json:"constructor"`
}

type Trust struct {
	Level string `json:"level"`
	Notes string `json:"notes"`
}

type Constructor struct {
	Addable    bool   `json:"addable"`
	Package    string `json:"package"`
	Function   string `json:"function"`
	Options    string `json:"options,omitempty"`
	OptionsCLI string `json:"optionsCLI,omitempty"`
}

func Bundled() (Registry, error) {
	contents, err := registryFS.ReadFile("registry.json")
	if err != nil {
		return Registry{}, err
	}
	return Parse(contents)
}

func Parse(contents []byte) (Registry, error) {
	var registry Registry
	if err := json.Unmarshal(contents, &registry); err != nil {
		return Registry{}, err
	}
	if errors := Validate(registry); len(errors) > 0 {
		return Registry{}, errors[0]
	}
	sort.Slice(registry.Addons, func(i, j int) bool {
		return registry.Addons[i].Name < registry.Addons[j].Name
	})
	return registry, nil
}

func Validate(registry Registry) []error {
	var errors []error
	if registry.SchemaVersion != CurrentSchemaVersion {
		errors = append(errors, fmt.Errorf("addon registry schemaVersion must be %d", CurrentSchemaVersion))
	}
	seen := map[string]bool{}
	for index, entry := range registry.Addons {
		prefix := fmt.Sprintf("addons[%d]", index)
		if strings.TrimSpace(entry.Name) == "" {
			errors = append(errors, fmt.Errorf("%s.name is required", prefix))
		}
		if seen[entry.Name] {
			errors = append(errors, fmt.Errorf("%s.name %q is duplicated", prefix, entry.Name))
		}
		seen[entry.Name] = true
		requireOneOf(&errors, prefix+".kind", entry.Kind, "built-in", "documented-external")
		requireOneOf(&errors, prefix+".lifecycle", entry.Lifecycle, "stable", "experimental", "deprecated")
		requireOneOf(&errors, prefix+".compatibility", entry.Compatibility, "compatible", "incompatible", "unknown")
		requireNonEmpty(&errors, prefix+".summary", entry.Summary)
		requireNonEmpty(&errors, prefix+".modulePath", entry.ModulePath)
		requireNonEmpty(&errors, prefix+".packagePath", entry.PackagePath)
		requireNonEmpty(&errors, prefix+".importPath", entry.ImportPath)
		requireNonEmpty(&errors, prefix+".owner", entry.Owner)
		requireNonEmpty(&errors, prefix+".sourceRepository", entry.SourceRepository)
		requireNonEmpty(&errors, prefix+".license", entry.License)
		requireNonEmpty(&errors, prefix+".documentation", entry.Documentation)
		requireOneOf(&errors, prefix+".trust.level", entry.Trust.Level, "gowdk-owned", "documented-external")
		requireNonEmpty(&errors, prefix+".trust.notes", entry.Trust.Notes)
		if entry.Constructor.Addable {
			requireNonEmpty(&errors, prefix+".constructor.package", entry.Constructor.Package)
			if entry.Constructor.Function == "" {
				errors = append(errors, fmt.Errorf("%s.constructor.function is required when addable", prefix))
			}
		}
		if entry.Kind == "documented-external" && entry.Constructor.Addable {
			errors = append(errors, fmt.Errorf("%s documented external addons must not be addable by gowdk add", prefix))
		}
	}
	return errors
}

func requireNonEmpty(errors *[]error, field string, value string) {
	if strings.TrimSpace(value) == "" {
		*errors = append(*errors, fmt.Errorf("%s is required", field))
	}
}

func requireOneOf(errors *[]error, field string, value string, allowed ...string) {
	for _, candidate := range allowed {
		if value == candidate {
			return
		}
	}
	*errors = append(*errors, fmt.Errorf("%s must be one of %s", field, strings.Join(allowed, ", ")))
}
