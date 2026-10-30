package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/taolicus/coverletter/internal/auth"
	"github.com/taolicus/coverletter/internal/players"
)

func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		player, ok := s.currentPlayer(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, player)
	case http.MethodPatch:
		s.updatePlayer(w, r)
	default:
		methodNotAllowed(w, "GET, PATCH")
	}
}

// currentPlayer returns the player for the request identity. A missing or
// unknown cookie creates a new player and sets a new cookie.
func (s *Server) currentPlayer(w http.ResponseWriter, r *http.Request) (players.Player, bool) {
	if token, ok := auth.Token(r); ok {
		if hash, ok := auth.HashToken(token); ok {
			player, found, err := s.players.ByTokenHash(r.Context(), hash)
			if err != nil {
				log.Printf("find player: %v", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "could not read player")
				return players.Player{}, false
			}
			if found {
				return player, true
			}
		}
	}

	token, err := auth.NewToken()
	if err != nil {
		log.Printf("create identity token: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create identity")
		return players.Player{}, false
	}
	hash, ok := auth.HashToken(token)
	if !ok {
		log.Printf("hash new identity token: invalid token")
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create identity")
		return players.Player{}, false
	}
	player, err := s.players.Create(r.Context(), hash)
	if err != nil {
		log.Printf("create player: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create player")
		return players.Player{}, false
	}
	auth.SetCookie(w, r, token)
	return player, true
}

// optionalPlayer returns the player for the request identity without creating
// one when the cookie is absent or unknown.
func (s *Server) optionalPlayer(r *http.Request) (players.Player, bool, error) {
	token, ok := auth.Token(r)
	if !ok {
		return players.Player{}, false, nil
	}
	hash, ok := auth.HashToken(token)
	if !ok {
		return players.Player{}, false, nil
	}
	return s.players.ByTokenHash(r.Context(), hash)
}

func (s *Server) updatePlayer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var request struct {
		DisplayName string `json:"displayName"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return
	}

	name, err := normalizeDisplayName(request.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_name", err.Error())
		return
	}

	player, ok := s.currentPlayer(w, r)
	if !ok {
		return
	}
	updated, err := s.players.UpdateDisplayName(r.Context(), player.ID, name)
	if err != nil {
		log.Printf("update player %d: %v", player.ID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not update player")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func normalizeDisplayName(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", errors.New("display name must be valid UTF-8")
	}
	name := strings.TrimSpace(raw)
	count := utf8.RuneCountInString(name)
	if count < 1 || count > 32 {
		return "", errors.New("display name must be 1-32 characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", errors.New("display name must not contain control characters")
		}
	}
	return name, nil
}
