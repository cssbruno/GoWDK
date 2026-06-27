// Package testkit provides small helpers for generated runtime integration
// tests.
package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// Scenario describes one HTTP expectation against a generated app handler.
type Scenario struct {
	Name             string
	Method           string
	Path             string
	Body             string
	Headers          map[string]string
	WantStatus       int
	WantHeader       map[string]string
	WantBodyContains string
}

// Request describes one generated-handler test request.
type Request struct {
	Method     string
	Path       string
	Body       string
	Headers    map[string]string
	Query      url.Values
	Form       url.Values
	JSON       any
	Cookies    []*http.Cookie
	Host       string
	RemoteAddr string
	Context    context.Context
}

// Result captures one test response.
type Result struct {
	StatusCode int
	Header     http.Header
	Body       string
	Cookies    []*http.Cookie
	Request    Request
}

// Client drives generated app handlers with a cookie jar. It can execute
// requests directly against an http.Handler or through an httptest.Server.
type Client struct {
	handler    http.Handler
	server     *httptest.Server
	httpClient *http.Client
	jar        http.CookieJar
	baseURL    string
}

// Get builds a GET request.
func Get(path string) Request {
	return Request{Method: http.MethodGet, Path: path}
}

// PostForm builds an application/x-www-form-urlencoded POST request.
func PostForm(path string, form url.Values) Request {
	return Request{Method: http.MethodPost, Path: path, Form: cloneValues(form)}
}

// PostJSON builds an application/json POST request.
func PostJSON(path string, value any) Request {
	return Request{Method: http.MethodPost, Path: path, JSON: value}
}

// WithHeader returns a copy of request with a header set.
func (request Request) WithHeader(name, value string) Request {
	request.Headers = cloneHeaderMap(request.Headers)
	request.Headers[name] = value
	return request
}

// WithCookie returns a copy of request with a cookie attached.
func (request Request) WithCookie(cookie *http.Cookie) Request {
	if cookie != nil {
		request.Cookies = append(append([]*http.Cookie(nil), request.Cookies...), cookie)
	}
	return request
}

// WithQuery returns a copy of request with a query value appended.
func (request Request) WithQuery(name, value string) Request {
	request.Query = cloneValues(request.Query)
	request.Query.Add(name, value)
	return request
}

// NewClient creates a cookie-preserving client that executes requests directly
// against handler without opening a listener.
func NewClient(tb testing.TB, handler http.Handler) *Client {
	tb.Helper()
	if handler == nil {
		tb.Fatal("testkit client requires a handler")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		tb.Fatalf("create cookie jar: %v", err)
	}
	return &Client{handler: handler, jar: jar, baseURL: "http://gowdk.test"}
}

// NewServerClient creates a cookie-preserving client backed by an httptest
// server. Use this when behavior depends on the HTTP transport.
func NewServerClient(tb testing.TB, handler http.Handler) *Client {
	tb.Helper()
	if handler == nil {
		tb.Fatal("testkit server client requires a handler")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		tb.Fatalf("create cookie jar: %v", err)
	}
	server := httptest.NewServer(handler)
	httpClient := server.Client()
	httpClient.Jar = jar
	client := &Client{server: server, httpClient: httpClient, jar: jar, baseURL: server.URL}
	tb.Cleanup(client.Close)
	return client
}

// BaseURL returns the client's absolute base URL.
func (client *Client) BaseURL() string {
	if client == nil {
		return ""
	}
	return client.baseURL
}

// Close shuts down the backing test server, if one exists.
func (client *Client) Close() {
	if client != nil && client.server != nil {
		client.server.Close()
		client.server = nil
	}
}

// Do executes one request and returns the captured result.
func (client *Client) Do(tb testing.TB, request Request) Result {
	tb.Helper()
	if client == nil {
		tb.Fatal("testkit client is nil")
		return Result{}
	}
	if client.server != nil {
		return client.doServer(tb, request)
	}
	return client.doDirect(tb, request)
}

// Get executes a GET request.
func (client *Client) Get(tb testing.TB, path string) Result {
	tb.Helper()
	return client.Do(tb, Get(path))
}

// PostForm executes a form POST request.
func (client *Client) PostForm(tb testing.TB, path string, form url.Values) Result {
	tb.Helper()
	return client.Do(tb, PostForm(path, form))
}

// PostJSON executes a JSON POST request.
func (client *Client) PostJSON(tb testing.TB, path string, value any) Result {
	tb.Helper()
	return client.Do(tb, PostJSON(path, value))
}

// AssertStatus checks the response status code.
func (result Result) AssertStatus(tb testing.TB, want int) {
	tb.Helper()
	if result.StatusCode != want {
		tb.Fatalf("status = %d, want %d with body %s", result.StatusCode, want, responseBodySummary(result.Body))
	}
}

