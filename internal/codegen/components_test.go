package codegen

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/gowdk/gowdk/internal/manifest"
)

func TestGenerateComponentPackageEmitsFormattedGoSource(t *testing.T) {
	source, err := GenerateComponentPackage([]manifest.Component{
		{
			Name:  "Hero",
			Props: []manifest.Prop{{Name: "title", Type: "string"}, {Name: "tagline", Type: "string"}},
			Blocks: manifest.Blocks{
				ViewBody: `<section><h1>{title}</h1><p>{tagline}</p></section>`,
			},
		},
	}, ComponentPackageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "components.go", source, parser.AllErrors); err != nil {
		t.Fatalf("generated component package is not valid Go: %v\n%s", err, source)
	}

	text := string(source)
	for _, want := range []string{
		`package components`,
		`gowdkrender "github.com/gowdk/gowdk/runtime/render"`,
		`type HeroProps struct`,
		`Title   string`,
		`Tagline string`,
		`func RenderHero(props HeroProps) (string, error)`,
		`out.Text(props.Title)`,
		`out.Text(props.Tagline)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated source to contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateComponentPackageMatchesGoldenSource(t *testing.T) {
	source, err := GenerateComponentPackage([]manifest.Component{{
		Name:  "Hero",
		Props: []manifest.Prop{{Name: "title", Type: "string"}, {Name: "tagline", Type: "string"}},
		Blocks: manifest.Blocks{
			ViewBody: `<section><h1>{title}</h1><p>{tagline}</p></section>`,
		},
	}}, ComponentPackageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/components.golden.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != string(expected) {
		t.Fatalf("component package golden mismatch\nexpected:\n%s\nactual:\n%s", expected, source)
	}
}

func TestGenerateComponentPackageSortsComponents(t *testing.T) {
	source, err := GenerateComponentPackage([]manifest.Component{
		{Name: "Stats", Blocks: manifest.Blocks{ViewBody: `<section>Stats</section>`}},
		{Name: "Hero", Blocks: manifest.Blocks{ViewBody: `<section>Hero</section>`}},
	}, ComponentPackageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Index(text, "RenderHero") > strings.Index(text, "RenderStats") {
		t.Fatalf("expected components to be sorted:\n%s", text)
	}
}

func TestGenerateComponentPackagePreservesClassShorthand(t *testing.T) {
	source, err := GenerateComponentPackage([]manifest.Component{
		{
			Name:  "Hero",
			Props: []manifest.Prop{{Name: "title", Type: "string"}},
			Blocks: manifest.Blocks{
				ViewBody: `<section .hero-card .featured class="lead"><h1>{title}</h1></section>`,
			},
		},
	}, ComponentPackageOptions{})
	if err != nil {
		t.Fatal(err)
	}

	text := string(source)
	for _, want := range []string{
		`out.Static("<section class=\"hero-card featured lead\"><h1>")`,
		`out.Text(props.Title)`,
		`out.Static("</h1></section>")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated source to contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateComponentPackageUsesCustomPackageName(t *testing.T) {
	source, err := GenerateComponentPackage([]manifest.Component{
		{Name: "Hero", Blocks: manifest.Blocks{ViewBody: `<section>Hero</section>`}},
	}, ComponentPackageOptions{PackageName: "ui"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(source), "package ui\n") {
		t.Fatalf("expected custom package name:\n%s", source)
	}
}

func TestGenerateComponentPackageRejectsInvalidPackageName(t *testing.T) {
	_, err := GenerateComponentPackage(nil, ComponentPackageOptions{PackageName: "bad-name"})
	if err == nil {
		t.Fatal("expected invalid package name error")
	}
	if !strings.Contains(err.Error(), `invalid component package name "bad-name"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateComponentPackageRejectsInvalidComponentShapes(t *testing.T) {
	tests := []struct {
		name       string
		components []manifest.Component
		want       string
	}{
		{
			name: "unsupported prop type",
			components: []manifest.Component{{
				Name:  "Hero",
				Props: []manifest.Prop{{Name: "count", Type: "int"}},
			}},
			want: `component Hero prop count uses unsupported type "int"`,
		},
		{
			name: "duplicate prop",
			components: []manifest.Component{{
				Name:  "Hero",
				Props: []manifest.Prop{{Name: "title", Type: "string"}, {Name: "title", Type: "string"}},
			}},
			want: `component Hero declares duplicate prop "title"`,
		},
		{
			name: "reserved generated name",
			components: []manifest.Component{{
				Name: "Action",
			}},
			want: `component "Action" does not produce a valid Go identifier`,
		},
		{
			name: "duplicate generated name",
			components: []manifest.Component{
				{Name: "Hero"},
				{Name: "hero"},
			},
			want: `duplicate generated component name "Hero"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateComponentPackage(tt.components, ComponentPackageOptions{})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q in %v", tt.want, err)
			}
		})
	}
}

func TestGenerateComponentPackageRejectsUndeclaredPropInterpolation(t *testing.T) {
	_, err := GenerateComponentPackage([]manifest.Component{{
		Name:  "Hero",
		Props: []manifest.Prop{{Name: "title", Type: "string"}},
		Blocks: manifest.Blocks{
			ViewBody: `<section>{missing}</section>`,
		},
	}}, ComponentPackageOptions{})
	if err == nil {
		t.Fatal("expected undeclared prop error")
	}
	if !strings.Contains(err.Error(), `component Hero view references undeclared prop "missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
