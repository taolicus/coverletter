package server

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/taolicus/coverletter/internal/auth"
	"github.com/taolicus/coverletter/internal/players"
	"github.com/taolicus/coverletter/internal/rooms"
)

func TestCreateRoom(t *testing.T) {
	srv, db := newTestServer(t)
	cookie, _ := createIdentity(t, srv)

	rec := doRequest(srv, http.MethodPost, "/api/rooms", "", cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body)
	}
	var room roomResponse
	decodeJSON(t, rec, &room)

	if len(room.ID) != 10 {
		t.Errorf("room id = %q, want 10 characters", room.ID)
	}
	if room.State != rooms.StateWaiting {
		t.Errorf("state = %q, want waiting", room.State)
	}
	if room.CreatorSeat != 1 {
		t.Errorf("creatorSeat = %d, want 1", room.CreatorSeat)
	}
	if room.ViewerSeat == nil || *room.ViewerSeat != 1 {
		t.Errorf("viewerSeat = %v, want 1", room.ViewerSeat)
	}
	if room.CanStart {
		t.Error("canStart = true for a waiting room")
	}
	if len(room.Seats) != rooms.MaxPlayers {
		t.Fatalf("seats = %d, want %d", len(room.Seats), rooms.MaxPlayers)
	}
	if !room.Seats[0].Occupied || room.Seats[0].DisplayName != "Seat 1" {
		t.Errorf("seat 1 = %+v, want the creator with a fallback name", room.Seats[0])
	}
	if room.Seats[1].Occupied {
		t.Error("seat 2 is occupied")
	}
	if room.ShareURL != "http://example.com/room/"+room.ID {
		t.Errorf("shareURL = %q", room.ShareURL)
	}
	if got := rec.Header().Get("Location"); got != room.ShareURL {
		t.Errorf("Location = %q, want %q", got, room.ShareURL)
	}
	if strings.Contains(rec.Body.String(), cookie.Value) {
		t.Error("room response contains the raw identity token")
	}
	hash, _ := auth.HashToken(cookie.Value)
	if strings.Contains(rec.Body.String(), hex.EncodeToString(hash)) {
		t.Error("room response contains the token hash")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM rooms`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("room rows = %d, want 1", count)
	}
}

func TestGetRoomViewerAndNames(t *testing.T) {
	srv, _ := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	room := createRoom(t, srv, creatorCookie)

	// An unnamed player is shown as their seat.
	rec := doRequest(srv, http.MethodGet, "/api/rooms/"+room.ID, "", creatorCookie)
	var viewed roomResponse
	decodeJSON(t, rec, &viewed)
	if viewed.ViewerSeat == nil || *viewed.ViewerSeat != 1 {
		t.Errorf("viewerSeat = %v, want 1", viewed.ViewerSeat)
	}
	if viewed.Seats[0].DisplayName != "Seat 1" {
		t.Errorf("display name = %q, want Seat 1", viewed.Seats[0].DisplayName)
	}

	// A visitor without an identity does not get one from a room view.
	anon := doRequest(srv, http.MethodGet, "/api/rooms/"+room.ID, "", nil)
	if anon.Code != http.StatusOK {
		t.Fatalf("anonymous status = %d: %s", anon.Code, anon.Body)
	}
	if cookies := anon.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("anonymous room view set %d cookies", len(cookies))
	}
	var anonymous roomResponse
	decodeJSON(t, anon, &anonymous)
	if anonymous.ViewerSeat != nil {
		t.Errorf("anonymous viewerSeat = %v, want null", anonymous.ViewerSeat)
	}

	// A display name replaces the fallback.
	patch := doRequest(srv, http.MethodPatch, "/api/player", `{"displayName":"Ada"}`, creatorCookie)
	if patch.Code != http.StatusOK {
		t.Fatalf("rename status = %d: %s", patch.Code, patch.Body)
	}
	named := doRequest(srv, http.MethodGet, "/api/rooms/"+room.ID, "", creatorCookie)
	var namedRoom roomResponse
	decodeJSON(t, named, &namedRoom)
	if namedRoom.Seats[0].DisplayName != "Ada" {
		t.Errorf("display name = %q, want Ada", namedRoom.Seats[0].DisplayName)
	}
}

func TestJoinRoom(t *testing.T) {
	srv, db := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	joinerCookie, _ := createIdentity(t, srv)
	room := createRoom(t, srv, creatorCookie)

	rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", joinerCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("join status = %d: %s", rec.Code, rec.Body)
	}
	var joined roomResponse
	decodeJSON(t, rec, &joined)
	if joined.State != rooms.StateReady {
		t.Errorf("state = %q, want ready", joined.State)
	}
	if joined.ViewerSeat == nil || *joined.ViewerSeat != 2 {
		t.Errorf("viewerSeat = %v, want 2", joined.ViewerSeat)
	}
	if joined.CanStart {
		t.Error("non-creator canStart = true")
	}
	if !joined.Seats[1].Occupied {
		t.Error("seat 2 is empty after join")
	}

	// Joining again is idempotent.
	again := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", joinerCookie)
	if again.Code != http.StatusOK {
		t.Fatalf("second join status = %d: %s", again.Code, again.Body)
	}
	var againRoom roomResponse
	decodeJSON(t, again, &againRoom)
	if len(againRoom.Seats) != rooms.MaxPlayers {
		t.Fatalf("seats = %d", len(againRoom.Seats))
	}
	var seatCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM room_seats WHERE room_id = ?`, room.ID).Scan(&seatCount); err != nil {
		t.Fatal(err)
	}
	if seatCount != 2 {
		t.Errorf("seat rows = %d, want 2", seatCount)
	}

	// Only the creator can start, and only once the room is ready.
	creatorView := doRequest(srv, http.MethodGet, "/api/rooms/"+room.ID, "", creatorCookie)
	var creatorRoom roomResponse
	decodeJSON(t, creatorView, &creatorRoom)
	if !creatorRoom.CanStart {
		t.Error("creator canStart = false in a ready room")
	}
	if creatorRoom.ViewerSeat == nil || *creatorRoom.ViewerSeat != 1 {
		t.Errorf("creator viewerSeat = %v, want 1", creatorRoom.ViewerSeat)
	}
}

