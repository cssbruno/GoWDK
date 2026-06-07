// Package contracts declares the contract-driven runtime compiler capability.
package contracts

import "github.com/cssbruno/gowdk"

// ImportPath is the canonical Go import path for the contracts addon.
const ImportPath = "github.com/cssbruno/gowdk/addons/contracts"

// Addon enables contract-driven runtime metadata and generated adapters once
// the compiler integration lands.
func Addon() gowdk.Addon {
	return gowdk.NewAddon("contracts", gowdk.FeatureContracts)
}
