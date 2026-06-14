package buildgen

import (
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/view"
)

func pageActionInputFields(ir gwdkir.Program) map[string]map[string][]view.ActionInputField {
	out := map[string]map[string][]view.ActionInputField{}
	for _, endpoint := range ir.Endpoints {
		if endpoint.Kind != gwdkir.EndpointAction || endpoint.PageID == "" || len(endpoint.Binding.InputFields) == 0 {
			continue
		}
		if out[endpoint.PageID] == nil {
			out[endpoint.PageID] = map[string][]view.ActionInputField{}
		}
		fields := make([]view.ActionInputField, 0, len(endpoint.Binding.InputFields))
		for _, field := range endpoint.Binding.InputFields {
			fields = append(fields, view.ActionInputField{
				FormName: field.FormName,
				Type:     field.Type,
			})
		}
		out[endpoint.PageID][endpoint.Symbol] = fields
	}
	return out
}
