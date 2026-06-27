package corsorigin

import "testing"

func TestParseCanonicalizesOrigins(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "lowercase host", raw: "https://APP.EXAMPLE", want: "https://app.example"},
		{name: "default https port", raw: "https://app.example:443", want: "https://app.example"},
		{name: "default http port", raw: "http://app.example:80", want: "http://app.example"},
		{name: "non default port", raw: "https://app.example:8443", want: "https://app.example:8443"},
		{name: "ipv6", raw: "https://[2001:db8::1]:8443", want: "https://[2001:db8::1]:8443"},
		{name: "trailing dot", raw: "https://app.example.", want: "https://app.example"},
		{name: "punycode", raw: "https://xn--bcher-kva.example", want: "https://xn--bcher-kva.example"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			origin, err := Parse(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := origin.String(); got != test.want {
				t.Fatalf("origin = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseRejectsInvalidOrigins(t *testing.T) {
	for _, raw := range []string{
		"https://app.example:bad",
		"https://app.example:",
		"https://[2001:db8::1",
		"https://2001:db8::1",
		"https://bücher.example",
		"https://app..example",
		"https://-app.example",
		"ftp://app.example",
		"https://app.example/path",
		"https://user@app.example",
		"https://app.example?x=1",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Parse(raw); err == nil {
				t.Fatal("expected invalid origin")
			}
		})
	}
}
