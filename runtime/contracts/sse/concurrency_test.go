package sse

import (
	"context"
	"sync"
	"testing"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

func TestHubConcurrentSendAndDisconnect(t *testing.T) {
	hub := New(WithBufferSize(2))
	const clients = 20
	const sends = 100

	connected := make([]*sseClient, 0, clients)
	for i := 0; i < clients; i++ {
		client := &sseClient{
			queue:      make(chan sseMessage, 2),
			disconnect: make(chan struct{}),
		}
		hub.add(client)
		connected = append(connected, client)
	}

	var done sync.WaitGroup
	done.Add(2)
	go func() {
		defer done.Done()
		for i := 0; i < sends; i++ {
			if err := hub.SendPresentationEvents(context.Background(), []contracts.EventEnvelope{{
				Category: contracts.PresentationEvent,
				Type:     "PatientNotice",
				Value:    patientNotice{ID: "patient"},
			}}); err != nil {
				t.Errorf("send presentation events: %v", err)
			}
		}
	}()
	go func() {
		defer done.Done()
		for _, client := range connected {
			hub.remove(client)
		}
	}()
	done.Wait()

	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("client count = %d, want 0", got)
	}
}
