package appgen

const actionRuntimeSource = `type formValues map[string][]string

type actionFragment struct {
	Target string
	HTML   string
}

func formValuesFromURLValues(values map[string][]string) formValues {
	out := formValues{}
	for key, list := range values {
		out[key] = append([]string(nil), list...)
	}
	return out
}

func decodeExpectedFields(values formValues, expected []string) (formValues, error) {
	allowed := map[string]bool{}
	for _, field := range expected {
		if field == "" {
			return nil, formDecodeError("expected form field name is required")
		}
		if allowed[field] {
			return nil, formDecodeError("duplicate expected form field")
		}
		allowed[field] = true
	}
	for field := range values {
		if !allowed[field] {
			return nil, formDecodeError("unexpected form field")
		}
	}
	out := formValues{}
	for _, field := range expected {
		if submitted, ok := values[field]; ok {
			out[field] = append([]string(nil), submitted...)
		}
	}
	return out, nil
}

func validateRequiredFields(values formValues, required []string) validationResult {
	var result validationResult
	for _, field := range required {
		if !hasSubmittedValue(values[field]) {
			result.Add(field, "required")
		}
	}
	return result
}

func hasSubmittedValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func isPartialRequest(request *http.Request) bool {
	value := strings.TrimSpace(request.Header.Get("X-GOWDK-Partial"))
	return value != "" && value != "0"
}

func writeActionFragment(response http.ResponseWriter, request *http.Request, fragments []actionFragment) bool {
	target := strings.TrimSpace(request.Header.Get("X-GOWDK-Target"))
	for _, fragment := range fragments {
		if target != "" && target != fragment.Target {
			continue
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("X-GOWDK-Fragment-Target", fragment.Target)
		if swap := partialSwapMode(request.Header.Get("X-GOWDK-Swap")); swap != "" {
			response.Header().Set("X-GOWDK-Fragment-Swap", swap)
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(fragment.HTML))
		return true
	}
	return false
}

func partialSwapMode(value string) string {
	switch strings.TrimSpace(value) {
	case "innerHTML", "outerHTML":
		return strings.TrimSpace(value)
	default:
		return ""
	}
}

type validationError struct {
	Field   string
	Message string
}

type validationResult struct {
	Errors []validationError
}

func (result *validationResult) Add(field, message string) {
	result.Errors = append(result.Errors, validationError{Field: field, Message: message})
}

func (result validationResult) OK() bool {
	return len(result.Errors) == 0
}

type formDecodeError string

func (err formDecodeError) Error() string {
	return string(err)
}

const (
	actionErrorInvalidForm      = "invalid form"
	actionErrorRequestTooLarge  = "request body too large"
	actionErrorValidationFailed = "validation failed"
	actionErrorFragmentNotFound = "partial fragment not found"
)

func writeActionError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Cache-Control", "no-store")
	http.Error(response, message, status)
}
`
