package appgen

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

func actionHandlerSource(actions []ActionRoute) string {
	if len(actions) == 0 {
		return emptyActionHandlerSource
	}

	sorted := sortedActionRoutes(actions)
	var builder strings.Builder
	builder.WriteString("func action(response http.ResponseWriter, request *http.Request) bool {\n")
	builder.WriteString("\trequestPath := actionRequestPath(request.URL.Path)\n")
	builder.WriteString("\tswitch requestPath {\n")
	for _, action := range sorted {
		writeActionCase(&builder, action)
	}
	builder.WriteString("\tdefault:\n")
	builder.WriteString("\t\treturn false\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}")
	builder.WriteString("\n\n")
	builder.WriteString(actionRequestPathSource)
	builder.WriteString("\n")
	builder.WriteString(actionDecoderSource(sorted))
	return builder.String()
}

const emptyActionHandlerSource = `func action(response http.ResponseWriter, request *http.Request) bool {
	return false
}`

const actionRequestPathSource = `func actionRequestPath(value string) string {
	return path.Clean("/" + value)
}
`

func actionsUseValidation(actions []ActionRoute) bool {
	for _, action := range actions {
		if action.ValidatesInput {
			return true
		}
	}
	return false
}

func sortedActionRoutes(actions []ActionRoute) []ActionRoute {
	sorted := append([]ActionRoute(nil), actions...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Route == sorted[j].Route {
			return sorted[i].ActionName < sorted[j].ActionName
		}
		return sorted[i].Route < sorted[j].Route
	})
	return sorted
}

func writeActionCase(builder *strings.Builder, action ActionRoute) {
	builder.WriteString("\tcase ")
	builder.WriteString(quote(action.Route))
	builder.WriteString(":\n")
	writeActionParseForm(builder)
	builder.WriteString("\t\tvalues := gowdkform.FromURLValues(request.PostForm)\n")
	writeActionInputDecode(builder, action)
	writeActionPartialBranch(builder, action)
	writeActionResult(builder, action)
	builder.WriteString("\t\treturn true\n")
}

func writeActionParseForm(builder *strings.Builder) {
	builder.WriteString("\t\trequest.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)\n")
	builder.WriteString("\t\tif err := request.ParseForm(); err != nil {\n")
	builder.WriteString("\t\t\tif strings.Contains(err.Error(), \"request body too large\") {\n")
	builder.WriteString("\t\t\t\tgowdkresponse.WriteNoStoreError(response, http.StatusRequestEntityTooLarge, \"request body too large\")\n")
	builder.WriteString("\t\t\t\treturn true\n")
	builder.WriteString("\t\t\t}\n")
	builder.WriteString("\t\t\tgowdkresponse.WriteNoStoreError(response, http.StatusBadRequest, \"invalid form\")\n")
	builder.WriteString("\t\t\treturn true\n")
	builder.WriteString("\t\t}\n")
}

func writeActionInputDecode(builder *strings.Builder, action ActionRoute) {
	if action.InputType == "" {
		builder.WriteString("\t\tif _, err := gowdkform.DecodeExpected(values, ")
		builder.WriteString(formSchemaLiteral(action.InputFields))
		builder.WriteString("); err != nil {\n")
		builder.WriteString("\t\t\tgowdkresponse.WriteNoStoreError(response, http.StatusBadRequest, \"invalid form\")\n")
		builder.WriteString("\t\t\treturn true\n")
		builder.WriteString("\t\t}\n")
		return
	}

	builder.WriteString("\t\tinput, err := ")
	builder.WriteString(actionDecoderName(action))
	builder.WriteString("(values)\n")
	builder.WriteString("\t\tif err != nil {\n")
	builder.WriteString("\t\t\tgowdkresponse.WriteNoStoreError(response, http.StatusBadRequest, \"invalid form\")\n")
	builder.WriteString("\t\t\treturn true\n")
	builder.WriteString("\t\t}\n")
	builder.WriteString("\t\t_ = input\n")
	if action.ValidatesInput {
		builder.WriteString("\t\tvalidation := gowdkvalidation.Result{}\n")
		builder.WriteString("\t\tfor _, field := range ")
		builder.WriteString(stringSliceLiteral(action.RequiredFields))
		builder.WriteString(" {\n")
		builder.WriteString("\t\t\tif !input.Values.HasSubmitted(field) {\n")
		builder.WriteString("\t\t\t\tvalidation.Add(field, \"required\")\n")
		builder.WriteString("\t\t\t}\n")
		builder.WriteString("\t\t}\n")
		builder.WriteString("\t\tif !validation.OK() {\n")
		builder.WriteString("\t\t\tgowdkresponse.WriteNoStoreError(response, http.StatusUnprocessableEntity, \"validation failed\")\n")
		builder.WriteString("\t\t\treturn true\n")
		builder.WriteString("\t\t}\n")
	}
}

