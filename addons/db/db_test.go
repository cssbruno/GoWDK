package db

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonEnablesDBFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "db" {
		t.Fatalf("addon.Name() = %q, want db", addon.Name())
	}
	config := gowdk.Config{Addons: []gowdk.Addon{addon}}
	if !config.HasFeature(gowdk.FeatureDB) {
		t.Fatal("expected db feature to be enabled")
	}
}

func TestOpenRequiresDriver(t *testing.T) {
	if _, err := Open("", "some-dsn"); err == nil {
		t.Fatal("expected an error for an empty driver name")
	}
}

func TestOpenRequiresDSN(t *testing.T) {
	if _, err := Open("postgres", "   "); err == nil {
		t.Fatal("expected an error for an empty DSN")
	}
}

func TestOpenUnknownDriver(t *testing.T) {
	// No driver is registered in this test binary, so sql.Open reports an
	// unknown driver. We assert the helper surfaces that as a wrapped error
	// rather than panicking or returning a usable handle.
	_, err := Open("definitely-not-registered", "dsn")
	if err == nil {
		t.Fatal("expected an error for an unregistered driver")
	}
	if !strings.Contains(err.Error(), "gowdk db:") {
		t.Fatalf("error %q is not wrapped by the helper", err.Error())
	}
}
