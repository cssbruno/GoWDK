package codegen

import (
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"

	"github.com/gowdk/gowdk/internal/manifest"
)

// APIPackageOptions configures generated API handler package source.
type APIPackageOptions struct {
	PackageName string
}

// GenerateAPIPackage emits Go HTTP handler stubs for declared api {} blocks.
// The stubs are deliberate 501 responses until user/application logic is wired.
func GenerateAPIPackage(app manifest.Manifest, options APIPackageOptions) ([]byte, error) {
	packageName := strings.TrimSpace(options.PackageName)
	if packageName == "" {
		packageName = "api"
	}
	if !goIdentifierPattern.MatchString(packageName) {
		return nil, fmt.Errorf("invalid API package name %q", packageName)
	}

	handlers, err := apiHandlers(app)
	if err != nil {
		return nil, err
	}

	var source strings.Builder
	source.WriteString("package ")
	source.WriteString(packageName)
	source.WriteString("\n\n")
	if len(handlers) > 0 {
		source.WriteString("import \"net/http\"\n\n")
	}
	for _, handler := range handlers {
		source.WriteString("func ")
		source.WriteString(handler.Name)
		source.WriteString("(w http.ResponseWriter, r *http.Request) {\n")
		source.WriteString("\thttp.Error(w, ")
		source.WriteString(strconv.Quote("GOWDK API handler " + handler.Name + " is not implemented"))
		source.WriteString(", http.StatusNotImplemented)\n")
		source.WriteString("}\n\n")
	}

	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format API package source: %w", err)
	}
	return formatted, nil
}

type apiHandler struct {
	Name string
}

func apiHandlers(app manifest.Manifest) ([]apiHandler, error) {
	names := map[string]bool{}
	var handlers []apiHandler
	for _, page := range app.Pages {
		for _, api := range page.Blocks.APIs {
			name := exportedName(page.ID)
			if api.Name != "" {
				name += exportedName(api.Name)
			}
			if name == "" {
				return nil, fmt.Errorf("api on page %q does not produce a valid Go identifier", page.ID)
			}
			if names[name] {
				return nil, fmt.Errorf("duplicate generated API handler name %q", name)
			}
			names[name] = true
			handlers = append(handlers, apiHandler{Name: name})
		}
	}
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].Name < handlers[j].Name
	})
	return handlers, nil
}
