package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

type interceptRW struct {
	http.ResponseWriter
	status int
	size   int
}

var (
	_ http.Flusher  = (*interceptRW)(nil)
	_ http.Hijacker = (*interceptRW)(nil)
)

func (w *interceptRW) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *interceptRW) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *interceptRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("connection does not support hijacking")
}

func (w *interceptRW) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *interceptRW) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	w.size = len(b)
	return w.ResponseWriter.Write(b)
}
