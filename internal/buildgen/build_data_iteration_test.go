package buildgen

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func evalBuildData(t *testing.T, body string, routeParams map[string]string) map[string]string {
	t.Helper()
	data, err := parseBuildData(body, routeParams, "", nil, nil, "")
	if err != nil {
		t.Fatalf("parseBuildData(%q): %v", body, err)
	}
	return data
}

func assertBuildField(t *testing.T, data map[string]string, field, want string) {
	t.Helper()
	if got := data[field]; got != want {
		t.Fatalf("build field %q = %q, want %q\nall fields: %#v", field, got, want, data)
	}
}

func TestBuildDataReviewRegressions(t *testing.T) {
	t.Run("raw string list elements", func(t *testing.T) {
		data := evalBuildData(t, "=> { labels: [`a,b`, `c`] }", nil)
		assertBuildField(t, data, "labels", `["a,b","c"]`)
	})

	t.Run("leading-zero integers serialize as canonical json", func(t *testing.T) {
		data := evalBuildData(t, `=> { nums: [01, 02] }`, nil)
		assertBuildField(t, data, "nums", "[1,2]")
	})

	t.Run("wide integer literals keep full precision", func(t *testing.T) {
		data := evalBuildData(t, `=> { ids: [9007199254740993] }`, nil)
		assertBuildField(t, data, "ids", "[9007199254740993]")
	})

	errorCases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "reverse charges the element budget",
			body: `=> { xs: seq(0, 30000) }
=> { ys: reverse(field("xs")) }`,
			want: "exceeded the limit",
		},
		{
			name: "take charges the element budget",
			body: `=> { xs: seq(0, 30000) }
=> { ys: take(field("xs"), 30000) }`,
			want: "exceeded the limit",
		},
		{
			name: "seq bounds cannot overflow",
			body: `=> { x: seq(-9000000000000000000, 9000000000000000000) }`,
			want: "seq bounds must be within",
		},
		{
			name: "deeply nested expressions are bounded",
			body: `=> { deep: ` + strings.Repeat("(", 70) + "1" + strings.Repeat(")", 70) + ` }`,
			want: "nested too deeply",
		},
	}
	for _, testCase := range errorCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := parseBuildData(testCase.body, nil, "", nil, nil, "")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", testCase.want)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), testCase.want)
			}
		})
	}
}

func TestBuildFunctionOutputPreservesWideIntegers(t *testing.T) {
	data, err := parseBuildFunctionOutput([]byte(`{"big":9007199254740993,"ids":[9007199254740993,2]}`))
	if err != nil {
		t.Fatal(err)
	}
	assertBuildField(t, data, "big", "9007199254740993")
	assertBuildField(t, data, "ids", "[9007199254740993,2]")
}

func TestBuildDataComprehensionFilterAndReductions(t *testing.T) {
	data := evalBuildData(t, `=> { nums: seq(1, 6) }
=> { evens: [n for n in field("nums") if n % 2 == 0] }
=> { total: sum(field("evens")), label: join(field("evens"), ", "), howMany: count(field("evens")) }`, nil)

	assertBuildField(t, data, "nums", "[1,2,3,4,5]")
	assertBuildField(t, data, "evens", "[2,4]")
	assertBuildField(t, data, "total", "6")
	assertBuildField(t, data, "label", "2, 4")
	assertBuildField(t, data, "howMany", "2")
}

func TestBuildDataComprehensionMapsObjectsWithSelectorAndIndex(t *testing.T) {
	data := evalBuildData(t, `=> { items: [ {id: n, sq: n * n} for n in seq(1, 4) ] }
=> { firstID: first(field("items")).id, middle: field("items")[1].sq }`, nil)

	assertBuildField(t, data, "items", `[{"id":1,"sq":1},{"id":2,"sq":4},{"id":3,"sq":9}]`)
	assertBuildField(t, data, "firstID", "1")
	assertBuildField(t, data, "middle", "4")
}

func TestBuildDataComprehensionBindsIndexVariable(t *testing.T) {
	data := evalBuildData(t, `=> { names: ["alpha", "beta"] }
=> { rows: [ {i: idx, v: name} for name, idx in field("names") ] }`, nil)

	assertBuildField(t, data, "rows", `[{"i":0,"v":"alpha"},{"i":1,"v":"beta"}]`)
}

func TestBuildDataComprehensionTransformsStrings(t *testing.T) {
	data := evalBuildData(t, `=> { names: ["alpha", "beta"] }
=> { slugs: ["item-" + n for n in field("names")] }
=> { joined: join(field("slugs"), " ") }`, nil)

	assertBuildField(t, data, "slugs", `["item-alpha","item-beta"]`)
	assertBuildField(t, data, "joined", "item-alpha item-beta")
}

func TestBuildDataNestedComprehension(t *testing.T) {
	data := evalBuildData(t, `=> { grid: [ [a * b for b in seq(1, 3)] for a in seq(1, 3) ] }`, nil)
	assertBuildField(t, data, "grid", "[[1,2],[2,4]]")
}

