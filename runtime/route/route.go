package route

import (
	"path"
	"strconv"
	"strings"
)

// Match compares a concrete request path to a simple generated route pattern.
// Parameter segments use "{name}" and reject empty, ".", and ".." values. A
// final "{name...}" rest segment matches one or more remaining request
// segments; the captured value is those segments joined with "/".
func Match(pattern, requestPath string) (map[string]string, bool) {
	if hasUnsafeRequestSegment(requestPath) {
		return nil, false
	}
	patternParts := splitPath(pattern)
	requestParts := splitPath(requestPath)
	rest := len(patternParts) > 0 && isRestSegment(patternParts[len(patternParts)-1])
	if rest {
		if len(requestParts) < len(patternParts) {
			return nil, false
		}
		// splitPath cleans the request path, which silently rewrites empty,
		// "." and ".." segments. Rest captures span multiple segments, so a
		// cleaned match could serve a non-canonical path under a different
		// captured value. Reject those requests outright from the raw path.
		for _, value := range strings.Split(strings.Trim(requestPath, "/"), "/") {
			if value == "" || value == "." || value == ".." {
				return nil, false
			}
		}
	} else if len(patternParts) != len(requestParts) {
		return nil, false
	}

	params := map[string]string{}
	for index, part := range patternParts {
		if rest && index == len(patternParts)-1 {
			name := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}"), "...")
			remaining := requestParts[index:]
			for _, value := range remaining {
				if value == "" || value == "." || value == ".." {
					return nil, false
				}
			}
			params[name] = strings.Join(remaining, "/")
			return params, true
		}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if before, _, found := strings.Cut(name, ":"); found {
				name = before
			}
			value := requestParts[index]
			if value == "" || value == "." || value == ".." {
				return nil, false
			}
			params[name] = value
			continue
		}
		if part != requestParts[index] {
			return nil, false
		}
	}
	return params, true
}

func hasUnsafeRequestSegment(requestPath string) bool {
	// Consecutive slashes anywhere — including a leading "//" or trailing
	// "//" — are non-canonical. path.Clean would collapse them, letting
	// "//admin" or "/admin//" match "/admin" while a fronting proxy, cache,
	// or authorization layer may treat them as different paths. Reject them
	// outright so matching cannot disagree with what sits in front of it.
	if strings.Contains(requestPath, "//") {
		return true
	}
	trimmed := strings.Trim(requestPath, "/")
	if trimmed == "" {
		return false
	}
	for _, value := range strings.Split(trimmed, "/") {
		if value == "" || value == "." || value == ".." {
			return true
		}
	}
	return false
}

func isRestSegment(part string) bool {
	return strings.HasPrefix(part, "{") && strings.HasSuffix(part, "...}") && len(part) > len("{...}")
}

func splitPath(value string) []string {
	clean := path.Clean("/" + value)
	trimmed := strings.Trim(clean, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

// ParamError describes a failed route-param decode without exposing the raw
// request value.
type ParamError struct {
	Name    string
	Type    string
	Missing bool
	Err     error
}

func (err ParamError) Error() string {
	if err.Missing {
		return "missing route param " + strconv.Quote(err.Name)
	}
	return "invalid " + err.Type + " route param " + strconv.Quote(err.Name)
}

func (err ParamError) Unwrap() error {
	return err.Err
}

// Required returns a required route param.
func Required(params map[string]string, name string) (string, error) {
	value, ok := params[name]
	if !ok || value == "" {
		return "", ParamError{Name: name, Type: "string", Missing: true}
	}
	return value, nil
}

// String returns an optional route param as a string.
func String(params map[string]string, name string) (string, bool, error) {
	value, ok := params[name]
	if !ok || value == "" {
		return "", false, nil
	}
	return value, true, nil
}

// Int returns an optional route param decoded as an int.
func Int(params map[string]string, name string) (int, bool, error) {
	value, ok, err := String(params, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	decoded, parseErr := strconv.Atoi(value)
	if parseErr != nil {
		return 0, true, ParamError{Name: name, Type: "int", Err: parseErr}
	}
	return decoded, true, nil
}

// Int64 returns an optional route param decoded as an int64.
func Int64(params map[string]string, name string) (int64, bool, error) {
	value, ok, err := String(params, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	decoded, parseErr := strconv.ParseInt(value, 10, 64)
	if parseErr != nil {
		return 0, true, ParamError{Name: name, Type: "int64", Err: parseErr}
	}
	return decoded, true, nil
}

// Uint returns an optional route param decoded as a uint.
func Uint(params map[string]string, name string) (uint, bool, error) {
	value, ok, err := String(params, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	decoded, parseErr := strconv.ParseUint(value, 10, 0)
	if parseErr != nil {
		return 0, true, ParamError{Name: name, Type: "uint", Err: parseErr}
	}
	return uint(decoded), true, nil
}

// Uint64 returns an optional route param decoded as a uint64.
func Uint64(params map[string]string, name string) (uint64, bool, error) {
	value, ok, err := String(params, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	decoded, parseErr := strconv.ParseUint(value, 10, 64)
	if parseErr != nil {
		return 0, true, ParamError{Name: name, Type: "uint64", Err: parseErr}
	}
	return decoded, true, nil
}

// Bool returns an optional route param decoded as a bool.
func Bool(params map[string]string, name string) (bool, bool, error) {
	value, ok, err := String(params, name)
	if err != nil || !ok {
		return false, ok, err
	}
	decoded, parseErr := strconv.ParseBool(value)
	if parseErr != nil {
		return false, true, ParamError{Name: name, Type: "bool", Err: parseErr}
	}
	return decoded, true, nil
}

// Float64 returns an optional route param decoded as a float64.
func Float64(params map[string]string, name string) (float64, bool, error) {
	value, ok, err := String(params, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	decoded, parseErr := strconv.ParseFloat(value, 64)
	if parseErr != nil {
		return 0, true, ParamError{Name: name, Type: "float64", Err: parseErr}
	}
	return decoded, true, nil
}
