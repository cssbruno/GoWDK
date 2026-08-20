package flagshipexample

import (
	"github.com/cssbruno/gowdk"
	contractsaddon "github.com/cssbruno/gowdk/addons/contracts"
	"github.com/cssbruno/gowdk/addons/partial"
	"github.com/cssbruno/gowdk/addons/ratelimit"
	"github.com/cssbruno/gowdk/addons/ssr"
	flagship "github.com/cssbruno/gowdk/examples/flagship/src/app"
)

var Config = gowdk.Config{
	AppName: "GOWDK Flagship",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Interop: gowdk.InteropConfig{
		Loads:  []gowdk.LoadRegistration{gowdk.RegisterLoad("dashboard", flagship.LoadDashboard)},
		Guards: gowdk.RegisterGuards(flagship.Guards),
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
