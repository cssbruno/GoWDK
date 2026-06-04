package validation

// Error describes one validation failure.
type Error struct {
	Field   string
	Message string
}

// Result collects validation errors for generated actions.
type Result struct {
	Errors []Error
}

// Add records a validation failure.
func (result *Result) Add(field, message string) {
	result.Errors = append(result.Errors, Error{Field: field, Message: message})
}

// OK reports whether no validation failures were recorded.
func (result Result) OK() bool {
	return len(result.Errors) == 0
}
