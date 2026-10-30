package rooms

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/taolicus/coverletter/internal/players"
	"github.com/taolicus/coverletter/internal/storage"
)

func TestCreateAndJoin(t *testing.T) {
	store, playerStore, _ := newTestStore(t)
	ctx := context.Background()
	first := createPlayer(t, playerStore, "first")
	second := createPlayer(t, playerStore, "second")

	room, err := store.Create(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if room.State != StateWaiting {
		t.Errorf("state = %q, want waiting", room.State)
	}
	if room.CreatorSeat != 1 {
		t.Errorf("creator seat = %d, want 1", room.CreatorSeat)
	}
	if len(room.Seats) != 1 || room.Seats[0].PlayerID != first.ID {
		t.Fatalf("seats = %+v, want only the creator", room.Seats)
	}

	room, err = store.Join(ctx, room.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if room.State != StateReady {
		t.Errorf("state = %q, want ready", room.State)
	}
	if len(room.Seats) != 2 || room.Seats[1].PlayerID != second.ID {
		t.Fatalf("seats = %+v, want the second player in seat 2", room.Seats)
	}

	// Joining again as a seated player is idempotent.
	room, err = store.Join(ctx, room.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(room.Seats) != 2 {
		t.Errorf("seats = %d, want 2 after an idempotent join", len(room.Seats))
	}
}

func TestJoinFullRoom(t *testing.T) {
	store, playerStore, _ := newTestStore(t)
	ctx := context.Background()
	seated := make([]players.Player, MaxPlayers+1)
	for i := range seated {
		seated[i] = createPlayer(t, playerStore, string(rune('a'+i)))
	}

	room, err := store.Create(ctx, seated[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, player := range seated[1:MaxPlayers] {
		if _, err := store.Join(ctx, room.ID, player.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Join(ctx, room.ID, seated[MaxPlayers].ID); !errors.Is(err, ErrFull) {
		t.Fatalf("join full room error = %v, want ErrFull", err)
	}
}

func TestJoinStartedRoom(t *testing.T) {
	store, playerStore, db := newTestStore(t)
	ctx := context.Background()
	first := createPlayer(t, playerStore, "first")
	second := createPlayer(t, playerStore, "second")
	third := createPlayer(t, playerStore, "third")

	room, err := store.Create(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Join(ctx, room.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE rooms SET state = ? WHERE id = ?`, StateInProgress, room.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Join(ctx, room.ID, third.ID); !errors.Is(err, ErrStarted) {
		t.Fatalf("join started room error = %v, want ErrStarted", err)
	}
	if _, err := store.Join(ctx, room.ID, second.ID); err != nil {
		t.Fatalf("seated player could not rejoin: %v", err)
	}
}

func TestOneIdentityPerRoom(t *testing.T) {
	store, playerStore, db := newTestStore(t)
	ctx := context.Background()
	first := createPlayer(t, playerStore, "first")
	room, err := store.Create(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO room_seats (room_id, seat, player_id) VALUES (?, 2, ?)`, room.ID, first.ID)
	if err == nil {
		t.Fatal("database allowed one identity to hold two seats")
	}
}

func TestRoomIDCollisionRetry(t *testing.T) {
	store, playerStore, _ := newTestStore(t)
	ctx := context.Background()
	player := createPlayer(t, playerStore, "first")

	store.newID = func() (string, error) { return "fixedid123", nil }
	first, err := store.Create(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != "fixedid123" {
		t.Fatalf("first id = %q", first.ID)
	}

	calls := 0
	store.newID = func() (string, error) {
		calls++
		if calls == 1 {
			return first.ID, nil
		}
		return "newid45678", nil
	}
	second, err := store.Create(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != "newid45678" {
		t.Errorf("second id = %q, want newid45678", second.ID)
	}
	if calls != 2 {
		t.Errorf("id generator calls = %d, want 2", calls)
	}
}

func TestRoomsSurviveRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coverletter.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	playerStore := players.New(db)
	ctx := context.Background()
	first := createPlayer(t, playerStore, "first")
	second := createPlayer(t, playerStore, "second")
	room, err := New(db).Create(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(db).Join(ctx, room.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	reloaded, err := New(db).Get(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.State != StateReady {
		t.Errorf("state = %q, want ready", reloaded.State)
	}
	if len(reloaded.Seats) != 2 {
		t.Errorf("seats = %d, want 2", len(reloaded.Seats))
	}
}

func newTestStore(t *testing.T) (*Store, *players.Store, *sql.DB) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "coverletter.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return New(db), players.New(db), db
}

func createPlayer(t *testing.T, store *players.Store, hash string) players.Player {
	t.Helper()
	player, err := store.Create(context.Background(), []byte(hash))
	if err != nil {
		t.Fatal(err)
	}
	return player
}
