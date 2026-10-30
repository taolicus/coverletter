package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/taolicus/coverletter/internal/rooms"
)

type roomResponse struct {
	ID          string         `json:"id"`
	State       rooms.State    `json:"state"`
	Seats       []seatResponse `json:"seats"`
	CreatorSeat int            `json:"creatorSeat"`
	ViewerSeat  *int           `json:"viewerSeat"`
	CanStart    bool           `json:"canStart"`
	ShareURL    string         `json:"shareURL"`
}

type seatResponse struct {
	Number      int    `json:"number"`
	Occupied    bool   `json:"occupied"`
	DisplayName string `json:"displayName"`
}

func (s *Server) handleRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	if !s.allow(s.createLimiter, w, r) {
		return
	}

	player, ok := s.currentPlayer(w, r)
	if !ok {
		return
	}
	room, err := s.rooms.Create(r.Context(), player.ID)
	if err != nil {
		log.Printf("create room: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create room")
		return
	}

	w.Header().Set("Location", s.shareURL(r, room.ID))
	writeJSON(w, http.StatusCreated, s.roomView(r, room, player.ID))
}

func (s *Server) handleRoom(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
	parts := strings.Split(rest, "/")

	switch {
	case len(parts) == 1 && parts[0] != "":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, "GET")
			return
		}
		s.getRoom(w, r, parts[0])
	case len(parts) == 2 && parts[0] != "" && parts[1] == "join":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, "POST")
			return
		}
		s.joinRoom(w, r, parts[0])
	default:
		writeError(w, http.StatusNotFound, "not_found", "not found")
	}
}

func (s *Server) getRoom(w http.ResponseWriter, r *http.Request, id string) {
	player, found, err := s.optionalPlayer(r)
	if err != nil {
		log.Printf("find room viewer: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read player")
		return
	}

	room, err := s.rooms.Get(r.Context(), id)
	if errors.Is(err, rooms.ErrNotFound) {
		writeError(w, http.StatusNotFound, "room_not_found", "room not found")
		return
	}
	if err != nil {
		log.Printf("get room %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not read room")
		return
	}

	var viewerID int64
	if found {
		viewerID = player.ID
	}
	writeJSON(w, http.StatusOK, s.roomView(r, room, viewerID))
}

func (s *Server) joinRoom(w http.ResponseWriter, r *http.Request, id string) {
	if !s.allow(s.joinLimiter, w, r) {
		return
	}

	player, ok := s.currentPlayer(w, r)
	if !ok {
		return
	}
	room, err := s.rooms.Join(r.Context(), id, player.ID)
	switch {
	case errors.Is(err, rooms.ErrNotFound):
		writeError(w, http.StatusNotFound, "room_not_found", "room not found")
	case errors.Is(err, rooms.ErrFull):
		writeError(w, http.StatusConflict, "room_full", "room is full")
	case errors.Is(err, rooms.ErrStarted):
		writeError(w, http.StatusConflict, "room_started", "room has already started")
	case err != nil:
		log.Printf("join room %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not join room")
	default:
		writeJSON(w, http.StatusOK, s.roomView(r, room, player.ID))
	}
}

func (s *Server) allow(limiter *rateLimiter, w http.ResponseWriter, r *http.Request) bool {
	if limiter.allow(clientIP(r)) {
		return true
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(s.rateWindow.Seconds())))
	writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
	return false
}

func (s *Server) roomView(r *http.Request, room rooms.Room, viewerID int64) roomResponse {
	view := roomResponse{
		ID:          room.ID,
		State:       room.State,
		Seats:       make([]seatResponse, rooms.MaxPlayers),
		CreatorSeat: room.CreatorSeat,
		ShareURL:    s.shareURL(r, room.ID),
	}
	for i := range view.Seats {
		view.Seats[i].Number = i + 1
	}
	for _, seat := range room.Seats {
		if seat.Number < 1 || seat.Number > rooms.MaxPlayers {
			continue
		}
		name := seat.DisplayName
		if name == "" {
			name = fmt.Sprintf("Seat %d", seat.Number)
		}
		view.Seats[seat.Number-1] = seatResponse{
			Number:      seat.Number,
			Occupied:    true,
			DisplayName: name,
		}
		if seat.PlayerID == viewerID {
			number := seat.Number
			view.ViewerSeat = &number
		}
	}
	view.CanStart = viewerID != 0 &&
		viewerID == room.CreatorPlayerID &&
		room.State == rooms.StateReady
	return view
}

func (s *Server) shareURL(r *http.Request, id string) string {
	base := s.baseURL
	if base == "" {
		scheme := "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return strings.TrimRight(base, "/") + "/room/" + id
}

func (s *Server) handleAPINotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "not found")
}
