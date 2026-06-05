package main

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	AppName: "GOWDK Login Split Runtime",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "dist/site",
		Targets: []gowdk.BuildTargetConfig{
			{
				Name:   "frontend",
				Output: "dist/site",
				App:    ".gowdk/frontend",
				Binary: "bin/login-frontend",
			},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
		Output: gowdk.CSSOutputConfig{
			Dir:        ".",
			HrefPrefix: "/",
		},
	},
}