func TestBuildDataListTransformBuiltins(t *testing.T) {
	data := evalBuildData(t, `=> { nums: seq(1, 6) }
=> { top: take(reverse(field("nums")), 2), head: first(field("nums")), tail: last(field("nums")) }`, nil)

	assertBuildField(t, data, "top", "[5,4]")
	assertBuildField(t, data, "head", "1")
	assertBuildField(t, data, "tail", "5")
}

func TestBuildDataComprehensionUsesRouteParams(t *testing.T) {
	data := evalBuildData(t, `=> { tags: [param("topic") + "-" + n for n in ["a", "b"]] }`, map[string]string{"topic": "go"})
	assertBuildField(t, data, "tags", `["go-a","go-b"]`)
}

func TestBuildDataIterationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "source not a list",
			body: `=> { bad: [x for x in 5] }`,
			want: "comprehension source must be a list",
		},
		{
			name: "unknown loop variable",
			body: `=> { bad: [y for x in seq(1, 3)] }`,
			want: `unknown build field reference "y"`,
		},
		{
			name: "filter not boolean",
			body: `=> { bad: [x for x in seq(1, 3) if x] }`,
			want: "comprehension filter must be a boolean",
		},
		{
			name: "iteration budget exceeded",
			body: `=> { big: seq(0, 100000) }`,
			want: "exceeded the limit of 50000 elements",
		},
		{
			name: "sum of non-numbers",
			body: `=> { strs: ["a", "b"], bad: sum(field("strs")) }`,
			want: "sum requires a list of numbers",
		},
		{
			name: "index out of range",
			body: `=> { nums: seq(0, 2), bad: field("nums")[5] }`,
			want: "out of range",
		},
		{
			name: "selector on non-object",
			body: `=> { bad: first(seq(1, 3)).id }`,
			want: "cannot read field",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := parseBuildData(testCase.body, nil, "", nil, nil, "")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", testCase.want)
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), testCase.want)
			}
		})
	}
}

func TestBuildFunctionOutputAllowsSliceAndStructFields(t *testing.T) {
	data, err := parseBuildFunctionOutput([]byte(`{"title":"Digest","tags":["go","wasm"],"meta":{"b":2,"a":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	assertBuildField(t, data, "title", "Digest")
	assertBuildField(t, data, "tags", `["go","wasm"]`)
	// encoding/json sorts object keys, so a struct/map field serializes
	// deterministically regardless of the JSON the build helper emitted.
	assertBuildField(t, data, "meta", `{"a":1,"b":2}`)
}

func TestBuildRendersIterationDerivedScalars(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "digest",
		Route: "/",
		Blocks: gwdkir.Blocks{
			Build: true,
			BuildBody: `=> { prices: [10, 25, 5, 40] }
=> { expensive: [p for p in field("prices") if p >= 25] }
=> { count: count(field("expensive")), total: sum(field("expensive")), summary: join(field("expensive"), " + ") }`,
			View:     true,
			ViewBody: `<main data-count="{count}" data-total="{total}"><p>{summary}</p></main>`,
		},
	}}}

	if _, err := Build(gowdk.Config{}, app, outputDir); err != nil {
		t.Fatal(err)
	}
	output := readFile(t, filepath.Join(outputDir, "index.html"))
	if !strings.Contains(output, `<main data-count="2" data-total="65"><p>25 + 40</p></main>`) {
		t.Fatalf("expected iteration-derived scalars in output:\n%s", output)
	}
}

func TestBuildIterationOutputIsDeterministic(t *testing.T) {
	app := gwdkanalysis.Sources{Pages: []gwdkir.Page{{
		ID:    "digest",
		Route: "/",
		Blocks: gwdkir.Blocks{
			Build: true,
			BuildBody: `=> { rows: [ {id: n, label: "row-" + n} for n in seq(1, 13) ] }
=> { idList: [r.id for r in field("rows")] }
=> { ids: join(field("idList"), ",") }`,
			View:     true,
			ViewBody: `<main data-rows="{rows}"><p>{ids}</p></main>`,
		},
	}}}

	first := t.TempDir()
	if _, err := Build(gowdk.Config{}, app, first); err != nil {
		t.Fatal(err)
	}
	second := t.TempDir()
	if _, err := Build(gowdk.Config{}, app, second); err != nil {
		t.Fatal(err)
	}
	firstHTML := readFile(t, filepath.Join(first, "index.html"))
	secondHTML := readFile(t, filepath.Join(second, "index.html"))
	if firstHTML != secondHTML {
		t.Fatalf("build output is not deterministic\nfirst:\n%s\nsecond:\n%s", firstHTML, secondHTML)
	}
	if !strings.Contains(firstHTML, "<p>1,2,3,4,5,6,7,8,9,10,11,12</p>") {
		t.Fatalf("expected joined ids in output:\n%s", firstHTML)
	}
}
