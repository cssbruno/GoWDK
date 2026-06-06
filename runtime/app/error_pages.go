package app

import (
	"context"
	"io/fs"
	"net/http"
)

// ErrorPages stores optional generated HTML error documents.
type ErrorPages struct {
	NotFound            []byte
	InternalServerError []byte
}

// LoadErrorPages reads optional 404.html and 500.html files from generated
// output.
func LoadErrorPages(root fs.FS) ErrorPages {
	return ErrorPages{
		NotFound:            readOptionalErrorPage(root, "404.html"),
		InternalServerError: readOptionalErrorPage(root, "500.html"),
	}
}

func readOptionalErrorPage(root fs.FS, name string) []byte {
	payload, err := fs.ReadFile(root, name)
	if err != nil {
		return nil
	}
	return append([]byte(nil), payload...)
}

func withErrorPages(ctx context.Context, pages ErrorPages) context.Context {
	if len(pages.NotFound) == 0 && len(pages.InternalServerError) == 0 {
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
	payload := errorPagePayload(errorPages(request.Context()), status)
	writer.Header().Set("Cache-Control", "no-store")
	if len(payload) == 0 {
		http.Error(writer, message, status)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	if request.Method == http.MethodHead {
		return
	}
	_, _ = writer.Write(payload)
}

func errorPagePayload(pages ErrorPages, status int) []byte {
	switch status {
	case http.StatusNotFound:
		return pages.NotFound
	case http.StatusInternalServerError:
		return pages.InternalServerError
	default:
		return nil
	}
}
