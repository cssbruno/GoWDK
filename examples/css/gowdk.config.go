package main

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Build: gowdk.BuildConfig{
		Stylesheets: []gowdk.Stylesheet{
			{Href: "/assets/site.css"},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"examples/css/assets/*.css"},
	},
}
