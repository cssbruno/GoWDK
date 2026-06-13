package main

import (
	"github.com/cssbruno/gowdk"
	contractsaddon "github.com/cssbruno/gowdk/addons/contracts"
)

var Config = gowdk.Config{
	Addons: []gowdk.Addon{
		contractsaddon.Addon(),
	},
}
