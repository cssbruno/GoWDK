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

func TestDecodeExpectedIgnoresRuntimeFields(t *testing.T) {
	values, err := DecodeExpected(Values{
		"email":       {"reader@example.com"},
		"_gowdk_csrf": {"signed-token"},
		"_method":     {"POST"},
	}, Schema{Fields: []Field{{Name: "email"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := values.First("email"); got != "reader@example.com" {
		t.Fatalf("unexpected email: %q", got)
	}
	if values.HasSubmitted("_gowdk_csrf") {
		t.Fatalf("runtime field should not be copied: %#v", values)
	}
}

func TestDecodeExpectedAllowsDeclaredSubmitIntentFields(t *testing.T) {
	values, err := DecodeExpected(Values{
		"email":  {"reader@example.com"},
		"intent": {"publish"},
	}, Schema{Fields: []Field{{Name: "email"}, {Name: "intent"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := values.First("intent"); got != "publish" {
		t.Fatalf("unexpected intent: %q", got)
	}

	_, err = DecodeExpected(Values{
		"email":  {"reader@example.com"},
		"intent": {"publish"},
	}, Schema{Fields: []Field{{Name: "email"}}})
	if err == nil {
		t.Fatal("expected undeclared intent field to be rejected")
	}
	if !strings.Contains(err.Error(), "intent: unexpected field") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "publish") {
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

func TestScalarHelpersDecodeValuesWithoutReflection(t *testing.T) {
	values := Values{
		"email":  {"reader@example.com"},
		"active": {"on"},
		"age":    {"42"},
		"rank":   {"7"},
		"tag":    {"go", "web"},
	}

	email, ok, err := String(values, "email")
	if err != nil || !ok || email != "reader@example.com" {
		t.Fatalf("unexpected string decode: value=%q ok=%v err=%v", email, ok, err)
	}
	active, ok, err := Bool(values, "active")
	if err != nil || !ok || !active {
		t.Fatalf("unexpected bool decode: value=%v ok=%v err=%v", active, ok, err)
	}
	age, ok, err := Int(values, "age", 0)
	if err != nil || !ok || age != 42 {
		t.Fatalf("unexpected int decode: value=%d ok=%v err=%v", age, ok, err)
	}
	rank, ok, err := Uint(values, "rank", 8)
	if err != nil || !ok || rank != 7 {
		t.Fatalf("unexpected uint decode: value=%d ok=%v err=%v", rank, ok, err)
	}
	if tags := Strings(values, "tag"); strings.Join(tags, ",") != "go,web" {
		t.Fatalf("unexpected repeated strings: %#v", tags)
	}
}

func TestControlHelpersDecodeSelectRadioAndCheckboxes(t *testing.T) {
	values := Values{
		"topic":    {"news"},
		"tags":     {"go", "web"},
		"choice":   {"daily"},
		"enabled":  {"custom-value"},
		"disabled": {"off"},
	}

	topic, ok, err := Select(values, "topic")
	if err != nil || !ok || topic != "news" {
		t.Fatalf("unexpected select decode: value=%q ok=%v err=%v", topic, ok, err)
	}
	if got := SelectMultiple(values, "tags"); strings.Join(got, ",") != "go,web" {
		t.Fatalf("unexpected select multiple decode: %#v", got)
	}
	choice, ok, err := Radio(values, "choice")
	if err != nil || !ok || choice != "daily" {
		t.Fatalf("unexpected radio decode: value=%q ok=%v err=%v", choice, ok, err)
	}
	enabled, err := Checkbox(values, "enabled")
	if err != nil || !enabled {
		t.Fatalf("expected custom checkbox value to mean checked, got value=%v err=%v", enabled, err)
	}
	disabled, err := Checkbox(values, "disabled")
	if err != nil || disabled {
		t.Fatalf("expected off checkbox to decode false, got value=%v err=%v", disabled, err)
	}
	missing, err := Checkbox(values, "missing")
	if err != nil || missing {
		t.Fatalf("expected absent checkbox to decode false, got value=%v err=%v", missing, err)
	}
	if got := CheckboxGroup(values, "tags"); strings.Join(got, ",") != "go,web" {
		t.Fatalf("unexpected checkbox group values: %#v", got)
	}
}

func TestCheckboxRejectsRepeatedScalarWithoutLeakingValues(t *testing.T) {
	_, err := Checkbox(Values{"enabled": {"value-a", "value-b"}}, "enabled")
	if err == nil || !strings.Contains(err.Error(), "enabled: repeated checkbox field") {
		t.Fatalf("expected repeated checkbox error, got %v", err)
	}
	if strings.Contains(err.Error(), "value-a") || strings.Contains(err.Error(), "value-b") {
		t.Fatalf("error leaked submitted value: %v", err)
	}
}

func TestScalarHelpersRejectRepeatedScalarWithoutLeakingValues(t *testing.T) {
	_, _, err := String(Values{"email": {"a@example.com", "b@example.com"}}, "email")
	if err == nil || !strings.Contains(err.Error(), "email: repeated scalar field") {
		t.Fatalf("expected repeated scalar error, got %v", err)
	}
	if strings.Contains(err.Error(), "a@example.com") || strings.Contains(err.Error(), "b@example.com") {
		t.Fatalf("error leaked submitted value: %v", err)
	}
}

func TestScalarHelpersLeaveBlankNumbersAndBooleansZero(t *testing.T) {
	flag, ok, err := Bool(Values{"flag": {""}}, "flag")
	if err != nil || !ok || flag {
		t.Fatalf("expected blank bool zero value, got value=%v ok=%v err=%v", flag, ok, err)
	}
	count, ok, err := Int(Values{"count": {""}}, "count", 0)
	if err != nil || !ok || count != 0 {
		t.Fatalf("expected blank int zero value, got value=%d ok=%v err=%v", count, ok, err)
	}
}

func TestValuesHasSubmitted(t *testing.T) {
	values := Values{
		"blank": {"", " \t "},
		"name":  {"", "Bruno"},
	}

	if values.HasSubmitted("blank") {
		t.Fatal("blank field should not be submitted")
	}
	if !values.HasSubmitted("name") {
		t.Fatal("name field should be submitted")
	}
	if values.HasSubmitted("missing") {
		t.Fatal("missing field should not be submitted")
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
