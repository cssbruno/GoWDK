package seo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSitemapNormalizesSortsAndDeduplicatesURLs(t *testing.T) {
	payload, err := Sitemap("https://example.com/docs?ignored=1#top", []URL{
		{Loc: "/b", LastMod: " 2026-06-01 "},
		{Loc: "https://example.com/a?x=1#fragment", ChangeFreq: " daily "},
		{Loc: "/products?page=2#top"},
		{Loc: "/b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	assertContains(t, text, `<loc>https://example.com/a</loc>`)
	assertContains(t, text, `<changefreq>daily</changefreq>`)
	assertContains(t, text, `<loc>https://example.com/docs/b</loc>`)
	assertContains(t, text, `<lastmod>2026-06-01</lastmod>`)
	assertContains(t, text, `<loc>https://example.com/docs/products</loc>`)
	if strings.Contains(text, `%3F`) || strings.Contains(text, `page=2`) {
		t.Fatalf("root-relative URL query leaked into sitemap:\n%s", text)
	}
	if strings.Count(text, `<loc>`) != 3 {
		t.Fatalf("expected three unique URLs, got:\n%s", text)
	}
}

func TestSitemapRejectsInvalidURL(t *testing.T) {
	if _, err := Sitemap("https://example.com", []URL{{Loc: "relative/path"}}); err == nil {
		t.Fatal("expected relative sitemap URL to fail")
	}
}

func TestHandlerMergesDynamicURLs(t *testing.T) {
	handler := Handler(HandlerOptions{
		BaseURL:    "https://example.com",
		StaticURLs: []URL{{Loc: "/static"}},
	}, func(context.Context) ([]URL, error) {
		return []URL{{Loc: "/dynamic"}}, nil
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	assertContains(t, recorder.Body.String(), "https://example.com/static")
	assertContains(t, recorder.Body.String(), "https://example.com/dynamic")
	if cache := recorder.Header().Get("Cache-Control"); cache != "no-store" {
		t.Fatalf("unexpected cache header: %q", cache)
	}
}

func TestHandlerRejectsProviderOverflow(t *testing.T) {
	handler := Handler(HandlerOptions{
		BaseURL:        "https://example.com",
		MaxDynamicURLs: 1,
	}, func(context.Context) ([]URL, error) {
		return []URL{{Loc: "/one"}, {Loc: "/two"}}, nil
	})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "/one") {
		t.Fatalf("overflow response leaked sitemap data: %s", recorder.Body.String())
	}
}

func TestHandlerRejectsUnsupportedMethod(t *testing.T) {
	handler := Handler(HandlerOptions{BaseURL: "https://example.com"}, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sitemap.xml", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", recorder.Code)
	}
	if allow := recorder.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("unexpected allow header: %q", allow)
	}
}

func assertContains(t *testing.T, value, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("expected %q in:\n%s", want, value)
	}
}
