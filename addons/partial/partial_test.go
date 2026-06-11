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

func TestSwapReturnsExplicitSwapResponse(t *testing.T) {
	result, err := Swap("#messages", SwapOuterHTML, "<section>Hello</section>")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != response.Fragment || result.Target != "#messages" || result.Body != "<section>Hello</section>" || result.Swap != response.SwapOuterHTML {
		t.Fatalf("unexpected fragment swap response: %#v", result)
	}
}

func TestPartialConstants(t *testing.T) {
	if HookBeforeRequest != "before-request" || HookAfterSwap != "after-swap" || HookRequestError != "request-error" {
		t.Fatalf("unexpected hooks: %q %q %q", HookBeforeRequest, HookAfterSwap, HookRequestError)
	}
	if SwapInnerHTML != "innerHTML" || SwapOuterHTML != "outerHTML" {
		t.Fatalf("unexpected swap modes")
	}
}
