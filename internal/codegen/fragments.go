package codegen

import (
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/gowdk/gowdk/internal/manifest"
)

// FragmentPackageOptions configures generated server fragment package source.
type FragmentPackageOptions struct {
	PackageName string
}

// GenerateFragmentPackage emits Go render functions for parsed server
// fragments. Each function returns the runtime fragment response envelope.
func GenerateFragmentPackage(app manifest.Manifest, options FragmentPackageOptions) ([]byte, error) {
	packageName := strings.TrimSpace(options.PackageName)
	if packageName == "" {
		packageName = "fragments"
	}
	if !goIdentifierPattern.MatchString(packageName) {
		return nil, fmt.Errorf("invalid fragment package name %q", packageName)
	}

	fragments, err := fragmentRenderers(app)
	if err != nil {
		return nil, err
	}

	var source strings.Builder
	source.WriteString("package ")
	source.WriteString(packageName)
	source.WriteString("\n\n")
	if len(fragments) > 0 {
		source.WriteString("import (\n")
		source.WriteString("\t\"net/http\"\n")
		source.WriteString("\tgowdkresponse \"github.com/gowdk/gowdk/runtime/response\"\n")
		source.WriteString(")\n\n")
	}
	for _, fragment := range fragments {
		source.WriteString("func ")
		source.WriteString(fragment.Name)
		source.WriteString("() gowdkresponse.Response {\n")
		source.WriteString("\treturn gowdkresponse.FragmentFor(")
		source.WriteString(strconv.Quote(fragment.Target))
		source.WriteString(", ")
		source.WriteString(strconv.Quote(fragment.Body))
		source.WriteString(")\n")
		source.WriteString("}\n\n")
		source.WriteString("func Handle")
		source.WriteString(strings.TrimPrefix(fragment.Name, "Render"))
		source.WriteString("(w http.ResponseWriter, r *http.Request) {\n")
		source.WriteString("\t_ = gowdkresponse.WriteHTTP(w, ")
		source.WriteString(fragment.Name)
		source.WriteString("())\n")
		source.WriteString("}\n\n")
	}

	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format fragment package source: %w", err)
	}
	return formatted, nil
}

type fragmentRenderer struct {
	Name   string
	Target string
	Body   string
}

func fragmentRenderers(app manifest.Manifest) ([]fragmentRenderer, error) {
	names := map[string]bool{}
	var renderers []fragmentRenderer
	for _, page := range app.Pages {
		for _, action := range page.Blocks.Actions {
			for _, fragment := range action.Fragments {
				targetName := fragmentTargetName(fragment.Target)
				if targetName == "" {
					return nil, fmt.Errorf("fragment target %q does not produce a valid Go identifier", fragment.Target)
				}
				name := "Render" + exportedName(page.ID) + exportedName(action.Name) + targetName
				if !goIdentifierPattern.MatchString(name) {
					return nil, fmt.Errorf("fragment %q does not produce a valid Go identifier", fragment.Target)
				}
				if names[name] {
					return nil, fmt.Errorf("duplicate generated fragment renderer name %q", name)
				}
				names[name] = true
				renderers = append(renderers, fragmentRenderer{Name: name, Target: fragment.Target, Body: fragment.Body})
			}
		}
	}
	sort.Slice(renderers, func(i, j int) bool {
		return renderers[i].Name < renderers[j].Name
	})
	return renderers, nil
}

func fragmentTargetName(target string) string {
	var out strings.Builder
	upperNext := true
	for _, r := range target {
		switch {
		case unicode.IsLetter(r):
			if upperNext {
				out.WriteRune(unicode.ToUpper(r))
				upperNext = false
				continue
			}
			out.WriteRune(r)
		case unicode.IsDigit(r):
			if out.Len() == 0 {
				continue
			}
			out.WriteRune(r)
			upperNext = false
		default:
			upperNext = true
		}
	}
	return out.String()
}