// AssertStatusRange checks that the response status code is within [minimum, maximum].
func (result Result) AssertStatusRange(tb testing.TB, minimum, maximum int) {
	tb.Helper()
	if result.StatusCode < minimum || result.StatusCode > maximum {
		tb.Fatalf("status = %d, want range %d..%d with body %s", result.StatusCode, minimum, maximum, responseBodySummary(result.Body))
	}
}

// AssertHeader checks one exact response header value.
func (result Result) AssertHeader(tb testing.TB, name, want string) {
	tb.Helper()
	if got := result.Header.Get(name); got != want {
		tb.Fatalf("header %s = %q, want %q", name, got, want)
	}
}

// AssertHeaderContains checks that one response header contains text.
func (result Result) AssertHeaderContains(tb testing.TB, name, want string) {
	tb.Helper()
	if got := result.Header.Get(name); !strings.Contains(got, want) {
		tb.Fatalf("header %s = %q, want it to contain %q", name, got, want)
	}
}

// AssertContentType checks the response Content-Type prefix.
func (result Result) AssertContentType(tb testing.TB, want string) {
	tb.Helper()
	if got := result.Header.Get("Content-Type"); !strings.HasPrefix(got, want) {
		tb.Fatalf("content type = %q, want prefix %q", got, want)
	}
}

// AssertCookie checks that a response Set-Cookie with name exists.
func (result Result) AssertCookie(tb testing.TB, name string) *http.Cookie {
	tb.Helper()
	for _, cookie := range result.Cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	tb.Fatalf("missing Set-Cookie %q in %#v", name, result.Cookies)
	return nil
}

// AssertBodyEquals checks the complete response body.
func (result Result) AssertBodyEquals(tb testing.TB, want string) {
	tb.Helper()
	if result.Body != want {
		tb.Fatalf("body = %q, want %q", result.Body, want)
	}
}

// AssertBodyContains checks that the response body contains text.
func (result Result) AssertBodyContains(tb testing.TB, want string) {
	tb.Helper()
	if !strings.Contains(result.Body, want) {
		tb.Fatalf("body does not contain %q: %s", want, responseBodySummary(result.Body))
	}
}

