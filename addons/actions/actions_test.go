package actions

import (
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonRegistersActionsFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "actions" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureActions) {
		t.Fatal("expected actions feature")
	}
}
