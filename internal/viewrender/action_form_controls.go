package viewrender

import (
	"fmt"
	"mime"
	"strconv"
	"strings"
)

func validateActionForm(element Element) (bool, error) {
	multipart := false
	for _, attr := range element.Attrs {
		if attr.Name != "enctype" {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			continue
		}
		value := strings.TrimSpace(attr.Value)
		if strings.ContainsAny(value, "{}") {
			return false, fmt.Errorf("action form enctype %q must be literal", value)
		}
		if strings.EqualFold(value, "multipart/form-data") {
			multipart = true
		}
	}
	return multipart, nil
}

func collectNamedControls(nodes []Node, fields map[string]ActionFormField, multipart bool) error {
	for _, node := range nodes {
		switch typed := node.(type) {
		case Element:
			if field, ok, err := controlField(typed, multipart); err != nil {
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
			if err := collectNamedControls(typed.Children, fields, multipart); err != nil {
				return err
			}
		case ComponentCall:
			if err := collectNamedControls(typed.Children, fields, multipart); err != nil {
				return err
			}
		case AwaitBlock:
			if err := collectNamedControls(typed.Pending, fields, multipart); err != nil {
				return err
			}
			if err := collectNamedControls(typed.Then, fields, multipart); err != nil {
				return err
			}
			if err := collectNamedControls(typed.Catch, fields, multipart); err != nil {
				return err
			}
		}
	}
	return nil
}

func controlField(element Element, multipart bool) (ActionFormField, bool, error) {
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
		if !multipart {
			return ActionFormField{}, false, fmt.Errorf("file input %q requires enctype=\"multipart/form-data\"", field.Name)
		}
		filePolicy, err := fileUploadPolicy(element, field.Name)
		if err != nil {
			return ActionFormField{}, false, err
		}
		field.File = true
		field.MaxFiles = filePolicy.MaxFiles
		field.MaxFileSize = filePolicy.MaxFileSize
		field.AllowedFileTypes = filePolicy.AllowedFileTypes
		if err := validateFileValidationConstraints(element.Name, field); err != nil {
			return ActionFormField{}, false, err
		}
	} else if hasUploadPolicyAttr(element) {
		return ActionFormField{}, false, fmt.Errorf("action form %s %q declares upload policy on a non-file control", element.Name, field.Name)
	}
	if err := validateValidationMessages(element.Name, field); err != nil {
		return ActionFormField{}, false, err
	}
	return field, true, nil
}

func hasUploadPolicyAttr(element Element) bool {
	for _, attr := range element.Attrs {
		if attr.Name == "g:max-file-size" || attr.Name == "g:max-files" {
			return true
		}
	}
	return false
}

type uploadPolicy struct {
	MaxFiles         int
	MaxFileSize      int64
	AllowedFileTypes []string
}

func fileUploadPolicy(element Element, fieldName string) (uploadPolicy, error) {
	maxFileSize, hasMaxFileSize, err := literalPositiveInt64Attr(element, "g:max-file-size")
	if err != nil {
		return uploadPolicy{}, err
	}
	if !hasMaxFileSize {
		return uploadPolicy{}, fmt.Errorf("file input %q must declare g:max-file-size", fieldName)
	}
	maxFiles, hasMaxFiles, err := literalPositiveIntAttr(element, "g:max-files")
	if err != nil {
		return uploadPolicy{}, err
	}
	if !hasMaxFiles {
		return uploadPolicy{}, fmt.Errorf("file input %q must declare g:max-files", fieldName)
	}
	allowed, err := literalAcceptedContentTypes(element, fieldName)
	if err != nil {
		return uploadPolicy{}, err
	}
	return uploadPolicy{MaxFiles: maxFiles, MaxFileSize: maxFileSize, AllowedFileTypes: allowed}, nil
}

