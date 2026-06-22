package clientrt

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/clientlang"
)

type expressionConformanceCase struct {
	Name   string `json:"name"`
	Expr   string `json:"expr"`
	Values map[string]string
	State  map[string]any `json:"state"`
}

type expressionExpectation struct {
	Value any
	Error string
}

func TestIslandExpressionRuntimeMatchesGoEvaluator(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatalf("node is required for client expression conformance tests: %v", err)
	}

	cases := []expressionConformanceCase{
		{
			Name:   "arithmetic precedence",
			Expr:   `Count + Step * 2`,
			Values: map[string]string{"Count": "3", "Step": "4"},
			State:  map[string]any{"Count": 3, "Step": 4},
		},
		{
			Name:   "comparison and boolean logic",
			Expr:   `(Count + Step) >= 7 && Open == true`,
			Values: map[string]string{"Count": "3", "Step": "4", "Open": "true"},
			State:  map[string]any{"Count": 3, "Step": 4, "Open": true},
		},
		{
			Name:   "go style conditional",
			Expr:   `if Open { Label } else { "closed" }`,
			Values: map[string]string{"Open": "true", "Label": "Ada"},
			State:  map[string]any{"Open": true, "Label": "Ada"},
		},
		{
			Name:   "switch expression",
			Expr:   `switch Status { case "draft": "Draft" case "live": Label default: "Unknown" }`,
			Values: map[string]string{"Status": "live", "Label": "Published"},
			State:  map[string]any{"Status": "live", "Label": "Published"},
		},
		{
			Name:   "match expression",
			Expr:   `match Count { case 0: "empty" case 1: "single" default: string(Count) }`,
			Values: map[string]string{"Count": "3"},
			State:  map[string]any{"Count": 3},
		},
		{
			Name:   "nested member and index",
			Expr:   `User.Open && Items[0].Name == "first"`,
			Values: map[string]string{"User": `{"Name":"Ada","Open":true}`, "Items": `[{"Name":"first"},{"Name":"second"}]`},
			State: map[string]any{
				"User":  map[string]any{"Name": "Ada", "Open": true},
				"Items": []any{map[string]any{"Name": "first"}, map[string]any{"Name": "second"}},
			},
		},
		{
			Name:   "string filter builtins",
			Expr:   `contains(lower(User.Name), lower(Query)) || upper(Query) == "ALL"`,
			Values: map[string]string{"User": `{"Name":"Ada Lovelace"}`, "Query": "LOVE"},
			State: map[string]any{
				"User":  map[string]any{"Name": "Ada Lovelace"},
				"Query": "LOVE",
			},
		},
		{
			Name:   "numeric and string builtins",
			Expr:   `string(len(Items) + int("2")) + ":" + string(float("1.5"))`,
			Values: map[string]string{"Items": `[{"Name":"first"},{"Name":"second"}]`},
			State:  map[string]any{"Items": []any{map[string]any{"Name": "first"}, map[string]any{"Name": "second"}}},
		},
		{
			Name:   "fixed number formatting",
			Expr:   `fixed(Price, 2)`,
			Values: map[string]string{"Price": "3.14159"},
			State:  map[string]any{"Price": 3.14159},
		},
		{
			Name:   "fixed rounds half away from zero",
			Expr:   `fixed(Half, 0) + "/" + fixed(-2.5, 0)`,
			Values: map[string]string{"Half": "2.5"},
			State:  map[string]any{"Half": 2.5},
		},
		{
			Name:   "round returns a number",
			Expr:   `round(Price, 2) == 3.14`,
			Values: map[string]string{"Price": "3.14159"},
			State:  map[string]any{"Price": 3.14159},
		},
		{
			Name:   "percent formatting",
			Expr:   `percent(Ratio, 1)`,
			Values: map[string]string{"Ratio": "0.1234"},
			State:  map[string]any{"Ratio": 0.1234},
		},
		{
			Name:   "format time utc",
			Expr:   `formatTime(Ts, "YYYY-MM-DD HH:mm:ss")`,
			Values: map[string]string{"Ts": "1700000000"},
			State:  map[string]any{"Ts": 1700000000},
		},
		{
			Name:   "format time before epoch",
			Expr:   `formatTime(-86401, "YYYY-MM-DD")`,
			Values: map[string]string{},
			State:  map[string]any{},
		},
		{
			Name:   "fixed rejects fractional digits",
			Expr:   `fixed(Price, 1.5)`,
			Values: map[string]string{"Price": "3.14"},
			State:  map[string]any{"Price": 3.14},
		},
		{
			Name:   "format time rejects fractional timestamp",
			Expr:   `formatTime(1.5, "YYYY")`,
			Values: map[string]string{},
			State:  map[string]any{},
		},
		{
			Name:   "nil literal comparison",
			Expr:   `nil == nil`,
			Values: map[string]string{},
			State:  map[string]any{},
		},
		{
			Name:   "unary operators",
			Expr:   `!Open && -(Count - 10) == 7`,
			Values: map[string]string{"Open": "false", "Count": "3"},
			State:  map[string]any{"Open": false, "Count": 3},
		},
		{
			Name:   "array numeric fields",
			Expr:   `Items[1].Count % 2 == 1`,
			Values: map[string]string{"Items": `[{"Count":2},{"Count":3}]`},
			State:  map[string]any{"Items": []any{map[string]any{"Count": 2}, map[string]any{"Count": 3}}},
		},
		{
			Name:   "string comparison",
			Expr:   `Name < Query`,
			Values: map[string]string{"Name": "Ada", "Query": "Grace"},
			State:  map[string]any{"Name": "Ada", "Query": "Grace"},
		},
		{
			Name:   "object equality",
			Expr:   `User == OtherUser`,
			Values: map[string]string{"User": `{"Name":"Ada","Open":true}`, "OtherUser": `{"Name":"Ada","Open":true}`},
			State:  map[string]any{"User": map[string]any{"Name": "Ada", "Open": true}, "OtherUser": map[string]any{"Name": "Ada", "Open": true}},
		},
		{
			Name:   "array equality",
			Expr:   `Items == Copy`,
			Values: map[string]string{"Items": `[{"Count":2},{"Count":3}]`, "Copy": `[{"Count":2},{"Count":3}]`},
			State:  map[string]any{"Items": []any{map[string]any{"Count": 2}, map[string]any{"Count": 3}}, "Copy": []any{map[string]any{"Count": 2}, map[string]any{"Count": 3}}},
		},
		{
			Name:   "invalid integer conversion",
			Expr:   `int("bad")`,
			Values: map[string]string{},
			State:  map[string]any{},
		},
		{
			Name:   "invalid float conversion",
			Expr:   `float("bad")`,
			Values: map[string]string{},
			State:  map[string]any{},
		},
		{
			Name:   "len rejects numbers",
			Expr:   `len(Count)`,
			Values: map[string]string{"Count": "3"},
			State:  map[string]any{"Count": 3},
		},
		{
			Name:   "lower rejects numbers",
			Expr:   `lower(Count)`,
			Values: map[string]string{"Count": "3"},
			State:  map[string]any{"Count": 3},
		},
		{
			Name:   "contains rejects non-string argument",
			Expr:   `contains(Name, Count)`,
			Values: map[string]string{"Name": "Ada", "Count": "3"},
			State:  map[string]any{"Name": "Ada", "Count": 3},
		},
		{
			Name:   "addition rejects mixed types",
			Expr:   `Count + Name`,
			Values: map[string]string{"Count": "3", "Name": "Ada"},
			State:  map[string]any{"Count": 3, "Name": "Ada"},
		},
		{
			Name:   "logical operator rejects non-bool",
			Expr:   `Count && Open`,
			Values: map[string]string{"Count": "3", "Open": "true"},
			State:  map[string]any{"Count": 3, "Open": true},
		},
		{
			Name:   "logical operator evaluates both sides",
			Expr:   `false && Missing`,
			Values: map[string]string{},
			State:  map[string]any{},
		},
		{
			Name:   "conditional rejects non-bool",
			Expr:   `if Count { "bad" } else { "ok" }`,
			Values: map[string]string{"Count": "3"},
			State:  map[string]any{"Count": 3},
		},
		{
			Name:   "unknown identifier",
			Expr:   `Missing`,
			Values: map[string]string{},
			State:  map[string]any{},
		},
		{
			Name:   "unknown member",
			Expr:   `User.Missing`,
			Values: map[string]string{"User": `{"Name":"Ada"}`},
			State:  map[string]any{"User": map[string]any{"Name": "Ada"}},
		},
		{
			Name:   "index out of range",
			Expr:   `Items[5]`,
			Values: map[string]string{"Items": `[{"Name":"first"}]`},
			State:  map[string]any{"Items": []any{map[string]any{"Name": "first"}}},
		},
	}

	expected := make(map[string]expressionExpectation, len(cases))
	for _, tc := range cases {
		value, err := clientlang.EvalValue(tc.Expr, tc.Values)
		if err != nil {
			expected[tc.Name] = expressionExpectation{Error: err.Error()}
			continue
		}
		expected[tc.Name] = expressionExpectation{Value: normalizeJSONValue(t, value)}
	}

	results := runIslandExpressionConformanceHarness(t, node, cases)
	for _, result := range results {
		want, ok := expected[result.Name]
		if !ok {
			t.Fatalf("%s: unexpected JS result", result.Name)
		}
		if want.Error != "" {
			if result.Error == "" {
				t.Fatalf("%s: JS evaluator succeeded, Go evaluator failed: %s", result.Name, want.Error)
			}
			continue
		}
		if result.Error != "" {
			t.Fatalf("%s: JS evaluator failed: %s", result.Name, result.Error)
		}
		if !reflect.DeepEqual(result.Value, want.Value) {
			t.Fatalf("%s: evaluator mismatch\nJS: %#v\nGo: %#v", result.Name, result.Value, want.Value)
		}
	}
}

