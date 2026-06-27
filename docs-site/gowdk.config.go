package site

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/tailwind"
)

var Config = gowdk.Config{
	AppName: "GOWDK Page",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "dist/site",
		Head: gowdk.HeadConfig{
			SiteName:    "GOWDK",
			Favicon:     "/favicon.ico",
			Image:       "https://gowdk.com/assets/wdk_logo.png",
			TwitterCard: "summary",
		},
	},
	Addons: []gowdk.Addon{
		tailwind.Addon(tailwind.Options{
			Input:   "app.css",
			Command: "tools/tailwindcss",
			Minify:  true,
		}),
	},
}
