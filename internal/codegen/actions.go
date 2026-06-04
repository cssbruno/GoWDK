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

// ActionPackageOptions configures generated action handler package source.
type ActionPackageOptions struct {
	PackageName string
}

// GenerateActionPackage emits HTTP handlers for declared act {} blocks.
func GenerateActionPackage(config gowdk.Config, app manifest.Manifest, options ActionPackageOptions) ([]byte, error) {
	if err := compiler.ValidateManifest(config, app); err != nil {
		return nil, err
	}

	packageName := strings.TrimSpace(options.PackageName)
	if packageName == "" {
		packageName = "actions"
	}
	if !goIdentifierPattern.MatchString(packageName) {
		return nil, fmt.Errorf("invalid action package name %q", packageName)
	}

	handlers, err := actionHandlers(app)
	if err != nil {
		return nil, err
	}

	var source strings.Builder
	source.WriteString("package ")
	source.WriteString(packageName)
	source.WriteString("\n\n")
	if len(handlers) > 0 {
		source.WriteString("import (\n")
		source.WriteString("\t\"net/http\"\n\n")
		source.WriteString("\tgowdkactions \"github.com/gowdk/gowdk/addons/actions\"\n")
		source.WriteString("\tgowdkresponse \"github.com/gowdk/gowdk/runtime/response\"\n")
		source.WriteString(")\n\n")
		source.WriteString("var Registry = gowdkactions.Registry{}\n\n")
		source.WriteString("func writeActionError(w http.ResponseWriter, status int, message string) {\n")
		source.WriteString("\tw.Header().Set(\"Cache-Control\", \"no-store\")\n")
		source.WriteString("\thttp.Error(w, message, status)\n")
		source.WriteString("}\n\n")
	}
	for _, handler := range handlers {
		source.WriteString("func Register")
		source.WriteString(handler.Name)
		source.WriteString("(handler gowdkactions.Handler) {\n")
		source.WriteString("\tRegistry.Register(")
		source.WriteString(strconv.Quote(handler.Name))
		source.WriteString(", handler)\n")
		source.WriteString("}\n\n")
		source.WriteString("func ")
		source.WriteString(handler.Name)
		source.WriteString("(w http.ResponseWriter, r *http.Request) {\n")
		source.WriteString("\tvalues, err := gowdkactions.DecodeForm(r)\n")
		source.WriteString("\tif err != nil {\n")
		source.WriteString("\t\twriteActionError(w, http.StatusBadRequest, \"invalid action form\")\n")
		source.WriteString("\t\treturn\n")
		source.WriteString("\t}\n")
		source.WriteString("\thandler, ok := Registry[")
		source.WriteString(strconv.Quote(handler.Name))
		source.WriteString("]\n")
		source.WriteString("\tif !ok || handler == nil {\n")
		source.WriteString("\t\twriteActionError(w, http.StatusNotImplemented, ")
		source.WriteString(strconv.Quote("GOWDK action handler " + handler.Name + " is not registered"))
		source.WriteString(")\n")
		source.WriteString("\t\treturn\n")
		source.WriteString("\t}\n")
		source.WriteString("\tresult, err := handler(r.Context(), values)\n")
		source.WriteString("\tif err != nil {\n")
		source.WriteString("\t\twriteActionError(w, gowdkresponse.HandlerStatus(err, http.StatusInternalServerError), err.Error())\n")
		source.WriteString("\t\treturn\n")
		source.WriteString("\t}\n")
		source.WriteString("\tif err := gowdkresponse.WriteHTTP(w, result); err != nil {\n")
		source.WriteString("\t\twriteActionError(w, http.StatusInternalServerError, err.Error())\n")
		source.WriteString("\t}\n")
		source.WriteString("}\n\n")
	}

	formatted, err := format.Source([]byte(source.String()))
	if err != nil {
		return nil, fmt.Errorf("format action package source: %w", err)
	}
	return formatted, nil
}

type actionHandler struct {
	Name string
}

func actionHandlers(app manifest.Manifest) ([]actionHandler, error) {
	names := map[string]bool{}
	var handlers []actionHandler
	for _, page := range app.Pages {
		for _, action := range page.Blocks.Actions {
			name := exportedName(page.ID) + exportedName(action.Name)
			if name == "" || !goIdentifierPattern.MatchString(name) {
				return nil, fmt.Errorf("action %s.%s does not produce a valid Go identifier", page.ID, action.Name)
			}
			if names[name] {
				return nil, fmt.Errorf("duplicate generated action handler name %q", name)
			}
			names[name] = true
			handlers = append(handlers, actionHandler{Name: name})
		}
	}
	sort.Slice(handlers, func(i, j int) bool {
		return handlers[i].Name < handlers[j].Name
	})
	return handlers, nil
}
