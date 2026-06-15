package validation

import "testing"

func TestResultOK(t *testing.T) {
	if !(Result{}).OK() {
		t.Fatal("empty result should be OK")
	}
}

func TestResultAddRecordsErrors(t *testing.T) {
	var result Result
	result.Add("email", "is required")

	if result.OK() {
		t.Fatal("result with errors should not be OK")
	}
	if len(result.Errors) != 1 || result.Errors[0].Field != "email" || result.Errors[0].Message != "is required" {
		t.Fatalf("unexpected errors: %#v", result.Errors)
	}
}

func TestResultMessages(t *testing.T) {
	var result Result
	result.Add("email", "is required")
	result.Add("email", "must be valid")
	result.Add("name", "is required")

	messages := result.Messages()
	if len(messages) != 3 || messages[0] != "is required" || messages[2] != "is required" {
		t.Fatalf("unexpected messages: %#v", messages)
	}

	emailMessages := result.FieldMessages("email")
	if len(emailMessages) != 2 || emailMessages[1] != "must be valid" {
		t.Fatalf("unexpected email messages: %#v", emailMessages)
	}

	byField := result.ByField()
	if len(byField["email"]) != 2 || byField["name"][0] != "is required" {
		t.Fatalf("unexpected field messages: %#v", byField)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{`[a-z]+@[a-z]+[.][a-z]{2,4}`, "me@example.com", true},
		{`[a-z]+@[a-z]+[.][a-z]{2,4}`, "me@example.company", false},
		{`^go[wd]?k$`, "gowk", true},
		{`^go[wd]?k$`, "gowdk", false},
		{`a{2,}`, "aaaa", true},
		{`a{2,3}`, "a", false},
		{`file[0-9][.]txt`, "file7.txt", true},
		{`(?:cat|dog)-\d+`, "dog-42", true},
		{`(?:cat|dog)-\d+`, "bird-42", false},
		{`\w+@\w+[.]\w{2,4}`, "me@example.dev", true},
		{`[\dA-F]+`, "19AF", true},
		{`[\dA-F]+`, "19AG", false},
	}

	for _, test := range tests {
		got, err := MatchPattern(test.pattern, test.value)
		if err != nil {
			t.Fatalf("MatchPattern(%q, %q) error: %v", test.pattern, test.value, err)
		}
		if got != test.want {
			t.Fatalf("MatchPattern(%q, %q) = %v, want %v", test.pattern, test.value, got, test.want)
		}
	}
}

func TestValidatePatternRejectsUnsupportedOperators(t *testing.T) {
	tests := []string{
		`(?=a)`,
		`(?P<name>a)`,
		`a+?`,
		`[\D]`,
	}
	for _, test := range tests {
		if err := ValidatePattern(test); err == nil {
			t.Fatalf("expected unsupported pattern %q to fail", test)
		}
	}
}

func TestMatchPatternTreatsInnerAnchorsAsLiterals(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{`a^b`, "a^b", true},
		{`a^b`, "ab", false},
		{`a$b`, "a$b", true},
		{`a$b`, "ab", false},
	}
	for _, test := range tests {
		got, err := MatchPattern(test.pattern, test.value)
		if err != nil {
			t.Fatalf("MatchPattern(%q, %q) error: %v", test.pattern, test.value, err)
		}
		if got != test.want {
			t.Fatalf("MatchPattern(%q, %q) = %v, want %v", test.pattern, test.value, got, test.want)
		}
	}
}
