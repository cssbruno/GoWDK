package lang

import "testing"

func TestFormatNormalizesTopLevelBlocks(t *testing.T) {
	source := []byte("@page home\n\n@route \"/\"\n\n\nview {\n<h1>GOWDK</h1>\n}\n")
	got := string(Format(source))
	want := "@page home\n@route \"/\"\n\nview {\n  <h1>GOWDK</h1>\n}\n"
	if got != want {
		t.Fatalf("unexpected format:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}