func literalPositiveIntAttr(element Element, name string) (int, bool, error) {
	value, ok, err := literalUploadPolicyValue(element, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	number, err := strconv.Atoi(value)
	if err != nil || number <= 0 {
		return 0, true, fmt.Errorf("action form %s must be a positive integer", name)
	}
	return number, true, nil
}

func literalPositiveInt64Attr(element Element, name string) (int64, bool, error) {
	value, ok, err := literalUploadPolicyValue(element, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 {
		return 0, true, fmt.Errorf("action form %s must be a positive integer", name)
	}
	return number, true, nil
}

func literalUploadPolicyValue(element Element, name string) (string, bool, error) {
	for _, attr := range element.Attrs {
		if attr.Name != name {
			continue
		}
		if attr.Boolean || strings.TrimSpace(attr.Value) == "" {
			return "", false, fmt.Errorf("action form input %s requires a value", name)
		}
		value := strings.TrimSpace(attr.Value)
		if attr.Expression {
			return "", false, fmt.Errorf("action form input %s %q must be literal", name, value)
		}
		return value, true, nil
	}
	return "", false, nil
}

func literalAcceptedContentTypes(element Element, fieldName string) ([]string, error) {
	value, ok, err := literalUploadPolicyValue(element, "accept")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("file input %q must declare accept with MIME types", fieldName)
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		contentType, err := normalizeAcceptedContentType(part)
		if err != nil {
			return nil, fmt.Errorf("file input %q accept: %w", fieldName, err)
		}
		if seen[contentType] {
			continue
		}
		seen[contentType] = true
		out = append(out, contentType)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("file input %q must declare accept with MIME types", fieldName)
	}
	return out, nil
}

func normalizeAcceptedContentType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", fmt.Errorf("content type must not be empty")
	}
	if strings.HasPrefix(value, ".") {
		return "", fmt.Errorf("extensions such as %q are not supported; use MIME types", value)
	}
	if strings.HasSuffix(value, "/*") {
		major := strings.TrimSuffix(value, "/*")
		if major == "" || strings.ContainsAny(major, " /;\t\r\n") {
			return "", fmt.Errorf("wildcard content type %q is invalid", value)
		}
		return major + "/*", nil
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil || mediaType == "" {
		return "", fmt.Errorf("content type %q is invalid", value)
	}
	if len(params) > 0 {
		return "", fmt.Errorf("content type %q must not include parameters", value)
	}
	return mediaType, nil
}

func validateFileValidationConstraints(elementName string, field ActionFormField) error {
	if field.MinLength != 0 {
		return fmt.Errorf("action form %s file input %q must not declare minlength", elementName, field.Name)
	}
	if field.MaxLength != 0 {
		return fmt.Errorf("action form %s file input %q must not declare maxlength", elementName, field.Name)
	}
	if field.Pattern != "" {
		return fmt.Errorf("action form %s file input %q must not declare pattern", elementName, field.Name)
	}
	if field.MinLengthMessage != "" || field.MaxLengthMessage != "" || field.PatternMessage != "" {
		return fmt.Errorf("action form %s file input %q must not declare text validation messages", elementName, field.Name)
	}
	return nil
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
	if previous.File != next.File {
		return ActionFormField{}, fmt.Errorf("action form field %q cannot mix file and value controls", next.Name)
	}
	next.Required = next.Required || previous.Required
	var err error
	next.MaxFiles, err = mergeIntConstraint(next.Name, "g:max-files", previous.MaxFiles, next.MaxFiles)
	if err != nil {
		return ActionFormField{}, err
	}
	next.MaxFileSize, err = mergeInt64Constraint(next.Name, "g:max-file-size", previous.MaxFileSize, next.MaxFileSize)
	if err != nil {
		return ActionFormField{}, err
	}
	next.AllowedFileTypes, err = mergeStringSliceConstraint(next.Name, "accept", previous.AllowedFileTypes, next.AllowedFileTypes)
	if err != nil {
		return ActionFormField{}, err
	}
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

func mergeInt64Constraint(fieldName, constraint string, previous, next int64) (int64, error) {
	if previous == 0 {
		return next, nil
	}
	if next == 0 || previous == next {
		return previous, nil
	}
	return 0, fmt.Errorf("action form field %q declares conflicting %s values", fieldName, constraint)
}

func mergeStringSliceConstraint(fieldName, constraint string, previous, next []string) ([]string, error) {
	if len(previous) == 0 {
		return append([]string(nil), next...), nil
	}
	if len(next) == 0 {
		return append([]string(nil), previous...), nil
	}
	if len(previous) != len(next) {
		return nil, fmt.Errorf("action form field %q declares conflicting %s values", fieldName, constraint)
	}
	for index := range previous {
		if previous[index] != next[index] {
			return nil, fmt.Errorf("action form field %q declares conflicting %s values", fieldName, constraint)
		}
	}
	return append([]string(nil), previous...), nil
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
