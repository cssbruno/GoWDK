package app

import (
	"bufio"
	"net"
	"net/http"
)

const (
	responseWriterFlusher = 1 << iota
	responseWriterHijacker
	responseWriterPusher
)

func optionalResponseWriterMask(writer http.ResponseWriter) int {
	mask := 0
	if _, ok := writer.(http.Flusher); ok {
		mask |= responseWriterFlusher
	}
	if _, ok := writer.(http.Hijacker); ok {
		mask |= responseWriterHijacker
	}
	if _, ok := writer.(http.Pusher); ok {
		mask |= responseWriterPusher
	}
	return mask
}

func wrapBoundaryResponseWriter(writer *boundaryResponseWriter) http.ResponseWriter {
	switch optionalResponseWriterMask(writer.ResponseWriter) {
	case responseWriterFlusher:
		return boundaryFlusher{writer}
	case responseWriterHijacker:
		return boundaryHijacker{writer}
	case responseWriterPusher:
		return boundaryPusher{writer}
	case responseWriterFlusher | responseWriterHijacker:
		return boundaryFlusherHijacker{writer}
	case responseWriterFlusher | responseWriterPusher:
		return boundaryFlusherPusher{writer}
	case responseWriterHijacker | responseWriterPusher:
		return boundaryHijackerPusher{writer}
	case responseWriterFlusher | responseWriterHijacker | responseWriterPusher:
		return boundaryFlusherHijackerPusher{writer}
	default:
		return writer
	}
}

func (writer *boundaryResponseWriter) flush() {
	writer.wrote = true
	writer.ResponseWriter.(http.Flusher).Flush()
}

func (writer *boundaryResponseWriter) hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := writer.ResponseWriter.(http.Hijacker).Hijack()
	if err == nil {
		writer.wrote = true
	}
	return conn, rw, err
}

func (writer *boundaryResponseWriter) push(target string, options *http.PushOptions) error {
	return writer.ResponseWriter.(http.Pusher).Push(target, options)
}

type boundaryFlusher struct{ *boundaryResponseWriter }

func (writer boundaryFlusher) Flush() { writer.flush() }

type boundaryHijacker struct{ *boundaryResponseWriter }

func (writer boundaryHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.hijack()
}

type boundaryPusher struct{ *boundaryResponseWriter }

func (writer boundaryPusher) Push(target string, options *http.PushOptions) error {
	return writer.push(target, options)
}

type boundaryFlusherHijacker struct{ *boundaryResponseWriter }

func (writer boundaryFlusherHijacker) Flush() { writer.flush() }
func (writer boundaryFlusherHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.hijack()
}

type boundaryFlusherPusher struct{ *boundaryResponseWriter }

func (writer boundaryFlusherPusher) Flush() { writer.flush() }
func (writer boundaryFlusherPusher) Push(target string, options *http.PushOptions) error {
	return writer.push(target, options)
}

type boundaryHijackerPusher struct{ *boundaryResponseWriter }

func (writer boundaryHijackerPusher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.hijack()
}
func (writer boundaryHijackerPusher) Push(target string, options *http.PushOptions) error {
	return writer.push(target, options)
}

type boundaryFlusherHijackerPusher struct{ *boundaryResponseWriter }

func (writer boundaryFlusherHijackerPusher) Flush() { writer.flush() }
func (writer boundaryFlusherHijackerPusher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.hijack()
}
func (writer boundaryFlusherHijackerPusher) Push(target string, options *http.PushOptions) error {
	return writer.push(target, options)
}

func wrapTraceResponseWriter(writer *traceResponseWriter) http.ResponseWriter {
	switch optionalResponseWriterMask(writer.ResponseWriter) {
	case responseWriterFlusher:
		return traceFlusher{writer}
	case responseWriterHijacker:
		return traceHijacker{writer}
	case responseWriterPusher:
		return tracePusher{writer}
	case responseWriterFlusher | responseWriterHijacker:
		return traceFlusherHijacker{writer}
	case responseWriterFlusher | responseWriterPusher:
		return traceFlusherPusher{writer}
	case responseWriterHijacker | responseWriterPusher:
		return traceHijackerPusher{writer}
	case responseWriterFlusher | responseWriterHijacker | responseWriterPusher:
		return traceFlusherHijackerPusher{writer}
	default:
		return writer
	}
}

func (writer *traceResponseWriter) flush() {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	writer.ResponseWriter.(http.Flusher).Flush()
}

func (writer *traceResponseWriter) hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.ResponseWriter.(http.Hijacker).Hijack()
}

func (writer *traceResponseWriter) push(target string, options *http.PushOptions) error {
	return writer.ResponseWriter.(http.Pusher).Push(target, options)
}

type traceFlusher struct{ *traceResponseWriter }

func (writer traceFlusher) Flush() { writer.flush() }

type traceHijacker struct{ *traceResponseWriter }

func (writer traceHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.hijack()
}

type tracePusher struct{ *traceResponseWriter }

func (writer tracePusher) Push(target string, options *http.PushOptions) error {
	return writer.push(target, options)
}

type traceFlusherHijacker struct{ *traceResponseWriter }

func (writer traceFlusherHijacker) Flush() { writer.flush() }
func (writer traceFlusherHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.hijack()
}

type traceFlusherPusher struct{ *traceResponseWriter }

func (writer traceFlusherPusher) Flush() { writer.flush() }
func (writer traceFlusherPusher) Push(target string, options *http.PushOptions) error {
	return writer.push(target, options)
}

type traceHijackerPusher struct{ *traceResponseWriter }

func (writer traceHijackerPusher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.hijack()
}
func (writer traceHijackerPusher) Push(target string, options *http.PushOptions) error {
	return writer.push(target, options)
}

type traceFlusherHijackerPusher struct{ *traceResponseWriter }

func (writer traceFlusherHijackerPusher) Flush() { writer.flush() }
func (writer traceFlusherHijackerPusher) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return writer.hijack()
}
func (writer traceFlusherHijackerPusher) Push(target string, options *http.PushOptions) error {
	return writer.push(target, options)
}
