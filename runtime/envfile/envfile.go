// Package envfile loads simple dotenv-style files without overriding process
// environment values.
package envfile

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// MaxLineBytes is the maximum logical line size accepted in an env file.
	MaxLineBytes = 1 << 20

	DiagnosticLineTooLong = "env_file_line_too_long"
)

// DiagnosticError describes an env-file parse diagnostic without exposing the
// input value.
type DiagnosticError struct {
	Code  string
	Path  string
	Line  int
	Limit int
}

func (err *DiagnosticError) Error() string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s:%d: %s: env-file line exceeds the %d-byte limit; use process environment or secret injection for larger values",
		err.Path,
		err.Line,
		err.Code,
		err.Limit,
	)
}

// LoadResult describes one env-file load without exposing values.
type LoadResult struct {
	Path     string
	Loaded   bool
	Applied  []string
	Skipped  []string
	Explicit bool
}

// Environment is an explicit environment overlay. Process values win over
// file values. Constructors and accessors make defensive copies so project
// loading can pass environments between concurrent workspaces without global
// mutable state.
type Environment struct {
	Process map[string]string
	File    map[string]string
}

// NewEnvironment constructs an overlay from a process-style NAME=value slice
// and parsed file entries.
func NewEnvironment(process []string, entries []Entry) Environment {
	env := Environment{Process: environmentMap(process), File: map[string]string{}}
	for _, entry := range entries {
		if _, exists := env.Process[entry.Name]; exists {
			continue
		}
		env.File[entry.Name] = entry.Value
	}
	return env
}

// Lookup returns a process value first, then a file overlay value.
func (env Environment) Lookup(name string) (string, bool) {
	if value, ok := env.Process[name]; ok {
		return value, true
	}
	value, ok := env.File[name]
	return value, ok
}

// ForSubprocess applies the file overlay to base without replacing explicitly
// supplied base values. Overlay-only names are appended in sorted order for
// deterministic tests and command construction.
func (env Environment) ForSubprocess(base []string) []string {
	result := append([]string(nil), base...)
	seen := environmentMap(base)
	if base == nil {
		names := sortedEnvironmentNames(env.Process)
		for _, name := range names {
			result = append(result, name+"="+env.Process[name])
			seen[name] = env.Process[name]
		}
	}
	for _, name := range sortedEnvironmentNames(env.File) {
		if _, exists := seen[name]; exists {
			continue
		}
		result = append(result, name+"="+env.File[name])
	}
	return result
}

// Load parses path into an explicit environment overlay without mutating the
// process environment.
func Load(path string, explicit bool, process []string) (Environment, LoadResult, error) {
	result := LoadResult{Path: path, Explicit: explicit}
	if strings.TrimSpace(path) == "" {
		return NewEnvironment(process, nil), result, nil
	}
	values, err := ParseFile(path)
	if err != nil {
		return Environment{}, result, err
	}
	result.Loaded = true
	env := NewEnvironment(process, values)
	for _, entry := range values {
		if _, exists := env.Process[entry.Name]; exists {
			result.Skipped = append(result.Skipped, entry.Name)
		} else {
			result.Applied = append(result.Applied, entry.Name)
		}
	}
	return env, result, nil
}

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

// LoadIntoEnv is retained for generated application startup compatibility.
// Compiler/project loading must use Load and pass the returned overlay
// explicitly. Existing process values always win.
func LoadIntoEnv(path string, explicit bool) (LoadResult, error) {
	env, result, err := Load(path, explicit, os.Environ())
	if err != nil {
		return result, err
	}
	for _, name := range result.Applied {
		if err := os.Setenv(name, env.File[name]); err != nil {
			return result, err
		}
	}
	return result, nil
}

func environmentMap(entries []string) map[string]string {
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name != "" {
			values[name] = value
		}
	}
	return values
}

func sortedEnvironmentNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
	defer func() {
		_ = file.Close()
	}()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), MaxLineBytes+2)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		if len(rawLine) > MaxLineBytes {
			return nil, lineTooLongError(path, lineNumber)
		}
		line := strings.TrimSpace(rawLine)
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
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, lineTooLongError(path, lineNumber+1)
		}
		return nil, err
	}
	return entries, nil
}

func lineTooLongError(path string, lineNumber int) error {
	return &DiagnosticError{
		Code:  DiagnosticLineTooLong,
		Path:  path,
		Line:  lineNumber,
		Limit: MaxLineBytes,
	}
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
