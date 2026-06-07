package contracts

import (
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonEnablesContractsFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "contracts" {
		t.Fatalf("addon.Name() = %q, want contracts", addon.Name())
	}
	config := gowdk.Config{Addons: []gowdk.Addon{addon}}
	if !config.HasFeature(gowdk.FeatureContracts) {
		t.Fatal("expected contracts feature")
	}
}
