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

// Messages returns validation messages in insertion order.
func (result Result) Messages() []string {
	messages := make([]string, 0, len(result.Errors))
	for _, validationErr := range result.Errors {
		messages = append(messages, validationErr.Message)
	}
	return messages
}

// FieldMessages returns messages for one submitted field.
func (result Result) FieldMessages(field string) []string {
	var messages []string
	for _, validationErr := range result.Errors {
		if validationErr.Field == field {
			messages = append(messages, validationErr.Message)
		}
	}
	return messages
}

// ByField groups validation messages by field while preserving message order.
func (result Result) ByField() map[string][]string {
	fields := map[string][]string{}
	for _, validationErr := range result.Errors {
		fields[validationErr.Field] = append(fields[validationErr.Field], validationErr.Message)
	}
	return fields
}
