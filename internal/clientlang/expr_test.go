package clientlang

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckExprParsesArithmeticComparisonAndBooleanLogic(t *testing.T) {
	typ, fields, err := CheckExpr(`(Count + step) >= 3 && Open == false`, map[string]ValueType{
		"Count": TypeInt,
		"step":  TypeInt,
		"Open":  TypeBool,
	})
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeBool {
		t.Fatalf("expected bool type, got %s", typ)
	}
	if strings.Join(fields, ",") != "Count,Open,step" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestParseExprWithSpansRecordsNestedSourceColumns(t *testing.T) {
	expr, err := ParseExprWithSpans(`  Count + Next(Items[0].Name)`)
	if err != nil {
		t.Fatal(err)
	}
	root, ok := expr.(BinaryExpr)
	if !ok {
		t.Fatalf("expected binary expression, got %#v", expr)
	}
	if got, want := ExprSpan(root), (ExprSourceSpan{StartColumn: 3, EndColumn: 30}); got != want {
		t.Fatalf("unexpected root span: got %#v want %#v", got, want)
	}
	call, ok := root.Right.(CallExpr)
	if !ok {
		t.Fatalf("expected call on right, got %#v", root.Right)
	}
	if got, want := ExprSpan(call), (ExprSourceSpan{StartColumn: 11, EndColumn: 30}); got != want {
		t.Fatalf("unexpected call span: got %#v want %#v", got, want)
	}
	member, ok := call.Args[0].(MemberExpr)
	if !ok {
		t.Fatalf("expected member call arg, got %#v", call.Args[0])
	}
	if got, want := ExprSpan(member), (ExprSourceSpan{StartColumn: 16, EndColumn: 29}); got != want {
		t.Fatalf("unexpected member span: got %#v want %#v", got, want)
	}
	index, ok := member.X.(IndexExpr)
	if !ok {
		t.Fatalf("expected index member base, got %#v", member.X)
	}
	if got, want := ExprSpan(index), (ExprSourceSpan{StartColumn: 16, EndColumn: 24}); got != want {
		t.Fatalf("unexpected index span: got %#v want %#v", got, want)
	}
}

func TestCanonicalExprNormalizesWhitespaceAndLiterals(t *testing.T) {
	got, err := CanonicalExpr(`if Open{Count+1}else{int("2")}`)
	if err != nil {
		t.Fatal(err)
	}
	want := `if Open { (Count + 1) } else { int("2") }`
	if got != want {
		t.Fatalf("unexpected canonical expression:\nwant %s\ngot  %s", want, got)
	}
}

func TestCanonicalExprNormalizesMatchToSwitch(t *testing.T) {
	got, err := CanonicalExpr(`match Status{case "draft":Label default:"Unknown"}`)
	if err != nil {
		t.Fatal(err)
	}
	want := `switch Status { case "draft": Label default: "Unknown" }`
	if got != want {
		t.Fatalf("unexpected canonical expression:\nwant %s\ngot  %s", want, got)
	}
}

func TestCheckExprRejectsTypeMismatch(t *testing.T) {
	_, _, err := CheckExpr(`Count && Open`, map[string]ValueType{
		"Count": TypeInt,
		"Open":  TypeBool,
	})
	if err == nil {
		t.Fatal("expected type mismatch")
	}
	if !strings.Contains(err.Error(), "operator && requires bools") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprRejectsUnsupportedExpressionWithSpan(t *testing.T) {
	_, _, err := CheckExpr(`Count && 1`, map[string]ValueType{
		"Count": TypeInt,
	})
	if err == nil {
		t.Fatal("expected type mismatch")
	}
	var validation ExprValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected ExprValidationError, got %T: %v", err, err)
	}
	if got, want := validation.Span, (ExprSourceSpan{StartColumn: 1, EndColumn: 11}); got != want {
		t.Fatalf("unexpected validation span: got %#v want %#v", got, want)
	}
}

func TestCheckExprNestedFailuresExposeNarrowSpan(t *testing.T) {
	_, _, err := CheckExprWithFunctions(`Next(Open)`, map[string]ValueType{
		"Open": TypeBool,
	}, map[string]ExprFunction{
		"Next": {Params: []ValueType{TypeInt}, Return: TypeInt},
	})
	if err == nil {
		t.Fatal("expected helper arg error")
	}
	var validation ExprValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected ExprValidationError, got %T: %v", err, err)
	}
	if got, want := validation.Span, (ExprSourceSpan{StartColumn: 6, EndColumn: 10}); got != want {
		t.Fatalf("unexpected validation span: got %#v want %#v", got, want)
	}
}

