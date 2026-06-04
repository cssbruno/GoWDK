package codegen

import (
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/compiler"
	"github.com/gowdk/gowdk/internal/manifest"
)

// SSRPackageOptions configures generated SSR handler package source.
type SSRPackageOptions struct {
	PackageName string
}

// GenerateSSRPackage emits HTTP handler stubs for pages that are explicitly
// request-time full-page routes: @render ssr and accepted @render hybrid pages.
func GenerateSSRPackage(config gowdk.Config, app manifest.Manifest, options SSRPackageOptions) ([]byte, error) {
	if err := compiler.ValidateManifest(config, app); err != nil {
		return nil, err
	}

	packageName := strings.TrimSpace(options.PackageName)
	if packageName == "" {
		packageName = "ssr"
	}
	if !goIdentifierPattern.MatchString(packageName) {
		return nil, fmt.Errorf("invalid SSR package name %q", packageName)
	}

	handlers, err := ssrHandlers(config, app)
	if err != nil {
		return nil, err
	}

	var source strings.Builder
	source.WriteString("package ")
	source.WriteString(packageName)
	source.WriteString("\n\n")
	imports := ssrImports(handlers)
	if len(imports) == 1 {
		source.WriteString("import ")
		source.WriteString(strconv.Quote(imports[0]))
		source.WriteString("\n\n")
	} else if len(imports) > 1 {
		source.WriteString("import (\n")
		for _, item := range imports {
			source.WriteString("\t")
			source.WriteString(strconv.Quote(item))
			source.WriteString("\n")
		}
		source.WriteString(")\n\n")
	}
	for _, handler := range handlers {
		if handler.LoadName != "" {
			source.WriteString("var _ ssr.LoadFunc = ")
			source.WriteString(handler.LoadName)
			source.WriteString("\n\n")
			source.WriteString("func ")
			source.WriteString(handler.LoadName)
			source.WriteString("(ctx ssr.LoadContext) (map[string]any, error) {\n")
			source.WriteString("\t_ = ctx\n")
			source.WriteString("\treturn nil, fmt.Errorf(")
			source.WriteString(strconv.Quote("GOWDK load function " + handler.LoadName + " is not implemented"))
			source.WriteString(")\n")
			source.WriteString("}\n\n")
		}
		source.WriteString("func ")
		source.WriteString(handler.Name)
		source.WriteString("(w http.ResponseWriter, r *http.Request) {\n")
		if handler.LoadName != "" {
			source.WriteString("\t_, _ = ")
			source.WriteString(handler.LoadName)
			source.WriteString("(ssr.NewLoadContext(r, nil))\n")
		}
		source.WriteString("\t_, _ = (render.Renderer{}).Render(r.Context())\n")
		source.WriteString("\thttp.Error(w, ")
		source.WriteString(strconv.Quote("GOWDK SSR handler " + handler.Name + " is not implemented"))
		source.WriteString(", http.StatusNotImplemented)\n")
		source.WriteString("}\n\n")
	}

	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format SSR package source: %w", err)
	}
	return formatted, nil
}

type ssrHandler struct {
	Name     string
	LoadName string
}

func ssrHandlers(config gowdk.Config, app manifest.Manifest) ([]ssrHandler, error) {
	names := map[string]bool{}
	var handlers []ssrHandler
	for _, page := range app.Pages {
		switch page.RenderMode(config.Render.DefaultMode()) {
		case gowdk.SSR, gowdk.Hybrid:
		default:
			continue
		}
		name := "Render" + exportedName(page.ID)
		if name == "Render" || !goIdentifierPattern.MatchString(name) {
			return nil, fmt.Errorf("page %q does not produce a valid SSR handler name", page.ID)
		}
		if names[name] {
			return nil, fmt.Errorf("duplicate generated SSR handler name %q", name)
		}
		names[name] = true
		handler := ssrHandler{Name: name}
		if page.Blocks.Load {
			handler.LoadName = "Load" + exportedName(page.ID)
			if handler.LoadName == "Load" || !goIdentifierPattern.MatchString(handler.LoadName) {
				return nil, fmt.Errorf("page %q does not produce a valid SSR load function name", page.ID)
			}
			if names[handler.LoadName] {
				return nil, fmt.Errorf("duplicate generated SSR load function name %q", handler.LoadName)
			}
			names[handler.LoadName] = true
		}
		handlers = append(handlers, handler)
	}
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].Name < handlers[j].Name
	})
	return handlers, nil
}

func ssrImports(handlers []ssrHandler) []string {
	if len(handlers) == 0 {
		return nil
	}
	imports := []string{"net/http"}
	for _, handler := range handlers {
		if handler.LoadName != "" {
			return []string{
				"fmt",
				"net/http",
				"github.com/gowdk/gowdk/addons/ssr",
				"github.com/gowdk/gowdk/runtime/render",
			}
		}
	}
	return append(imports, "github.com/gowdk/gowdk/runtime/render")
}
