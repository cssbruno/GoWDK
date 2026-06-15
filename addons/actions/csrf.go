package actions

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

const (
	defaultCSRFCookie         = "__Host-gowdk-csrf"
	defaultInsecureCSRFCookie = "gowdk-csrf"
	defaultCSRFField          = "_gowdk_csrf"
	defaultCSRFHeader         = "X-GOWDK-CSRF"
	csrfNonceBytes            = 32
	csrfMACBytes              = sha256.Size
)

// CSRFValidator validates action requests before generated handlers run.
type CSRFValidator interface {
	Validate(*http.Request) error
}

// CSRFTokenSource generates tokens for generated forms.
type CSRFTokenSource interface {
	Token(http.ResponseWriter, *http.Request) (string, error)
	FieldName() string
}

// CSRFOptions configures signed double-submit CSRF tokens.
type CSRFOptions struct {
	Secret     []byte
	CookieName string
	FieldName  string
	HeaderName string
	Insecure   bool
	SameSite   http.SameSite
	// Binding, when set, ties each token to a per-request identity (typically
	// the authenticated principal). The returned value is mixed into the token
	// signature, so a token minted for one principal is rejected once the
	// request resolves to a different principal. This upgrades the plain signed
	// double-submit cookie to a session-bound token, the OWASP-recommended
	// hardening. Returning nil binds the token to the anonymous context, which
	// still yields a valid signed double-submit token (backwards compatible).
	Binding func(*http.Request) []byte
}

// CSRF validates signed double-submit CSRF tokens for generated actions.
type CSRF struct {
	secret     []byte
	cookieName string
	fieldName  string
	headerName string
	secure     bool
	sameSite   http.SameSite
	binding    func(*http.Request) []byte
}

// NewCSRF creates a validator with secure cookie defaults.
func NewCSRF(options CSRFOptions) (*CSRF, error) {
	if len(options.Secret) < 32 {
		return nil, fmt.Errorf("csrf secret must be at least 32 bytes")
	}
	cookieName := options.CookieName
	if cookieName == "" {
		cookieName = defaultCSRFCookie
		if options.Insecure {
			cookieName = defaultInsecureCSRFCookie
		}
	}
	if options.Insecure && secureCookiePrefix(cookieName) {
		return nil, fmt.Errorf("csrf cookie name %q requires Secure and cannot be used with insecure CSRF mode", cookieName)
	}
	fieldName := options.FieldName
	if fieldName == "" {
		fieldName = defaultCSRFField
	}
	headerName := options.HeaderName
	if headerName == "" {
		headerName = defaultCSRFHeader
	}
	sameSite := options.SameSite
	if sameSite == 0 {
		sameSite = http.SameSiteLaxMode
	}
	return &CSRF{
		secret:     append([]byte(nil), options.Secret...),
		cookieName: cookieName,
		fieldName:  fieldName,
		headerName: headerName,
		secure:     !options.Insecure,
		sameSite:   sameSite,
		binding:    options.Binding,
	}, nil
}

// bindingFor returns the per-request binding value, or nil when no binding is
// configured. A token is signed and validated against this value so it is only
// accepted for the identity it was minted for.
func (csrf *CSRF) bindingFor(request *http.Request) []byte {
	if csrf.binding == nil || request == nil {
		return nil
	}
	return csrf.binding(request)
}

func secureCookiePrefix(name string) bool {
	return strings.HasPrefix(name, "__Host-") || strings.HasPrefix(name, "__Secure-")
}

// Token returns the CSRF token for a generated hidden form field. It reuses
// the request's valid CSRF cookie when present so concurrently open tabs keep
// working, and only mints and stores a new token when the cookie is absent or
// invalid.
func (csrf *CSRF) Token(response http.ResponseWriter, request *http.Request) (string, error) {
	binding := csrf.bindingFor(request)
	if request != nil {
		if cookie, err := request.Cookie(csrf.cookieName); err == nil && csrf.valid(cookie.Value, binding) {
			return cookie.Value, nil
		}
	}
	var nonce [csrfNonceBytes]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	token := csrf.sign(nonce[:], binding)
	http.SetCookie(response, &http.Cookie{
		Name:     csrf.cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   csrf.secure,
		SameSite: csrf.sameSite,
	})
	return token, nil
}

// CookieName returns the cookie name used for CSRF token storage.
func (csrf *CSRF) CookieName() string {
	return csrf.cookieName
}

// FieldName returns the form field name used for submitted CSRF tokens.
func (csrf *CSRF) FieldName() string {
	return csrf.fieldName
}

// HeaderName returns the header name used for submitted CSRF tokens.
func (csrf *CSRF) HeaderName() string {
	return csrf.headerName
}

// Validate checks the submitted token against the CSRF cookie and signature.
func (csrf *CSRF) Validate(request *http.Request) error {
	cookie, err := request.Cookie(csrf.cookieName)
	if err != nil {
		return fmt.Errorf("csrf cookie is missing")
	}
	submitted := request.Header.Get(csrf.headerName)
	if submitted == "" {
		if err := request.ParseForm(); err != nil {
			return fmt.Errorf("parse csrf form: %w", err)
		}
		submitted = request.PostForm.Get(csrf.fieldName)
	}
	if submitted == "" {
		return fmt.Errorf("csrf token is missing")
	}
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(submitted)) != 1 {
		return fmt.Errorf("csrf token mismatch")
	}
	if !csrf.valid(submitted, csrf.bindingFor(request)) {
		return fmt.Errorf("csrf token signature is invalid")
	}
	return nil
}

// sign returns base64(nonce || HMAC(secret, nonce || binding)). The nonce is a
// fixed length, so writing it before the binding is an unambiguous encoding of
// the (nonce, binding) pair.
func (csrf *CSRF) sign(nonce, binding []byte) string {
	mac := hmac.New(sha256.New, csrf.secret)
	mac.Write(nonce)
	mac.Write(binding)
	raw := append(append([]byte(nil), nonce...), mac.Sum(nil)...)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (csrf *CSRF) valid(token string, binding []byte) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != csrfNonceBytes+csrfMACBytes {
		return false
	}
	nonce := raw[:csrfNonceBytes]
	signature := raw[csrfNonceBytes:]
	mac := hmac.New(sha256.New, csrf.secret)
	mac.Write(nonce)
	mac.Write(binding)
	expected := mac.Sum(nil)
	return subtle.ConstantTimeCompare(signature, expected) == 1
}
