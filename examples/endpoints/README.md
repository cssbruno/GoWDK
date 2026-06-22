# Endpoint Examples Cookbook

This cookbook groups broader native endpoint examples in one generated app. The
`.gwdk` files declare endpoint metadata and forms; `src/endpoints/handlers.go`
owns request behavior, validation, redirects, JSON, and fragments in normal Go.

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
| `settings.page.gwdk` | Settings save/reset actions returning partial fragments. |
| `upload.page.gwdk` | Multipart action form with generated file policy and typed `form.File` handler input. |
| `api.page.gwdk` | Session, search, JSON CRUD, and webhook API declarations backed by Go handlers. |
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

- API handlers own JSON body validation and authentication policy. Generated
  API declarations do not create typed request structs yet.
- Action generated validation covers direct literal form constraints; domain
  validation remains in `src/endpoints/handlers.go`.
- Multipart action uploads enforce generated request/file limits, but storage,
  scanning, and persistence remain user-owned Go behavior.
- Fragment hooks and static fragment bodies are HTML strings today; richer
  typed fragment render helpers are still planned.
