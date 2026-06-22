package i18nexample

import (
	"github.com/cssbruno/gowdk"
	gowdki18n "github.com/cssbruno/gowdk/runtime/i18n"
)

type messageKey string

const (
	messageTitle messageKey = "title"
	messageIntro messageKey = "intro"
)

type HomeCopy struct {
	Title string `json:"title"`
	Intro string `json:"intro"`
}

var homeMessages = gowdki18n.NewBundle("en", map[string]gowdki18n.Catalog[messageKey]{
	"en": gowdki18n.NewCatalog("en", map[messageKey]string{
		messageTitle: "GOWDK ships apps",
		messageIntro: "This page was generated for the {locale} locale.",
	}),
	"pt": gowdki18n.NewCatalog("pt", map[messageKey]string{
		messageTitle: "GOWDK entrega apps",
		messageIntro: "Esta pagina foi gerada para o locale {locale}.",
	}),
})

func HomeCopyForBuild(params gowdk.BuildParams) HomeCopy {
	locale := params.LocaleCode()
	if locale == "" {
		locale = homeMessages.DefaultLocale
	}
	catalog, _ := homeMessages.Catalog(locale)
	return HomeCopy{
		Title: catalog.Must(messageTitle),
		Intro: catalog.MustFormat(messageIntro, map[string]string{
			"locale": locale,
		}),
	}
}