func TestCheckExprParsesNestedAndIndexReads(t *testing.T) {
	typ, fields, err := CheckExpr(`User.Name == Items[0].Name && Flags[step]`, map[string]ValueType{
		"User":         TypeObject,
		"User.Name":    TypeString,
		"Items":        TypeArray,
		"Items[]":      TypeObject,
		"Items[].Name": TypeString,
		"Flags":        TypeArray,
		"Flags[]":      TypeBool,
		"step":         TypeInt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeBool {
		t.Fatalf("expected bool type, got %s", typ)
	}
	if strings.Join(fields, ",") != "Flags,Items,User,step" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestCheckExprParsesGoishConditionalExpression(t *testing.T) {
	typ, fields, err := CheckExpr(`if Open { Count + step } else { 0 }`, map[string]ValueType{
		"Count": TypeInt,
		"Open":  TypeBool,
		"step":  TypeInt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeInt {
		t.Fatalf("expected int type, got %s", typ)
	}
	if strings.Join(fields, ",") != "Count,Open,step" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestCheckExprParsesConditionalInParenthesesAndIndex(t *testing.T) {
	typ, _, err := CheckExpr(`(if Open { Count } else { 0 })`, map[string]ValueType{
		"Open":  TypeBool,
		"Count": TypeInt,
	})
	if err != nil {
		t.Fatalf("parenthesized conditional: %v", err)
	}
	if typ != TypeInt {
		t.Fatalf("expected int type for parenthesized conditional, got %s", typ)
	}

	typ, _, err = CheckExpr(`Items[if First { 0 } else { 1 }]`, map[string]ValueType{
		"Items":   TypeArray,
		"Items[]": TypeString,
		"First":   TypeBool,
	})
	if err != nil {
		t.Fatalf("conditional index: %v", err)
	}
	if typ != TypeString {
		t.Fatalf("expected string type for conditional index, got %s", typ)
	}
}

func TestCheckExprParsesSwitchExpression(t *testing.T) {
	typ, fields, err := CheckExpr(`switch Status { case "draft": "Draft" case "live": Label default: "Unknown" }`, map[string]ValueType{
		"Status": TypeString,
		"Label":  TypeString,
	})
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeString {
		t.Fatalf("expected string type, got %s", typ)
	}
	if strings.Join(fields, ",") != "Label,Status" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestCheckExprWithFunctionsParsesHelperCalls(t *testing.T) {
	typ, fields, err := CheckExprWithFunctions(`Next(Count) + Double(step)`, map[string]ValueType{
		"Count": TypeInt,
		"step":  TypeInt,
	}, map[string]ExprFunction{
		"Next":   {Params: []ValueType{TypeInt}, Return: TypeInt},
		"Double": {Params: []ValueType{TypeInt}, Return: TypeInt},
	})
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeInt {
		t.Fatalf("expected int type, got %s", typ)
	}
	if strings.Join(fields, ",") != "Count,step" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestCheckExprWithFunctionsRejectsBadHelperArg(t *testing.T) {
	_, _, err := CheckExprWithFunctions(`Next(Open)`, map[string]ValueType{
		"Open": TypeBool,
	}, map[string]ExprFunction{
		"Next": {Params: []ValueType{TypeInt}, Return: TypeInt},
	})
	if err == nil {
		t.Fatal("expected helper arg error")
	}
	if !strings.Contains(err.Error(), "argument 1 expects int, got bool") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprParsesBuiltins(t *testing.T) {
	typ, fields, err := CheckExpr(`string(len(Items)) + ":" + string(int("2") + len(Name))`, map[string]ValueType{
		"Items": TypeArray,
		"Name":  TypeString,
	})
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeString {
		t.Fatalf("expected string type, got %s", typ)
	}
	if strings.Join(fields, ",") != "Items,Name" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestCheckExprParsesStringFilterBuiltins(t *testing.T) {
	typ, fields, err := CheckExpr(`contains(lower(item.Name), lower(Query)) || upper(Query) == "ALL"`, map[string]ValueType{
		"Query":     TypeString,
		"item":      TypeObject,
		"item.Name": TypeString,
	})
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeBool {
		t.Fatalf("expected bool type, got %s", typ)
	}
	if strings.Join(fields, ",") != "Query,item" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestCheckExprRejectsBadBuiltinArg(t *testing.T) {
	_, _, err := CheckExpr(`len(Count)`, map[string]ValueType{
		"Count": TypeInt,
	})
	if err == nil {
		t.Fatal("expected bad len argument")
	}
	if !strings.Contains(err.Error(), "built-in len expects string or array") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprAllowsNilScalarComparison(t *testing.T) {
	typ, fields, err := CheckExpr(`Name != nil`, map[string]ValueType{
		"Name": TypeString,
	})
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeBool {
		t.Fatalf("expected bool type, got %s", typ)
	}
	if strings.Join(fields, ",") != "Name" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestCheckExprRejectsNilConditionalBranch(t *testing.T) {
	_, _, err := CheckExpr(`if Open { nil } else { Name }`, map[string]ValueType{
		"Name": TypeString,
		"Open": TypeBool,
	})
	if err == nil {
		t.Fatal("expected nil conditional branch diagnostic")
	}
	if !strings.Contains(err.Error(), "nil is only supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprRejectsNilObjectComparison(t *testing.T) {
	_, _, err := CheckExpr(`User == nil`, map[string]ValueType{
		"User": TypeObject,
	})
	if err == nil {
		t.Fatal("expected nil object comparison diagnostic")
	}
	if !strings.Contains(err.Error(), "supports nil only with scalar values") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExprCallsCollectsHelperCalls(t *testing.T) {
	calls, err := ExprCalls(`A(B(Count), if Open { C() } else { switch Kind { case "d": D() default: E() } })`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "A,B,C,D,E" {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestCheckExprRejectsGoishConditionalTypeMismatch(t *testing.T) {
	_, _, err := CheckExpr(`if Open { Count } else { "closed" }`, map[string]ValueType{
		"Count": TypeInt,
		"Open":  TypeBool,
	})
	if err == nil {
		t.Fatal("expected branch type mismatch")
	}
	if !strings.Contains(err.Error(), "if expression branches must have matching types") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprRejectsGoishConditionalNonBoolCondition(t *testing.T) {
	_, _, err := CheckExpr(`if Count { 1 } else { 0 }`, map[string]ValueType{
		"Count": TypeInt,
	})
	if err == nil {
		t.Fatal("expected non-bool condition diagnostic")
	}
	if !strings.Contains(err.Error(), "if expression condition requires bool") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprRejectsSwitchCaseTypeMismatch(t *testing.T) {
	_, _, err := CheckExpr(`switch Count { case "1": 1 default: 0 }`, map[string]ValueType{
		"Count": TypeInt,
	})
	if err == nil {
		t.Fatal("expected case type mismatch")
	}
	if !strings.Contains(err.Error(), "switch case requires comparable matching types") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprRejectsSwitchBranchTypeMismatch(t *testing.T) {
	_, _, err := CheckExpr(`switch Count { case 1: 1 default: "many" }`, map[string]ValueType{
		"Count": TypeInt,
	})
	if err == nil {
		t.Fatal("expected branch type mismatch")
	}
	if !strings.Contains(err.Error(), "switch expression branches must have matching types") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprRejectsUnknownNestedField(t *testing.T) {
	_, _, err := CheckExpr(`User.Missing`, map[string]ValueType{
		"User":      TypeObject,
		"User.Name": TypeString,
	})
	if err == nil {
		t.Fatal("expected unknown nested field")
	}
	if !strings.Contains(err.Error(), `unknown client value "User.Missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckExprRejectsNonIntIndex(t *testing.T) {
	_, _, err := CheckExpr(`Items["0"]`, map[string]ValueType{
		"Items":   TypeArray,
		"Items[]": TypeString,
	})
	if err == nil {
		t.Fatal("expected non-int index diagnostic")
	}
	if !strings.Contains(err.Error(), "index expression requires int") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExprFieldsDoesNotNeedTypes(t *testing.T) {
	fields, err := ExprFields(`(Count + step) * 2`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fields, ",") != "Count,step" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestEvalBoolEvaluatesScalarExpression(t *testing.T) {
	got, err := EvalBool(`Count > 2 && Open`, map[string]string{
		"Count": "3",
		"Open":  "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected expression to be true")
	}
}

func TestEvalScalarEvaluatesGoishConditionalExpression(t *testing.T) {
	got, err := EvalScalar(`if Open { Name } else { "closed" }`, map[string]string{
		"Name": "Ada",
		"Open": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Ada" {
		t.Fatalf("expected Ada, got %q", got)
	}
}

func TestEvalScalarEvaluatesSwitchExpression(t *testing.T) {
	got, err := EvalScalar(`match Status { case "open": Count case "closed": 0 default: 1 }`, map[string]string{
		"Count":  "3",
		"Status": "open",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "3" {
		t.Fatalf("expected 3, got %q", got)
	}
}

func TestEvalBoolEvaluatesNestedAndIndexExpression(t *testing.T) {
	got, err := EvalBool(`User.Open && Items[0].Name == "first"`, map[string]string{
		"User":  `{"Name":"Ada","Open":true}`,
		"Items": `[{"Name":"first"},{"Name":"second"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected expression to be true")
	}
}

func TestEvalScalarEvaluatesBuiltins(t *testing.T) {
	got, err := EvalScalar(`string(len(Items) + int("2")) + ":" + string(float("1.5"))`, map[string]string{
		"Items": `[{"Name":"first"},{"Name":"second"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "4:1.5" {
		t.Fatalf("expected 4:1.5, got %q", got)
	}
}

func TestEvalBoolEvaluatesStringFilterBuiltins(t *testing.T) {
	got, err := EvalBool(`contains(lower(Name), lower(Query))`, map[string]string{
		"Name":  "First result",
		"Query": "fir",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("expected filter expression to match")
	}
}
