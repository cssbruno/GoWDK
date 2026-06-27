package endpoints

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	gowdkapi "github.com/cssbruno/gowdk/runtime/api"
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

type UploadInput struct {
	Avatar  form.File `form:"avatar"`
	Caption string    `form:"caption"`
}

func UploadAvatar(_ context.Context, input UploadInput) (response.Response, error) {
	uploaded, err := input.Avatar.Open()
	if err != nil {
		return response.FragmentFor("#upload-result", alertUploadHTML("Upload could not be opened.")), nil
	}
	defer func() {
		_ = uploaded.Close()
	}()
	bytes, err := io.Copy(io.Discard, uploaded)
	if err != nil {
		return response.FragmentFor("#upload-result", alertUploadHTML("Upload could not be read.")), nil
	}
	caption := strings.TrimSpace(input.Caption)
	if caption == "" {
		caption = "uncaptioned"
	}
	body := fmt.Sprintf(`<section id="upload-result"><p>Received %s (%s, %d bytes streamed) with caption %s.</p></section>`,
		escape(input.Avatar.Filename),
		escape(input.Avatar.ContentType),
		bytes,
		escape(caption),
	)
	return response.FragmentFor("#upload-result", body), nil
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

type SessionResult struct {
	Authenticated bool   `json:"authenticated"`
	User          string `json:"user"`
	IssuedAt      string `json:"issuedAt"`
}

func Session(context.Context) (SessionResult, error) {
	return SessionResult{
		Authenticated: true,
		User:          "demo@example.com",
		IssuedAt:      time.Now().UTC().Format(time.RFC3339),
	}, nil
}

type SearchInput struct {
	Query string `json:"q"`
}

type SearchResult struct {
	Query  string `json:"query"`
	Count  int    `json:"count"`
	First  string `json:"first"`
	Second string `json:"second"`
}

func Search(_ context.Context, input SearchInput) (SearchResult, error) {
	query := strings.TrimSpace(input.Query)
	if len(query) < 2 {
		return SearchResult{}, response.NewHandlerError(http.StatusBadRequest, "q must contain at least two characters", nil)
	}
	return SearchResult{Query: query, Count: 2, First: "GOWDK", Second: "runtime"}, nil
}

type ListItemsResult struct {
	FirstID    int    `json:"firstId"`
	FirstName  string `json:"firstName"`
	SecondID   int    `json:"secondId"`
	SecondName string `json:"secondName"`
}

func ListItems(context.Context) (ListItemsResult, error) {
	return ListItemsResult{FirstID: 1, FirstName: "Keyboard", SecondID: 2, SecondName: "Mouse"}, nil
}

type CreateItemInput struct {
	Name string `json:"name"`
}

type CreateItemResult struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (CreateItemResult) APIStatus() int {
	return http.StatusCreated
}

func CreateItem(_ context.Context, input CreateItemInput) (CreateItemResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return CreateItemResult{}, response.ValidationFailed("name is required", nil)
	}
	return CreateItemResult{ID: 3, Name: name}, nil
}

type UpdateItemInput struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type UpdateItemResult struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func UpdateItem(_ context.Context, input UpdateItemInput) (UpdateItemResult, error) {
	if input.ID <= 0 {
		return UpdateItemResult{}, response.NewHandlerError(http.StatusBadRequest, "id must be a positive integer", nil)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return UpdateItemResult{}, response.ValidationFailed("name is required", nil)
	}
	return UpdateItemResult{ID: input.ID, Name: name}, nil
}

type DeleteItemInput struct {
	ID int `json:"id"`
}

type DeleteItemResult struct {
	Deleted int `json:"deleted"`
}

func DeleteItem(_ context.Context, input DeleteItemInput) (DeleteItemResult, error) {
	if input.ID <= 0 {
		return DeleteItemResult{}, response.NewHandlerError(http.StatusBadRequest, "id must be a positive integer", nil)
	}
	return DeleteItemResult{Deleted: input.ID}, nil
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

func contactInvalid(values form.Values) bool {
	return strings.TrimSpace(values.First("email")) == "" || len(strings.TrimSpace(values.First("message"))) < 12
}

func alertHTML(message string) string {
	return `<section id="contact-result" role="status"><p>` + escape(message) + `</p></section>`
}

func alertUploadHTML(message string) string {
	return `<section id="upload-result" role="status"><p>` + escape(message) + `</p></section>`
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
