package loginexample

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	AppName: "GOWDK Login",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Targets: []gowdk.BuildTargetConfig{
			{
				Name:   "login",
				App:    ".gowdk/login",
				Binary: "bin/login",
			},
			{
				Name:          "split",
				App:           ".gowdk/login-frontend",
				Binary:        "bin/login-frontend",
				BackendApp:    ".gowdk/login-backend",
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
