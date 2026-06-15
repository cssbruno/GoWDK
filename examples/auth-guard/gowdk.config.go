package main

import (
	"github.com/cssbruno/gowdk"
	authaddon "github.com/cssbruno/gowdk/addons/auth"
	"github.com/cssbruno/gowdk/addons/ssr"
)

var Config = gowdk.Config{
	AppName: "GOWDK Auth Guard",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Env: gowdk.EnvConfig{
		Secrets: []gowdk.SecretEnv{
			{Name: "GOWDK_AUTH_SESSION_SECRET", Required: true},
			{Name: "GOWDK_CSRF_SECRET", Required: true},
		},
	},
	Build: gowdk.BuildConfig{
		Output: "examples/auth-guard/dist",
		CSRF: gowdk.CSRFConfig{
			Insecure: true,
		},
		Targets: []gowdk.BuildTargetConfig{
			{
				Name:   "auth-guard",
				Output: "examples/auth-guard/dist",
				App:    "examples/auth-guard/.gowdk/app",
				Binary: "examples/auth-guard/bin/auth-guard",
			},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"examples/auth-guard/styles/**/*.css"},
		Output: gowdk.CSSOutputConfig{
			Dir:        "assets/gowdk",
			HrefPrefix: "/assets/gowdk",
		},
	},
	Addons: []gowdk.Addon{
		authaddon.Addon(),
		ssr.Addon(),
	},
}
