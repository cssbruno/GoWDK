package clientlang

import (
	"errors"
	"strings"
	"testing"
)

func TestParseClientFunctions(t *testing.T) {
	program, err := Parse(`
fn Add(step int, label string) {
  Count = step
  Label = label;
  Open = !Open;
}

fn Reset() {
  Count = 0
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Functions) != 2 {
		t.Fatalf("expected two functions, got %#v", program.Functions)
	}
	if program.Functions[0].Name != "Add" || len(program.Functions[0].Params) != 2 ||
		program.Functions[0].Params[0].Name != "step" || program.Functions[0].Params[0].Type != "int" ||
		strings.Join(program.Functions[0].Statements, ",") != "Count = step,Label = label,Open = !Open" {
		t.Fatalf("unexpected Add function: %#v", program.Functions[0])
	}
	if program.Functions[0].Span.StartLine != 2 || program.Functions[0].Span.EndLine != 6 ||
		len(program.Functions[0].StatementSpans) != 3 ||
		program.Functions[0].StatementSpans[0].StartLine != 3 ||
		program.Functions[0].StatementSpans[2].StartLine != 5 {
		t.Fatalf("unexpected Add function spans: %#v", program.Functions[0])
	}
	handlers := program.HandlerMap()
	if len(handlers["Add"].Params) != 2 || handlers["Add"].Params[0] != "step" ||
		len(handlers["Reset"].Statements) != 1 || handlers["Reset"].Statements[0] != "Count = 0" {
		t.Fatalf("unexpected handlers: %#v", handlers)
	}
}

func TestParseClearStatement(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		valid bool
	}{
		{"clear cart", "cart", true},
		{"clear cart;", "cart", true},
		{"clear  prefs", "prefs", true},
		{"clear shop.cart", "shop.cart", true},
		{"clear", "", false},
		{"clear ", "", false},
		{"clearcart", "", false},
		{"clear cart now", "", false},
		{"Count = clear", "", false},
	}
	for _, tc := range cases {
		got, ok := ParseClearStatement(tc.in)
		if ok != tc.valid {
			t.Fatalf("ParseClearStatement(%q) ok = %v, want %v", tc.in, ok, tc.valid)
		}
		if ok && got != tc.want {
			t.Fatalf("ParseClearStatement(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseUseStoreTypeAnnotation(t *testing.T) {
	program, err := Parse(`
use cart ui.CartState
use prefs
use shop.wishlist ui.Wishlist
`)
	if err != nil {
		t.Fatal(err)
	}
	uses := program.UseMap()
	if got := uses["cart"]; got.Type != "ui.CartState" || got.StoreName != "cart" || got.PackageAlias != "" {
		t.Fatalf("unexpected cart use: %#v", got)
	}
	if got := uses["prefs"]; got.Type != "" {
		t.Fatalf("untyped use should have empty type: %#v", got)
	}
	if got := uses["shop.wishlist"]; got.Type != "ui.Wishlist" || got.PackageAlias != "shop" || got.StoreName != "wishlist" {
		t.Fatalf("unexpected qualified use: %#v", got)
	}
}

func TestParseRejectsClearAsFunctionName(t *testing.T) {
	_, err := Parse(`
fn clear() {
  Count = 0
}
`)
	if err == nil || !strings.Contains(err.Error(), "reserved built-in name") {
		t.Fatalf("expected reserved name error, got %v", err)
	}
}

func TestParseDuplicateClientSymbolReportsLine(t *testing.T) {
	_, err := Parse(`
fn Save() {
  Count = 1
}

fn Save() {
  Count = 2
}
`)
	if err == nil {
		t.Fatal("expected duplicate function error")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ParseError, got %T: %v", err, err)
	}
	if parseErr.Line != 6 {
		t.Fatalf("expected duplicate declaration on line 6, got %d", parseErr.Line)
	}
}

func TestParseClientFunctionsAllowsFuncAlias(t *testing.T) {
	program, err := Parse(`
func Increment() {
  Count++
}
`)
	if err != nil {
		t.Fatal(err)
	}
	handlers := program.HandlerMap()
	if len(handlers["Increment"].Statements) != 1 || handlers["Increment"].Statements[0] != "Count++" {
		t.Fatalf("unexpected handlers: %#v", handlers)
	}
}

func TestParseAsyncClientFunction(t *testing.T) {
	program, err := Parse(`
async fn Search() {
  Loading = true
  Items = await fetchJSON[[]ui.Item]("/api/items")
  Loading = false
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Functions) != 1 || !program.Functions[0].Async || program.Functions[0].Name != "Search" {
		t.Fatalf("unexpected async function: %#v", program.Functions)
	}
	handlers := program.HandlerMap()
	if !handlers["Search"].Async || len(handlers["Search"].Statements) != 3 {
		t.Fatalf("unexpected async handler: %#v", handlers["Search"])
	}
}

func TestParseRejectsAsyncHelperReturn(t *testing.T) {
	_, err := Parse(`
async fn Search() int {
  return 1
}
`)
	if err == nil {
		t.Fatal("expected async return type error")
	}
	if !strings.Contains(err.Error(), "cannot declare a return type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseHelperFunctionReturns(t *testing.T) {
	program, err := Parse(`
fn Next(value int) int {
  return value + 1
}

fn Add() {
  Count = Next(Count)
}
`)
	if err != nil {
		t.Fatal(err)
	}
	handlers := program.HandlerMap()
	if _, ok := handlers["Next"]; ok {
		t.Fatalf("helper should not be an event handler: %#v", handlers)
	}
	helpers := program.HelperMap()
	if helpers["Next"].Return != "value + 1" || helpers["Next"].ReturnType != TypeInt || len(helpers["Next"].Params) != 1 {
		t.Fatalf("unexpected helper map: %#v", helpers)
	}
	if !program.NeedsBootstrap() {
		t.Fatal("expected helper program to need bootstrap")
	}
}

func TestParseHelperFunctionAllowsLocalStatementsBeforeReturn(t *testing.T) {
	program, err := Parse(`
fn Next(value int) int {
  let doubled int = value * 2
  let adjusted int = switch doubled { case 0: 1 default: doubled + 1 }
  return adjusted
}
`)
	if err != nil {
		t.Fatal(err)
	}
	helpers := program.HelperMap()
	helper := helpers["Next"]
	if helper.Return != "adjusted" || helper.ReturnType != TypeInt {
		t.Fatalf("unexpected helper return: %#v", helper)
	}
	if len(helper.Locals) != 2 ||
		helper.Locals[0].Name != "doubled" || helper.Locals[0].Expr != "value * 2" || helper.Locals[0].Type != TypeInt ||
		helper.Locals[1].Name != "adjusted" || helper.Locals[1].Expr != `switch doubled { case 0: 1 default: doubled + 1 }` || helper.Locals[1].Type != TypeInt {
		t.Fatalf("unexpected helper locals: %#v", helper.Locals)
	}
}

func TestParseRejectsHelperNonLetBeforeReturn(t *testing.T) {
	_, err := Parse(`
fn Next(value int) int {
  value = value + 1
  return value
}
`)
	if err == nil {
		t.Fatal("expected helper local syntax error")
	}
	if !strings.Contains(err.Error(), "can only declare `let name type = expr`") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsReturnInEventHandler(t *testing.T) {
	_, err := Parse(`
fn Add() {
  return Count + 1
}
`)
	if err == nil {
		t.Fatal("expected return without type error")
	}
	if !strings.Contains(err.Error(), "cannot return a value without declaring a return type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseLifecycleAndEffects(t *testing.T) {
	program, err := Parse(`
on mount {
  Focused = true
}

effect when Query {
  Dirty = true
  return {
    Dirty = false
  }
}

on destroy {
  Open = false
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(program.Mount, ",") != "Focused = true" {
		t.Fatalf("unexpected mount statements: %#v", program.Mount)
	}
	if len(program.Effects) != 1 || program.Effects[0].Field != "Query" || strings.Join(program.Effects[0].Statements, ",") != "Dirty = true" || strings.Join(program.Effects[0].Cleanup, ",") != "Dirty = false" {
		t.Fatalf("unexpected effects: %#v", program.Effects)
	}
	if len(program.MountSpans) != 1 || program.MountSpans[0].StartLine != 3 ||
		len(program.Effects[0].StatementSpans) != 1 || program.Effects[0].StatementSpans[0].StartLine != 7 ||
		len(program.Effects[0].CleanupSpans) != 1 || program.Effects[0].CleanupSpans[0].StartLine != 9 ||
		len(program.DestroySpans) != 1 || program.DestroySpans[0].StartLine != 14 {
		t.Fatalf("unexpected lifecycle spans: mount=%#v effects=%#v destroy=%#v", program.MountSpans, program.Effects, program.DestroySpans)
	}
	if strings.Join(program.Destroy, ",") != "Open = false" {
		t.Fatalf("unexpected destroy statements: %#v", program.Destroy)
	}
	if !program.HasLifecycle() {
		t.Fatal("expected lifecycle program")
	}
}

func TestParseComputedValues(t *testing.T) {
	program, err := Parse(`
computed Label string {
  return if Open { "open" } else { "closed" }
}

computed Next int {
  return Count + 1
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Computed) != 2 {
		t.Fatalf("expected two computed values, got %#v", program.Computed)
	}
	if program.Computed[0].Name != "Label" || program.Computed[0].Type != "string" ||
		program.Computed[0].Expr != `if Open { "open" } else { "closed" }` {
		t.Fatalf("unexpected Label computed: %#v", program.Computed[0])
	}
	if program.Computed[0].Span.StartLine != 2 || program.Computed[0].Span.EndLine != 4 ||
		program.Computed[0].ExprSpan.StartLine != 3 {
		t.Fatalf("unexpected Label computed spans: %#v", program.Computed[0])
	}
	if program.Computed[1].Name != "Next" || program.Computed[1].Type != "int" || program.Computed[1].Expr != "Count + 1" {
		t.Fatalf("unexpected Next computed: %#v", program.Computed[1])
	}
	if !program.NeedsBootstrap() {
		t.Fatal("expected computed program to need bootstrap")
	}
}

func TestParseComputedAllowsGoStyleIfReturn(t *testing.T) {
	program, err := Parse(`
computed Label string {
  if Count == 0 {
    return "Start"
  }
  return string(Count)
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Computed) != 1 {
		t.Fatalf("expected one computed value, got %#v", program.Computed)
	}
	computed := program.Computed[0]
	if computed.Name != "Label" || computed.Type != "string" ||
		computed.Expr != `if Count == 0 { "Start" } else { string(Count) }` {
		t.Fatalf("unexpected computed: %#v", computed)
	}
	if computed.ExprSpan.StartLine != 3 {
		t.Fatalf("unexpected computed span: %#v", computed.ExprSpan)
	}
}

func TestOrderComputedDependencyGraph(t *testing.T) {
	ordered, err := OrderComputed([]Computed{
		{Name: "Visible", Type: "bool", Expr: `Label == "open"`},
		{Name: "Label", Type: "string", Expr: `if Open { "open" } else { "closed" }`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 2 || ordered[0].Name != "Label" || ordered[1].Name != "Visible" {
		t.Fatalf("unexpected computed order: %#v", ordered)
	}
}

func TestOrderComputedRejectsCycle(t *testing.T) {
	_, err := OrderComputed([]Computed{
		{Name: "A", Type: "string", Expr: "B"},
		{Name: "B", Type: "string", Expr: "A"},
	})
	if err == nil {
		t.Fatal("expected computed cycle error")
	}
	if !strings.Contains(err.Error(), "computed dependency cycle") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsComputedWithoutReturn(t *testing.T) {
	_, err := Parse(`
computed Label string {
  Label = "open"
}
`)
	if err == nil {
		t.Fatal("expected computed return error")
	}
	if !strings.Contains(err.Error(), "must use `return expr`") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRefs(t *testing.T) {
	program, err := Parse(`
ref searchInput HTMLInputElement

fn FocusSearch() {
  searchInput.Focus()
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Refs) != 1 || program.Refs[0].Name != "searchInput" || program.Refs[0].Kind != "HTMLInputElement" {
		t.Fatalf("unexpected refs: %#v", program.Refs)
	}
	refs := program.RefMap()
	if refs["searchInput"].Kind != "HTMLInputElement" {
		t.Fatalf("unexpected ref map: %#v", refs)
	}
}

func TestParseStoreUses(t *testing.T) {
	program, err := Parse(`
use cart

fn Add() {
  Count++
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Uses) != 1 || program.Uses[0].Name != "cart" || program.Uses[0].Span.StartLine != 2 {
		t.Fatalf("unexpected uses: %#v", program.Uses)
	}
	uses := program.UseMap()
	if uses["cart"].Name != "cart" {
		t.Fatalf("unexpected use map: %#v", uses)
	}
	if !program.NeedsBootstrap() {
		t.Fatal("expected store use to require bootstrap envelope")
	}
	if got := program.StoreNames(); len(got) != 1 || got[0] != "cart" {
		t.Fatalf("unexpected store names: %#v", got)
	}
}

func TestParseQualifiedStoreUses(t *testing.T) {
	program, err := Parse(`
use cart.current

fn Add() {
  Count++
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Uses) != 1 {
		t.Fatalf("expected one qualified use, got %#v", program.Uses)
	}
	use := program.Uses[0]
	if use.Name != "cart.current" || use.PackageAlias != "cart" || use.StoreName != "current" {
		t.Fatalf("unexpected qualified store use: %#v", use)
	}
	if got := program.StoreNames(); len(got) != 1 || got[0] != "cart.current" {
		t.Fatalf("unexpected store names: %#v", got)
	}
}

func TestParseRejectsDuplicateRef(t *testing.T) {
	_, err := Parse(`
ref searchInput HTMLInputElement
ref searchInput HTMLInputElement
`)
	if err == nil {
		t.Fatal("expected duplicate ref error")
	}
	if !strings.Contains(err.Error(), `client ref "searchInput" is declared more than once`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsDuplicateStoreUse(t *testing.T) {
	_, err := Parse(`
use cart
use cart
`)
	if err == nil {
		t.Fatal("expected duplicate use error")
	}
	if !strings.Contains(err.Error(), `client store "cart" is used more than once`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsDuplicateClientFunction(t *testing.T) {
	_, err := Parse(`
fn Toggle() {
  Open = !Open
}

fn Toggle() {
  Open = false
}
`)
	if err == nil {
		t.Fatal("expected duplicate function error")
	}
	if !strings.Contains(err.Error(), `client function "Toggle" is declared more than once`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseRejectsUnsupportedClientSyntax(t *testing.T) {
	_, err := Parse(`
let Count = 0
`)
	if err == nil {
		t.Fatal("expected unsupported syntax error")
	}
	if !strings.Contains(err.Error(), `unsupported syntax`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseErrorReportsLine(t *testing.T) {
	_, err := Parse(`fn Bad() {
  if Count {
  }
}`)
	if err == nil {
		t.Fatal("expected parse error")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ParseError, got %T: %v", err, err)
	}
	if parseErr.Line != 2 {
		t.Fatalf("unexpected parse error line: got %d, want 2", parseErr.Line)
	}
}

func TestParseRejectsUnsupportedParamType(t *testing.T) {
	_, err := Parse(`
fn Add(step uint) {
  Count = step
}
`)
	if err == nil {
		t.Fatal("expected unsupported param type error")
	}
	if !strings.Contains(err.Error(), `unsupported parameter type "uint"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIsFunctionCall(t *testing.T) {
	name, ok := IsFunctionCall("Increment()")
	if !ok || name != "Increment" {
		t.Fatalf("expected Increment call, got %q %v", name, ok)
	}
	if _, ok := IsFunctionCall("Count++"); ok {
		t.Fatal("did not expect Count++ to be a function call")
	}
}

func TestParseCallWithArgs(t *testing.T) {
	call, ok := ParseCall(`Add(1, Count, "saved, ok")`)
	if !ok {
		t.Fatal("expected call")
	}
	if call.Name != "Add" || strings.Join(call.Args, "|") != `1|Count|"saved, ok"` {
		t.Fatalf("unexpected call: %#v", call)
	}
}

func TestParseCallWithObjectLiteralArg(t *testing.T) {
	call, ok := ParseCall(`append(Items, { ID: "third", Name: "third", Done: false })`)
	if !ok {
		t.Fatal("expected call")
	}
	if call.Name != "append" || len(call.Args) != 2 {
		t.Fatalf("unexpected call: %#v", call)
	}
	if call.Args[0] != "Items" || call.Args[1] != `{ ID: "third", Name: "third", Done: false }` {
		t.Fatalf("unexpected args: %#v", call.Args)
	}
}

func TestParseRejectsReservedFunctionNames(t *testing.T) {
	_, err := Parse(`
fn append() {
  Count++
}`)
	if err == nil {
		t.Fatal("expected reserved function name error")
	}
	if !strings.Contains(err.Error(), "reserved built-in name") {
		t.Fatalf("unexpected error: %v", err)
	}
}
