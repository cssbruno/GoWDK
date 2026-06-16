package main

import (
	"time"

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
			{Name: "GOWDK_AUTH_SESSION_SECRET", Required: true, MinBytes: 32},
			{Name: "GOWDK_CSRF_SECRET", Required: true, MinBytes: 32},
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
		authaddon.Addon(authaddon.Options{
			SecretEnv:  "GOWDK_AUTH_SESSION_SECRET",
			CookieName: "gowdk_auth_guard_session",
			TTL:        12 * time.Hour,
			Insecure:   true,
		}),
		ssr.Addon(),
	},
}
