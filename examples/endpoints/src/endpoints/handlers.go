package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gowdkapi "github.com/cssbruno/gowdk/addons/api"
	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

func Contact(_ context.Context, values form.Values) (response.Response, error) {
	if contactInvalid(values) {
		return response.RedirectTo("/endpoints/contact?invalid=1"), nil
	}
	return response.RedirectTo("/endpoints/contact?sent=1"), nil
}

func ValidateContact(_ context.Context, values form.Values) (response.Response, error) {
	if contactInvalid(values) {
		return response.FragmentFor("#contact-result", alertHTML("Email and a 12 character message are required.")), nil
	}
	return response.FragmentFor("#contact-result", alertHTML("Contact request is ready to submit.")), nil
}

func SaveSettings(_ context.Context, values form.Values) (response.Response, error) {
	theme := valueOr(values.First("theme"), "system")
	email := valueOr(values.First("email"), "off")
	body := fmt.Sprintf(`<section id="settings-result"><p>Saved theme %s with email %s.</p></section>`, escape(theme), escape(email))
	return response.FragmentFor("#settings-result", body), nil
}

func ResetSettings(context.Context, form.Values) (response.Response, error) {
	return response.FragmentFor("#settings-result", `<section id="settings-result"><p>Settings reset to defaults.</p></section>`), nil
}

func RefreshInventory(context.Context, form.Values) (response.Response, error) {
	return response.FragmentSwap("#inventory", response.SwapOuterHTML, `<tbody id="inventory"><tr><td>Keyboard</td><td>stocked</td></tr><tr><td>Mouse</td><td>stocked</td></tr></tbody>`)
}

func UpdateInventoryRow(_ context.Context, values form.Values) (response.Response, error) {
	item := valueOr(values.First("item"), "Keyboard")
	body := fmt.Sprintf(`<tr><td>%s</td><td>updated</td></tr>`, escape(item))
	return response.FragmentFor("#inventory", body), nil
}

func OpenModal(context.Context, form.Values) (response.Response, error) {
	return response.FragmentFor("#modal-body", `<section id="modal-body"><h2>Modal body</h2><p>Loaded from a partial action.</p></section>`), nil
}

func RefreshDashboardCard(context.Context, form.Values) (response.Response, error) {
	body := `<article id="dashboard-card"><h2>Dashboard card</h2><p>Refreshed by a fragment response.</p></article>`
	return response.FragmentSwap("#dashboard-card", response.SwapOuterHTML, body)
}

func InlineValidation(context.Context) (response.Response, error) {
	return response.FragmentFor("#inline-validation", `<section id="inline-validation"><p>Request-time inline validation fragment.</p></section>`), nil
}

func InventoryRow(context.Context) (response.Response, error) {
	return response.FragmentFor("#inventory", `<tr><td>Dynamic item</td><td>ready</td></tr>`), nil
}

func InventoryList(context.Context) (response.Response, error) {
	return response.FragmentSwap("#inventory", response.SwapOuterHTML, `<tbody id="inventory"><tr><td>Dynamic list</td><td>ready</td></tr></tbody>`)
}

func ModalBody(context.Context) (response.Response, error) {
	return response.FragmentFor("#modal-body", `<section id="modal-body"><p>Request-time modal body.</p></section>`), nil
}

func DashboardCard(context.Context) (response.Response, error) {
	return response.FragmentSwap("#dashboard-card", response.SwapOuterHTML, `<article id="dashboard-card"><h2>Request-time card</h2><p>Ready.</p></article>`)
}

func Session(context.Context, *http.Request) (response.Response, error) {
	return gowdkapi.JSON(http.StatusOK, map[string]any{
		"authenticated": true,
		"user":          "demo@example.com",
		"issuedAt":      time.Now().UTC().Format(time.RFC3339),
	})
}

func Search(_ context.Context, request *http.Request) (response.Response, error) {
	query := strings.TrimSpace(request.URL.Query().Get("q"))
	if len(query) < 2 {
		return gowdkapi.Error(http.StatusBadRequest, "query_required", "q must contain at least two characters")
	}
	return gowdkapi.JSON(http.StatusOK, map[string]any{"query": query, "results": []string{"GOWDK", "runtime"}})
}

func ListItems(context.Context, *http.Request) (response.Response, error) {
	return gowdkapi.JSON(http.StatusOK, []item{{ID: 1, Name: "Keyboard"}, {ID: 2, Name: "Mouse"}})
}

func CreateItem(_ context.Context, request *http.Request) (response.Response, error) {
	input, err := decodeItem(request)
	if err != nil {
		return gowdkapi.Error(http.StatusBadRequest, "invalid_json", err.Error())
	}
	if strings.TrimSpace(input.Name) == "" {
		return gowdkapi.Error(http.StatusUnprocessableEntity, "name_required", "name is required")
	}
	input.ID = 3
	return gowdkapi.JSON(http.StatusCreated, input)
}

func UpdateItem(_ context.Context, request *http.Request) (response.Response, error) {
	id, err := queryID(request)
	if err != nil {
		return gowdkapi.Error(http.StatusBadRequest, "invalid_id", err.Error())
	}
	input, err := decodeItem(request)
	if err != nil {
		return gowdkapi.Error(http.StatusBadRequest, "invalid_json", err.Error())
	}
	input.ID = id
	return gowdkapi.JSON(http.StatusOK, input)
}

func DeleteItem(_ context.Context, request *http.Request) (response.Response, error) {
	id, err := queryID(request)
	if err != nil {
		return gowdkapi.Error(http.StatusBadRequest, "invalid_id", err.Error())
	}
	return gowdkapi.JSON(http.StatusOK, map[string]any{"deleted": id})
}

func DeployWebhook(_ context.Context, request *http.Request) (response.Response, error) {
	event := strings.TrimSpace(request.Header.Get("X-GOWDK-Event"))
	if event == "" {
		return gowdkapi.Error(http.StatusBadRequest, "event_required", "X-GOWDK-Event is required")
	}
	payload, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		return gowdkapi.Error(http.StatusBadRequest, "read_failed", "could not read webhook body")
	}
	return gowdkapi.JSON(http.StatusAccepted, map[string]any{"event": event, "bytes": len(payload)})
}

type item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func contactInvalid(values form.Values) bool {
	return strings.TrimSpace(values.First("email")) == "" || len(strings.TrimSpace(values.First("message"))) < 12
}

func decodeItem(request *http.Request) (item, error) {
	defer request.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var input item
	if err := decoder.Decode(&input); err != nil {
		return item{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return item{}, fmt.Errorf("request body must contain one JSON value")
	}
	return input, nil
}

func queryID(request *http.Request) (int, error) {
	id, err := strconv.Atoi(request.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("id must be a positive integer")
	}
	return id, nil
}

func alertHTML(message string) string {
	return `<section id="contact-result" role="status"><p>` + escape(message) + `</p></section>`
}

func valueOr(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func escape(value string) string {
	return html.EscapeString(value)
}