func TestJoinFullRoom(t *testing.T) {
	srv, _ := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	room := createRoom(t, srv, creatorCookie)
	for i := 0; i < rooms.MaxPlayers-1; i++ {
		cookie, _ := createIdentity(t, srv)
		rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("join %d status = %d: %s", i+2, rec.Code, rec.Body)
		}
	}
	extraCookie, _ := createIdentity(t, srv)
	rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", extraCookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("full join status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body)
	}
	var response errorResponse
	decodeJSON(t, rec, &response)
	if response.Error.Code != "room_full" {
		t.Errorf("error code = %q, want room_full", response.Error.Code)
	}
}

func TestJoinStartedRoom(t *testing.T) {
	srv, db := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	seatedCookie, _ := createIdentity(t, srv)
	outsiderCookie, _ := createIdentity(t, srv)
	room := createRoom(t, srv, creatorCookie)
	if rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", seatedCookie); rec.Code != http.StatusOK {
		t.Fatalf("setup join status = %d: %s", rec.Code, rec.Body)
	}
	if _, err := db.Exec(`UPDATE rooms SET state = ? WHERE id = ?`, rooms.StateInProgress, room.ID); err != nil {
		t.Fatal(err)
	}

	rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", outsiderCookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("outsider join status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body)
	}
	var response errorResponse
	decodeJSON(t, rec, &response)
	if response.Error.Code != "room_started" {
		t.Errorf("error code = %q, want room_started", response.Error.Code)
	}

	reclaim := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", seatedCookie)
	if reclaim.Code != http.StatusOK {
		t.Fatalf("seated player rejoin status = %d: %s", reclaim.Code, reclaim.Body)
	}
}

func TestRoomRateLimit(t *testing.T) {
	srv, db := newTestServerWithConfig(t, func(config *Config) {
		config.CreateLimit = 1
		config.JoinLimit = 1
		config.RateWindow = time.Minute
	})
	creatorCookie, _ := createIdentity(t, srv)
	room := createRoom(t, srv, creatorCookie)

	rec := doRequest(srv, http.MethodPost, "/api/rooms", "", creatorCookie)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second create status = %d, want %d: %s", rec.Code, http.StatusTooManyRequests, rec.Body)
	}
	var response errorResponse
	decodeJSON(t, rec, &response)
	if response.Error.Code != "rate_limited" {
		t.Errorf("error code = %q, want rate_limited", response.Error.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("rate limit response has no Retry-After")
	}
	var roomsCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM rooms`).Scan(&roomsCount); err != nil {
		t.Fatal(err)
	}
	if roomsCount != 1 {
		t.Errorf("room rows = %d, want 1", roomsCount)
	}

	joinerCookie, _ := createIdentity(t, srv)
	if rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", joinerCookie); rec.Code != http.StatusOK {
		t.Fatalf("first join status = %d: %s", rec.Code, rec.Body)
	}
	extraCookie, _ := createIdentity(t, srv)
	rec = doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", extraCookie)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second join status = %d, want %d: %s", rec.Code, http.StatusTooManyRequests, rec.Body)
	}
	var seatCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM room_seats WHERE room_id = ?`, room.ID).Scan(&seatCount); err != nil {
		t.Fatal(err)
	}
	if seatCount != 2 {
		t.Errorf("seat rows = %d, want 2", seatCount)
	}
}

func TestRoomShareURLFromProxyHeaders(t *testing.T) {
	srv, _ := newTestServer(t)
	cookie, _ := createIdentity(t, srv)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms", nil)
	req.Host = "coverletter.pifia.net"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var room roomResponse
	decodeJSON(t, rec, &room)
	if want := "https://coverletter.pifia.net/room/" + room.ID; room.ShareURL != want {
		t.Errorf("shareURL = %q, want %q", room.ShareURL, want)
	}
}

func TestRoomBaseURLOverride(t *testing.T) {
	srv, _ := newTestServerWithConfig(t, func(config *Config) {
		config.BaseURL = "https://games.example.test/"
	})
	cookie, _ := createIdentity(t, srv)
	room := createRoom(t, srv, cookie)
	if want := "https://games.example.test/room/" + room.ID; room.ShareURL != want {
		t.Errorf("shareURL = %q, want %q", room.ShareURL, want)
	}
}

func TestUnknownAPIPath(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doRequest(srv, http.MethodGet, "/api/nope", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	var response errorResponse
	decodeJSON(t, rec, &response)
	if response.Error.Code != "not_found" {
		t.Errorf("error code = %q, want not_found", response.Error.Code)
	}
}

func createIdentity(t *testing.T, srv http.Handler) (*http.Cookie, players.Player) {
	t.Helper()
	rec := doRequest(srv, http.MethodGet, "/api/player", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("create identity status = %d: %s", rec.Code, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d identity cookies, want 1", len(cookies))
	}
	var player players.Player
	decodeJSON(t, rec, &player)
	return cookies[0], player
}

func createRoom(t *testing.T, srv http.Handler, cookie *http.Cookie) roomResponse {
	t.Helper()
	rec := doRequest(srv, http.MethodPost, "/api/rooms", "", cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create room status = %d: %s", rec.Code, rec.Body)
	}
	var room roomResponse
	decodeJSON(t, rec, &room)
	return room
}
