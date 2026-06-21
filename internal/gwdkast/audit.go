package gwdkast

import "github.com/cssbruno/gowdk/internal/source"

// AuditFile is the typed AST for a *.audit.gwdk source.
type AuditFile struct {
	Package  *Package
	Policies []AuditPolicy
	Tests    []AuditTest
}

// AuditPolicy declares a composable security policy.
type AuditPolicy struct {
	Name    string
	Extends []string
	Applies []AuditApply
	Rules   []AuditRule
	Span    source.SourceSpan
}

// AuditApply applies a policy to one selector.
type AuditApply struct {
	Selector string
	Span     source.SourceSpan
}

// AuditRule declares one policy rule. Attrs carries structured arguments for
// rules that need more than a single value (for example a raw-HTML exception's
// owner, justification, expiry, and sanitizer contract).
type AuditRule struct {
	Kind  string
	Value string
	Code  string
	Attrs map[string]string
	Span  source.SourceSpan
}

// AuditTest preserves one declared audit integration test block for generated
// runtime tests.
type AuditTest struct {
	Name string
	Body string
	Span source.SourceSpan
}
