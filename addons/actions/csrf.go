package actions

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
)

const (
	defaultCSRFCookie = "__Host-gowdk-csrf"
	defaultCSRFField  = "_gowdk_csrf"
	defaultCSRFHeader = "X-GOWDK-CSRF"
	csrfNonceBytes    = 32
	csrfMACBytes      = sha256.Size
)

// CSRFValidator validates action requests before generated handlers run.
type CSRFValidator interface {
	Validate(*http.Request) error
}

// CSRFOptions configures signed double-submit CSRF tokens.
type CSRFOptions struct {
	Secret     []byte
	CookieName string
	FieldName  string
	HeaderName string
	Insecure   bool
	SameSite   http.SameSite
}

// CSRF validates signed double-submit CSRF tokens for generated actions.
type CSRF struct {
	secret     []byte
	cookieName string
	fieldName  string
	headerName string
	secure     bool
	sameSite   http.SameSite
}

// NewCSRF creates a validator with secure cookie defaults.
func NewCSRF(options CSRFOptions) (*CSRF, error) {
	if len(options.Secret) < 32 {
		return nil, fmt.Errorf("csrf secret must be at least 32 bytes")
	}
	cookieName := options.CookieName
	if cookieName == "" {
		cookieName = defaultCSRFCookie
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
	}, nil
}

// Token generates a CSRF token, stores it in a cookie, and returns the value for
// a generated hidden form field.
func (csrf *CSRF) Token(response http.ResponseWriter) (string, error) {
	var nonce [csrfNonceBytes]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}
	token := csrf.sign(nonce[:])
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
	if !csrf.valid(submitted) {
		return fmt.Errorf("csrf token signature is invalid")
	}
	return nil
}

func (csrf *CSRF) sign(nonce []byte) string {
	mac := hmac.New(sha256.New, csrf.secret)
	mac.Write(nonce)
	raw := append(append([]byte(nil), nonce...), mac.Sum(nil)...)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (csrf *CSRF) valid(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != csrfNonceBytes+csrfMACBytes {
		return false
	}
	nonce := raw[:csrfNonceBytes]
	signature := raw[csrfNonceBytes:]
	mac := hmac.New(sha256.New, csrf.secret)
	mac.Write(nonce)
	expected := mac.Sum(nil)
	return subtle.ConstantTimeCompare(signature, expected) == 1
}

// NoopCSRF is for tests only.
type NoopCSRF struct{}

// Validate accepts every request.
func (NoopCSRF) Validate(*http.Request) error {
	return nil
}
