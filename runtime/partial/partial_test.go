package partial

import (
	"testing"

	"github.com/cssbruno/gowdk/runtime/response"
)

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
