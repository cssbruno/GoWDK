package endpointsexample

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/partial"
)

var Config = gowdk.Config{
	AppName: "GOWDK Endpoint Cookbook",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "examples/endpoints/dist",
		Targets: []gowdk.BuildTargetConfig{
			{
				Name:   "endpoints",
				Output: "examples/endpoints/dist",
				App:    "examples/endpoints/.gowdk/app",
				Binary: "examples/endpoints/bin/endpoints",
			},
		},
	},
	Addons: []gowdk.Addon{
		partial.Addon(),
	},
}
