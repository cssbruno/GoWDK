package view

import (
	"fmt"
	"strings"
)

func blockedViewElement(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "script")
}

func validateParsedHTMLAttrSafety(attr Attr) error {
	if inlineEventHandlerAttr(attr.Name) {
		return fmt.Errorf("inline event handler attribute %q is not supported; use g:on:* inside stateful components", attr.Name)
	}
	if strings.EqualFold(strings.TrimSpace(attr.Name), "srcdoc") {
		return fmt.Errorf("srcdoc attribute is not supported; use g:unsafe-html only with trusted sanitized HTML")
	}
	if attr.Boolean {
		if urlBearingAttr(attr.Name) {
			return fmt.Errorf("%q attributes require a URL value", attr.Name)
		}
		return nil
	}
	if strings.Contains(attr.Value, "{") {
		return nil
	}
	return validateURLAttrValue(attr.Name, attr.Value)
}

func validateRenderedHTMLAttrSafety(name, value string) error {
	if inlineEventHandlerAttr(name) {
		return fmt.Errorf("inline event handler attribute %q is not supported; use g:on:* inside stateful components", name)
	}
	if strings.EqualFold(strings.TrimSpace(name), "srcdoc") {
		return fmt.Errorf("srcdoc attribute is not supported; use g:unsafe-html only with trusted sanitized HTML")
	}
	return validateURLAttrValue(name, value)
}

func inlineEventHandlerAttr(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(name, "on") && len(name) > 2
}

func urlBearingAttr(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "href", "src", "srcset", "action", "formaction", "poster", "cite", "data", "longdesc", "manifest", "xlink:href":
		return true
	default:
		return false
	}
}

func validateURLAttrValue(name, value string) error {
	if !urlBearingAttr(name) {
		return nil
	}
	value = decodeSourceTextEntities(value)
	if strings.EqualFold(strings.TrimSpace(name), "srcset") {
		return validateSrcsetAttrValue(name, value)
	}
	return validateSingleURLAttrValue(name, value)
}

func validateSrcsetAttrValue(name, value string) error {
	for _, candidate := range strings.Split(value, ",") {
		fields := strings.Fields(strings.TrimSpace(candidate))
		if len(fields) == 0 {
			continue
		}
		if err := validateSingleURLAttrValue(name, fields[0]); err != nil {
			return err
		}
	}
	return nil
}

func validateSingleURLAttrValue(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if containsURLControl(value) {
		return fmt.Errorf("unsafe URL in %q attribute: control characters are not allowed", name)
	}
	if len(value) > 1 && value[0] == '/' && (value[1] == '/' || value[1] == '\\') {
		return fmt.Errorf("unsafe URL in %q attribute: protocol-relative URLs are not supported", name)
	}
	if scheme, ok := explicitURLScheme(value); ok && !safeURLScheme(scheme) {
		return fmt.Errorf("unsafe URL in %q attribute: scheme %q is not supported", name, scheme)
	}
	return nil
}

func containsURLControl(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}

func explicitURLScheme(value string) (string, bool) {
	if value == "" {
		return "", false
	}
	for index, char := range value {
		switch {
		case char == ':':
			if index == 0 {
				return "", false
			}
			candidate := value[:index]
			if !validURLScheme(candidate) {
				return "", false
			}
			return strings.ToLower(candidate), true
		case char == '/', char == '?', char == '#':
			return "", false
		}
	}
	return "", false
}

func validURLScheme(value string) bool {
	for index, char := range value {
		switch {
		case index == 0 && ((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')):
		case index > 0 && ((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '+' || char == '-' || char == '.'):
		default:
			return false
		}
	}
	return true
}

func safeURLScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "http", "https", "mailto", "tel":
		return true
	default:
		return false
	}
}
