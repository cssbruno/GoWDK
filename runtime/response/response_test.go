package response

import (
	"errors"
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
		`<div data-gowdk-validation>`,
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
}
