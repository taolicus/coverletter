// Package server wires the HTTP routes to the application.
package server

import (
	"io/fs"
	"net/http"

	"github.com/taolicus/coverletter/web"
)

// New returns the application handler.
func New() http.Handler {
	index, err := fs.ReadFile(web.Files, "index.html")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
	return mux
}
