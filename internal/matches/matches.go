// Package matches persists the game state attached to each room.
package matches

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/taolicus/coverletter/internal/game"
	"github.com/taolicus/coverletter/internal/rooms"
)

const formatVersion = 1

var (
	// ErrNotReady means the room does not have enough seated players to start.
	ErrNotReady = errors.New("room is not ready")
	// ErrNotCreator means the player who asked to start is not the creator.
	ErrNotCreator = errors.New("only the room creator can start")
	// ErrNotSeated means the player does not occupy a seat in the room.
	ErrNotSeated = errors.New("player is not seated in this room")
	// ErrNotStarted means the room has no game yet.
	ErrNotStarted = errors.New("game has not started")
	// ErrNotInProgress means the game is not accepting moves.
	ErrNotInProgress = errors.New("game is not in progress")
)

// Store reads and writes the game attached to a room.
type Store struct {
	db *sql.DB
}

// New returns a match store backed by db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// Start creates the room's match from the seated players, deals, and marks the
// room in progress. It is idempotent in the sense that a second start never
// creates another match: it returns rooms.ErrStarted instead.
func (s *Store) Start(ctx context.Context, roomID string, playerID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin start game: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var roomState string
	var creatorID int64
	err = tx.QueryRowContext(ctx,
		`SELECT state, creator_player_id FROM rooms WHERE id = ?`, roomID,
	).Scan(&roomState, &creatorID)
	if errors.Is(err, sql.ErrNoRows) {
		return rooms.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("find room to start: %w", err)
	}
	if playerID != creatorID {
		return ErrNotCreator
	}
	switch rooms.State(roomState) {
	case rooms.StateReady:
	case rooms.StateInProgress, rooms.StateCompleted:
		return rooms.ErrStarted
	default:
		return ErrNotReady
	}

	names, err := seatedNames(ctx, tx, roomID)
	if err != nil {
		return err
	}
	if len(names) < rooms.MinPlayers {
		return ErrNotReady
	}

	seed, err := newSeed()
	if err != nil {
		return fmt.Errorf("create game seed: %w", err)
	}
	state, err := game.NewMatch(seed, names)
	if err != nil {
		return fmt.Errorf("create match: %w", err)
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode match: %w", err)
	}

	result, err := tx.ExecContext(ctx,
		`INSERT INTO games (room_id, format_version, state_version, state)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (room_id) DO NOTHING`,
		roomID, formatVersion, state.Version, encoded,
	)
	if err != nil {
		return fmt.Errorf("store match: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read store match result: %w", err)
	}
	if inserted == 0 {
		return rooms.ErrStarted
	}

	result, err = tx.ExecContext(ctx,
		`UPDATE rooms SET state = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ? AND state = ?`,
		rooms.StateInProgress, roomID, rooms.StateReady,
	)
	if err != nil {
		return fmt.Errorf("mark room in progress: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read room update result: %w", err)
	}
	if changed == 0 {
		return ErrNotReady
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit start game: %w", err)
	}
	return nil
}

// View returns the persisted match and the viewer's seat. Seat zero means the
// viewer is not seated.
func (s *Store) View(ctx context.Context, roomID string, playerID int64) (game.State, int, error) {
	_, state, _, started, err := loadRoomGame(ctx, s.db, roomID)
	if err != nil {
		return game.State{}, 0, err
	}
	if !started {
		return game.State{}, 0, ErrNotStarted
	}
	seat, err := loadSeat(ctx, s.db, roomID, playerID)
	if err != nil {
		return game.State{}, 0, err
	}
	return state, seat, nil
}

