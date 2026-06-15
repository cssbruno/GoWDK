package addonregistry

import (
	"strings"
	"testing"
)

func TestBundledRegistryValidatesAndSorts(t *testing.T) {
	registry, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	if registry.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("unexpected schema version: %d", registry.SchemaVersion)
	}
	if len(registry.Addons) == 0 {
		t.Fatal("expected bundled addon entries")
	}
	for index := 1; index < len(registry.Addons); index++ {
		if registry.Addons[index-1].Name > registry.Addons[index].Name {
			t.Fatalf("registry not sorted: %q before %q", registry.Addons[index-1].Name, registry.Addons[index].Name)
		}
	}
}

func TestValidateRejectsInvalidMetadata(t *testing.T) {
	errors := Validate(Registry{
		SchemaVersion: CurrentSchemaVersion,
		Addons: []Entry{
			{
				Name:          "brand",
				Summary:       "brand addon",
				Kind:          "documented-external",
				Lifecycle:     "stable",
				Compatibility: "compatible",
				ModulePath:    "github.com/example/gowdk-brand",
				PackagePath:   ".",
				ImportPath:    "github.com/example/gowdk-brand",
				Owner:         "Example",
				Trust:         Trust{Level: "documented-external", Notes: "docs only"},
				Constructor:   Constructor{Addable: true, Package: "brand", Function: "Addon"},
			},
		},
	})
	joined := joinErrors(errors)
	for _, expected := range []string{
		"sourceRepository is required",
		"license is required",
		"documentation is required",
		"documented external addons must not be addable",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q in validation errors:\n%s", expected, joined)
		}
	}
}

func joinErrors(errors []error) string {
	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}