func writeActionPartialBranch(builder *strings.Builder, action ActionRoute) {
	if len(action.Fragments) == 0 {
		writeActionPartialRequestCondition(builder)
		builder.WriteString("\t\t\tgowdkresponse.WriteNoStoreError(response, http.StatusBadRequest, \"partial fragment not found\")\n")
		builder.WriteString("\t\t\treturn true\n")
		builder.WriteString("\t\t}\n")
		return
	}

	writeActionPartialRequestCondition(builder)
	builder.WriteString("\t\t\ttarget := strings.TrimSpace(request.Header.Get(\"X-GOWDK-Target\"))\n")
	for index, fragment := range action.Fragments {
		if index == 0 {
			builder.WriteString("\t\t\tswitch target {\n")
			builder.WriteString("\t\t\tcase \"\", ")
		} else {
			builder.WriteString("\t\t\tcase ")
		}
		builder.WriteString(goString(fragment.Target))
		builder.WriteString(":\n")
		builder.WriteString("\t\t\t\tfragment := gowdkresponse.Response{Kind: gowdkresponse.Fragment, Status: http.StatusOK, Target: ")
		builder.WriteString(goString(fragment.Target))
		builder.WriteString(", Body: ")
		builder.WriteString(goString(fragment.HTML))
		builder.WriteString("}\n")
		builder.WriteString("\t\t\t\tif swap := strings.TrimSpace(request.Header.Get(\"X-GOWDK-Swap\")); swap != \"\" {\n")
		builder.WriteString("\t\t\t\t\tif swapped, err := gowdkresponse.FragmentSwap(fragment.Target, gowdkresponse.SwapMode(swap), fragment.Body); err == nil {\n")
		builder.WriteString("\t\t\t\t\t\tfragment = swapped\n")
		builder.WriteString("\t\t\t\t\t}\n")
		builder.WriteString("\t\t\t\t}\n")
		builder.WriteString("\t\t\t\t_ = gowdkresponse.WriteNoStoreHTTP(response, fragment)\n")
		builder.WriteString("\t\t\t\treturn true\n")
	}
	builder.WriteString("\t\t\tdefault:\n")
	builder.WriteString("\t\t\t\tgowdkresponse.WriteNoStoreError(response, http.StatusNotFound, \"partial fragment not found\")\n")
	builder.WriteString("\t\t\t\treturn true\n")
	builder.WriteString("\t\t\t}\n")
	builder.WriteString("\t\t}\n")
}

func writeActionPartialRequestCondition(builder *strings.Builder) {
	builder.WriteString("\t\tpartial := strings.TrimSpace(request.Header.Get(\"X-GOWDK-Partial\"))\n")
	builder.WriteString("\t\tif partial != \"\" && partial != \"0\" {\n")
}

func writeActionResult(builder *strings.Builder, action ActionRoute) {
	if strings.TrimSpace(action.Redirect) == "" {
		builder.WriteString("\t\tresponse.WriteHeader(http.StatusNoContent)\n")
		return
	}
	builder.WriteString("\t\t_ = gowdkresponse.WriteHTTP(response, gowdkresponse.RedirectTo(")
	builder.WriteString(quote(action.Redirect))
	builder.WriteString("))\n")
}

func actionDecoderSource(actions []ActionRoute) string {
	var builder strings.Builder
	inputTypes := uniqueInputTypes(actions)
	for _, inputType := range inputTypes {
		builder.WriteString("type ")
		builder.WriteString(inputType)
		builder.WriteString(" struct {\n\tValues gowdkform.Values\n}\n\n")
	}
	for _, action := range actions {
		if action.InputType == "" {
			continue
		}
		builder.WriteString("func ")
		builder.WriteString(actionDecoderName(action))
		builder.WriteString("(values gowdkform.Values) (")
		builder.WriteString(action.InputType)
		builder.WriteString(", error) {\n")
		builder.WriteString("\tdecoded, err := gowdkform.DecodeExpected(values, ")
		builder.WriteString(formSchemaLiteral(action.InputFields))
		builder.WriteString(")\n")
		builder.WriteString("\tif err != nil {\n")
		builder.WriteString("\t\treturn ")
		builder.WriteString(action.InputType)
		builder.WriteString("{}, err\n")
		builder.WriteString("\t}\n")
		builder.WriteString("\treturn ")
		builder.WriteString(action.InputType)
		builder.WriteString("{Values: decoded}, nil\n")
		builder.WriteString("}\n\n")
	}
	return strings.TrimSpace(builder.String())
}

func uniqueInputTypes(actions []ActionRoute) []string {
	seen := map[string]bool{}
	var types []string
	for _, action := range actions {
		if action.InputType == "" || seen[action.InputType] {
			continue
		}
		seen[action.InputType] = true
		types = append(types, action.InputType)
	}
	sort.Strings(types)
	return types
}

func actionDecoderName(action ActionRoute) string {
	return "decode" + exportedIdentifier(action.PageID) + exportedIdentifier(action.ActionName) + "Input"
}

func exportedIdentifier(value string) string {
	var builder strings.Builder
	upperNext := true
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
		if !valid {
			upperNext = true
			continue
		}
		if builder.Len() == 0 && char >= '0' && char <= '9' {
			builder.WriteByte('X')
		}
		if upperNext && char >= 'a' && char <= 'z' {
			char -= 'a' - 'A'
		}
		builder.WriteRune(char)
		upperNext = false
	}
	if builder.Len() == 0 {
		return "Action"
	}
	return builder.String()
}

func stringSliceLiteral(values []string) string {
	if len(values) == 0 {
		return "nil"
	}
	var builder strings.Builder
	builder.WriteString("[]string{")
	for index, value := range values {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("%q", value))
	}
	builder.WriteString("}")
	return builder.String()
}

func formSchemaLiteral(fields []string) string {
	var builder strings.Builder
	builder.WriteString("gowdkform.Schema{Fields: []gowdkform.Field{")
	for index, field := range fields {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString("{Name: ")
		builder.WriteString(goString(field))
		builder.WriteString("}")
	}
	builder.WriteString("}}")
	return builder.String()
}

func goString(value string) string {
	return fmt.Sprintf("%q", value)
}

func quote(value string) string {
	return fmt.Sprintf("%q", path.Clean("/"+value))
}
