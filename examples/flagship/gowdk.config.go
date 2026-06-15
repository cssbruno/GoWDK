package main

import (
	"github.com/cssbruno/gowdk"
	contractsaddon "github.com/cssbruno/gowdk/addons/contracts"
	"github.com/cssbruno/gowdk/addons/partial"
	"github.com/cssbruno/gowdk/addons/ratelimit"
	"github.com/cssbruno/gowdk/addons/ssr"
)

var Config = gowdk.Config{
	AppName: "GOWDK Flagship",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "examples/flagship/dist",
		Targets: []gowdk.BuildTargetConfig{
			{
				Name:   "flagship",
				Output: "examples/flagship/dist",
				App:    "examples/flagship/.gowdk/app",
				Binary: "examples/flagship/bin/flagship",
			},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"examples/flagship/styles/**/*.css"},
		Output: gowdk.CSSOutputConfig{
			Dir:        "assets/gowdk",
			HrefPrefix: "/assets/gowdk",
		},
	},
	Addons: []gowdk.Addon{
		ssr.Addon(),
		partial.Addon(),
		contractsaddon.Addon(),
		ratelimit.Addon(),
	},
}
