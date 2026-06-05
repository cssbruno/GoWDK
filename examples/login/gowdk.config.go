package main

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	AppName: "GOWDK Login",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "dist/site",
		Targets: []gowdk.BuildTargetConfig{
			{
				Name:   "app",
				Output: "dist/site",
				App:    ".gowdk/app",
				Binary: "bin/login",
			},
			{
				Name:          "split",
				Output:        "dist/site",
				App:           ".gowdk/frontend",
				Binary:        "bin/login-frontend",
				BackendApp:    ".gowdk/backend",
				BackendBinary: "bin/login-backend",
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
