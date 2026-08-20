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

const (
	framingHeaderTooLarge           = "lsp_header_too_large"
	framingTooManyHeaders           = "lsp_too_many_headers"
	framingMissingContentLength     = "lsp_missing_content_length"
	framingConflictingContentLength = "lsp_conflicting_content_length"
	framingInvalidContentLength     = "lsp_invalid_content_length"
	framingMessageTooLarge          = "lsp_message_too_large"
	framingTruncatedMessage         = "lsp_truncated_message"
)

// FramingError classifies an LSP transport failure without retaining or
// exposing the raw header or message body. Framing failures are fatal because
// the next byte boundary cannot be trusted without consuming attacker-sized
// input.
type FramingError struct {
	Code    string
	Fatal   bool
	Message string
	Limit   int64
	Actual  int64
}

func (err *FramingError) Error() string {
	if err == nil {
		return ""
	}
	if err.Message != "" {
		return err.Code + ": " + err.Message
	}
	return err.Code
}

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
	_, _ = fmt.Fprintf(server.log, format+"\n", args...)
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	return readMessageWithLimits(reader, DefaultLimits())
}

func readMessageWithLimits(reader *bufio.Reader, limits Limits) ([]byte, error) {
	limits = limits.normalized()
	var contentLength int64 = -1
	var headerBytes int64
	headerCount := 0
	for {
		line, bytesRead, err := readBoundedHeaderLine(reader, limits.MaxHeaderLineBytes)
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		headerBytes += bytesRead
		if headerBytes > limits.MaxHeaderBytes {
			return nil, &FramingError{Code: framingHeaderTooLarge, Fatal: true, Message: "aggregate headers exceed configured limit", Limit: limits.MaxHeaderBytes, Actual: headerBytes}
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		headerCount++
		if headerCount > limits.MaxHeaderCount {
			return nil, &FramingError{Code: framingTooManyHeaders, Fatal: true, Message: "header count exceeds configured limit", Limit: int64(limits.MaxHeaderCount), Actual: int64(headerCount)}
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, &FramingError{Code: framingHeaderTooLarge, Fatal: true, Message: "malformed header line"}
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := parseContentLength(value)
			if err != nil {
				return nil, err
			}
			if contentLength >= 0 && contentLength != parsed {
				return nil, &FramingError{Code: framingConflictingContentLength, Fatal: true, Message: "conflicting Content-Length headers"}
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, &FramingError{Code: framingMissingContentLength, Fatal: true, Message: "missing Content-Length header"}
	}
	if contentLength > limits.MaxMessageBytes {
		return nil, &FramingError{Code: framingMessageTooLarge, Fatal: true, Message: "declared message body exceeds configured limit", Limit: limits.MaxMessageBytes, Actual: contentLength}
	}
	body := make([]byte, int(contentLength))
	read, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, &FramingError{Code: framingTruncatedMessage, Fatal: true, Message: "message body ended before Content-Length bytes were read", Actual: int64(read)}
	}
	return body, nil
}

func readBoundedHeaderLine(reader *bufio.Reader, limit int) (string, int64, error) {
	var line []byte
	var total int64
	for {
		chunk, err := reader.ReadSlice('\n')
		total += int64(len(chunk))
		if total > int64(limit) {
			return "", total, &FramingError{Code: framingHeaderTooLarge, Fatal: true, Message: "header line exceeds configured limit", Limit: int64(limit), Actual: total}
		}
		line = append(line, chunk...)
		switch {
		case err == nil:
			return string(line), total, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(line) > 0:
			return string(line), total, io.ErrUnexpectedEOF
		default:
			return string(line), total, err
		}
	}
}

func parseContentLength(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, &FramingError{Code: framingInvalidContentLength, Fatal: true, Message: "Content-Length is empty"}
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, &FramingError{Code: framingInvalidContentLength, Fatal: true, Message: "Content-Length must be an unsigned decimal integer"}
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 63)
	if err != nil {
		return 0, &FramingError{Code: framingInvalidContentLength, Fatal: true, Message: "Content-Length overflows the supported range"}
	}
	return int64(parsed), nil
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
