package view

import (
	"fmt"
	"strconv"
	"strings"
)

func validateActionForm(element Element) error {
	for _, attr := range element.Attrs {
		if attr.Name != "enctype" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			continue
		}
		value := strings.TrimSpace(attr.Value)
		if strings.ContainsAny(value, "{}") {
			return fmt.Errorf("action form enctype %q must be literal", value)
		}
		if strings.EqualFold(value, "multipart/form-data") {
			return fmt.Errorf("multipart action forms are not supported before upload security rules are defined")
		}
	}
	return nil
}

func collectNamedControls(nodes []Node, fields map[string]ActionFormField) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			if field, ok, err := controlField(typed); err != nil {
				return err
			} else if ok {
				previous := fields[field.Name]
				var err error
				field, err = mergeActionFormField(previous, field)
				if err != nil {
					return err
				}
				fields[field.Name] = field
			}
			if err := collectNamedControls(typed.Children, fields); err != nil {
				return err
			}
		case ComponentCall:
			if err := collectNamedControls(typed.Children, fields); err != nil {
				return err
			}
		}
	}
	return nil
}

func controlField(element Element) (ActionFormField, bool, error) {
	switch element.Name {
	case "button", "input", "textarea", "select":
	default:
		return ActionFormField{}, false, nil
	}
	var field ActionFormField
	controlType := ""
	for _, attr := range element.Attrs {
		if attr.Name == "required" && element.Name != "button" {
			field.Required = true
			continue
		}
		switch attr.Name {
		case "minlength":
			value, ok, err := literalConstraintValue(element, attr)
			if err != nil {
				return ActionFormField{}, false, err
			}
			if ok {
				field.MinLength, err = parseLengthConstraint("minlength", value)
				if err != nil {
					return ActionFormField{}, false, err
				}
			}
			continue
		case "maxlength":
			value, ok, err := literalConstraintValue(element, attr)
			if err != nil {
				return ActionFormField{}, false, err
			}
			if ok {
				field.MaxLength, err = parseLengthConstraint("maxlength", value)
				if err != nil {
					return ActionFormField{}, false, err
				}
			}
			continue
		case "pattern":
			value, ok, err := literalConstraintValue(element, attr)
			if err != nil {
				return ActionFormField{}, false, err
			}
			if ok {
				if strings.TrimSpace(value) == "" {
					return ActionFormField{}, false, fmt.Errorf("action form %s pattern must not be empty", element.Name)
				}
				field.Pattern = value
			}
			continue
		case "g:message:required", "g:message:minlength", "g:message:maxlength", "g:message:pattern":
			value, ok, err := literalValidationMessage(element, attr)
			if err != nil {
				return ActionFormField{}, false, err
			}
			if ok {
				switch attr.Name {
				case "g:message:required":
					field.RequiredMessage = value
				case "g:message:minlength":
					field.MinLengthMessage = value
				case "g:message:maxlength":
					field.MaxLengthMessage = value
				case "g:message:pattern":
					field.PatternMessage = value
				}
			}
			continue
		}
		if (element.Name == "button" || element.Name == "input") && attr.Name == "type" {
			if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
				continue
			}
			controlType = strings.TrimSpace(attr.Value)
			continue
		}
		if attr.Name != "name" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			continue
		}
		name := strings.TrimSpace(attr.Value)
		if strings.ContainsAny(name, "{}") {
			return ActionFormField{}, false, fmt.Errorf("action form field name %q must be literal", name)
		}
		field.Name = name
	}
	if field.Name == "" {
		return ActionFormField{}, false, nil
	}
	if strings.ContainsAny(controlType, "{}") {
		return ActionFormField{}, false, fmt.Errorf("action form %s %q type %q must be literal", element.Name, field.Name, controlType)
	}
	if isNonSubmittingControl(element.Name, controlType) {
		return ActionFormField{}, false, nil
	}
	if strings.EqualFold(controlType, "file") {
		return ActionFormField{}, false, fmt.Errorf("file input %q is not supported before upload security rules are defined", field.Name)
	}
	if err := validateValidationMessages(element.Name, field); err != nil {
		return ActionFormField{}, false, err
	}
	return field, true, nil
}

