package server

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/taolicus/coverletter/internal/players"
	"github.com/taolicus/coverletter/internal/storage"
)

func newTestServer(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "coverletter.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(players.New(db)), db
}
