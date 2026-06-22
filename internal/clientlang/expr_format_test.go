package clientlang

import (
	"math"
	"testing"
)

func TestFormatFixed(t *testing.T) {
	cases := []struct {
		value  float64
		digits int
		want   string
	}{
		{3.14159, 2, "3.14"},
		{3.14159, 0, "3"},
		{2.5, 0, "3"},
		{-2.5, 0, "-3"},
		{1, 2, "1.00"},
		{-0.001, 2, "0.00"},
		{1234.5, 2, "1234.50"},
		{0.005, 2, "0.01"},
	}
	for _, testCase := range cases {
		got, err := formatFixed(testCase.value, testCase.digits)
		if err != nil {
			t.Fatalf("formatFixed(%v, %d): %v", testCase.value, testCase.digits, err)
		}
		if got != testCase.want {
			t.Fatalf("formatFixed(%v, %d) = %q, want %q", testCase.value, testCase.digits, got, testCase.want)
		}
	}
	if _, err := formatFixed(1, 21); err == nil {
		t.Fatal("expected error for out-of-range digits")
	}
	if _, err := formatFixed(1e21, 0); err == nil {
		t.Fatal("expected error for value beyond the safe-integer range")
	}
}

func TestRoundCollapsesNegativeZero(t *testing.T) {
	got, err := roundTo(-0.001, 2)
	if err != nil {
		t.Fatal(err)
	}
	if math.Signbit(got) {
		t.Fatal("roundTo(-0.001, 2) returned negative zero; want positive zero")
	}
}

func TestRoundTo(t *testing.T) {
	cases := []struct {
		value  float64
		digits int
		want   float64
	}{
		{3.14159, 2, 3.14},
		{2.5, 0, 3},
		{-2.5, 0, -3},
		{1.005, 2, 1}, // 1.005*100 == 100.49999... in IEEE-754, rounds down
	}
	for _, testCase := range cases {
		got, err := roundTo(testCase.value, testCase.digits)
		if err != nil {
			t.Fatalf("roundTo(%v, %d): %v", testCase.value, testCase.digits, err)
		}
		if got != testCase.want {
			t.Fatalf("roundTo(%v, %d) = %v, want %v", testCase.value, testCase.digits, got, testCase.want)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	got, err := formatPercent(0.1234, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != "12.3%" {
		t.Fatalf("formatPercent(0.1234, 1) = %q, want %q", got, "12.3%")
	}
}

func TestFormatUnixTime(t *testing.T) {
	cases := []struct {
		unix   float64
		layout string
		want   string
	}{
		{0, "YYYY-MM-DD HH:mm:ss", "1970-01-01 00:00:00"},
		{1700000000, "YYYY-MM-DD HH:mm:ss", "2023-11-14 22:13:20"},
		{1700000000, "DD/MM/YYYY", "14/11/2023"},
		{-86401, "YYYY-MM-DD HH:mm:ss", "1969-12-30 23:59:59"},
	}
	for _, testCase := range cases {
		got, err := formatUnixTime(testCase.unix, testCase.layout)
		if err != nil {
			t.Fatalf("formatUnixTime(%v, %q): %v", testCase.unix, testCase.layout, err)
		}
		if got != testCase.want {
			t.Fatalf("formatUnixTime(%v, %q) = %q, want %q", testCase.unix, testCase.layout, got, testCase.want)
		}
	}
	if _, err := formatUnixTime(1.5, "YYYY"); err == nil {
		t.Fatal("expected error for fractional timestamp")
	}
	if _, err := formatUnixTime(1e16, "YYYY"); err == nil {
		t.Fatal("expected error for out-of-range timestamp")
	}
}

func TestCheckFormattingBuiltins(t *testing.T) {
	symbols := map[string]ValueType{"Price": TypeFloat, "Ts": TypeInt, "Name": TypeString}
	typeCases := []struct {
		expr string
		want ValueType
	}{
		{`fixed(Price, 2)`, TypeString},
		{`percent(Price, 1)`, TypeString},
		{`round(Price, 2)`, TypeFloat},
		{`formatTime(Ts, "YYYY")`, TypeString},
	}
	for _, testCase := range typeCases {
		got, _, err := CheckExpr(testCase.expr, symbols)
		if err != nil {
			t.Fatalf("CheckExpr(%q): %v", testCase.expr, err)
		}
		if got != testCase.want {
			t.Fatalf("CheckExpr(%q) = %s, want %s", testCase.expr, got, testCase.want)
		}
	}
	errorCases := []string{
		`fixed(Price)`,
		`fixed(Name, 2)`,
		`round(Price, Name)`,
		`formatTime(Ts, 5)`,
		`formatTime(Name, "YYYY")`,
	}
	for _, expr := range errorCases {
		if _, _, err := CheckExpr(expr, symbols); err == nil {
			t.Fatalf("CheckExpr(%q) expected an error", expr)
		}
	}
}

func TestParseIslandRefCallSupportsSelect(t *testing.T) {
	for _, method := range []string{"Focus", "Blur", "ScrollIntoView", "Select"} {
		name, ok := parseIslandRefCall("Box." + method + "()")
		if !ok || name != "Box" {
			t.Fatalf("parseIslandRefCall(Box.%s()) = (%q, %v), want (\"Box\", true)", method, name, ok)
		}
	}
	if _, ok := parseIslandRefCall("Box.Reset()"); ok {
		t.Fatal("parseIslandRefCall should reject unknown ref methods")
	}
}
