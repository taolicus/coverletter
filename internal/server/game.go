package server

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/taolicus/coverletter/internal/game"
	"github.com/taolicus/coverletter/internal/matches"
	"github.com/taolicus/coverletter/internal/rooms"
)

// gameResponse combines the public room, the viewer's game view, and whether
// that viewer may submit a move now. A seated viewer gets a PlayerView; everyone
// else gets a PublicView without hands or snapshots.
type gameResponse struct {
	Room    roomResponse `json:"room"`
	Game    any          `json:"game"`
	CanMove bool         `json:"canMove"`
}

type moveRequest struct {
	Version int       `json:"version"`
	Card    game.Card `json:"card"`
	Target  int       `json:"target"`
	Guess   game.Card `json:"guess"`
}

func (s *Server) startGame(w http.ResponseWriter, r *http.Request, id string) {
	player, ok := s.currentPlayer(w, r)
	if !ok {
		return
	}
	err := s.matches.Start(r.Context(), id, player.ID)
	switch {
	case errors.Is(err, rooms.ErrNotFound):
		writeError(w, http.StatusNotFound, "room_not_found", "room not found")
		return
	case errors.Is(err, matches.ErrNotCreator):
		writeError(w, http.StatusForbidden, "not_creator", "only the room creator can start")
		return
	case errors.Is(err, matches.ErrNotReady):
		writeError(w, http.StatusConflict, "room_not_ready", "room does not have enough players")
		return
	case errors.Is(err, rooms.ErrStarted):
		writeError(w, http.StatusConflict, "room_started", "room has already started")
		return
	case err != nil:
		log.Printf("start game in room %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not start game")
		return
	}

	s.writeRoom(w, r, id, player.ID, http.StatusOK)
}

func (s *Server) getGame(w http.ResponseWriter, r *http.Request, id string) {
	player, found, err := s.optionalPlayer(r)
	if err != nil {
		log.Printf("find game viewer: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read player")
		return
	}
	var playerID int64
	if found {
		playerID = player.ID
	}

	state, seat, err := s.matches.View(r.Context(), id, playerID)
	switch {
	case errors.Is(err, rooms.ErrNotFound):
		writeError(w, http.StatusNotFound, "room_not_found", "room not found")
		return
	case errors.Is(err, matches.ErrNotStarted):
		writeError(w, http.StatusConflict, "game_not_started", "game has not started")
		return
	case err != nil:
		log.Printf("get game in room %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read game")
		return
	}

	room, err := s.rooms.Get(r.Context(), id)
	if err != nil {
		log.Printf("get room %s for game: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read room")
		return
	}
	writeJSON(w, http.StatusOK, s.gameView(r, room, state, playerID, seat))
}

func (s *Server) moveGame(w http.ResponseWriter, r *http.Request, id string) {
	player, ok := s.currentPlayer(w, r)
	if !ok {
		return
	}
	move, ok := decodeMove(w, r)
	if !ok {
		return
	}

	state, seat, err := s.matches.Move(r.Context(), id, player.ID, move)
	switch {
	case errors.Is(err, rooms.ErrNotFound):
		writeError(w, http.StatusNotFound, "room_not_found", "room not found")
		return
	case errors.Is(err, matches.ErrNotStarted):
		writeError(w, http.StatusConflict, "game_not_started", "game has not started")
		return
	case errors.Is(err, matches.ErrNotInProgress):
		writeError(w, http.StatusConflict, "game_not_in_progress", "game is not in progress")
		return
	case errors.Is(err, matches.ErrNotSeated):
		writeError(w, http.StatusForbidden, "not_seated", "only seated players can move")
		return
	case errors.Is(err, game.ErrStaleVersion):
		writeError(w, http.StatusConflict, "stale_version", "the game has changed; refresh and try again")
		return
	case errors.Is(err, game.ErrNotYourTurn):
		writeError(w, http.StatusConflict, "not_your_turn", "it is not your turn")
		return
	case errors.Is(err, game.ErrMatchOver):
		writeError(w, http.StatusConflict, "match_over", "the match is over")
		return
	case errors.Is(err, game.ErrInvalidTarget),
		errors.Is(err, game.ErrInvalidGuess),
		errors.Is(err, game.ErrCardNotInHand),
		errors.Is(err, game.ErrMustPlayNonCompete),
		errors.Is(err, game.ErrInvalidSeat),
		errors.Is(err, game.ErrDeckEmpty):
		writeError(w, http.StatusBadRequest, "invalid_move", err.Error())
		return
	case err != nil:
		log.Printf("move in room %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not apply move")
		return
	}

	room, err := s.rooms.Get(r.Context(), id)
	if err != nil {
		log.Printf("get room %s after move: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read room")
		return
	}
	writeJSON(w, http.StatusOK, s.gameView(r, room, state, player.ID, seat))
}

func (s *Server) writeRoom(w http.ResponseWriter, r *http.Request, id string, viewerID int64, status int) {
	room, err := s.rooms.Get(r.Context(), id)
	if err != nil {
		log.Printf("get room %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read room")
		return
	}
	writeJSON(w, status, s.roomView(r, room, viewerID))
}

func (s *Server) gameView(r *http.Request, room rooms.Room, state game.State, viewerID int64, seat int) gameResponse {
	var view any = state.PublicView()
	if seat != 0 {
		if playerView, err := state.View(seat); err == nil {
			view = playerView
		}
	}
	return gameResponse{
		Room: s.roomView(r, room, viewerID),
		Game: view,
		CanMove: room.State == rooms.StateInProgress &&
			seat != 0 &&
			state.Phase == game.PhasePlaying &&
			state.ActiveSeat == seat,
	}
}

func decodeMove(w http.ResponseWriter, r *http.Request) (game.Move, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var request moveRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return game.Move{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return game.Move{}, false
	}
	return game.Move{
		Version: request.Version,
		Card:    request.Card,
		Target:  request.Target,
		Guess:   request.Guess,
	}, true
}
