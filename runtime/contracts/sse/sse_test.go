package sse

import (
	"bufio"
	"context"
	"errors"
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
	defer func() {
		_ = response.Body.Close()
	}()

	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read first SSE line: %v", err)
	}
	if line != "retry: 1000\n" {
		t.Fatalf("first SSE line = %q, want default retry directive", line)
	}
}

func TestHubSendsConfiguredRetryDirective(t *testing.T) {
	hub := New(WithRetryMillis(2500))
	server := httptest.NewServer(hub)
	defer server.Close()

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read first SSE line: %v", err)
	}
	if line != "retry: 2500\n" {
		t.Fatalf("first SSE line = %q, want configured retry directive", line)
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
	defer func() {
		_ = response.Body.Close()
	}()

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
	defer func() {
		_ = broadcast.Body.Close()
	}()
	ada, err := http.Get(server.URL + "?client=ada")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ada.Body.Close()
	}()

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
		queue:      make(chan sseMessage, 1),
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
	if stats := hub.Stats(); stats.DroppedClients != 1 {
		t.Fatalf("dropped client stats = %#v, want one drop", stats)
	}
}

func TestHubReplaysEventsAfterLastEventID(t *testing.T) {
	hub := New(WithReplayLimit(4))
	err := hub.SendPresentationEvents(context.Background(), []contracts.EventEnvelope{
		{ID: "event-1", Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "patient-1"}},
		{ID: "event-2", Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "patient-2"}},
	})
	if err != nil {
		t.Fatalf("send presentation events: %v", err)
	}

	server := httptest.NewServer(hub)
	defer server.Close()
	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Last-Event-ID", "event-1")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	reader := bufio.NewReader(response.Body)
	deadline := time.Now().Add(2 * time.Second)
	var idLine, dataLine string
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE line: %v", err)
		}
		if strings.HasPrefix(line, "id: ") {
			idLine = line
		}
		if strings.HasPrefix(line, "data: ") {
			dataLine = line
			break
		}
	}
	if idLine != "id: event-2\n" {
		t.Fatalf("replayed id line = %q, want event-2", idLine)
	}
	if !strings.Contains(dataLine, `"ID":"event-2"`) || !strings.Contains(dataLine, `"id":"patient-2"`) {
		t.Fatalf("unexpected replayed data line: %q", dataLine)
	}
	if stats := hub.Stats(); stats.ReplayedEvents != 1 || stats.ReplayMisses != 0 {
		t.Fatalf("unexpected replay stats: %#v", stats)
	}
}

func TestHubReplayKeepsAudienceScope(t *testing.T) {
	hub := New(
		WithReplayLimit(4),
		WithAudienceFromRequest(func(request *http.Request) []string {
			return []string{"tenant:clinic"}
		}),
	)
	err := hub.SendPresentationEvents(context.Background(), []contracts.EventEnvelope{
		{ID: "event-1", Category: contracts.PresentationEvent, Type: "PatientNotice", Audience: []string{"tenant:other"}, Value: patientNotice{ID: "private-other"}},
		{ID: "event-2", Category: contracts.PresentationEvent, Type: "PatientNotice", Audience: []string{"tenant:clinic"}, Value: patientNotice{ID: "private-clinic"}},
	})
	if err != nil {
		t.Fatalf("send presentation events: %v", err)
	}

	server := httptest.NewServer(hub)
	defer server.Close()
	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Last-Event-ID", "event-1")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	data := readSSEDataLines(t, response.Body, 1)[0]
	if !strings.Contains(data, `"id":"private-clinic"`) || strings.Contains(data, "private-other") {
		t.Fatalf("audience-scoped replay data = %q", data)
	}
}

func TestHubAddWithReplayDoesNotReplayEventsQueuedAfterConnect(t *testing.T) {
	hub := New(WithReplayLimit(4))
	if err := hub.SendPresentationEvents(context.Background(), []contracts.EventEnvelope{
		{ID: "event-1", Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "patient-1"}},
	}); err != nil {
		t.Fatalf("seed replay: %v", err)
	}
	client := &sseClient{
		queue:      make(chan sseMessage, 2),
		disconnect: make(chan struct{}),
	}
	replay := hub.addWithReplay(client, "event-1")
	defer hub.remove(client)
	if len(replay) != 0 {
		t.Fatalf("expected no replay before post-connect event, got %+v", replay)
	}
	if err := hub.SendPresentationEvents(context.Background(), []contracts.EventEnvelope{
		{ID: "event-2", Category: contracts.PresentationEvent, Type: "PatientNotice", Value: patientNotice{ID: "patient-2"}},
	}); err != nil {
		t.Fatalf("send post-connect event: %v", err)
	}
	select {
	case message := <-client.queue:
		if message.id != "event-2" {
			t.Fatalf("queued message id = %q, want event-2", message.id)
		}
	default:
		t.Fatal("expected post-connect event to be queued")
	}
	select {
	case duplicate := <-client.queue:
		t.Fatalf("did not expect duplicate queued event: %+v", duplicate)
	default:
	}
}

func TestHubRevokeAudienceDisconnectsMatchingClients(t *testing.T) {
	hub := New()
	ada := &sseClient{
		queue:      make(chan sseMessage, 1),
		disconnect: make(chan struct{}),
		audience:   audienceSet([]string{"tenant:clinic", "session:ada"}),
	}
	bob := &sseClient{
		queue:      make(chan sseMessage, 1),
		disconnect: make(chan struct{}),
		audience:   audienceSet([]string{"tenant:clinic", "session:bob"}),
	}
	hub.add(ada)
	hub.add(bob)
	defer hub.remove(ada)
	defer hub.remove(bob)

	if got := hub.RevokeAudience("session:ada"); got != 1 {
		t.Fatalf("RevokeAudience disconnected %d clients, want 1", got)
	}
	if stats := hub.Stats(); stats.ClientCount != 2 || stats.RevokedClients != 1 {
		t.Fatalf("unexpected revoke stats: %#v", stats)
	}
	select {
	case <-ada.disconnect:
	default:
		t.Fatal("expected matching client to be disconnected")
	}
	select {
	case <-bob.disconnect:
		t.Fatal("did not expect non-matching client to be disconnected")
	default:
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
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendPresentationEvents error = %v, want context canceled", err)
	}
}
