//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/cssbruno/gowdk/playground"
)

type compileRequest struct {
	Files     map[string]string `json:"files"`
	OutputDir string            `json:"outputDir,omitempty"`
}

func main() {
	js.Global().Set("gowdkCompile", js.FuncOf(compile))
	select {}
}

func compile(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return encode(playground.Result{Diagnostics: []playground.Diagnostic{{
			Severity: "error",
			Message:  "gowdkCompile expects a JSON request string",
		}}})
	}

	var request compileRequest
	if err := json.Unmarshal([]byte(args[0].String()), &request); err != nil {
		return encode(playground.Result{Diagnostics: []playground.Diagnostic{{
			Severity: "error",
			Message:  "decode compile request: " + err.Error(),
		}}})
	}

	return encode(playground.Compile(playground.Project{
		Files:     request.Files,
		OutputDir: request.OutputDir,
	}))
}

func encode(result playground.Result) string {
	payload, err := json.Marshal(result)
	if err != nil {
		fallback, _ := json.Marshal(playground.Result{Diagnostics: []playground.Diagnostic{{
			Severity: "error",
			Message:  "encode compile result: " + err.Error(),
		}}})
		return string(fallback)
	}
	return string(payload)
}