func literalConstraintValue(element Element, attr Attr) (string, bool, error) {
	if element.Name == "button" || attr.Boolean || strings.TrimSpace(attr.Value) == "" {
		return "", false, nil
	}
	value := strings.TrimSpace(attr.Value)
	if attr.Expression {
		return "", false, fmt.Errorf("action form %s %s %q must be literal", element.Name, attr.Name, value)
	}
	return value, true, nil
}

func literalValidationMessage(element Element, attr Attr) (string, bool, error) {
	if element.Name == "button" || attr.Boolean || strings.TrimSpace(attr.Value) == "" {
		return "", false, nil
	}
	value := strings.TrimSpace(attr.Value)
	if attr.Expression {
		return "", false, fmt.Errorf("action form %s %s %q must be literal", element.Name, attr.Name, value)
	}
	return value, true, nil
}

func validateValidationMessages(elementName string, field ActionFormField) error {
	if field.RequiredMessage != "" && !field.Required {
		return fmt.Errorf("action form %s %q declares g:message:required without required", elementName, field.Name)
	}
	if field.MinLengthMessage != "" && field.MinLength == 0 {
		return fmt.Errorf("action form %s %q declares g:message:minlength without minlength", elementName, field.Name)
	}
	if field.MaxLengthMessage != "" && field.MaxLength == 0 {
		return fmt.Errorf("action form %s %q declares g:message:maxlength without maxlength", elementName, field.Name)
	}
	if field.PatternMessage != "" && field.Pattern == "" {
		return fmt.Errorf("action form %s %q declares g:message:pattern without pattern", elementName, field.Name)
	}
	return nil
}

func parseLengthConstraint(name string, value string) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("action form %s must be a non-negative integer", name)
	}
	return number, nil
}

func mergeActionFormField(previous, next ActionFormField) (ActionFormField, error) {
	if previous.Name == "" {
		return next, nil
	}
	next.Required = next.Required || previous.Required
	var err error
	next.RequiredMessage, err = mergeStringConstraint(next.Name, "required message", previous.RequiredMessage, next.RequiredMessage)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MinLength, err = mergeIntConstraint(next.Name, "minlength", previous.MinLength, next.MinLength)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MinLengthMessage, err = mergeStringConstraint(next.Name, "minlength message", previous.MinLengthMessage, next.MinLengthMessage)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MaxLength, err = mergeIntConstraint(next.Name, "maxlength", previous.MaxLength, next.MaxLength)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MaxLengthMessage, err = mergeStringConstraint(next.Name, "maxlength message", previous.MaxLengthMessage, next.MaxLengthMessage)
	if err != nil {
		return ActionFormField{}, err
	}
	next.Pattern, err = mergeStringConstraint(next.Name, "pattern", previous.Pattern, next.Pattern)
	if err != nil {
		return ActionFormField{}, err
	}
	next.PatternMessage, err = mergeStringConstraint(next.Name, "pattern message", previous.PatternMessage, next.PatternMessage)
	if err != nil {
		return ActionFormField{}, err
	}
	return next, nil
}

func mergeIntConstraint(fieldName, constraint string, previous, next int) (int, error) {
	if previous == 0 {
		return next, nil
	}
	if next == 0 || previous == next {
		return previous, nil
	}
	return 0, fmt.Errorf("action form field %q declares conflicting %s constraints", fieldName, constraint)
}

func mergeStringConstraint(fieldName, constraint string, previous, next string) (string, error) {
	if previous == "" {
		return next, nil
	}
	if next == "" || previous == next {
		return previous, nil
	}
	return "", fmt.Errorf("action form field %q declares conflicting %s constraints", fieldName, constraint)
}

func isNonSubmittingControl(elementName, controlType string) bool {
	typ := strings.ToLower(strings.TrimSpace(controlType))
	switch elementName {
	case "button":
		return typ == "button" || typ == "reset"
	case "input":
		return typ == "button" || typ == "reset"
	default:
		return false
	}
}
