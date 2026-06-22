package response

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/runtime/validation"
)

func TestHTMLBody(t *testing.T) {
	result := HTMLBody(201, "<h1>Created</h1>")
	if result.Kind != HTML || result.Status != 201 || result.Body != "<h1>Created</h1>" {
		t.Fatalf("unexpected html response: %#v", result)
	}
}

func TestRedirectTo(t *testing.T) {
	result := RedirectTo("/done")
	if result.Kind != Redirect || result.Status != 303 || result.URL != "/done" {
		t.Fatalf("unexpected redirect response: %#v", result)
	}
}

func TestFragmentFor(t *testing.T) {
	result := FragmentFor("#target", "<p>Updated</p>")
	if result.Kind != Fragment || result.Status != 200 || result.Target != "#target" || result.Swap != SwapInnerHTML || result.Body != "<p>Updated</p>" {
		t.Fatalf("unexpected fragment response: %#v", result)
	}
}

func TestFragmentSwap(t *testing.T) {
	result, err := FragmentSwap("#target", SwapOuterHTML, "<section>Updated</section>")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != Fragment || result.Swap != SwapOuterHTML || result.Body != "<section>Updated</section>" {
		t.Fatalf("unexpected fragment swap response: %#v", result)
	}

	_, err = FragmentSwap("#target", "append", "<p>Updated</p>")
	if err == nil {
		t.Fatal("expected unsupported swap mode error")
	}
	if !strings.Contains(err.Error(), `unsupported fragment swap mode "append"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJSONBody(t *testing.T) {
	result := JSONBody(202, `{"ok":true}`)
	if result.Kind != JSON || result.Status != 202 || result.Body != `{"ok":true}` {
		t.Fatalf("unexpected json response: %#v", result)
	}
}

func TestJSONValue(t *testing.T) {
	result, err := JSONValue(200, map[string]string{"message": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != JSON || result.Body != `{"message":"ok"}` {
		t.Fatalf("unexpected json value response: %#v", result)
	}

	_, err = JSONValue(200, make(chan string))
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestValidationJSON(t *testing.T) {
	var validationResult validation.Result
	validationResult.Add("email", "is required")

	result, err := ValidationJSON(validationResult)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != JSON || result.Status != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected validation json response: %#v", result)
	}
	for _, expected := range []string{`"ok":false`, `"Field":"email"`, `"Message":"is required"`} {
		if !strings.Contains(result.Body, expected) {
			t.Fatalf("expected %q in validation json body: %s", expected, result.Body)
		}
	}
}

func TestValidationFragmentEscapesMessages(t *testing.T) {
	var validationResult validation.Result
	validationResult.Add(`email"`, `<required>`)

	result := ValidationFragment("#errors", validationResult)
	if result.Kind != Fragment || result.Target != "#errors" || result.Status != http.StatusOK {
		t.Fatalf("unexpected validation fragment response: %#v", result)
	}
	for _, expected := range []string{
		`<div data-gowdk-validation role="alert" aria-live="polite">`,
		`data-gowdk-field="email&#34;"`,
		`&lt;required&gt;`,
	} {
		if !strings.Contains(result.Body, expected) {
			t.Fatalf("expected %q in validation fragment body: %s", expected, result.Body)
		}
	}
}

func TestWriteHTTPWritesHTML(t *testing.T) {
	recorder := httptest.NewRecorder()

	if err := WriteHTTP(recorder, HTMLBody(201, "<h1>Created</h1>")); err != nil {
		t.Fatal(err)
	}

	if recorder.Code != 201 {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if recorder.Body.String() != "<h1>Created</h1>" {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestWriteHTTPWritesRedirect(t *testing.T) {
	recorder := httptest.NewRecorder()

	if err := WriteHTTP(recorder, RedirectTo("/done")); err != nil {
		t.Fatal(err)
	}

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/done" {
		t.Fatalf("unexpected location: %q", location)
	}
}

func TestWriteHTTPRejectsOpenRedirect(t *testing.T) {
	for _, target := range []string{"//evil.com", "https://evil.com", "/\\evil.com", "/ok\r\nSet-Cookie: x=1", ""} {
		recorder := httptest.NewRecorder()
		err := WriteHTTP(recorder, RedirectTo(target))
		if err == nil {
			t.Fatalf("expected error for unsafe redirect %q", target)
		}
		if location := recorder.Header().Get("Location"); location != "" {
			t.Fatalf("unsafe redirect %q leaked Location header: %q", target, location)
		}
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("unsafe redirect %q: unexpected status %d", target, recorder.Code)
		}
	}
}

func TestWriteHTTPRejectsUnsafeRedirectBeforeCookies(t *testing.T) {
	recorder := httptest.NewRecorder()
	result := WithCookie(RedirectTo("//evil.com"), http.Cookie{
		Name:     "gowdk_session",
		Value:    "signed",
		Path:     "/",
		HttpOnly: true,
	})

	err := WriteHTTP(recorder, result)
	if err == nil {
		t.Fatal("expected unsafe redirect error")
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "" {
		t.Fatalf("unsafe redirect leaked Location header: %q", location)
	}
	if setCookie := recorder.Header().Get("Set-Cookie"); setCookie != "" {
		t.Fatalf("unsafe redirect leaked Set-Cookie header: %q", setCookie)
	}
}

func TestValidateLocalRedirect(t *testing.T) {
	for _, ok := range []string{"/", "/dashboard", "/a/b?c=1#d"} {
		if err := ValidateLocalRedirect(ok); err != nil {
			t.Fatalf("expected %q to be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "//evil.com", "https://evil.com", "/\\x", "a/b", "/x\ry"} {
		if err := ValidateLocalRedirect(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestWriteNoStoreHTTP(t *testing.T) {
	recorder := httptest.NewRecorder()

	if err := WriteNoStoreHTTP(recorder, FragmentFor("#target", "<p>Updated</p>")); err != nil {
		t.Fatal(err)
	}

	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestWriteNoStoreHTML(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	if err := WriteNoStoreHTML(recorder, request, "<main>SSR</main>"); err != nil {
		t.Fatal(err)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
	if recorder.Body.String() != "<main>SSR</main>" {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestWriteHTMLUsesExplicitCachePolicy(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	if err := WriteHTML(recorder, request, "<main>SSR</main>", "public, max-age=60"); err != nil {
		t.Fatal(err)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "public, max-age=60" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
	if recorder.Body.String() != "<main>SSR</main>" {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestWriteHTMLPreservesExistingNoStore(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Cache-Control", "no-store")
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	if err := WriteHTML(recorder, request, "<main>SSR</main>", "public, max-age=60"); err != nil {
		t.Fatal(err)
	}

	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestWriteHTMLDefaultsToNoStore(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	if err := WriteHTML(recorder, request, "<main>SSR</main>", " "); err != nil {
		t.Fatal(err)
	}

	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
}

func TestWriteNoStoreHTMLSuppressesHeadBody(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodHead, "/", nil)

	if err := WriteNoStoreHTML(recorder, request, "<main>SSR</main>"); err != nil {
		t.Fatal(err)
	}

	if recorder.Body.String() != "" {
		t.Fatalf("expected empty HEAD body, got %s", recorder.Body.String())
	}
}

func TestWriteNoStoreError(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteNoStoreError(recorder, http.StatusBadRequest, "invalid form")

	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("unexpected cache control: %q", cacheControl)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "invalid form") {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestWriteHTTPWritesReloadResponse(t *testing.T) {
	recorder := httptest.NewRecorder()

	if err := WriteHTTP(recorder, ReloadPage()); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if reload := recorder.Header().Get("X-GOWDK-Reload"); reload != "1" {
		t.Fatalf("reload header = %q, want 1", reload)
	}
	if body := recorder.Body.String(); body != "" {
		t.Fatalf("reload body = %q, want empty", body)
	}
}

func TestWriteHTTPWritesFragment(t *testing.T) {
	recorder := httptest.NewRecorder()

	if err := WriteHTTP(recorder, FragmentFor("#target", "<p>Updated</p>")); err != nil {
		t.Fatal(err)
	}

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if target := recorder.Header().Get("X-GOWDK-Fragment-Target"); target != "#target" {
		t.Fatalf("unexpected fragment target: %q", target)
	}
	if swap := recorder.Header().Get("X-GOWDK-Fragment-Swap"); swap != string(SwapInnerHTML) {
		t.Fatalf("unexpected fragment swap: %q", swap)
	}
	if recorder.Body.String() != "<p>Updated</p>" {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestWriteHTTPWritesFragmentSwap(t *testing.T) {
	recorder := httptest.NewRecorder()
	result, err := FragmentSwap("#target", SwapOuterHTML, "<section>Updated</section>")
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteHTTP(recorder, result); err != nil {
		t.Fatal(err)
	}

	if target := recorder.Header().Get("X-GOWDK-Fragment-Target"); target != "#target" {
		t.Fatalf("unexpected fragment target: %q", target)
	}
	if swap := recorder.Header().Get("X-GOWDK-Fragment-Swap"); swap != string(SwapOuterHTML) {
		t.Fatalf("unexpected fragment swap: %q", swap)
	}
	if recorder.Body.String() != "<section>Updated</section>" {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestWriteHTTPWritesJSON(t *testing.T) {
	recorder := httptest.NewRecorder()

	if err := WriteHTTP(recorder, JSONBody(200, `{"ok":true}`)); err != nil {
		t.Fatal(err)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestWriteHTTPSkipsBodyForNoBodyStatuses(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotModified} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			writeErr := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writeErr <- WriteHTTP(writer, JSONBody(status, `{"ignored":true}`))
			}))
			defer server.Close()

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, status)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "" {
				t.Fatalf("body = %q, want empty", string(body))
			}
			if err := <-writeErr; err != nil {
				t.Fatalf("WriteHTTP error = %v, want nil", err)
			}
		})
	}
}

func TestHandlerError(t *testing.T) {
	cause := errors.New("database unavailable")
	err := NewHandlerError(503, "handler unavailable", cause)

	if err.Error() != "handler unavailable" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause")
	}
	if got := HandlerStatus(err, 500); got != 503 {
		t.Fatalf("unexpected handler status: %d", got)
	}
	if got := HandlerStatus(errors.New("ordinary"), 500); got != 500 {
		t.Fatalf("unexpected fallback status: %d", got)
	}
	if got := HandlerStatus(&http.MaxBytesError{Limit: 1024}, 500); got != http.StatusRequestEntityTooLarge {
		t.Fatalf("unexpected max bytes status: %d", got)
	}
}

func TestHandlerErrorKeepsUnkeyedLiteralShape(t *testing.T) {
	err := HandlerError{http.StatusServiceUnavailable, "handler unavailable", errors.New("database unavailable")}

	if got := HandlerStatus(err, http.StatusInternalServerError); got != http.StatusServiceUnavailable {
		t.Fatalf("unexpected handler status: %d", got)
	}
	if got := HandlerErrorMessage(err, err.Status); got != "handler unavailable" {
		t.Fatalf("unexpected handler message: %q", got)
	}
}

func TestExpectedErrorsMapToStatuses(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		kind   ErrorKind
		status int
	}{
		{name: "not found", err: NotFound("missing patient", nil), kind: ErrorNotFound, status: http.StatusNotFound},
		{name: "forbidden", err: Forbidden("session required", nil), kind: ErrorForbidden, status: http.StatusForbidden},
		{name: "validation", err: ValidationFailed("invalid filter", nil), kind: ErrorValidation, status: http.StatusUnprocessableEntity},
		{name: "server", err: ServerError("temporarily unavailable", nil), kind: ErrorServer, status: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expectedErr expectedError
			if !errors.As(tt.err, &expectedErr) {
				t.Fatalf("expected expectedError, got %T", tt.err)
			}
			if expectedErr.kind != tt.kind {
				t.Fatalf("unexpected kind: %q", expectedErr.kind)
			}
			var handlerErr HandlerError
			if !errors.As(tt.err, &handlerErr) {
				t.Fatalf("expected HandlerError, got %T", tt.err)
			}
			if got := HandlerStatus(tt.err, http.StatusInternalServerError); got != tt.status {
				t.Fatalf("unexpected status: %d", got)
			}
			if HandlerErrorMessage(tt.err, tt.status) == "" {
				t.Fatalf("expected non-empty message")
			}
		})
	}
}

func TestExpectedErrorUsesDefaultMessage(t *testing.T) {
	err := NotFound("", nil)

	if got := HandlerErrorMessage(err, HandlerStatus(err, http.StatusInternalServerError)); got != http.StatusText(http.StatusNotFound) {
		t.Fatalf("unexpected default message: %q", got)
	}
}

func TestHandlerErrorMessageHidesOrdinary5xxDetails(t *testing.T) {
	err := errors.New("sql: password=secret dsn=postgres://root:secret@db/app")

	message := HandlerErrorMessage(err, http.StatusInternalServerError)

	if message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("expected generic 500 message, got %q", message)
	}
	if strings.Contains(message, "secret") || strings.Contains(message, "postgres") {
		t.Fatalf("internal detail leaked through message: %q", message)
	}
}

func TestHandlerErrorMessageUsesExplicitHandlerErrorMessage(t *testing.T) {
	err := NewHandlerError(http.StatusServiceUnavailable, "temporarily unavailable", errors.New("dial tcp password=secret"))

	if got := HandlerErrorMessage(err, HandlerStatus(err, http.StatusInternalServerError)); got != "temporarily unavailable" {
		t.Fatalf("unexpected handler error message: %q", got)
	}
}

func TestHandlerErrorMessageKeeps4xxDetails(t *testing.T) {
	err := NewHandlerError(http.StatusForbidden, "session expired", nil)

	if got := HandlerErrorMessage(err, HandlerStatus(err, http.StatusInternalServerError)); got != "session expired" {
		t.Fatalf("unexpected 4xx handler message: %q", got)
	}
}

func TestWriteNoStoreHandlerError(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteNoStoreHandlerError(recorder, errors.New("internal database password=secret"), http.StatusInternalServerError)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store, got %q", cache)
	}
	if body := recorder.Body.String(); !strings.Contains(body, http.StatusText(http.StatusInternalServerError)) || strings.Contains(body, "secret") {
		t.Fatalf("unexpected safe handler error body: %q", body)
	}
}

func TestWriteNoStoreHandlerJSONError(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteNoStoreHandlerJSONError(recorder, errors.New("internal database password=secret"), http.StatusInternalServerError)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("expected no-store, got %q", cache)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != `{"error":"Internal Server Error"}` {
		t.Fatalf("unexpected JSON handler error body: %q", body)
	}
}

func TestWriteNoStoreHandlerJSONErrorUsesExplicitHandlerErrorMessage(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := NewHandlerError(http.StatusConflict, "duplicate patient", errors.New("unique constraint detail"))

	WriteNoStoreHandlerJSONError(recorder, err, http.StatusInternalServerError)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != `{"error":"duplicate patient"}` {
		t.Fatalf("unexpected JSON handler error body: %q", body)
	}
}
