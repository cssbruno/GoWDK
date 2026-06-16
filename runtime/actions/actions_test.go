package actions

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

func TestDecodeFormParsesPostFormValues(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader("email=a%40example.com&tag=go&tag=web"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	values, err := DecodeForm(request)
	if err != nil {
		t.Fatal(err)
	}
	if got := values.First("email"); got != "a@example.com" {
		t.Fatalf("unexpected email: %q", got)
	}
	if got := values.All("tag"); len(got) != 2 || got[1] != "web" {
		t.Fatalf("unexpected repeated tags: %#v", got)
	}
}

func TestRegistryRegisterStoresActionHandler(t *testing.T) {
	registry := Registry{}
	registry.Register("submit", func(context.Context, form.Values) (response.Response, error) {
		return response.RedirectTo("/done"), nil
	})

	result, err := registry["submit"](context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != response.Redirect || result.URL != "/done" {
		t.Fatalf("unexpected response: %#v", result)
	}
}

func TestValidateRequiredUsesRuntimeValidationResult(t *testing.T) {
	result := ValidateRequired(form.Values{
		"email": []string{"  "},
		"name":  []string{"Ada"},
	}, []string{"email", "name", "topic"})

	if result.OK() {
		t.Fatal("expected validation errors")
	}
	if got := result.ByField(); len(got["email"]) != 1 || got["email"][0] != "required" || len(got["topic"]) != 1 {
		t.Fatalf("unexpected validation errors: %#v", got)
	}
}

func TestCSRFGeneratesSecureCookieAndValidatesFormToken(t *testing.T) {
	csrf, err := NewCSRF(CSRFOptions{Secret: []byte(strings.Repeat("s", 32))})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	token, err := csrf.Token(response, nil)
	if err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected csrf cookie, got %#v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != defaultCSRFCookie || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected csrf cookie defaults: %#v", cookie)
	}
	if csrf.CookieName() != defaultCSRFCookie || csrf.FieldName() != defaultCSRFField || csrf.HeaderName() != defaultCSRFHeader {
		t.Fatalf("unexpected csrf names: cookie=%q field=%q header=%q", csrf.CookieName(), csrf.FieldName(), csrf.HeaderName())
	}

	form := url.Values{defaultCSRFField: []string{token}}
	request := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)

	if err := csrf.Validate(request); err != nil {
		t.Fatalf("expected csrf validation to pass: %v", err)
	}
}

func TestCSRFInsecureDefaultUsesBrowserAcceptedCookieName(t *testing.T) {
	csrf, err := NewCSRF(CSRFOptions{
		Secret:   []byte(strings.Repeat("s", 32)),
		Insecure: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	token, err := csrf.Token(response, nil)
	if err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected csrf cookie, got %#v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != defaultInsecureCSRFCookie || cookie.Secure {
		t.Fatalf("unexpected insecure csrf cookie: %#v", cookie)
	}
	if csrf.CookieName() != defaultInsecureCSRFCookie {
		t.Fatalf("unexpected insecure csrf cookie name: %q", csrf.CookieName())
	}

	form := url.Values{defaultCSRFField: []string{token}}
	request := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(cookie)

	if err := csrf.Validate(request); err != nil {
		t.Fatalf("expected csrf validation to pass: %v", err)
	}
}

func TestCSRFValidatesHeaderToken(t *testing.T) {
	csrf, err := NewCSRF(CSRFOptions{Secret: []byte(strings.Repeat("s", 32))})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	token, err := csrf.Token(response, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/signup", nil)
	request.Header.Set(defaultCSRFHeader, token)
	request.AddCookie(response.Result().Cookies()[0])

	if err := csrf.Validate(request); err != nil {
		t.Fatalf("expected csrf header validation to pass: %v", err)
	}
}

func TestCSRFRejectsMissingMismatchAndInvalidTokens(t *testing.T) {
	csrf, err := NewCSRF(CSRFOptions{Secret: []byte(strings.Repeat("s", 32))})
	if err != nil {
		t.Fatal(err)
	}
	if err := csrf.Validate(httptest.NewRequest(http.MethodPost, "/signup", nil)); err == nil || !strings.Contains(err.Error(), "csrf cookie is missing") {
		t.Fatalf("expected missing cookie error, got %v", err)
	}

	response := httptest.NewRecorder()
	token, err := csrf.Token(response, nil)
	if err != nil {
		t.Fatal(err)
	}
	cookie := response.Result().Cookies()[0]

	mismatch := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(url.Values{defaultCSRFField: []string{"other"}}.Encode()))
	mismatch.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatch.AddCookie(cookie)
	if err := csrf.Validate(mismatch); err == nil || !strings.Contains(err.Error(), "csrf token mismatch") {
		t.Fatalf("expected mismatch error, got %v", err)
	}

	invalidCookie := *cookie
	invalidCookie.Value = tamperToken(token)
	invalid := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(url.Values{defaultCSRFField: []string{invalidCookie.Value}}.Encode()))
	invalid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invalid.AddCookie(&invalidCookie)
	if err := csrf.Validate(invalid); err == nil || !strings.Contains(err.Error(), "csrf token signature is invalid") {
		t.Fatalf("expected invalid signature error, got %v", err)
	}
}

