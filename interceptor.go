package main

import (
	"net/http"
)

type interceptRW struct {
	http.ResponseWriter
	status int
	size   int
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
