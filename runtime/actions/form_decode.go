package actions

import (
	"net/http"

	"github.com/cssbruno/gowdk/runtime/form"
)

// DecodeForm parses request form data for generated typed action decoders.
func DecodeForm(request *http.Request) (form.Values, error) {
	if err := request.ParseForm(); err != nil {
		return nil, err
	}
	return form.FromURLValues(request.PostForm), nil
}

// DecodeMultipartForm parses request multipart data for generated typed action
// decoders.
func DecodeMultipartForm(request *http.Request) (form.Data, error) {
	if err := request.ParseMultipartForm(form.DefaultMultipartMemoryBytes); err != nil {
		return form.Data{}, err
	}
	return form.FromMultipartForm(request.MultipartForm), nil
}
