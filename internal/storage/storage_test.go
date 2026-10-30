package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseAndMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "coverletter.db")

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("database mode = %o, want 600", got)
	}

	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}

	if _, err := db.Exec(`INSERT INTO players (token_hash) VALUES (?)`, []byte("hash")); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// Opening the same file again must not re-run migrations or lose rows.
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var players int
	if err := db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&players); err != nil {
		t.Fatal(err)
	}
	if players != 1 {
		t.Errorf("player rows = %d, want 1", players)
	}

	var migrations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 3 {
		t.Errorf("migration rows = %d, want 3", migrations)
	}
}