// AssertJSONEqual compares the response body with want after JSON
// normalization.
func (result Result) AssertJSONEqual(tb testing.TB, want any) {
	tb.Helper()
	var gotValue any
	if err := json.Unmarshal([]byte(result.Body), &gotValue); err != nil {
		tb.Fatalf("decode response JSON: %v with body %s", err, responseBodySummary(result.Body))
	}
	wantPayload, err := json.Marshal(want)
	if err != nil {
		tb.Fatalf("encode expected JSON: %v", err)
	}
	var wantValue any
	if err := json.Unmarshal(wantPayload, &wantValue); err != nil {
		tb.Fatalf("decode expected JSON: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		tb.Fatalf("JSON response = %#v, want %#v", gotValue, wantValue)
	}
}

// Run executes scenarios against handler.
func Run(tb testing.TB, handler http.Handler, scenarios []Scenario) {
	tb.Helper()
	if runner, ok := tb.(interface {
		Run(string, func(*testing.T)) bool
	}); ok {
		for _, scenario := range scenarios {
			runner.Run(scenarioName(scenario), func(t *testing.T) {
				t.Helper()
				runScenario(t, handler, scenario)
			})
		}
		return
	}
	for _, scenario := range scenarios {
		runScenario(tb, handler, scenario)
	}
}

func runScenario(tb testing.TB, handler http.Handler, scenario Scenario) {
	tb.Helper()
	response := Response(handler, scenario)
	if scenario.WantStatus != 0 && response.Code != scenario.WantStatus {
		tb.Errorf("expected status %d, got %d with body %s", scenario.WantStatus, response.Code, responseBodySummary(response.Body.String()))
	}
	for name, want := range scenario.WantHeader {
		if got := response.Header().Get(name); got != want {
			tb.Errorf("expected header %s=%q, got %q", name, want, got)
		}
	}
	if want := strings.TrimSpace(scenario.WantBodyContains); want != "" && !strings.Contains(response.Body.String(), want) {
		tb.Errorf("expected body to contain %q, got %s", want, responseBodySummary(response.Body.String()))
	}
}

// AssertStatus checks one request's response status.
func AssertStatus(tb testing.TB, handler http.Handler, method, requestPath, body string, want int) {
	tb.Helper()
	Run(tb, handler, []Scenario{{
		Name:       method + " " + requestPath,
		Method:     method,
		Path:       requestPath,
		Body:       body,
		WantStatus: want,
	}})
}

// AssertHeader checks one response header value.
func AssertHeader(tb testing.TB, handler http.Handler, method, requestPath, name, want string) {
	tb.Helper()
	Run(tb, handler, []Scenario{{
		Name:       method + " " + requestPath,
		Method:     method,
		Path:       requestPath,
		WantHeader: map[string]string{name: want},
	}})
}

// Response executes one scenario and returns the recorder for custom checks.
func Response(handler http.Handler, scenario Scenario) *httptest.ResponseRecorder {
	method := strings.TrimSpace(scenario.Method)
	if method == "" {
		method = http.MethodGet
	}
	requestPath := strings.TrimSpace(scenario.Path)
	if requestPath == "" {
		requestPath = "/"
	}
	request := httptest.NewRequest(method, requestPath, strings.NewReader(scenario.Body))
	for name, value := range scenario.Headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func (client *Client) doDirect(tb testing.TB, request Request) Result {
	tb.Helper()
	httpRequest := client.newHTTPRequest(tb, request)
	for _, cookie := range client.jar.Cookies(httpRequest.URL) {
		httpRequest.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	client.handler.ServeHTTP(recorder, httpRequest)
	response := recorder.Result()
	client.jar.SetCookies(httpRequest.URL, response.Cookies())
	return Result{
		StatusCode: recorder.Code,
		Header:     response.Header.Clone(),
		Body:       recorder.Body.String(),
		Cookies:    response.Cookies(),
		Request:    request,
	}
}

func (client *Client) doServer(tb testing.TB, request Request) Result {
	tb.Helper()
	httpRequest := client.newHTTPRequest(tb, request)
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		tb.Fatalf("execute %s %s: %v", httpRequest.Method, httpRequest.URL.String(), err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		tb.Fatalf("read response body: %v", err)
	}
	return Result{
		StatusCode: response.StatusCode,
		Header:     response.Header.Clone(),
		Body:       string(body),
		Cookies:    response.Cookies(),
		Request:    request,
	}
}

func (client *Client) newHTTPRequest(tb testing.TB, request Request) *http.Request {
	tb.Helper()
	method := strings.TrimSpace(request.Method)
	if method == "" {
		method = http.MethodGet
	}
	target := client.requestURL(tb, request)
	body, contentType := requestBody(tb, request)
	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	httpRequest, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		tb.Fatalf("build request %s %s: %v", method, target, err)
	}
	if contentType != "" {
		httpRequest.Header.Set("Content-Type", contentType)
	}
	for name, value := range request.Headers {
		httpRequest.Header.Set(name, value)
	}
	for _, cookie := range request.Cookies {
		httpRequest.AddCookie(cookie)
	}
	if request.Host != "" {
		httpRequest.Host = request.Host
	}
	if request.RemoteAddr != "" {
		httpRequest.RemoteAddr = request.RemoteAddr
	}
	return httpRequest
}

func (client *Client) requestURL(tb testing.TB, request Request) string {
	tb.Helper()
	baseURL := strings.TrimRight(client.baseURL, "/")
	if baseURL == "" {
		baseURL = "http://gowdk.test"
	}
	requestPath := strings.TrimSpace(request.Path)
	if requestPath == "" {
		requestPath = "/"
	}
	parsed, err := url.Parse(requestPath)
	if err != nil {
		tb.Fatalf("parse request path %q: %v", requestPath, err)
	}
	if !parsed.IsAbs() {
		if !strings.HasPrefix(requestPath, "/") {
			requestPath = "/" + requestPath
		}
		parsed, err = url.Parse(baseURL + requestPath)
		if err != nil {
			tb.Fatalf("parse request URL %q: %v", baseURL+requestPath, err)
		}
	}
	query := parsed.Query()
	for name, values := range request.Query {
		for _, value := range values {
			query.Add(name, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func requestBody(tb testing.TB, request Request) (io.Reader, string) {
	tb.Helper()
	if request.JSON != nil {
		payload, err := json.Marshal(request.JSON)
		if err != nil {
			tb.Fatalf("encode request JSON: %v", err)
		}
		return bytes.NewReader(payload), "application/json"
	}
	if len(request.Form) > 0 {
		return strings.NewReader(request.Form.Encode()), "application/x-www-form-urlencoded"
	}
	if request.Body != "" {
		return strings.NewReader(request.Body), ""
	}
	return nil, ""
}

func scenarioName(scenario Scenario) string {
	if strings.TrimSpace(scenario.Name) != "" {
		return scenario.Name
	}
	return strings.TrimSpace(scenario.Method + " " + scenario.Path)
}

func cloneValues(values url.Values) url.Values {
	if values == nil {
		return url.Values{}
	}
	out := make(url.Values, len(values))
	for name, entries := range values {
		out[name] = append([]string(nil), entries...)
	}
	return out
}

func cloneHeaderMap(values map[string]string) map[string]string {
	out := make(map[string]string, len(values)+1)
	for name, value := range values {
		out[name] = value
	}
	return out
}

func responseBodySummary(body string) string {
	if body == "" {
		return "<empty>"
	}
	return fmt.Sprintf("<%d byte response body redacted>", len(body))
}
