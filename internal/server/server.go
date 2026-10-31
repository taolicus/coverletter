// Package server wires the HTTP routes to the application.
package server

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/taolicus/coverletter/internal/matches"
	"github.com/taolicus/coverletter/internal/players"
	"github.com/taolicus/coverletter/internal/rooms"
	"github.com/taolicus/coverletter/web"
)

const (
	defaultCreateLimit = 10
	defaultJoinLimit   = 30
	defaultRateWindow  = time.Minute
)

// Config holds the server dependencies and optional overrides.
type Config struct {
	Players *players.Store
	Rooms   *rooms.Store
	Matches *matches.Store
	BaseURL string
	// CreateLimit and JoinLimit are requests per RateWindow per client IP.
	// Zero uses the default.
	CreateLimit int
	JoinLimit   int
	RateWindow  time.Duration
}

// Server holds the dependencies used by the HTTP handlers.
type Server struct {
	players       *players.Store
	rooms         *rooms.Store
	matches       *matches.Store
	baseURL       string
	createLimiter *rateLimiter
	joinLimiter   *rateLimiter
	rateWindow    time.Duration
}

// New returns the application handler.
func New(config Config) http.Handler {
	if config.CreateLimit == 0 {
		config.CreateLimit = defaultCreateLimit
	}
	if config.JoinLimit == 0 {
		config.JoinLimit = defaultJoinLimit
	}
	if config.RateWindow == 0 {
		config.RateWindow = defaultRateWindow
	}

	readAsset := func(name string) []byte {
		data, err := fs.ReadFile(web.Files, name)
		if err != nil {
			panic(err)
		}
		return data
	}

	index := readAsset("index.html")
	assets := []struct {
		path        string
		contentType string
		data        []byte
	}{
		{path: "/style.css", contentType: "text/css; charset=utf-8", data: readAsset("style.css")},
		{path: "/app.js", contentType: "text/javascript; charset=utf-8", data: readAsset("app.js")},
	}

	s := &Server{
		players:       config.Players,
		rooms:         config.Rooms,
		matches:       config.Matches,
		baseURL:       config.BaseURL,
		createLimiter: newRateLimiter(config.CreateLimit, config.RateWindow),
		joinLimiter:   newRateLimiter(config.JoinLimit, config.RateWindow),
		rateWindow:    config.RateWindow,
	}
	mux := http.NewServeMux()
	for _, asset := range assets {
		asset := asset
		mux.HandleFunc(asset.path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				methodNotAllowed(w, "GET, HEAD")
				return
			}
			w.Header().Set("Content-Type", asset.contentType)
			w.Header().Set("Cache-Control", "no-cache")
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = w.Write(asset.data)
		})
	}
	mux.HandleFunc("/api/player", s.handlePlayer)
	mux.HandleFunc("/api/rooms", s.handleRooms)
	mux.HandleFunc("/api/rooms/", s.handleRoom)
	mux.HandleFunc("/api/", s.handleAPINotFound)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
	return mux
}
