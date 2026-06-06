package app

import (
	"context"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// ErrorPages stores optional generated HTML error documents.
type ErrorPages struct {
	NotFound            []byte
	InternalServerError []byte
	Custom              map[string][]byte
}

// ErrorPage describes one additional generated HTML error document to load.
type ErrorPage struct {
	Path string
}

// LoadErrorPages reads optional 404.html and 500.html files from generated
// output.
func LoadErrorPages(root fs.FS) ErrorPages {
	return ErrorPages{
		NotFound:            readOptionalErrorPage(root, "404.html"),
		InternalServerError: readOptionalErrorPage(root, "500.html"),
	}
}

// LoadErrorPagesWith reads default error pages plus extra generated error
// documents selected by generated route metadata.
func LoadErrorPagesWith(root fs.FS, custom ...ErrorPage) ErrorPages {
	pages := LoadErrorPages(root)
	for _, page := range custom {
		pagePath := cleanErrorPagePath(page.Path)
		if pagePath == "" {
			continue
		}
		payload := readOptionalErrorPage(root, pagePath)
		if len(payload) == 0 {
			continue
		}
		if pages.Custom == nil {
			pages.Custom = map[string][]byte{}
		}
		pages.Custom[pagePath] = payload
	}
	return pages
}

func readOptionalErrorPage(root fs.FS, name string) []byte {
	payload, err := fs.ReadFile(root, name)
	if err != nil {
		return nil
	}
	return append([]byte(nil), payload...)
}

func withErrorPages(ctx context.Context, pages ErrorPages) context.Context {
	if len(pages.NotFound) == 0 && len(pages.InternalServerError) == 0 && len(pages.Custom) == 0 {
		return ctx
	}
	return context.WithValue(ctx, errorPagesContextKey, pages)
}

func errorPages(ctx context.Context) ErrorPages {
	pages, _ := ctx.Value(errorPagesContextKey).(ErrorPages)
	return pages
}

// WriteErrorPage writes a no-store generated HTML error page when available,
// otherwise it falls back to http.Error.
func WriteErrorPage(writer http.ResponseWriter, request *http.Request, status int, message string) {
	var pages ErrorPages
	if request != nil {
		pages = errorPages(request.Context())
	}
	payload := errorPagePayload(request, pages, status)
	writer.Header().Set("Cache-Control", "no-store")
	if len(payload) == 0 {
		http.Error(writer, message, status)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	if request != nil && request.Method == http.MethodHead {
		return
	}
	_, _ = writer.Write(payload)
}

func errorPagePayload(request *http.Request, pages ErrorPages, status int) []byte {
	switch status {
	case http.StatusNotFound:
		return pages.NotFound
	case http.StatusInternalServerError:
		if payload := endpointErrorPagePayload(request, pages); len(payload) > 0 {
			return payload
		}
		if payload := routeErrorPagePayload(request, pages); len(payload) > 0 {
			return payload
		}
		return pages.InternalServerError
	default:
		return nil
	}
}

func routeErrorPagePayload(request *http.Request, pages ErrorPages) []byte {
	if request == nil || len(pages.Custom) == 0 {
		return nil
	}
	route, ok := Route(request.Context())
	if !ok || route.ErrorPage == "" {
		return nil
	}
	return pages.Custom[cleanErrorPagePath(route.ErrorPage)]
}

func endpointErrorPagePayload(request *http.Request, pages ErrorPages) []byte {
	if request == nil || len(pages.Custom) == 0 {
		return nil
	}
	endpoint, ok := Endpoint(request.Context())
	if !ok || endpoint.ErrorPage == "" {
		return nil
	}
	return pages.Custom[cleanErrorPagePath(endpoint.ErrorPage)]
}

func cleanErrorPagePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\\?#") {
		return ""
	}
	for _, part := range strings.Split(strings.TrimPrefix(value, "/"), "/") {
		if part == ".." {
			return ""
		}
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(value, "/")), "/")
	if cleaned == "." || cleaned == "" || !strings.HasSuffix(strings.ToLower(cleaned), ".html") {
		return ""
	}
	return cleaned
}
