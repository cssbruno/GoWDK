package source

import (
	"strings"
	"testing"
)

func TestSupportedBackendInputFieldTypes(t *testing.T) {
	got := strings.Join(SupportedBackendInputFieldTypes(), ",")
	want := "[]form.File,[]string,bool,byte,form.File,int,int16,int32,int64,int8,rune,string,uint,uint16,uint32,uint64,uint8"
	if got != want {
		t.Fatalf("SupportedBackendInputFieldTypes() = %q, want %q", got, want)
	}
	for _, name := range SupportedBackendInputFieldTypes() {
		info, ok := LookupBackendInputFieldType(name)
		if !ok || info.Name != name || info.Kind == "" {
			t.Fatalf("LookupBackendInputFieldType(%q) = %#v, %v", name, info, ok)
		}
	}
	if _, ok := LookupBackendInputFieldType("float64"); ok {
		t.Fatal("float64 must not be a supported backend input field type until decoders support it")
	}
	if info, ok := LookupBackendInputFieldType("byte"); !ok || info.Name != "byte" || info.Kind != BackendInputFieldKindUnsignedInt || info.BitSize != 8 {
		t.Fatalf("LookupBackendInputFieldType(byte) = %#v, %v", info, ok)
	}
	if info, ok := LookupBackendInputFieldType("rune"); !ok || info.Name != "rune" || info.Kind != BackendInputFieldKindSignedInt || info.BitSize != 32 {
		t.Fatalf("LookupBackendInputFieldType(rune) = %#v, %v", info, ok)
	}
}

func TestValidateBackendRoutePath(t *testing.T) {
	valid := []string{
		"/",
		"/patients",
		"/patients/list",
	}
	for _, path := range valid {
		if err := ValidateBackendRoutePath(path); err != nil {
			t.Fatalf("expected %q to be valid, got %v", path, err)
		}
	}

	invalid := []string{
		"",
		"patients",
		"//example.com/pay",
		`/\example.com/pay`,
		"https://example.com/pay",
		"/https://example.com/pay",
		"/patients?filter=active",
		"/patients#form",
		"/patients/{id}",
		"/patients\nadmin",
		"/patients\\admin",
		"/patients/../admin",
		"/patients//active",
		"/patients/./active",
		"/patients/",
		" /patients",
	}
	for _, path := range invalid {
		if err := ValidateBackendRoutePath(path); err == nil {
			t.Fatalf("expected %q to be invalid", path)
		}
	}
}

func TestValidateBackendRoutePattern(t *testing.T) {
	valid := []string{
		"/",
		"/patients",
		"/patients/{id}",
		"/patients/{id:int}",
		"/docs/{path...}",
	}
	for _, path := range valid {
		if err := ValidateBackendRoutePattern(path); err != nil {
			t.Fatalf("expected %q to be valid, got %v", path, err)
		}
	}

	invalid := []string{
		"",
		"patients",
		"//example.com/pay",
		`/\example.com/pay`,
		"/patients?filter=active",
		"/patients#form",
		"/patients/{id:uuid}",
		"/patients/{id}/{id}",
		"/docs/{path...}/edit",
		"/docs/{path...:int}",
		"/patients/{id?}",
		"/patients/{id",
		"/patients/{id}/",
		"/patients//{id}",
		"/patients/../{id}",
	}
	for _, path := range invalid {
		if err := ValidateBackendRoutePattern(path); err == nil {
			t.Fatalf("expected %q to be invalid", path)
		}
	}
}

func TestPositionAtAndOffsetOf(t *testing.T) {
	// Multi-line, multi-byte (the euro sign is 3 bytes) so rune columns and byte
	// offsets diverge.
	src := []byte("ab\ncd€f\ngh")

	cases := []struct {
		offset int
		line   int
		column int
	}{
		{0, 1, 1},  // 'a'
		{1, 1, 2},  // 'b'
		{2, 1, 3},  // '\n' at end of line 1
		{3, 2, 1},  // 'c'
		{5, 2, 3},  // start of the 3-byte euro rune
		{8, 2, 4},  // 'f', immediately after the euro rune
		{10, 3, 1}, // 'g' on line 3
		{12, 3, 3}, // end of buffer
	}

	for _, tc := range cases {
		got := PositionAt(src, tc.offset)
		if got.Line != tc.line || got.Column != tc.column || got.Offset != tc.offset {
			t.Fatalf("PositionAt(%d) = {Line:%d Column:%d Offset:%d}, want {Line:%d Column:%d Offset:%d}",
				tc.offset, got.Line, got.Column, got.Offset, tc.line, tc.column, tc.offset)
		}
		if back := OffsetOf(src, got); back != tc.offset {
			t.Fatalf("OffsetOf(PositionAt(%d)) = %d, want %d", tc.offset, back, tc.offset)
		}
	}
}

func TestPositionAtClampsBounds(t *testing.T) {
	src := []byte("abc")
	if got := PositionAt(src, -5); got.Offset != 0 || got.Line != 1 || got.Column != 1 {
		t.Fatalf("PositionAt(-5) = %+v, want clamped to start", got)
	}
	if got := PositionAt(src, 99); got.Offset != len(src) {
		t.Fatalf("PositionAt(99) Offset = %d, want %d", got.Offset, len(src))
	}
}

func TestOffsetOfUnsetPosition(t *testing.T) {
	src := []byte("abc")
	if got := OffsetOf(src, SourcePosition{}); got != 0 {
		t.Fatalf("OffsetOf(unset) = %d, want 0", got)
	}
	if got := OffsetOf(src, SourcePosition{Line: 9, Column: 9}); got != len(src) {
		t.Fatalf("OffsetOf(out-of-range) = %d, want clamp %d", got, len(src))
	}
}

func TestBackendRouteMethod(t *testing.T) {
	if got := BackendRouteMethod(" post "); got != "POST" {
		t.Fatalf("expected normalized method POST, got %q", got)
	}
}

func TestBackendRoutePath(t *testing.T) {
	tests := map[string]string{
		"/patients":  "/patients",
		"/patients/": "/patients",
		"patients":   "/patients",
	}
	for input, want := range tests {
		if got := BackendRoutePath(input); got != want {
			t.Fatalf("expected %q to normalize to %q, got %q", input, want, got)
		}
	}
}