type expressionConformanceResult struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
	Error string `json:"error"`
}

func runIslandExpressionConformanceHarness(t *testing.T, node string, cases []expressionConformanceCase) []expressionConformanceResult {
	t.Helper()
	script := islandExpressionConformanceHarness(t, string(IslandRuntimeSource()), cases)
	path := filepath.Join(t.TempDir(), "gowdk-expression-conformance.js")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, path).CombinedOutput()
	if err != nil {
		t.Fatalf("expression conformance harness failed: %v\n%s", err, output)
	}
	var results []expressionConformanceResult
	decoder := json.NewDecoder(bytes.NewReader(output))
	if err := decoder.Decode(&results); err != nil {
		t.Fatalf("invalid expression conformance output: %v\n%s", err, output)
	}
	return results
}

func islandExpressionConformanceHarness(t *testing.T, runtime string, cases []expressionConformanceCase) string {
	t.Helper()
	runtimeJSON, err := json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	casesJSON, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	return `
'use strict';

const runtime = ` + string(runtimeJSON) + `;
const cases = ` + string(casesJSON) + `;
globalThis.window = {
  __gowdkIslandRegistry: { components: Object.create(null), roots: new WeakMap() },
  addEventListener() {}
};
globalThis.document = {
  querySelectorAll() { return []; }
};

const hook = "\n  window.__gowdkEvalExpressionForTest = (expr, state) => valueOf(expr, state || {}, null, {}, []);\n})();";
const source = runtime.replace(/\}\)\(\);\s*$/, hook);
if (source === runtime) throw new Error('failed to inject expression test hook');
eval(source);

const results = cases.map((tc) => {
  try {
    return { name: tc.name, value: window.__gowdkEvalExpressionForTest(tc.expr, tc.state) };
  } catch (error) {
    return { name: tc.name, error: error && error.message || String(error) };
  }
});
process.stdout.write(JSON.stringify(results));
`
}

func normalizeJSONValue(t *testing.T, value any) any {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var normalized any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	if err := decoder.Decode(&normalized); err != nil {
		t.Fatal(err)
	}
	return normalized
}
