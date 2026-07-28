package parser

import (
	"strings"
	"testing"
)

func TestParseSyntaxAcceptsLinesLargerThanScannerDefault(t *testing.T) {
	payload := strings.Repeat("x", 70<<10)
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "metadata",
			source: "page " + payload + "\n",
		},
		{
			name:   "inline js",
			source: "js {\nconst payload = \"" + payload + "\"\n}\n",
		},
		{
			name:   "inline css",
			source: "style {\n.long { --payload: " + payload + "; }\n}\n",
		},
		{
			name:   "inline go",
			source: "go {\nvar payload = \"" + payload + "\"\n}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseSyntax([]byte(test.source)); err != nil {
				t.Fatalf("expected long line to parse: %v", err)
			}
		})
	}
}

func TestParseSyntaxRejectsOversizedLineWithStableDiagnostic(t *testing.T) {
	source := "js {\n" + strings.Repeat("x", MaxSourceLineBytes+3) + "\n}\n"
	_, err := ParseSyntax([]byte(source))
	if err == nil {
		t.Fatal("expected oversized source line error")
	}
	diagnostic, ok := ParserDiagnostic(err)
	if !ok {
		t.Fatalf("expected typed parser diagnostic, got %T: %v", err, err)
	}
	if diagnostic.Code != DiagnosticSourceLineTooLong || diagnostic.Span.Start.Line != 2 {
		t.Fatalf("unexpected diagnostic: %#v", diagnostic)
	}
	if !strings.Contains(diagnostic.Message, "1048576-byte limit") {
		t.Fatalf("expected documented source limit, got %q", diagnostic.Message)
	}
	if strings.Contains(err.Error(), "block missing closing") {
		t.Fatalf("scanner limit should not cause a cascading block error: %v", err)
	}
}

func TestParseAuditSyntaxAcceptsLineLargerThanScannerDefault(t *testing.T) {
	selector := strings.Repeat("x", 70<<10)
	source := "policy long {\n  match \"" + selector + "\"\n}\n"
	file, err := ParseAuditSyntax([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Policies) != 1 || len(file.Policies[0].Applies) != 1 || file.Policies[0].Applies[0].Selector != selector {
		t.Fatalf("unexpected long audit policy: %#v", file.Policies)
	}
}

func TestParseAuditSyntaxRejectsOversizedLineWithStableDiagnostic(t *testing.T) {
	source := "policy long {\n" + strings.Repeat("x", MaxAuditLineBytes+3) + "\n}\n"
	_, err := ParseAuditSyntax([]byte(source))
	if err == nil {
		t.Fatal("expected oversized audit line error")
	}
	diagnostic, ok := ParserDiagnostic(err)
	if !ok {
		t.Fatalf("expected typed parser diagnostic, got %T: %v", err, err)
	}
	if diagnostic.Code != DiagnosticSourceLineTooLong || diagnostic.Span.Start.Line != 2 {
		t.Fatalf("unexpected diagnostic: %#v", diagnostic)
	}
	if !strings.Contains(diagnostic.Message, "1048576-byte limit") {
		t.Fatalf("expected documented audit limit, got %q", diagnostic.Message)
	}
	if strings.Contains(err.Error(), "unterminated policy") {
		t.Fatalf("scanner limit should not cause a cascading policy error: %v", err)
	}
}
