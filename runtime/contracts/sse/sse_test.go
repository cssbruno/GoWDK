package sse

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cssbruno/gowdk/runtime/contracts"
)

type patientNotice struct {
	ID string `json:"id"`
}

func TestHubStreamsPresentationEvents(t *testing.T) {
	hub := New()
	server := httptest.NewServer(hub)
	defer server.Close()

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ClientCount() != 1 {
		t.Fatalf("hub.ClientCount() = %d, want 1", hub.ClientCount())
	}

	err = hub.SendPresentationEvents(context.Background(), []contracts.EventEnvelope{
		{Category: contracts.DomainEvent, Type: "PatientCreated", Value: patientNotice{ID: "ignored"}},
		{Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "patient-1"}},
	})
	if err != nil {
		t.Fatalf("send presentation events: %v", err)
	}

	reader := bufio.NewReader(response.Body)
	var dataLine string
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE line: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			dataLine = line
			break
		}
	}
	if !strings.Contains(dataLine, `"Category":"presentation"`) ||
		!strings.Contains(dataLine, `"Type":"PatientNotice"`) ||
		!strings.Contains(dataLine, `"id":"patient-1"`) {
		t.Fatalf("unexpected SSE data line: %q", dataLine)
	}
	if strings.Contains(dataLine, "ignored") {
		t.Fatalf("SSE data included non-presentation event: %q", dataLine)
	}
}

func TestHubReturnsContextError(t *testing.T) {
	hub := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := hub.SendPresentationEvents(ctx, []contracts.EventEnvelope{{
		Category: contracts.PresentationEvent,
		Type:     "PatientNotice",
		Value:    patientNotice{ID: "patient-1"},
	}})
	if err != context.Canceled {
		t.Fatalf("SendPresentationEvents error = %v, want context canceled", err)
	}
}
