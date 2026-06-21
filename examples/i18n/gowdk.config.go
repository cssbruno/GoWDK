package i18nexample

import "github.com/cssbruno/gowdk"

const (
	localeEnglish    = "en"
	localePortuguese = "pt"
)

var Config = gowdk.Config{
	AppName: "gowdk-i18n-example",
	Source: gowdk.SourceConfig{
		Include: []string{"examples/i18n/*.gwdk"},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"examples/i18n/*.css"},
	},
	I18N: gowdk.I18NConfig{
		DefaultLocale: localeEnglish,
		Locales: []gowdk.LocaleConfig{
			{Code: localeEnglish, Name: "English"},
			{Code: localePortuguese, Name: "Portuguese"},
		},
	},
}
