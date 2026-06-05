//go:build ignore

package gowdkconfig

import "github.com/cssbruno/gowdk"

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{
			"examples/actions/**/*.gwdk",
			"examples/api/**/*.gwdk",
			"examples/components/**/*.gwdk",
			"examples/embed/**/*.gwdk",
			"examples/go-interop/**/*.gwdk",
			"examples/pages/**/*.gwdk",
			"examples/partials/**/*.gwdk",
			"examples/ssr/**/*.gwdk",
		},
	},
	Build: gowdk.BuildConfig{
		Output: "gowdk_cache",
	},
	CSS: gowdk.CSSConfig{
		Include: []string{
			"examples/css/**/*.css",
			"examples/login/styles/**/*.css",
			"examples/tailwind/**/*.css",
		},
	},
}
