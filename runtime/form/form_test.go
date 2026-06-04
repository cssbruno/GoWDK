package form

import (
	"net/url"
	"strings"
	"testing"
)

func TestFromURLValuesCopiesRepeatedValues(t *testing.T) {
	raw := url.Values{
		"email": {"reader@example.com", "second@example.com"},
	}

	values := FromURLValues(raw)
	raw["email"][0] = "changed@example.com"

	if got := values.First("email"); got != "reader@example.com" {
		t.Fatalf("expected copied first value, got %q", got)
	}
	all := values.All("email")
	if len(all) != 2 || all[1] != "second@example.com" {
		t.Fatalf("expected repeated values to be preserved, got %#v", all)
	}
	all[0] = "changed@example.com"
	if got := values.First("email"); got != "reader@example.com" {
		t.Fatalf("expected All to return a copy, got %q", got)
	}
}

func TestDecodeExpectedRejectsUnknownFields(t *testing.T) {
	_, err := DecodeExpected(Values{
		"email": {"reader@example.com"},
		"role":  {"admin"},
	}, Schema{Fields: []Field{{Name: "email"}}})
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "role: unexpected field") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "admin") {
		t.Fatalf("error leaked submitted value: %v", err)
	}
}

func TestDecodeExpectedPreservesRepeatedValues(t *testing.T) {
	values, err := DecodeExpected(Values{
		"tag": {"go", "web"},
	}, Schema{Fields: []Field{{Name: "tag"}}})
	if err != nil {
		t.Fatal(err)
	}
	all := values.All("tag")
	if len(all) != 2 || all[0] != "go" || all[1] != "web" {
		t.Fatalf("expected repeated values, got %#v", all)
	}
}

func TestDecodeExpectedRejectsDuplicateSchemaFields(t *testing.T) {
	_, err := DecodeExpected(nil, Schema{Fields: []Field{
		{Name: "email"},
		{Name: "email"},
	}})
	if err == nil {
		t.Fatal("expected duplicate schema field error")
	}
	if !strings.Contains(err.Error(), "email: duplicate expected field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValuesNamesAreStable(t *testing.T) {
	names := Values{
		"z": {"last"},
		"a": {"first"},
	}.Names()
	if strings.Join(names, ",") != "a,z" {
		t.Fatalf("unexpected names: %#v", names)
	}
}
