// Package server wires the HTTP routes to the application.
package server

import (
	"io/fs"
	"net/http"

	"github.com/taolicus/coverletter/internal/players"
	"github.com/taolicus/coverletter/web"
)

// Server holds the dependencies used by the HTTP handlers.
type Server struct {
	players *players.Store
}

// New returns the application handler.
func New(playerStore *players.Store) http.Handler {
	index, err := fs.ReadFile(web.Files, "index.html")
	if err != nil {
		panic(err)
	}

	s := &Server{players: playerStore}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/player", s.handlePlayer)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
	return mux
}
