// Package envfile loads simple dotenv-style files without overriding process
// environment values.
package envfile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LoadResult describes one env-file load without exposing values.
type LoadResult struct {
	Path     string
	Loaded   bool
	Applied  []string
	Skipped  []string
	Explicit bool
}

// appliedValues records the value this loader last wrote for each name, so a
// reload can tell its own previous value apart from an external override.
var (
	appliedMu     sync.Mutex
	appliedValues = map[string]string{}
)

// LookupPath resolves the env file for a project root. explicit wins. Without
// an explicit path, .env.<GOWDK_ENV> wins over .env when it exists.
func LookupPath(projectRoot string, explicit string) (string, bool, error) {
	if strings.TrimSpace(explicit) != "" {
		path, err := resolvePath(projectRoot, explicit)
		if err != nil {
			return "", false, err
		}
		return path, true, nil
	}
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return "", false, err
		}
	}
	if name := strings.TrimSpace(os.Getenv("GOWDK_ENV")); name != "" {
		candidate := filepath.Join(projectRoot, ".env."+name)
		if fileExists(candidate) {
			return candidate, false, nil
		}
	}
	candidate := filepath.Join(projectRoot, ".env")
	if fileExists(candidate) {
		return candidate, false, nil
	}
	return "", false, nil
}

// LoadIntoEnv loads path and sets only names that are not already present in
// the process environment. Existing process values always win, including a
// value changed with os.Setenv after a previous load: a reload only overwrites
// a name whose current value is still the one this loader last applied.
func LoadIntoEnv(path string, explicit bool) (LoadResult, error) {
	appliedMu.Lock()
	defer appliedMu.Unlock()

	result := LoadResult{Path: path, Explicit: explicit}
	if strings.TrimSpace(path) == "" {
		return result, nil
	}
	values, err := ParseFile(path)
	if err != nil {
		return result, err
	}
	result.Loaded = true
	for _, entry := range values {
		if current, ok := os.LookupEnv(entry.Name); ok {
			// A value already in the environment wins, unless this loader set
			// it on a previous load and nothing has changed it since. If the
			// current value differs from what we last applied, it is an
			// external/manual override and must not be clobbered.
			if applied, mine := appliedValues[entry.Name]; !mine || applied != current {
				result.Skipped = append(result.Skipped, entry.Name)
				continue
			}
		}
		if err := os.Setenv(entry.Name, entry.Value); err != nil {
			return result, err
		}
		appliedValues[entry.Name] = entry.Value
		result.Applied = append(result.Applied, entry.Name)
	}
	return result, nil
}

// Entry is one parsed env-file assignment.
type Entry struct {
	Name  string
	Value string
}

// ParseFile parses KEY=value and export KEY=value lines. Values may be quoted
// with single or double quotes. Blank lines and # comments are ignored.
func ParseFile(path string) ([]Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, ok, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		if ok {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func parseLine(line string) (Entry, bool, error) {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "\ufeff")
	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	}
	name, value, ok := strings.Cut(line, "=")
	if !ok {
		return Entry{}, false, fmt.Errorf("expected NAME=value")
	}
	name = strings.TrimSpace(name)
	if !validName(name) {
		return Entry{}, false, fmt.Errorf("invalid env name %q", name)
	}
	value, err := parseValue(strings.TrimSpace(value))
	if err != nil {
		return Entry{}, false, err
	}
	return Entry{Name: name, Value: value}, true, nil
}

func parseValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	switch value[0] {
	case '\'':
		return quotedValue(value, '\'')
	case '"':
		quoted, err := quotedValue(value, '"')
		if err != nil {
			return "", err
		}
		replacer := strings.NewReplacer(`\n`, "\n", `\r`, "\r", `\t`, "\t", `\"`, `"`, `\\`, `\`)
		return replacer.Replace(quoted), nil
	default:
		return strings.TrimSpace(stripInlineComment(value)), nil
	}
}

func quotedValue(value string, quote byte) (string, error) {
	if len(value) < 2 {
		return "", fmt.Errorf("unterminated quoted value")
	}
	var builder strings.Builder
	escaped := false
	for i := 1; i < len(value); i++ {
		ch := value[i]
		if escaped {
			builder.WriteByte('\\')
			builder.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && quote == '"' {
			escaped = true
			continue
		}
		if ch == quote {
			if strings.TrimSpace(value[i+1:]) != "" && !strings.HasPrefix(strings.TrimSpace(value[i+1:]), "#") {
				return "", fmt.Errorf("unexpected text after quoted value")
			}
			return builder.String(), nil
		}
		builder.WriteByte(ch)
	}
	return "", fmt.Errorf("unterminated quoted value")
}

func stripInlineComment(value string) string {
	for i := 0; i < len(value); i++ {
		if value[i] == '#' && (i == 0 || value[i-1] == ' ' || value[i-1] == '\t') {
			return value[:i]
		}
	}
	return value
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for i, ch := range name {
		if ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || i > 0 && ch >= '0' && ch <= '9' {
			continue
		}
		return false
	}
	return true
}

func resolvePath(projectRoot string, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(projectRoot, path), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
