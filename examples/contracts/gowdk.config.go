package main

import (
	"github.com/cssbruno/gowdk"
	contractsaddon "github.com/cssbruno/gowdk/addons/contracts"
)

var Config = gowdk.Config{
	Source: gowdk.SourceConfig{
		Include: []string{"examples/contracts/*.gwdk"},
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"examples/contracts/**/*.css"},
	},
	Addons: []gowdk.Addon{
		contractsaddon.Addon(),
	},
}
