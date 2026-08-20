package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cssbruno/gowdk/runtime/i18n"
	"github.com/cssbruno/gowdk/runtime/validation"
)

func TestCodedHandlerErrorLocalizesThroughExistingCatalog(t *testing.T) {
	bundle := i18n.NewErrorBundleStrings("en", map[string]map[string]string{
		"pt-BR": {"patient_missing": "Paciente {id} não encontrado"},
	})
	err := NewExpectedCode(ErrorNotFound, "patient_missing", "Patient {id} was not found", map[string]string{"id": "42"}, errors.New("db miss"))
	recorder := httptest.NewRecorder()
	WriteNoStoreLocalizedHandlerJSONError(recorder, err, http.StatusInternalServerError, bundle, "pt-BR")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.OK || payload.Error.Code != "patient_missing" || payload.Error.Message != "Paciente 42 não encontrado" {
		t.Fatalf("unexpected localized payload: %#v", payload)
	}
}

func TestValidationErrorsExposeCodesAndLocalize(t *testing.T) {
	result := validation.Result{}
	result.AddCode("email", "validation_required", "{field} is required", map[string]string{"field": "Email"})
	bundle := i18n.NewErrorBundleStrings("en", map[string]map[string]string{
		"pt": {"validation_required": "{field} é obrigatório"},
	})
	localized := LocalizeValidation(result, bundle, "pt")
	if got := localized.Errors[0]; got.Code != "validation_required" || got.Message != "Email é obrigatório" {
		t.Fatalf("unexpected localized validation error: %#v", got)
	}
	response, err := ValidationJSON(localized)
	if err != nil {
		t.Fatal(err)
	}
	if response.Body == "" || !json.Valid([]byte(response.Body)) {
		t.Fatalf("expected valid coded validation JSON: %s", response.Body)
	}
}
