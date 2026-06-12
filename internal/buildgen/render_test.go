package buildgen

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEscapeScriptJSONEscapesEveryLiteralLessThan(t *testing.T) {
	input := `{"value":"</ScRiPt><img src=x onerror=1><!--"}`

	escaped := escapeScriptJSON(input)

	if strings.Contains(escaped, "<") {
		t.Fatalf("escaped script JSON still contains <: %q", escaped)
	}
	for _, expected := range []string{`\u003c/ScRiPt>`, `\u003cimg src=x onerror=1>`, `\u003c!--`} {
		if !strings.Contains(escaped, expected) {
			t.Fatalf("expected escaped JSON to contain %q, got %q", expected, escaped)
		}
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(escaped), &decoded); err != nil {
		t.Fatalf("escaped JSON is invalid: %v\n%s", err, escaped)
	}
	if decoded["value"] != `</ScRiPt><img src=x onerror=1><!--` {
		t.Fatalf("escaped JSON changed value: %q", decoded["value"])
	}
}
