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
	builder.WriteString("func (handler staticHandler) action(response http.ResponseWriter, request *http.Request) bool {\n")
	builder.WriteString("\tswitch request.URL.Path {\n")
	for _, action := range sorted {
		writeActionCase(&builder, action)
	}
	builder.WriteString("\tdefault:\n")
	builder.WriteString("\t\treturn false\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}")
	builder.WriteString("\n\n")
	builder.WriteString(actionDecoderSource(sorted))
	return builder.String()
}

const emptyActionHandlerSource = `func (handler staticHandler) action(response http.ResponseWriter, request *http.Request) bool {
	return false
}`

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
	builder.WriteString("\t\tvalues := formValuesFromURLValues(request.PostForm)\n")
	writeActionInputDecode(builder, action)
	writeActionPartialBranch(builder, action)
	writeActionResult(builder, action)
	builder.WriteString("\t\treturn true\n")
}

func writeActionParseForm(builder *strings.Builder) {
	builder.WriteString("\t\trequest.Body = http.MaxBytesReader(response, request.Body, maxActionBodyBytes)\n")
	builder.WriteString("\t\tif err := request.ParseForm(); err != nil {\n")
	builder.WriteString("\t\t\tif strings.Contains(err.Error(), \"request body too large\") {\n")
	builder.WriteString("\t\t\t\twriteActionError(response, http.StatusRequestEntityTooLarge, actionErrorRequestTooLarge)\n")
	builder.WriteString("\t\t\t\treturn true\n")
	builder.WriteString("\t\t\t}\n")
	builder.WriteString("\t\t\twriteActionError(response, http.StatusBadRequest, actionErrorInvalidForm)\n")
	builder.WriteString("\t\t\treturn true\n")
	builder.WriteString("\t\t}\n")
}

func writeActionInputDecode(builder *strings.Builder, action ActionRoute) {
	if action.InputType == "" {
		builder.WriteString("\t\tif _, err := decodeExpectedFields(values, ")
		builder.WriteString(stringSliceLiteral(action.InputFields))
		builder.WriteString("); err != nil {\n")
		builder.WriteString("\t\t\twriteActionError(response, http.StatusBadRequest, actionErrorInvalidForm)\n")
		builder.WriteString("\t\t\treturn true\n")
		builder.WriteString("\t\t}\n")
		return
	}

	builder.WriteString("\t\tinput, err := ")
	builder.WriteString(actionDecoderName(action))
	builder.WriteString("(values)\n")
	builder.WriteString("\t\tif err != nil {\n")
	builder.WriteString("\t\t\twriteActionError(response, http.StatusBadRequest, actionErrorInvalidForm)\n")
	builder.WriteString("\t\t\treturn true\n")
	builder.WriteString("\t\t}\n")
	builder.WriteString("\t\t_ = input\n")
	if action.ValidatesInput {
		builder.WriteString("\t\tvalidation := validateRequiredFields(input.Values, ")
		builder.WriteString(stringSliceLiteral(action.RequiredFields))
		builder.WriteString(")\n")
		builder.WriteString("\t\tif !validation.OK() {\n")
		builder.WriteString("\t\t\twriteActionError(response, http.StatusUnprocessableEntity, actionErrorValidationFailed)\n")
		builder.WriteString("\t\t\treturn true\n")
		builder.WriteString("\t\t}\n")
	}
}

func writeActionPartialBranch(builder *strings.Builder, action ActionRoute) {
	if len(action.Fragments) == 0 {
		builder.WriteString("\t\tif isPartialRequest(request) {\n")
		builder.WriteString("\t\t\twriteActionError(response, http.StatusBadRequest, actionErrorFragmentNotFound)\n")
		builder.WriteString("\t\t\treturn true\n")
		builder.WriteString("\t\t}\n")
		return
	}

	builder.WriteString("\t\tif isPartialRequest(request) {\n")
	builder.WriteString("\t\t\tif writeActionFragment(response, request, ")
	builder.WriteString(actionFragmentSliceLiteral(action.Fragments))
	builder.WriteString(") {\n")
	builder.WriteString("\t\t\t\treturn true\n")
	builder.WriteString("\t\t\t}\n")
	builder.WriteString("\t\t\twriteActionError(response, http.StatusNotFound, actionErrorFragmentNotFound)\n")
	builder.WriteString("\t\t\treturn true\n")
	builder.WriteString("\t\t}\n")
}

func writeActionResult(builder *strings.Builder, action ActionRoute) {
	if strings.TrimSpace(action.Redirect) == "" {
		builder.WriteString("\t\tresponse.WriteHeader(http.StatusNoContent)\n")
		return
	}
	builder.WriteString("\t\thttp.Redirect(response, request, ")
	builder.WriteString(quote(action.Redirect))
	builder.WriteString(", http.StatusSeeOther)\n")
}

func actionDecoderSource(actions []ActionRoute) string {
	var builder strings.Builder
	inputTypes := uniqueInputTypes(actions)
	for _, inputType := range inputTypes {
		builder.WriteString("type ")
		builder.WriteString(inputType)
		builder.WriteString(" struct {\n\tValues formValues\n}\n\n")
	}
	for _, action := range actions {
		if action.InputType == "" {
			continue
		}
		builder.WriteString("func ")
		builder.WriteString(actionDecoderName(action))
		builder.WriteString("(values formValues) (")
		builder.WriteString(action.InputType)
		builder.WriteString(", error) {\n")
		builder.WriteString("\tdecoded, err := decodeExpectedFields(values, ")
		builder.WriteString(stringSliceLiteral(action.InputFields))
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
	builder.WriteString(actionRuntimeSource)
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

func actionFragmentSliceLiteral(fragments []ActionFragment) string {
	if len(fragments) == 0 {
		return "nil"
	}
	var builder strings.Builder
	builder.WriteString("[]actionFragment{")
	for index, fragment := range fragments {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString("{Target: ")
		builder.WriteString(goString(fragment.Target))
		builder.WriteString(", HTML: ")
		builder.WriteString(goString(fragment.HTML))
		builder.WriteString("}")
	}
	builder.WriteString("}")
	return builder.String()
}

func goString(value string) string {
	return fmt.Sprintf("%q", value)
}

func quote(value string) string {
	return fmt.Sprintf("%q", path.Clean("/"+value))
}
