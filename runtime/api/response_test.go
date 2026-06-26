package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/runtime/response"
)

func TestJSONCreatesRuntimeResponse(t *testing.T) {
	result, err := JSON(http.StatusCreated, map[string]string{"id": "pat_123"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != response.JSON || result.Status != http.StatusCreated || result.Body != `{"id":"pat_123"}` {
		t.Fatalf("unexpected JSON response: %#v", result)
	}
}

type statusResult struct {
	status int
}

func (result statusResult) APIStatus() int {
	return result.status
}

func TestResultStatusUsesTypedStatusContract(t *testing.T) {
	if got := ResultStatus(statusResult{status: http.StatusAccepted}, http.StatusOK); got != http.StatusAccepted {
		t.Fatalf("ResultStatus = %d, want %d", got, http.StatusAccepted)
	}
	if got := ResultStatus(statusResult{}, http.StatusCreated); got != http.StatusCreated {
		t.Fatalf("ResultStatus fallback = %d, want %d", got, http.StatusCreated)
	}
	if got := ResultStatus(struct{}{}, 0); got != http.StatusOK {
		t.Fatalf("ResultStatus default fallback = %d, want %d", got, http.StatusOK)
	}
}

func TestErrorCreatesStructuredJSONError(t *testing.T) {
	result, err := Error(http.StatusBadRequest, "invalid_request", "Invalid request")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != response.JSON || result.Status != http.StatusBadRequest {
		t.Fatalf("unexpected error response: %#v", result)
	}
	for _, expected := range []string{`"ok":false`, `"code":"invalid_request"`, `"message":"Invalid request"`} {
		if !strings.Contains(result.Body, expected) {
			t.Fatalf("expected %q in error body: %s", expected, result.Body)
		}
	}
}

func TestErrorDefaultsCodeAndMessage(t *testing.T) {
	result, err := Error(http.StatusNotFound, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"code":"api_error"`, `"message":"Not Found"`} {
		if !strings.Contains(result.Body, expected) {
			t.Fatalf("expected %q in error body: %s", expected, result.Body)
		}
	}
}

func TestNoContentCreatesEmptyResponse(t *testing.T) {
	result := NoContent()
	if result.Kind != response.JSON || result.Status != http.StatusNoContent || result.Body != "" {
		t.Fatalf("unexpected no-content response: %#v", result)
	}
}
