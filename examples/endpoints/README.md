# Endpoint Examples Cookbook

This cookbook groups broader native endpoint examples in one generated app.
`.gwdk` files are the ownership boundary for endpoint declarations, forms, and
page or fragment markup. `src/endpoints/handlers.go` owns request data,
validation, redirects, JSON, and response decisions in normal Go. The
`src/endpoints/fragments/*.html` files exercise the current low-level custom
response-body compatibility path; do not use them as the preferred markup
ownership model for new application code.

Run from this directory:

```sh
make check
make routes
make build
GOWDK_CSRF_SECRET=development-endpoints-csrf-secret-32b GOWDK_ADDR=127.0.0.1:8093 bin/endpoints
```

## Pages

| Page | Demonstrates |
| --- | --- |
| `contact.page.gwdk` | Contact action redirect plus inline validation fragment response. |
| `settings.page.gwdk` | Settings save/reset actions choosing partial-update responses. |
| `upload.page.gwdk` | Multipart action form with generated file policy and `form.Data` handler input. |
| `api.page.gwdk` | Typed generated API handlers for session, search, and JSON CRUD plus a raw request webhook escape hatch. |
| `fragments.page.gwdk` | Inline validation, table row, list refresh, modal body, and dashboard card fragments. |

## Endpoint Inventory

- Actions: `Contact`, `ValidateContact`, `SaveSettings`, `ResetSettings`,
  `UploadAvatar`, `RefreshInventory`, `UpdateInventoryRow`, `OpenModal`, and
  `RefreshDashboardCard`.
- APIs: `Session`, `Search`, `ListItems`, `CreateItem`, `UpdateItem`,
  `DeleteItem`, and `DeployWebhook`.
- Standalone fragments: `InlineValidation`, `InventoryRow`, `InventoryList`,
  `ModalBody`, and `DashboardCard`.

## Current Limits

- Generated typed API handlers own query and JSON-body decoding for supported
  scalar struct fields. Handlers still own domain validation, authentication,
  authorization, persistence, and any raw request/header handling.
- Action generated validation covers direct literal form constraints; domain
  validation remains in `src/endpoints/handlers.go`.
- Multipart action uploads enforce generated request/file limits, but storage,
  scanning, and persistence remain user-owned Go behavior.
- This example keeps embedded `html/template` fragment bodies to cover the
  low-level response compatibility API. Generated typed binding from Go data
  into source-owned `.gwdk` fragment markup is still planned.
