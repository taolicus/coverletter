// Package rooms stores game rooms and their seats.
package rooms

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

const (
	// MinPlayers is the number of seated players needed for a room to be ready.
	MinPlayers = 2
	// MaxPlayers is the maximum number of seats in a room.
	MaxPlayers = 4

	idLength   = 10
	idAlphabet = "23456789abcdefghjkmnpqrstuvwxyz"
	maxIDTries = 10
)

// State is a room's lifecycle state.
type State string

const (
	StateWaiting    State = "waiting"
	StateReady      State = "ready"
	StateInProgress State = "in_progress"
	StateCompleted  State = "completed"
)

var (
	// ErrNotFound means the room does not exist.
	ErrNotFound = errors.New("room not found")
	// ErrFull means every seat is occupied.
	ErrFull = errors.New("room is full")
	// ErrStarted means the room has already started.
	ErrStarted = errors.New("room has started")
	// ErrIDExhausted means no unused room id could be generated.
	ErrIDExhausted = errors.New("could not allocate a room id")
)

// Room is a room with its occupied seats in seat order.
type Room struct {
	ID              string
	State           State
	CreatorPlayerID int64
	CreatorSeat     int
	Seats           []Seat
}

// Seat is one occupied seat.
type Seat struct {
	Number      int
	PlayerID    int64
	DisplayName string
}

// Store reads and writes rooms.
type Store struct {
	db    *sql.DB
	mu    sync.Mutex
	newID func() (string, error)
}

// New returns a room store backed by db.
func New(db *sql.DB) *Store {
	return &Store{db: db, newID: newRoomID}
}

// Create makes a room and seats the creator in seat 1.
func (s *Store) Create(ctx context.Context, creatorID int64) (Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for attempt := 0; attempt < maxIDTries; attempt++ {
		id, err := s.newID()
		if err != nil {
			return Room{}, fmt.Errorf("generate room id: %w", err)
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return Room{}, fmt.Errorf("begin create room: %w", err)
		}
		result, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO rooms (id, state, creator_player_id) VALUES (?, ?, ?)`,
			id, StateWaiting, creatorID,
		)
		if err != nil {
			_ = tx.Rollback()
			return Room{}, fmt.Errorf("create room: %w", err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			return Room{}, fmt.Errorf("read create room result: %w", err)
		}
		if inserted == 0 {
			// The generated id is already in use.
			_ = tx.Rollback()
			continue
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO room_seats (room_id, seat, player_id) VALUES (?, 1, ?)`,
			id, creatorID,
		); err != nil {
			_ = tx.Rollback()
			return Room{}, fmt.Errorf("seat creator: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return Room{}, fmt.Errorf("commit create room: %w", err)
		}
		return s.get(ctx, id)
	}
	return Room{}, ErrIDExhausted
}

// Get returns a room and its seats.
func (s *Store) Get(ctx context.Context, id string) (Room, error) {
	return s.get(ctx, id)
}

func (s *Store) get(ctx context.Context, id string) (Room, error) {
	return loadRoom(ctx, s.db, id)
}

// Join seats a player in the lowest free seat. Joining again as a seated player
// is idempotent.
func (s *Store) Join(ctx context.Context, roomID string, playerID int64) (Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Room{}, fmt.Errorf("begin join room: %w", err)
	}

	room, err := loadRoom(ctx, tx, roomID)
	if err != nil {
		_ = tx.Rollback()
		return Room{}, err
	}
	for _, seat := range room.Seats {
		if seat.PlayerID == playerID {
			_ = tx.Rollback()
			return room, nil
		}
	}
	if room.State != StateWaiting && room.State != StateReady {
		_ = tx.Rollback()
		return Room{}, ErrStarted
	}
	if len(room.Seats) >= MaxPlayers {
		_ = tx.Rollback()
		return Room{}, ErrFull
	}

	occupied := make(map[int]bool, len(room.Seats))
	for _, seat := range room.Seats {
		occupied[seat.Number] = true
	}
	next := 0
	for seat := 1; seat <= MaxPlayers; seat++ {
		if !occupied[seat] {
			next = seat
			break
		}
	}
	if next == 0 {
		_ = tx.Rollback()
		return Room{}, ErrFull
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO room_seats (room_id, seat, player_id) VALUES (?, ?, ?)`,
		roomID, next, playerID,
	); err != nil {
		_ = tx.Rollback()
		return Room{}, fmt.Errorf("join room: %w", err)
	}

	state := StateWaiting
	if len(room.Seats)+1 >= MinPlayers {
		state = StateReady
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE rooms SET state = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?`,
		state, roomID,
	); err != nil {
		_ = tx.Rollback()
		return Room{}, fmt.Errorf("update room state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Room{}, fmt.Errorf("commit join room: %w", err)
	}
	return s.get(ctx, roomID)
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadRoom(ctx context.Context, q queryer, id string) (Room, error) {
	var room Room
	var state string
	err := q.QueryRowContext(ctx,
		`SELECT id, state, creator_player_id FROM rooms WHERE id = ?`, id,
	).Scan(&room.ID, &state, &room.CreatorPlayerID)
	if errors.Is(err, sql.ErrNoRows) {
		return Room{}, ErrNotFound
	}
	if err != nil {
		return Room{}, fmt.Errorf("find room: %w", err)
	}
	room.State = State(state)

	rows, err := q.QueryContext(ctx,
		`SELECT s.seat, s.player_id, p.display_name
		 FROM room_seats s
		 JOIN players p ON p.id = s.player_id
		 WHERE s.room_id = ?
		 ORDER BY s.seat`, id,
	)
	if err != nil {
		return Room{}, fmt.Errorf("find room seats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var seat Seat
		if err := rows.Scan(&seat.Number, &seat.PlayerID, &seat.DisplayName); err != nil {
			return Room{}, fmt.Errorf("read room seat: %w", err)
		}
		if seat.PlayerID == room.CreatorPlayerID {
			room.CreatorSeat = seat.Number
		}
		room.Seats = append(room.Seats, seat)
	}
	if err := rows.Err(); err != nil {
		return Room{}, fmt.Errorf("read room seats: %w", err)
	}
	return room, nil
}

func newRoomID() (string, error) {
	buf := make([]byte, idLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = idAlphabet[int(b)%len(idAlphabet)]
	}
	return string(buf), nil
}
