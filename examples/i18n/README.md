# i18n Example

This example shows the first localization slice:

- `gowdk.config.go` declares `Config.I18N` locales.
- `messages.go` keeps typed message keys and catalogs in normal Go.
- `messages_test.go` checks the typed message references against each locale
  catalog, which is the current CI-friendly extraction/completeness path.
- `home.page.gwdk` calls a Go build helper that reads
  `gowdk.BuildParams.LocaleCode()`.

Build it from the repository root:

```sh
go run ./cmd/gowdk build --config examples/i18n/gowdk.config.go --out /tmp/gowdk-i18n-build examples/i18n/*.gwdk
test -f /tmp/gowdk-i18n-build/en/index.html
test -f /tmp/gowdk-i18n-build/pt/index.html
grep -F '<html lang="en">' /tmp/gowdk-i18n-build/en/index.html
grep -F '<html lang="pt">' /tmp/gowdk-i18n-build/pt/index.html
go test ./examples/i18n
```
