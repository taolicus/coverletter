package server

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/taolicus/coverletter/internal/players"
	"github.com/taolicus/coverletter/internal/rooms"
	"github.com/taolicus/coverletter/internal/storage"
)

func newTestServer(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	return newTestServerWithConfig(t, nil)
}

func newTestServerWithConfig(t *testing.T, configure func(*Config)) (http.Handler, *sql.DB) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "coverletter.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	config := Config{
		Players: players.New(db),
		Rooms:   rooms.New(db),
	}
	if configure != nil {
		configure(&config)
	}
	return New(config), db
}
