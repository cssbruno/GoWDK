package sse

import (
	"bufio"
	"context"
	"io"
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

func TestHubSendsRetryDirective(t *testing.T) {
	hub := New()
	server := httptest.NewServer(hub)
	defer server.Close()

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read first SSE line: %v", err)
	}
	if line != defaultRetryDirective {
		t.Fatalf("first SSE line = %q, want %q", line, defaultRetryDirective)
	}
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

func TestHubFiltersPresentationEventsByAudience(t *testing.T) {
	hub := New(WithAudienceFromRequest(func(request *http.Request) []string {
		if request.URL.Query().Get("client") == "ada" {
			return []string{"tenant:clinic", "user:ada"}
		}
		return nil
	}))
	server := httptest.NewServer(hub)
	defer server.Close()

	broadcast, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer broadcast.Body.Close()
	ada, err := http.Get(server.URL + "?client=ada")
	if err != nil {
		t.Fatal(err)
	}
	defer ada.Body.Close()

	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ClientCount() != 2 {
		t.Fatalf("hub.ClientCount() = %d, want 2", hub.ClientCount())
	}

	err = hub.SendPresentationEvents(context.Background(), []contracts.EventEnvelope{
		{Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "public"}},
		{Category: contracts.PresentationEvent, Type: "PatientNotice", Audience: []string{"tenant:clinic", "user:ada"}, Value: patientNotice{ID: "private"}},
	})
	if err != nil {
		t.Fatalf("send presentation events: %v", err)
	}

	broadcastData := readSSEDataLines(t, broadcast.Body, 1)
	if !strings.Contains(broadcastData[0], `"id":"public"`) || strings.Contains(broadcastData[0], `"id":"private"`) {
		t.Fatalf("broadcast client saw unexpected data: %#v", broadcastData)
	}
	adaData := readSSEDataLines(t, ada.Body, 2)
	joined := strings.Join(adaData, "\n")
	if !strings.Contains(joined, `"id":"public"`) || !strings.Contains(joined, `"id":"private"`) {
		t.Fatalf("audienced client did not receive public and private events: %#v", adaData)
	}
}

func TestHubDisconnectsSlowClient(t *testing.T) {
	hub := New(WithBufferSize(1))
	client := &sseClient{
		queue:      make(chan []byte, 1),
		disconnect: make(chan struct{}),
	}
	hub.add(client)
	defer hub.remove(client)

	err := hub.SendPresentationEvents(context.Background(), []contracts.EventEnvelope{
		{Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "patient-1"}},
		{Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "patient-2"}},
		{Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "patient-3"}},
	})
	if err != nil {
		t.Fatalf("send presentation events: %v", err)
	}
	// A client whose buffer overflows is disconnected so the browser
	// reconnects and resynchronizes, instead of silently dropping events.
	select {
	case <-client.disconnect:
	default:
		t.Fatal("expected the slow client to be disconnected on buffer overflow")
	}
}

func readSSEDataLines(t *testing.T, body io.Reader, want int) []string {
	t.Helper()
	buffered := bufio.NewReader(body)
	deadline := time.Now().Add(2 * time.Second)
	var data []string
	for len(data) < want && time.Now().Before(deadline) {
		line, err := buffered.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE line: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			data = append(data, line)
		}
	}
	if len(data) != want {
		t.Fatalf("read %d data lines, want %d: %#v", len(data), want, data)
	}
	return data
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