func TestCSRFBindsTokenToPrincipal(t *testing.T) {
	binding := func(r *http.Request) []byte { return []byte(r.Header.Get("X-Principal")) }
	csrf, err := NewCSRF(CSRFOptions{Secret: []byte(strings.Repeat("s", 32)), Binding: binding})
	if err != nil {
		t.Fatal(err)
	}

	mint := httptest.NewRequest(http.MethodGet, "/", nil)
	mint.Header.Set("X-Principal", "alice")
	response := httptest.NewRecorder()
	token, err := csrf.Token(response, mint)
	if err != nil {
		t.Fatal(err)
	}
	cookie := response.Result().Cookies()[0]

	same := httptest.NewRequest(http.MethodPost, "/signup", nil)
	same.Header.Set("X-Principal", "alice")
	same.Header.Set(defaultCSRFHeader, token)
	same.AddCookie(cookie)
	if err := csrf.Validate(same); err != nil {
		t.Fatalf("expected token to validate for the same principal: %v", err)
	}

	// A different principal must be rejected even though cookie == submitted.
	other := httptest.NewRequest(http.MethodPost, "/signup", nil)
	other.Header.Set("X-Principal", "mallory")
	other.Header.Set(defaultCSRFHeader, token)
	other.AddCookie(cookie)
	if err := csrf.Validate(other); err == nil || !strings.Contains(err.Error(), "csrf token signature is invalid") {
		t.Fatalf("expected token bound to alice to be rejected for mallory, got %v", err)
	}
}

func tamperToken(token string) string {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) == 0 {
		return token + "x"
	}
	raw[len(raw)-1] ^= 0xff
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestNewCSRFRejectsShortSecret(t *testing.T) {
	_, err := NewCSRF(CSRFOptions{Secret: []byte("short")})
	if err == nil {
		t.Fatal("expected short secret error")
	}
}

func TestNewCSRFRejectsSecureCookiePrefixInInsecureMode(t *testing.T) {
	for _, name := range []string{"__Host-gowdk-csrf", "__Secure-gowdk-csrf"} {
		_, err := NewCSRF(CSRFOptions{
			Secret:     []byte(strings.Repeat("s", 32)),
			CookieName: name,
			Insecure:   true,
		})
		if err == nil {
			t.Fatalf("expected insecure secure-prefix cookie error for %q", name)
		}
		if !strings.Contains(err.Error(), "requires Secure") {
			t.Fatalf("unexpected error for %q: %v", name, err)
		}
	}
}

func TestNoopCSRFAllowsRequests(t *testing.T) {
	if err := (NoopCSRF{}).Validate(httptest.NewRequest(http.MethodPost, "/", nil)); err != nil {
		t.Fatalf("expected noop csrf to allow request: %v", err)
	}
}
