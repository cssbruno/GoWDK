package seo

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const defaultMaxDynamicURLs = 1000

// URL describes one sitemap URL. Loc may be absolute or root-relative before
// normalization.
type URL struct {
	Loc        string
	LastMod    string
	ChangeFreq string
	Priority   string
}

// Provider supplies request-time sitemap URLs owned by application code.
type Provider func(context.Context) ([]URL, error)

// HandlerOptions configures a generated sitemap handler.
type HandlerOptions struct {
	BaseURL        string
	StaticURLs     []URL
	MaxDynamicURLs int
	CacheSeconds   int
}

type urlEntry struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	XMLNS   string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

// Handler returns an HTTP handler that merges generated static URLs with an
// optional app-owned provider. Provider errors return a generic unavailable
// response and do not expose application error text.
func Handler(options HandlerOptions, provider Provider) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		urls := append([]URL(nil), options.StaticURLs...)
		if provider != nil {
			dynamic, err := provider(request.Context())
			if err != nil {
				writeUnavailable(response)
				return
			}
			limit := options.MaxDynamicURLs
			if limit <= 0 {
				limit = defaultMaxDynamicURLs
			}
			if len(dynamic) > limit {
				writeUnavailable(response)
				return
			}
			urls = append(urls, dynamic...)
		}
		payload, err := Sitemap(options.BaseURL, urls)
		if err != nil {
			writeUnavailable(response)
			return
		}
		response.Header().Set("Content-Type", "application/xml; charset=utf-8")
		if options.CacheSeconds > 0 {
			response.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", options.CacheSeconds))
		} else {
			response.Header().Set("Cache-Control", "no-store")
		}
		response.WriteHeader(http.StatusOK)
		if request.Method != http.MethodHead {
			_, _ = response.Write(payload)
		}
	})
}

func writeUnavailable(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	http.Error(response, "sitemap unavailable", http.StatusServiceUnavailable)
}

// Sitemap normalizes, de-duplicates, sorts, and serializes sitemap URLs.
func Sitemap(baseURL string, urls []URL) ([]byte, error) {
	base, err := ParseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	entries, err := normalizeURLs(base, urls)
	if err != nil {
		return nil, err
	}
	payload, err := xml.MarshalIndent(urlSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  entries,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	out := append([]byte(xml.Header), payload...)
	out = append(out, '\n')
	return out, nil
}

func normalizeURLs(base *url.URL, urls []URL) ([]urlEntry, error) {
	seen := map[string]bool{}
	entries := make([]urlEntry, 0, len(urls))
	for _, candidate := range urls {
		loc := strings.TrimSpace(candidate.Loc)
		if loc == "" {
			return nil, fmt.Errorf("sitemap URL loc is required")
		}
		absolute, err := NormalizeURL(base, loc)
		if err != nil {
			return nil, err
		}
		if seen[absolute] {
			continue
		}
		seen[absolute] = true
		entries = append(entries, urlEntry{
			Loc:        absolute,
			LastMod:    strings.TrimSpace(candidate.LastMod),
			ChangeFreq: strings.TrimSpace(candidate.ChangeFreq),
			Priority:   strings.TrimSpace(candidate.Priority),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Loc < entries[j].Loc
	})
	return entries, nil
}

// ParseBaseURL validates the configured site base URL.
func ParseBaseURL(value string) (*url.URL, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, fmt.Errorf("seo BaseURL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("seo BaseURL is invalid: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("seo BaseURL must be an absolute http or https URL")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

// NormalizeURL validates one sitemap URL and returns its absolute form.
func NormalizeURL(base *url.URL, loc string) (string, error) {
	parsed, err := url.Parse(loc)
	if err != nil {
		return "", fmt.Errorf("sitemap URL %q is invalid: %w", loc, err)
	}
	if parsed.IsAbs() {
		if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return "", fmt.Errorf("sitemap URL %q must be an absolute http(s) URL or a root-relative path", loc)
		}
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String(), nil
	}
	if !strings.HasPrefix(loc, "/") {
		return "", fmt.Errorf("sitemap URL %q must be absolute or root-relative", loc)
	}
	return AbsoluteURL(base, loc), nil
}

// AbsoluteURL joins a root-relative route under the configured base URL.
func AbsoluteURL(base *url.URL, route string) string {
	resolved := *base
	resolved.RawQuery = ""
	resolved.Fragment = ""
	resolved.RawPath = ""

	basePath := strings.TrimRight(resolved.Path, "/")
	routePath := "/" + strings.TrimLeft(route, "/")
	if routePath == "/" {
		if basePath == "" {
			resolved.Path = "/"
		} else {
			resolved.Path = basePath + "/"
		}
		return resolved.String()
	}
	resolved.Path = basePath + routePath
	return resolved.String()
}