// Move applies a move for a seated player and persists the new state. It returns
// the state after the move and the player's seat.
func (s *Store) Move(ctx context.Context, roomID string, playerID int64, move game.Move) (game.State, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return game.State{}, 0, fmt.Errorf("begin move: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	roomState, state, stateVersion, started, err := loadRoomGame(ctx, tx, roomID)
	if err != nil {
		return game.State{}, 0, err
	}
	if !started {
		return game.State{}, 0, ErrNotStarted
	}
	if roomState == rooms.StateCompleted {
		return game.State{}, 0, game.ErrMatchOver
	}
	if roomState != rooms.StateInProgress {
		return game.State{}, 0, ErrNotInProgress
	}

	seat, err := loadSeat(ctx, tx, roomID, playerID)
	if err != nil {
		return game.State{}, 0, err
	}
	if seat == 0 {
		return game.State{}, 0, ErrNotSeated
	}

	move.Seat = seat
	next, err := state.Apply(move)
	if err != nil {
		return game.State{}, seat, err
	}
	encoded, err := json.Marshal(next)
	if err != nil {
		return game.State{}, seat, fmt.Errorf("encode match: %w", err)
	}

	// The version condition makes the update a compare-and-swap even if two
	// requests read the same state.
	result, err := tx.ExecContext(ctx,
		`UPDATE games
		 SET state = ?, state_version = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE room_id = ? AND state_version = ?`,
		encoded, next.Version, roomID, stateVersion,
	)
	if err != nil {
		return game.State{}, seat, fmt.Errorf("store match: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return game.State{}, seat, fmt.Errorf("read store match result: %w", err)
	}
	if changed == 0 {
		return game.State{}, seat, game.ErrStaleVersion
	}

	if next.Phase == game.PhaseMatchOver {
		if _, err := tx.ExecContext(ctx,
			`UPDATE rooms SET state = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
			 WHERE id = ? AND state = ?`,
			rooms.StateCompleted, roomID, rooms.StateInProgress,
		); err != nil {
			return game.State{}, seat, fmt.Errorf("mark room completed: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return game.State{}, seat, fmt.Errorf("commit move: %w", err)
	}
	return next, seat, nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// loadRoomGame returns the room state and its persisted match, if one exists.
func loadRoomGame(ctx context.Context, q queryer, roomID string) (rooms.State, game.State, int, bool, error) {
	var roomState string
	var format sql.NullInt64
	var stateVersion sql.NullInt64
	var encoded sql.NullString
	err := q.QueryRowContext(ctx,
		`SELECT r.state, g.format_version, g.state_version, g.state
		 FROM rooms r
		 LEFT JOIN games g ON g.room_id = r.id
		 WHERE r.id = ?`, roomID,
	).Scan(&roomState, &format, &stateVersion, &encoded)
	if errors.Is(err, sql.ErrNoRows) {
		return "", game.State{}, 0, false, rooms.ErrNotFound
	}
	if err != nil {
		return "", game.State{}, 0, false, fmt.Errorf("find room game: %w", err)
	}
	if !encoded.Valid {
		return rooms.State(roomState), game.State{}, 0, false, nil
	}
	if format.Int64 != formatVersion {
		return "", game.State{}, 0, false, fmt.Errorf("unsupported game format version %d", format.Int64)
	}

	var state game.State
	if err := json.Unmarshal([]byte(encoded.String), &state); err != nil {
		return "", game.State{}, 0, false, fmt.Errorf("decode match: %w", err)
	}
	if int64(state.Version) != stateVersion.Int64 {
		return "", game.State{}, 0, false, fmt.Errorf(
			"stored match version %d does not match column %d", state.Version, stateVersion.Int64,
		)
	}
	return rooms.State(roomState), state, int(stateVersion.Int64), true, nil
}

func loadSeat(ctx context.Context, q queryer, roomID string, playerID int64) (int, error) {
	if playerID == 0 {
		return 0, nil
	}
	var seat int
	err := q.QueryRowContext(ctx,
		`SELECT seat FROM room_seats WHERE room_id = ? AND player_id = ?`, roomID, playerID,
	).Scan(&seat)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find player seat: %w", err)
	}
	return seat, nil
}

func seatedNames(ctx context.Context, q queryer, roomID string) ([]string, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT s.seat, p.display_name
		 FROM room_seats s
		 JOIN players p ON p.id = s.player_id
		 WHERE s.room_id = ?
		 ORDER BY s.seat`, roomID,
	)
	if err != nil {
		return nil, fmt.Errorf("find seated players: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var seat int
		var name string
		if err := rows.Scan(&seat, &name); err != nil {
			return nil, fmt.Errorf("read seated player: %w", err)
		}
		if seat != len(names)+1 {
			return nil, fmt.Errorf("room seats are not contiguous at seat %d", seat)
		}
		if name == "" {
			name = fmt.Sprintf("Seat %d", seat)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read seated players: %w", err)
	}
	if len(names) < rooms.MinPlayers || len(names) > rooms.MaxPlayers {
		return nil, ErrNotReady
	}
	return names, nil
}

func newSeed() (int64, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf[:])), nil
}
