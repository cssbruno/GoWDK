package codegen

import (
	"fmt"
	"go/format"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gowdk/gowdk/internal/manifest"
	viewpkg "github.com/gowdk/gowdk/internal/view"
)

var interpolationPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

const componentPropMarkerPrefix = "__GOWDK_COMPONENT_PROP_"

// ComponentPackageOptions configures generated component package source.
type ComponentPackageOptions struct {
	PackageName string
}

// GenerateComponentPackage emits Go render functions for the current component
// subset: string props and direct {prop} interpolation inside component views.
func GenerateComponentPackage(components []manifest.Component, options ComponentPackageOptions) ([]byte, error) {
	packageName := strings.TrimSpace(options.PackageName)
	if packageName == "" {
		packageName = "components"
	}
	if !goIdentifierPattern.MatchString(packageName) {
		return nil, fmt.Errorf("invalid component package name %q", packageName)
	}

	var source strings.Builder
	source.WriteString("package ")
	source.WriteString(packageName)
	source.WriteString("\n\n")
	if len(components) > 0 {
		source.WriteString("import gowdkrender \"github.com/gowdk/gowdk/runtime/render\"\n\n")
	}

	names := map[string]bool{}
	for _, component := range sortedComponents(components) {
		name := exportedName(component.Name)
		if name == "" || name == "Action" {
			return nil, fmt.Errorf("component %q does not produce a valid Go identifier", component.Name)
		}
		if names[name] {
			return nil, fmt.Errorf("duplicate generated component name %q", name)
		}
		names[name] = true
		if err := writeComponent(&source, component, name); err != nil {
			return nil, err
		}
	}

	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format component package source: %w", err)
	}
	return formatted, nil
}

func writeComponent(source *strings.Builder, component manifest.Component, name string) error {
	propNames := map[string]bool{}
	source.WriteString("type ")
	source.WriteString(name)
	source.WriteString("Props struct {\n")
	for _, prop := range component.Props {
		if prop.Type != "string" {
			return fmt.Errorf("component %s prop %s uses unsupported type %q", component.Name, prop.Name, prop.Type)
		}
		if propNames[prop.Name] {
			return fmt.Errorf("component %s declares duplicate prop %q", component.Name, prop.Name)
		}
		propNames[prop.Name] = true
		source.WriteByte('\t')
		source.WriteString(exportedName(prop.Name))
		source.WriteString(" string\n")
	}
	source.WriteString("}\n\n")

	source.WriteString("func Render")
	source.WriteString(name)
	source.WriteString("(props ")
	source.WriteString(name)
	source.WriteString("Props) (string, error) {\n")
	source.WriteString("\tvar out gowdkrender.Builder\n")
	if err := writeInterpolatedView(source, component, propNames); err != nil {
		return err
	}
	source.WriteString("\treturn out.String(), nil\n")
	source.WriteString("}\n\n")
	return nil
}

func writeInterpolatedView(source *strings.Builder, component manifest.Component, props map[string]bool) error {
	parts, err := componentViewParts(component, props)
	if err != nil {
		return err
	}
	for _, part := range parts {
		if part.Prop != "" {
			source.WriteString("\tout.Text(props.")
			source.WriteString(exportedName(part.Prop))
			source.WriteString(")\n")
			continue
		}
		writeStringChunk(source, part.Static)
	}
	return nil
}

type componentViewPart struct {
	Static string
	Prop   string
}

func componentViewParts(component manifest.Component, props map[string]bool) ([]componentViewPart, error) {
	view := component.Blocks.ViewBody
	if strings.Contains(view, componentPropMarkerPrefix) {
		return nil, fmt.Errorf("component %s view contains reserved interpolation marker %q", component.Name, componentPropMarkerPrefix)
	}

	offset := 0
	var normalized strings.Builder
	var propMarkers []componentViewPart
	matches := interpolationPattern.FindAllStringSubmatchIndex(view, -1)
	for _, match := range matches {
		if match[0] > offset {
			normalized.WriteString(view[offset:match[0]])
		}
		prop := view[match[2]:match[3]]
		if !props[prop] {
			return nil, fmt.Errorf("component %s view references undeclared prop %q", component.Name, prop)
		}
		marker := componentPropMarkerPrefix + strconv.Itoa(len(propMarkers)) + "__"
		normalized.WriteString(marker)
		propMarkers = append(propMarkers, componentViewPart{Prop: prop})
		offset = match[1]
	}
	if offset < len(view) {
		normalized.WriteString(view[offset:])
	}

	rendered, err := viewpkg.RenderStatic(normalized.String())
	if err != nil {
		return nil, fmt.Errorf("component %s view: %w", component.Name, err)
	}

	parts := []componentViewPart{{Static: rendered}}
	for index, markerPart := range propMarkers {
		marker := componentPropMarkerPrefix + strconv.Itoa(index) + "__"
		parts = splitComponentViewParts(parts, marker, markerPart)
	}
	return parts, nil
}

func splitComponentViewParts(parts []componentViewPart, marker string, markerPart componentViewPart) []componentViewPart {
	var out []componentViewPart
	for _, part := range parts {
		if part.Prop != "" {
			out = append(out, part)
			continue
		}
		chunks := strings.Split(part.Static, marker)
		for index, chunk := range chunks {
			if chunk != "" {
				out = append(out, componentViewPart{Static: chunk})
			}
			if index < len(chunks)-1 {
				out = append(out, markerPart)
			}
		}
	}
	return out
}

func writeStringChunk(source *strings.Builder, value string) {
	if value == "" {
		return
	}
	source.WriteString("\tout.Static(")
	source.WriteString(strconv.Quote(value))
	source.WriteString(")\n")
}

func sortedComponents(components []manifest.Component) []manifest.Component {
	out := append([]manifest.Component(nil), components...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
