package realtime

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/runtime/contracts"
)

func TestAddonEnablesRealtimeFeature(t *testing.T) {
	config := gowdk.Config{Addons: []gowdk.Addon{Addon()}}
	if !config.HasFeature(gowdk.FeatureRealtime) {
		t.Fatal("expected realtime feature")
	}
}

func TestNewSSEReturnsPresentationFanout(t *testing.T) {
	var fanout contracts.PresentationFanout = NewSSE(WithSSEBufferSize(1))
	if fanout == nil {
		t.Fatal("expected SSE fanout")
	}
}
