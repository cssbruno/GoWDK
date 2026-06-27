package flagshipexample

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
		Output: "dist",
		Targets: []gowdk.BuildTargetConfig{
			{
				Name:   "flagship",
				Output: "dist",
				App:    ".gowdk/app",
				Binary: "bin/flagship",
			},
		},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/*.css"},
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
