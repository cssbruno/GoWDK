package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeFixture struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestDecodeJSONDecodesStrictBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/patients", strings.NewReader(`{"name":"Ada","age":41}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	input, err := DecodeJSON[decodeFixture](request)
	if err != nil {
		t.Fatal(err)
	}
	if input.Name != "Ada" || input.Age != 41 {
		t.Fatalf("unexpected decoded input: %#v", input)
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/patients", strings.NewReader(`{"name":"Ada","extra":true}`))
	request.Header.Set("Content-Type", "application/json")

	_, err := DecodeJSON[decodeFixture](request)
	if err == nil || !strings.Contains(err.Error(), `unknown field "extra"`) {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestDecodeJSONRejectsUnsupportedContentType(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/patients", strings.NewReader(`{"name":"Ada"}`))
	request.Header.Set("Content-Type", "text/plain")

	_, err := DecodeJSON[decodeFixture](request)
	if !errors.Is(err, ErrUnsupportedContentType) {
		t.Fatalf("expected unsupported content type error, got %v", err)
	}
}

func TestDecodeJSONRejectsTrailingValues(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/patients", strings.NewReader(`{"name":"Ada"} {"name":"Grace"}`))
	request.Header.Set("Content-Type", "application/json")

	_, err := DecodeJSON[decodeFixture](request)
	if !errors.Is(err, ErrMultipleJSONValues) {
		t.Fatalf("expected multiple JSON values error, got %v", err)
	}
}

func TestDecodeJSONRejectsNilBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/patients", nil)
	request.Body = nil

	_, err := DecodeJSON[decodeFixture](request)
	if err == nil || !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("expected EOF error, got %v", err)
	}
}

func TestQueryHelpers(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/search?q=ada&tag=go&tag=web&page=2&active=true&id=42", nil)

	query, ok := QueryString(request, "q")
	if !ok || query != "ada" {
		t.Fatalf("unexpected q value: %q %v", query, ok)
	}
	tags := QueryStrings(request, "tag")
	if len(tags) != 2 || tags[0] != "go" || tags[1] != "web" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	page, ok, err := QueryInt(request, "page")
	if err != nil || !ok || page != 2 {
		t.Fatalf("unexpected page: %d %v %v", page, ok, err)
	}
	active, ok, err := QueryBool(request, "active")
	if err != nil || !ok || !active {
		t.Fatalf("unexpected active: %v %v %v", active, ok, err)
	}
	id, ok, err := QueryInt64(request, "id")
	if err != nil || !ok || id != 42 {
		t.Fatalf("unexpected id: %d %v %v", id, ok, err)
	}
}

func TestQueryIntReportsParseError(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/search?page=soon", nil)

	_, ok, err := QueryInt(request, "page")
	if !ok || err == nil || !strings.Contains(err.Error(), `parse query "page" as int`) {
		t.Fatalf("expected page parse error, got ok=%v err=%v", ok, err)
	}
}
