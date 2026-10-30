// Package players stores pseudonymous players and their display names.
package players

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Player is the public representation of a player. It never contains the
// identity token or its hash.
type Player struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
}

// Store reads and writes players.
type Store struct {
	db *sql.DB
}

// New returns a player store backed by db.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// ByTokenHash returns the player with the given token hash. The boolean is
// false when no player has that hash.
func (s *Store) ByTokenHash(ctx context.Context, hash []byte) (Player, bool, error) {
	var player Player
	err := s.db.QueryRowContext(ctx,
		`SELECT id, display_name FROM players WHERE token_hash = ?`, hash,
	).Scan(&player.ID, &player.DisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return Player{}, false, nil
	}
	if err != nil {
		return Player{}, false, fmt.Errorf("find player: %w", err)
	}
	return player, true, nil
}

// Create inserts a player with the given token hash and an empty display name.
func (s *Store) Create(ctx context.Context, hash []byte) (Player, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO players (token_hash) VALUES (?)`, hash,
	)
	if err != nil {
		return Player{}, fmt.Errorf("create player: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Player{}, fmt.Errorf("read player id: %w", err)
	}
	return Player{ID: id}, nil
}

// UpdateDisplayName sets a player's display name and returns the updated
// player.
func (s *Store) UpdateDisplayName(ctx context.Context, id int64, name string) (Player, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE players
		 SET display_name = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = ?`, name, id,
	)
	if err != nil {
		return Player{}, fmt.Errorf("update display name: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Player{}, fmt.Errorf("read update result: %w", err)
	}
	if changed == 0 {
		return Player{}, fmt.Errorf("update display name: player %d not found", id)
	}
	return Player{ID: id, DisplayName: name}, nil
}
