package appgen

import (
	view "github.com/cssbruno/gowdk/internal/viewrender"
)

func actionInputFields(fields []view.ActionFormField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

func actionRequiredFields(fields []view.ActionFormField) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Required {
			names = append(names, field.Name)
		}
	}
	return names
}

func actionRequiredMessages(fields []view.ActionFormField) map[string]string {
	messages := map[string]string{}
	for _, field := range fields {
		if field.Required && field.RequiredMessage != "" {
			messages[field.Name] = field.RequiredMessage
		}
	}
	if len(messages) == 0 {
		return nil
	}
	return messages
}

func actionValidationRules(fields []view.ActionFormField) []ActionValidationRule {
	rules := make([]ActionValidationRule, 0, len(fields))
	for _, field := range fields {
		if field.MinLength == 0 && field.MaxLength == 0 && field.Pattern == "" {
			continue
		}
		rules = append(rules, ActionValidationRule{
			Field:            field.Name,
			MinLength:        field.MinLength,
			MinLengthMessage: field.MinLengthMessage,
			MaxLength:        field.MaxLength,
			MaxLengthMessage: field.MaxLengthMessage,
			Pattern:          field.Pattern,
			PatternMessage:   field.PatternMessage,
		})
	}
	return rules
}
