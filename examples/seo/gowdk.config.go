package seoexample

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/seo"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		seo.Addon(seo.Options{
			BaseURL:  "https://example.com",
			Disallow: []string{"/drafts"},
			ExtraURLs: []seo.URL{
				{Loc: "/rss.xml"},
			},
		}),
	},
}
