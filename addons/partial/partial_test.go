package partial

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/runtime/response"
)

func TestAddonRegistersPartialFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "partial" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeaturePartial) {
		t.Fatal("expected partial feature")
	}
}

func TestFragmentReturnsTargetedResponse(t *testing.T) {
	result := Fragment("#messages", "<li>Hello</li>")
	if result.Kind != response.Fragment || result.Target != "#messages" || result.Body != "<li>Hello</li>" || result.Status != 200 {
		t.Fatalf("unexpected fragment response: %#v", result)
	}
}

func TestPartialConstants(t *testing.T) {
	if HookBeforeRequest != "before-request" || HookAfterSwap != "after-swap" || HookRequestError != "request-error" {
		t.Fatalf("unexpected hooks: %q %q %q", HookBeforeRequest, HookAfterSwap, HookRequestError)
	}
	if SwapReplace != "replace" || SwapAppend != "append" || SwapPrepend != "prepend" || SwapBefore != "before" || SwapAfter != "after" {
		t.Fatalf("unexpected swap modes")
	}
}
