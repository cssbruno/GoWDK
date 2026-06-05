// Package clientlang parses GOWDK component-local client handlers.
package clientlang

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	functionHeaderPattern = regexp.MustCompile(`^fn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)(?:\s+([A-Za-z_][A-Za-z0-9_]*))?\s*\{$`)
	computedHeaderPattern = regexp.MustCompile(`^computed\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_.\[\]*]*)\s*\{$`)
	effectHeaderPattern   = regexp.MustCompile(`^effect\s+when\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{$`)
	refPattern            = regexp.MustCompile(`^ref\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	identifierPattern     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Program is the parsed representation of a component client {} block.
type Program struct {
	Functions []Function
	Mount     []string
	Destroy   []string
	Effects   []Effect
	Refs      []Ref
	Computed  []Computed
}

// Function is a component-local browser handler.
type Function struct {
	Name       string
	Params     []Param
	ReturnType string
	Statements []string
}

// Effect is a dependency-triggered client block.
type Effect struct {
	Field      string   `json:"field"`
	Statements []string `json:"statements"`
	Cleanup    []string `json:"cleanup,omitempty"`
}

// Computed describes one derived component-local value.
type Computed struct {
	Name string `json:"name"`
	Type string `json:"-"`
	Expr string `json:"expr"`
}

// Param describes one typed function parameter.
type Param struct {
	Name string
	Type string
}

// Ref is a declared DOM reference.
type Ref struct {
	Name string
	Kind string
}

// Handler is the runtime representation emitted into island bootstrap data.
type Handler struct {
	Params     []string    `json:"params,omitempty"`
	ParamTypes []ValueType `json:"-"`
	Statements []string    `json:"statements"`
}

// Helper is a return-valued component-local function callable from
// expressions. Helpers cannot be called directly as event handlers.
type Helper struct {
	Params     []string    `json:"params,omitempty"`
	ParamTypes []ValueType `json:"-"`
	ReturnType ValueType   `json:"-"`
	Return     string      `json:"return"`
}

// Bootstrap is the runtime payload emitted into data-gowdk-client when a
// component has lifecycle/effect blocks.
type Bootstrap struct {
	Handlers map[string]Handler `json:"handlers,omitempty"`
	Helpers  map[string]Helper  `json:"helpers,omitempty"`
	Mount    []string           `json:"mount,omitempty"`
	Destroy  []string           `json:"destroy,omitempty"`
	Effects  []Effect           `json:"effects,omitempty"`
	Computed []Computed         `json:"computed,omitempty"`
}

// Call is a component-local function invocation expression.
type Call struct {
	Name string
	Args []string
}

// Parse parses the first component client {} language slice.
func Parse(source string) (Program, error) {
	var program Program
	var current *Function
	var lifecycle *lifecycleBlock
	seen := map[string]bool{}
	seenRefs := map[string]bool{}

	lines := strings.Split(source, "\n")
	for index, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if current == nil && lifecycle == nil {
			match := functionHeaderPattern.FindStringSubmatch(line)
			if match != nil {
				name := match[1]
				if isReservedFunctionName(name) {
					return Program{}, fmt.Errorf("client function %q uses a reserved built-in name", name)
				}
				if seen[name] {
					return Program{}, fmt.Errorf("client function %q is declared more than once", name)
				}
				params, err := parseParams(match[2])
				if err != nil {
					return Program{}, fmt.Errorf("client function %s params: %w", name, err)
				}
				returnType := strings.TrimSpace(match[3])
				if returnType != "" && !isSupportedReturnType(returnType) {
					return Program{}, fmt.Errorf("client function %s uses unsupported return type %q", name, returnType)
				}
				seen[name] = true
				current = &Function{Name: name, Params: params, ReturnType: returnType}
				continue
			}
			switch line {
			case "on mount {":
				lifecycle = &lifecycleBlock{Kind: "mount"}
				continue
			case "on destroy {":
				lifecycle = &lifecycleBlock{Kind: "destroy"}
				continue
			}
			if match := effectHeaderPattern.FindStringSubmatch(line); match != nil {
				lifecycle = &lifecycleBlock{Kind: "effect", Field: match[1]}
				continue
			}
			if match := computedHeaderPattern.FindStringSubmatch(line); match != nil {
				name := match[1]
				if seen[name] {
					return Program{}, fmt.Errorf("client computed %q conflicts with a function", name)
				}
				seen[name] = true
				lifecycle = &lifecycleBlock{Kind: "computed", Field: name, Type: match[2]}
				continue
			}
			if match := refPattern.FindStringSubmatch(line); match != nil {
				name := match[1]
				if seenRefs[name] {
					return Program{}, fmt.Errorf("client ref %q is declared more than once", name)
				}
				seenRefs[name] = true
				program.Refs = append(program.Refs, Ref{Name: name, Kind: match[2]})
				continue
			}
			return Program{}, fmt.Errorf("client line %d has unsupported syntax %q", index+1, line)
		}

		if current != nil && line == "}" {
			if err := validateFunctionReturnShape(*current); err != nil {
				return Program{}, err
			}
			program.Functions = append(program.Functions, *current)
			current = nil
			continue
		}
		if lifecycle != nil && lifecycle.Cleanup {
			if line == "}" {
				lifecycle.Cleanup = false
				continue
			}
			if strings.ContainsAny(line, "{}") && !allowsInlineBraceExpression(line) {
				return Program{}, fmt.Errorf("client effect cleanup line %d has unsupported syntax %q", index+1, line)
			}
			statement := strings.TrimSpace(strings.TrimSuffix(line, ";"))
			if statement != "" {
				lifecycle.CleanupStatements = append(lifecycle.CleanupStatements, statement)
			}
			continue
		}
		if lifecycle != nil && lifecycle.Kind == "effect" && line == "return {" {
			lifecycle.Cleanup = true
			continue
		}
		if lifecycle != nil && line == "}" {
			switch lifecycle.Kind {
			case "mount":
				program.Mount = append(program.Mount, lifecycle.Statements...)
			case "destroy":
				program.Destroy = append(program.Destroy, lifecycle.Statements...)
			case "effect":
				program.Effects = append(program.Effects, Effect{
					Field:      lifecycle.Field,
					Statements: append([]string(nil), lifecycle.Statements...),
					Cleanup:    append([]string(nil), lifecycle.CleanupStatements...),
				})
			case "computed":
				if len(lifecycle.Statements) != 1 {
					return Program{}, fmt.Errorf("client computed %s must contain exactly one return statement", lifecycle.Field)
				}
				statement := strings.TrimSpace(lifecycle.Statements[0])
				if !strings.HasPrefix(statement, "return ") {
					return Program{}, fmt.Errorf("client computed %s must use `return expr`", lifecycle.Field)
				}
				expr := strings.TrimSpace(strings.TrimPrefix(statement, "return "))
				if expr == "" {
					return Program{}, fmt.Errorf("client computed %s must return an expression", lifecycle.Field)
				}
				program.Computed = append(program.Computed, Computed{Name: lifecycle.Field, Type: lifecycle.Type, Expr: expr})
			}
			lifecycle = nil
			continue
		}
		if strings.HasPrefix(line, "fn ") {
			if current != nil {
				return Program{}, fmt.Errorf("client function %s line %d cannot declare nested functions", current.Name, index+1)
			}
			return Program{}, fmt.Errorf("client %s block line %d cannot declare nested functions", lifecycle.Description(), index+1)
		}
		if strings.ContainsAny(line, "{}") && !allowsInlineBraceExpression(line) {
			return Program{}, fmt.Errorf("client block line %d has unsupported syntax %q", index+1, line)
		}
		statement := strings.TrimSuffix(line, ";")
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if current != nil {
			current.Statements = append(current.Statements, statement)
		} else {
			lifecycle.Statements = append(lifecycle.Statements, statement)
		}
	}

	if current != nil {
		return Program{}, fmt.Errorf("client function %s missing closing }", current.Name)
	}
	if lifecycle != nil {
		if lifecycle.Cleanup {
			return Program{}, fmt.Errorf("client effect cleanup block missing closing }")
		}
		return Program{}, fmt.Errorf("client %s block missing closing }", lifecycle.Description())
	}
	return program, nil
}

type lifecycleBlock struct {
	Kind              string
	Field             string
	Type              string
	Statements        []string
	Cleanup           bool
	CleanupStatements []string
}

func (block lifecycleBlock) Description() string {
	if block.Kind == "computed" {
		return "computed"
	}
	if block.Kind == "effect" {
		return "effect"
	}
	return "on " + block.Kind
}

// HandlerMap returns deterministic handlers keyed by function name.
func (program Program) HandlerMap() map[string]Handler {
	if len(program.Functions) == 0 {
		return nil
	}
	handlers := map[string]Handler{}
	for _, function := range program.Functions {
		if function.ReturnType != "" {
			continue
		}
		params := make([]string, 0, len(function.Params))
		paramTypes := make([]ValueType, 0, len(function.Params))
		for _, param := range function.Params {
			params = append(params, param.Name)
			paramTypes = append(paramTypes, NormalizeType(param.Type))
		}
		handlers[function.Name] = Handler{
			Params:     params,
			ParamTypes: paramTypes,
			Statements: append([]string(nil), function.Statements...),
		}
	}
	return handlers
}

// HelperMap returns deterministic return-valued helpers keyed by function name.
func (program Program) HelperMap() map[string]Helper {
	if len(program.Functions) == 0 {
		return nil
	}
	helpers := map[string]Helper{}
	for _, function := range program.Functions {
		if function.ReturnType == "" {
			continue
		}
		params := make([]string, 0, len(function.Params))
		paramTypes := make([]ValueType, 0, len(function.Params))
		for _, param := range function.Params {
			params = append(params, param.Name)
			paramTypes = append(paramTypes, NormalizeType(param.Type))
		}
		helpers[function.Name] = Helper{
			Params:     params,
			ParamTypes: paramTypes,
			ReturnType: NormalizeType(function.ReturnType),
			Return:     strings.TrimSpace(strings.TrimPrefix(function.Statements[0], "return ")),
		}
	}
	return helpers
}

// RefMap returns declared DOM refs keyed by name.
func (program Program) RefMap() map[string]Ref {
	if len(program.Refs) == 0 {
		return nil
	}
	refs := map[string]Ref{}
	for _, ref := range program.Refs {
		refs[ref.Name] = ref
	}
	return refs
}

// HasLifecycle reports whether the program needs the runtime bootstrap envelope.
func (program Program) HasLifecycle() bool {
	return len(program.Mount) > 0 || len(program.Destroy) > 0 || len(program.Effects) > 0
}

// NeedsBootstrap reports whether the program needs the runtime bootstrap
// envelope instead of the legacy direct handler map.
func (program Program) NeedsBootstrap() bool {
	return program.HasLifecycle() || len(program.Computed) > 0 || len(program.HelperMap()) > 0
}

// OrderedComputed returns computed values in dependency order. References to
// other computed names must be evaluated before the dependent value.
func (program Program) OrderedComputed() ([]Computed, error) {
	return OrderComputed(program.Computed)
}

// OrderComputed returns computed values in dependency order and rejects cycles.
func OrderComputed(computeds []Computed) ([]Computed, error) {
	if len(computeds) == 0 {
		return nil, nil
	}
	byName := map[string]Computed{}
	for _, computed := range computeds {
		if _, exists := byName[computed.Name]; exists {
			return nil, fmt.Errorf("computed %q is declared more than once", computed.Name)
		}
		byName[computed.Name] = computed
	}
	deps := map[string][]string{}
	for _, computed := range computeds {
		fields, err := ExprFields(computed.Expr)
		if err != nil {
			return nil, fmt.Errorf("computed %s expression: %w", computed.Name, err)
		}
		for _, field := range fields {
			if _, ok := byName[field]; ok {
				deps[computed.Name] = append(deps[computed.Name], field)
			}
		}
		sort.Strings(deps[computed.Name])
	}
	state := map[string]int{}
	var stack []string
	var ordered []Computed
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("computed dependency cycle %s", cyclePath(stack, name))
		case 2:
			return nil
		}
		state[name] = 1
		stack = append(stack, name)
		for _, dep := range deps[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = 2
		ordered = append(ordered, byName[name])
		return nil
	}
	for _, computed := range computeds {
		if err := visit(computed.Name); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func cyclePath(stack []string, repeated string) string {
	start := 0
	for index, name := range stack {
		if name == repeated {
			start = index
			break
		}
	}
	cycle := append([]string(nil), stack[start:]...)
	cycle = append(cycle, repeated)
	return strings.Join(cycle, " -> ")
}

// NormalizeType maps Go/GOWDK scalar type names into client expression types.
func NormalizeType(value string) ValueType {
	value = strings.TrimSpace(value)
	for strings.HasPrefix(value, "*") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "*"))
	}
	if strings.HasPrefix(value, "[]") || strings.HasPrefix(value, "[") {
		return TypeArray
	}
	switch value {
	case "string":
		return TypeString
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return TypeInt
	case "float", "float32", "float64":
		return TypeFloat
	case "bool":
		return TypeBool
	default:
		if strings.Contains(value, ".") {
			return TypeObject
		}
		return TypeUnknown
	}
}

// Canonical returns a deterministic representation used for component
// redundancy checks.
func (program Program) Canonical() string {
	if len(program.Functions) == 0 && len(program.Mount) == 0 && len(program.Destroy) == 0 && len(program.Effects) == 0 && len(program.Refs) == 0 && len(program.Computed) == 0 {
		return ""
	}
	functions := append([]Function(nil), program.Functions...)
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].Name < functions[j].Name
	})
	parts := make([]string, 0, len(functions))
	for _, function := range functions {
		params := make([]string, 0, len(function.Params))
		for _, param := range function.Params {
			params = append(params, param.Name+":"+param.Type)
		}
		statements := append([]string(nil), function.Statements...)
		for index, statement := range statements {
			statements[index] = strings.Join(strings.Fields(statement), " ")
		}
		parts = append(parts, function.Name+"("+strings.Join(params, ",")+"){"+strings.Join(statements, ";")+"}")
		if function.ReturnType != "" {
			parts[len(parts)-1] = function.Name + "(" + strings.Join(params, ",") + ")" + function.ReturnType + "{" + strings.Join(statements, ";") + "}"
		}
	}
	refs := append([]Ref(nil), program.Refs...)
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Name < refs[j].Name
	})
	for _, ref := range refs {
		parts = append(parts, "ref "+ref.Name+" "+ref.Kind)
	}
	computeds := append([]Computed(nil), program.Computed...)
	sort.Slice(computeds, func(i, j int) bool {
		return computeds[i].Name < computeds[j].Name
	})
	for _, computed := range computeds {
		parts = append(parts, "computed "+computed.Name+" "+computed.Type+"{return "+strings.Join(strings.Fields(computed.Expr), " ")+"}")
	}
	if len(program.Mount) > 0 {
		parts = append(parts, "mount{"+canonicalStatements(program.Mount)+"}")
	}
	if len(program.Destroy) > 0 {
		parts = append(parts, "destroy{"+canonicalStatements(program.Destroy)+"}")
	}
	effects := append([]Effect(nil), program.Effects...)
	sort.Slice(effects, func(i, j int) bool {
		if effects[i].Field == effects[j].Field {
			return strings.Join(effects[i].Statements, ";") < strings.Join(effects[j].Statements, ";")
		}
		return effects[i].Field < effects[j].Field
	})
	for _, effect := range effects {
		item := "effect(" + effect.Field + "){" + canonicalStatements(effect.Statements) + "}"
		if len(effect.Cleanup) > 0 {
			item += "cleanup{" + canonicalStatements(effect.Cleanup) + "}"
		}
		parts = append(parts, item)
	}
	return strings.Join(parts, "|")
}

func canonicalStatements(statements []string) string {
	items := append([]string(nil), statements...)
	for index, statement := range items {
		items[index] = strings.Join(strings.Fields(statement), " ")
	}
	return strings.Join(items, ";")
}

// IsFunctionCall reports whether expr is a no-argument client function call.
func IsFunctionCall(expr string) (string, bool) {
	call, ok := ParseCall(expr)
	if !ok || len(call.Args) != 0 {
		return "", false
	}
	return call.Name, true
}

// ParseCall reports whether expr is a component-local function call.
func ParseCall(expr string) (Call, bool) {
	expr = strings.TrimSpace(expr)
	open := strings.Index(expr, "(")
	if open < 1 || !strings.HasSuffix(expr, ")") {
		return Call{}, false
	}
	name := strings.TrimSpace(expr[:open])
	if !identifierPattern.MatchString(name) {
		return Call{}, false
	}
	args, err := splitCommaList(expr[open+1 : len(expr)-1])
	if err != nil {
		return Call{}, false
	}
	return Call{Name: name, Args: args}, true
}

func parseParams(source string) ([]Param, error) {
	items, err := splitCommaList(source)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	params := make([]Param, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		fields := strings.Fields(item)
		if len(fields) != 2 {
			return nil, fmt.Errorf("parameter %q must use `name type`", item)
		}
		name, typ := fields[0], fields[1]
		if !identifierPattern.MatchString(name) {
			return nil, fmt.Errorf("invalid parameter name %q", name)
		}
		if !isSupportedParamType(typ) {
			return nil, fmt.Errorf("unsupported parameter type %q", typ)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate parameter %q", name)
		}
		seen[name] = true
		params = append(params, Param{Name: name, Type: typ})
	}
	return params, nil
}

func validateFunctionReturnShape(function Function) error {
	if function.ReturnType == "" {
		for _, statement := range function.Statements {
			if strings.HasPrefix(strings.TrimSpace(statement), "return ") {
				return fmt.Errorf("client function %s cannot return a value without declaring a return type", function.Name)
			}
		}
		return nil
	}
	if len(function.Statements) != 1 {
		return fmt.Errorf("client helper function %s must contain exactly one return statement", function.Name)
	}
	statement := strings.TrimSpace(function.Statements[0])
	if !strings.HasPrefix(statement, "return ") {
		return fmt.Errorf("client helper function %s must use `return expr`", function.Name)
	}
	expr := strings.TrimSpace(strings.TrimPrefix(statement, "return "))
	if expr == "" {
		return fmt.Errorf("client helper function %s must return an expression", function.Name)
	}
	return nil
}

func isSupportedParamType(value string) bool {
	switch value {
	case "string", "int", "float", "bool":
		return true
	default:
		return false
	}
}

func isSupportedReturnType(value string) bool {
	return isSupportedParamType(value)
}

func isReservedFunctionName(name string) bool {
	switch name {
	case "append", "remove", "move", "len", "string", "int", "float":
		return true
	default:
		return false
	}
}

func splitCommaList(source string) ([]string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, nil
	}
	var items []string
	start := 0
	depth := 0
	inString := false
	escaped := false
	for index, char := range source {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced comma-separated item")
			}
		case ',':
			if depth > 0 {
				continue
			}
			item := strings.TrimSpace(source[start:index])
			if item == "" {
				return nil, fmt.Errorf("empty comma-separated item")
			}
			items = append(items, item)
			start = index + 1
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string")
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced comma-separated item")
	}
	item := strings.TrimSpace(source[start:])
	if item == "" {
		return nil, fmt.Errorf("empty comma-separated item")
	}
	items = append(items, item)
	return items, nil
}

func allowsInlineBraceExpression(statement string) bool {
	if !balancedInlineBraces(statement) {
		return false
	}
	if call, ok := ParseCall(statement); ok && call.Name == "append" && len(call.Args) == 2 {
		return strings.HasPrefix(strings.TrimSpace(call.Args[1]), "{")
	}
	if right, ok := strings.CutPrefix(strings.TrimSpace(statement), "return "); ok {
		right = strings.TrimSpace(right)
		if !strings.HasPrefix(right, "if ") {
			return false
		}
		_, err := ParseExpr(right)
		return err == nil
	}
	assign := strings.Index(statement, "=")
	if assign < 0 || strings.Contains(statement[:assign], "=") {
		return false
	}
	right := strings.TrimSpace(statement[assign+1:])
	if !strings.HasPrefix(right, "if ") {
		return false
	}
	_, err := ParseExpr(right)
	return err == nil
}

func balancedInlineBraces(source string) bool {
	depth := 0
	inString := false
	escaped := false
	for _, char := range source {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0 && !inString
}
