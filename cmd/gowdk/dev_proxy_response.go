package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// maxDevProxyHTMLBytes bounds the development-only HTML inspection used to
// inject live reload and the initial runtime-error overlay. Larger responses
// are forwarded unchanged and continue streaming from the backend.
const maxDevProxyHTMLBytes int64 = 4 << 20

type replayReadCloser struct {
	io.Reader
	closer io.Closer
}

func (body *replayReadCloser) Close() error {
	return body.closer.Close()
}

func modifyDevRuntimeProxyResponse(response *http.Response, reload *liveReloadBroker) error {
	if response == nil {
		return nil
	}
	runtimeErrorPayload := ""
	if response.StatusCode >= http.StatusInternalServerError {
		runtimeErrorPayload = devRuntimeErrorEventData(response.StatusCode)
		reload.notifyData("runtime-error", runtimeErrorPayload)
	}
	if response.Request == nil ||
		response.Request.Method != http.MethodGet ||
		response.Body == nil ||
		response.Header.Get("Content-Encoding") != "" ||
		!strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/html") {
		return nil
	}
	if response.StatusCode != http.StatusOK && runtimeErrorPayload == "" {
		return nil
	}
	if response.ContentLength > maxDevProxyHTMLBytes {
		notifyDevProxyInjectionSkipped(reload, response, "declared_body_too_large")
		return nil
	}

	originalBody := response.Body
	body, readErr := io.ReadAll(io.LimitReader(originalBody, maxDevProxyHTMLBytes+1))
	if readErr != nil {
		return errors.Join(readErr, originalBody.Close())
	}
	if int64(len(body)) > maxDevProxyHTMLBytes {
		response.Body = &replayReadCloser{
			Reader: io.MultiReader(bytes.NewReader(body), originalBody),
			closer: originalBody,
		}
		notifyDevProxyInjectionSkipped(reload, response, "inspected_body_too_large")
		return nil
	}
	if err := originalBody.Close(); err != nil {
		return err
	}

	if runtimeErrorPayload != "" {
		body = injectLiveReloadScriptWithInitialOverlay(body, []byte(runtimeErrorPayload))
	} else {
		body = injectLiveReloadScript(body)
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))
	if response.Header == nil {
		response.Header = http.Header{}
	}
	response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

type devProxyInjectionSkipEvent struct {
	Reason        string `json:"reason"`
	Status        int    `json:"status"`
	LimitBytes    int64  `json:"limitBytes"`
	DeclaredBytes int64  `json:"declaredBytes"`
}

func notifyDevProxyInjectionSkipped(reload *liveReloadBroker, response *http.Response, reason string) {
	if reload == nil || response == nil {
		return
	}
	payload, err := json.Marshal(devProxyInjectionSkipEvent{
		Reason:        reason,
		Status:        response.StatusCode,
		LimitBytes:    maxDevProxyHTMLBytes,
		DeclaredBytes: response.ContentLength,
	})
	if err != nil {
		return
	}
	reload.notifyData("proxy-injection-skipped", string(payload))
}
