package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
)

const (
	// MaxSourceLineBytes is the maximum line size accepted in ordinary .gwdk
	// source files.
	MaxSourceLineBytes = 1 << 20

	// MaxAuditLineBytes is the maximum line size accepted in *.audit.gwdk
	// policy files.
	MaxAuditLineBytes = MaxSourceLineBytes

	lineScannerInitialBytes = 64 << 10
)

func newSourceLineScanner(src []byte, maxLineBytes int) *bufio.Scanner {
	scanner := bufio.NewScanner(bytes.NewReader(src))
	// Leave room for a trailing CRLF. The explicit token-length check keeps
	// the documented limit independent from Scanner's delimiter buffering.
	scanner.Buffer(make([]byte, lineScannerInitialBytes), maxLineBytes+2)
	return scanner
}

func lineTooLong(scanner *bufio.Scanner, maxLineBytes int) bool {
	return len(scanner.Bytes()) > maxLineBytes || errors.Is(scanner.Err(), bufio.ErrTooLong)
}

func sourceLineTooLongError(lineNumber, maxLineBytes int, inputKind string) error {
	return lineDiagnosticError(
		DiagnosticSourceLineTooLong,
		lineNumber,
		"",
		"%s line exceeds the %d-byte limit; split the content across lines or move it to an external file",
		inputKind,
		maxLineBytes,
	)
}

func addScannerError(addError func(error), scanner *bufio.Scanner, lineNumber, maxLineBytes int, inputKind string) bool {
	err := scanner.Err()
	if err == nil {
		return false
	}
	if lineTooLong(scanner, maxLineBytes) {
		addError(sourceLineTooLongError(lineNumber, maxLineBytes, inputKind))
		return true
	}
	addError(fmt.Errorf("line %d: scan %s: %w", lineNumber, inputKind, err))
	return true
}
