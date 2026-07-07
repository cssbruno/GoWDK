package endpoints

import (
	"context"
	"embed"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	gowdkapi "github.com/cssbruno/gowdk/runtime/api"
	"github.com/cssbruno/gowdk/runtime/form"
	"github.com/cssbruno/gowdk/runtime/response"
)

//go:embed fragments/*.html
var fragmentFiles embed.FS

var fragmentTemplates = template.Must(template.ParseFS(fragmentFiles, "fragments/*.html"))

func Contact(_ context.Context, values form.Values) (response.Response, error) {
	if contactInvalid(values) {
		return response.RedirectTo("/endpoints/contact?invalid=1"), nil
	}
	return response.RedirectTo("/endpoints/contact?sent=1"), nil
}

func ValidateContact(_ context.Context, values form.Values) (response.Response, error) {
	if contactInvalid(values) {
		return response.FragmentFor("#contact-result", renderFragment("contact-result.html", map[string]string{"Message": "Email and a 12 character message are required."})), nil
	}
	return response.FragmentFor("#contact-result", renderFragment("contact-result.html", map[string]string{"Message": "Contact request is ready to submit."})), nil
}

func SaveSettings(_ context.Context, values form.Values) (response.Response, error) {
	theme := valueOr(values.First("theme"), "system")
	email := valueOr(values.First("email"), "off")
	return response.FragmentFor("#settings-result", renderFragment("settings-result.html", map[string]string{"Theme": theme, "Email": email})), nil
}

func ResetSettings(context.Context, form.Values) (response.Response, error) {
	return response.FragmentFor("#settings-result", renderFragment("settings-reset.html", nil)), nil
}

func UploadAvatar(_ context.Context, input form.Data) (response.Response, error) {
	files := input.Files["avatar"]
	if len(files) == 0 {
		return response.FragmentFor("#upload-result", renderFragment("upload-alert.html", map[string]string{"Message": "Upload is required."})), nil
	}
	avatar := files[0]
	uploaded, err := avatar.Open()
	if err != nil {
		return response.FragmentFor("#upload-result", renderFragment("upload-alert.html", map[string]string{"Message": "Upload could not be opened."})), nil
	}
	defer func() {
		_ = uploaded.Close()
	}()
	bytes, err := io.Copy(io.Discard, uploaded)
	if err != nil {
		return response.FragmentFor("#upload-result", renderFragment("upload-alert.html", map[string]string{"Message": "Upload could not be read."})), nil
	}
	caption := strings.TrimSpace(input.Values.First("caption"))
	if caption == "" {
		caption = "uncaptioned"
	}
	return response.FragmentFor("#upload-result", renderFragment("upload-result.html", uploadResult{
		Filename:    avatar.Filename,
		ContentType: avatar.ContentType,
		Bytes:       bytes,
		Caption:     caption,
	})), nil
}

func RefreshInventory(context.Context, form.Values) (response.Response, error) {
	return response.FragmentSwap("#inventory", response.SwapOuterHTML, renderFragment("inventory-list.html", nil))
}

func UpdateInventoryRow(_ context.Context, values form.Values) (response.Response, error) {
	item := valueOr(values.First("item"), "Keyboard")
	return response.FragmentFor("#inventory", renderFragment("inventory-row.html", map[string]string{"Item": item})), nil
}

func OpenModal(context.Context, form.Values) (response.Response, error) {
	return response.FragmentFor("#modal-body", renderFragment("modal-body.html", nil)), nil
}

func RefreshDashboardCard(context.Context, form.Values) (response.Response, error) {
	return response.FragmentSwap("#dashboard-card", response.SwapOuterHTML, renderFragment("dashboard-card.html", nil))
}

func InlineValidation(context.Context) (response.Response, error) {
	return response.FragmentFor("#inline-validation", renderFragment("inline-validation.html", nil)), nil
}

func InventoryRow(context.Context) (response.Response, error) {
	return response.FragmentFor("#inventory", renderFragment("runtime-inventory-row.html", nil)), nil
}

func InventoryList(context.Context) (response.Response, error) {
	return response.FragmentSwap("#inventory", response.SwapOuterHTML, renderFragment("runtime-inventory-list.html", nil))
}

func ModalBody(context.Context) (response.Response, error) {
	return response.FragmentFor("#modal-body", renderFragment("runtime-modal-body.html", nil)), nil
}

func DashboardCard(context.Context) (response.Response, error) {
	return response.FragmentSwap("#dashboard-card", response.SwapOuterHTML, renderFragment("runtime-dashboard-card.html", nil))
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

func valueOr(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

type uploadResult struct {
	Filename    string
	ContentType string
	Bytes       int64
	Caption     string
}

func renderFragment(name string, data any) string {
	var out strings.Builder
	if err := fragmentTemplates.ExecuteTemplate(&out, name, data); err != nil {
		panic(err)
	}
	return out.String()
}
