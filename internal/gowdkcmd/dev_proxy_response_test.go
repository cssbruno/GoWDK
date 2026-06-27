package gowdkcmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type trackingReadCloser struct {
	reader   io.Reader
	reads    int
	closes   int
	closeErr error
}

func (body *trackingReadCloser) Read(buffer []byte) (int, error) {
	body.reads++
	return body.reader.Read(buffer)
}

func (body *trackingReadCloser) Close() error {
	body.closes++
	return body.closeErr
}

func TestModifyDevRuntimeProxyResponseInjectsBoundedHTML(t *testing.T) {
	source := []byte("<html><body>small</body></html>")
	original := &trackingReadCloser{reader: bytes.NewReader(source)}
	response := devProxyHTMLResponse(http.StatusOK, int64(len(source)), original)

	if err := modifyDevRuntimeProxyResponse(response, nil); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("/__gowdk/reload")) {
		t.Fatalf("expected live reload injection in %q", body)
	}
	if !bytes.Contains(body, source[:12]) {
		t.Fatalf("expected original HTML to be preserved in %q", body)
	}
	if original.closes != 1 {
		t.Fatalf("expected original body to close once, got %d", original.closes)
	}
	if response.ContentLength != int64(len(body)) {
		t.Fatalf("unexpected content length: got %d want %d", response.ContentLength, len(body))
	}
	if response.Header.Get("Content-Length") != strconv.Itoa(len(body)) {
		t.Fatalf("unexpected Content-Length header: %q", response.Header.Get("Content-Length"))
	}
}

func TestModifyDevRuntimeProxyResponseSkipsKnownOversizedHTMLWithoutReading(t *testing.T) {
	original := &trackingReadCloser{reader: strings.NewReader("unread")}
	response := devProxyHTMLResponse(http.StatusOK, maxDevProxyHTMLBytes+1, original)
	response.Header.Set("Content-Length", strconv.FormatInt(response.ContentLength, 10))

	if err := modifyDevRuntimeProxyResponse(response, nil); err != nil {
		t.Fatal(err)
	}
	if response.Body != original {
		t.Fatal("expected oversized known-length body to remain untouched")
	}
	if original.reads != 0 || original.closes != 0 {
		t.Fatalf("expected no read or close, got reads=%d closes=%d", original.reads, original.closes)
	}
	if response.Header.Get("Content-Length") != strconv.FormatInt(maxDevProxyHTMLBytes+1, 10) {
		t.Fatalf("Content-Length changed on pass-through: %q", response.Header.Get("Content-Length"))
	}
}

func TestModifyDevRuntimeProxyResponseReplaysUnknownOversizedHTML(t *testing.T) {
	source := bytes.Repeat([]byte("x"), int(maxDevProxyHTMLBytes)+257)
	original := &trackingReadCloser{reader: bytes.NewReader(source)}
	response := devProxyHTMLResponse(http.StatusOK, -1, original)
	broker := newLiveReloadBroker()
	events := make(chan liveReloadEvent, 1)
	broker.clients[events] = true

	if err := modifyDevRuntimeProxyResponse(response, broker); err != nil {
		t.Fatal(err)
	}
	if response.ContentLength != -1 {
		t.Fatalf("expected unknown content length to remain unchanged, got %d", response.ContentLength)
	}
	if response.Header.Get("Content-Length") != "" {
		t.Fatalf("unexpected Content-Length header: %q", response.Header.Get("Content-Length"))
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, source) {
		t.Fatalf("streamed body changed: got %d bytes want %d", len(body), len(source))
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if original.closes != 1 {
		t.Fatalf("expected replay body to close original once, got %d", original.closes)
	}
	select {
	case event := <-events:
		if event.Name != "proxy-injection-skipped" || !strings.Contains(event.Data, "inspected_body_too_large") {
			t.Fatalf("unexpected skip event: %#v", event)
		}
	default:
		t.Fatal("expected an observable injection-skipped event")
	}
}

func TestModifyDevRuntimeProxyResponseLeavesIneligibleBodiesUntouched(t *testing.T) {
	for _, test := range []struct {
		name     string
		method   string
		content  string
		encoding string
	}{
		{name: "head", method: http.MethodHead, content: "text/html"},
		{name: "non html", method: http.MethodGet, content: "application/json"},
		{name: "compressed", method: http.MethodGet, content: "text/html", encoding: "gzip"},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := &trackingReadCloser{reader: strings.NewReader("untouched")}
			response := devProxyHTMLResponse(http.StatusOK, 9, original)
			response.Request = httptest.NewRequest(test.method, "http://example.test/", nil)
			response.Header.Set("Content-Type", test.content)
			response.Header.Set("Content-Encoding", test.encoding)

			if err := modifyDevRuntimeProxyResponse(response, nil); err != nil {
				t.Fatal(err)
			}
			if response.Body != original || original.reads != 0 || original.closes != 0 {
				t.Fatalf("ineligible response was consumed: bodyChanged=%t reads=%d closes=%d", response.Body != original, original.reads, original.closes)
			}
		})
	}
}

func TestModifyDevRuntimeProxyResponsePropagatesReadAndCloseErrors(t *testing.T) {
	readErr := errors.New("read failed")
	closeErr := errors.New("close failed")
	original := &trackingReadCloser{reader: errorReader{err: readErr}, closeErr: closeErr}
	response := devProxyHTMLResponse(http.StatusOK, -1, original)

	err := modifyDevRuntimeProxyResponse(response, nil)
	if !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
		t.Fatalf("expected joined read and close errors, got %v", err)
	}
	if original.closes != 1 {
		t.Fatalf("expected errored body to close once, got %d", original.closes)
	}
	_ = response.Body.Close()
}

func TestModifyDevRuntimeProxyResponseKeepsRuntimeErrorEventForOversizedHTML(t *testing.T) {
	original := &trackingReadCloser{reader: strings.NewReader("unread")}
	response := devProxyHTMLResponse(http.StatusInternalServerError, maxDevProxyHTMLBytes+1, original)
	defer func() {
		_ = response.Body.Close()
	}()
	broker := newLiveReloadBroker()
	events := make(chan liveReloadEvent, 2)
	broker.clients[events] = true

	if err := modifyDevRuntimeProxyResponse(response, broker); err != nil {
		t.Fatal(err)
	}
	first := <-events
	second := <-events
	if first.Name != "runtime-error" || second.Name != "proxy-injection-skipped" {
		t.Fatalf("unexpected events: first=%#v second=%#v", first, second)
	}
	if original.reads != 0 {
		t.Fatalf("expected oversized 5xx body not to be read, got %d reads", original.reads)
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}

func devProxyHTMLResponse(status int, contentLength int64, body io.ReadCloser) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Header:        http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:          body,
		ContentLength: contentLength,
		Request:       httptest.NewRequest(http.MethodGet, "http://example.test/", nil),
	}
}
