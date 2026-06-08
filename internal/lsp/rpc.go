package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"runtime"
	"strconv"
	"strings"
)

func publishDiagnostics(uri string, diagnostics []diagnostic) []byte {
	if diagnostics == nil {
		diagnostics = []diagnostic{}
	}
	return notification("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func singleMessage(message []byte) [][]byte {
	if len(message) == 0 {
		return nil
	}
	return [][]byte{message}
}

func response(id *json.RawMessage, result any) []byte {
	payload, err := json.Marshal(rpcResultResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	})
	if err != nil {
		return errorResponse(id, internalError, err.Error())
	}
	return payload
}

func errorResponse(id *json.RawMessage, code int, message string) []byte {
	payload, err := json.Marshal(rpcErrorResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return nil
	}
	return payload
}

func notification(method string, params any) []byte {
	payload, err := json.Marshal(rpcNotification{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil
	}
	return payload
}

func pathFromURI(rawURI string) string {
	parsed, err := url.Parse(rawURI)
	if err != nil || parsed.Scheme != "file" {
		return rawURI
	}
	if runtime.GOOS == "windows" {
		return strings.TrimPrefix(parsed.Path, "/")
	}
	return parsed.Path
}

func (server *Server) logf(format string, args ...any) {
	if server.log == nil {
		return
	}
	fmt.Fprintf(server.log, format+"\n", args...)
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q", value)
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	return body, err
}

func writeMessage(writer io.Writer, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}
