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
