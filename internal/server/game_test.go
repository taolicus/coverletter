package server

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/taolicus/coverletter/internal/auth"
	"github.com/taolicus/coverletter/internal/game"
	"github.com/taolicus/coverletter/internal/matches"
	"github.com/taolicus/coverletter/internal/players"
	"github.com/taolicus/coverletter/internal/rooms"
	"github.com/taolicus/coverletter/internal/storage"
)

type gameEnvelope struct {
	Room    roomResponse    `json:"room"`
	Game    json.RawMessage `json:"game"`
	CanMove bool            `json:"canMove"`
}

func TestStartGame(t *testing.T) {
	srv, db := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	joinerCookie, _ := createIdentity(t, srv)
	room := createRoom(t, srv, creatorCookie)

	// The room is not ready with only the creator seated.
	rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/start", "", creatorCookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("start waiting room status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body)
	}
	assertErrorCode(t, rec, "room_not_ready")

	if rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", joinerCookie); rec.Code != http.StatusOK {
		t.Fatalf("join status = %d: %s", rec.Code, rec.Body)
	}

	// A seated non-creator cannot start.
	rec = doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/start", "", joinerCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-creator start status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body)
	}
	assertErrorCode(t, rec, "not_creator")

	// The creator starts the game.
	rec = doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/start", "", creatorCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("creator start status = %d: %s", rec.Code, rec.Body)
	}
	var started roomResponse
	decodeJSON(t, rec, &started)
	if started.State != rooms.StateInProgress {
		t.Errorf("state = %q, want in_progress", started.State)
	}
	if started.CanStart {
		t.Error("canStart = true after starting")
	}
	if body := rec.Body.String(); strings.Contains(body, creatorCookie.Value) || strings.Contains(body, identityHash(t, creatorCookie)) {
		t.Error("start response contains an identity secret")
	}

	// Starting again does not create another match.
	rec = doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/start", "", creatorCookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second start status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body)
	}
	assertErrorCode(t, rec, "room_started")
	var gameCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM games WHERE room_id = ?`, room.ID).Scan(&gameCount); err != nil {
		t.Fatal(err)
	}
	if gameCount != 1 {
		t.Errorf("game rows = %d, want 1", gameCount)
	}

	// Before a game exists, a view is rejected.
	srvWithoutGame, _ := newTestServer(t)
	creator2, _ := createIdentity(t, srvWithoutGame)
	notStartedRoom := createRoom(t, srvWithoutGame, creator2)
	rec = doRequest(srvWithoutGame, http.MethodGet, "/api/rooms/"+notStartedRoom.ID+"/game", "", creator2)
	if rec.Code != http.StatusConflict {
		t.Fatalf("pre-start game status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body)
	}
	assertErrorCode(t, rec, "game_not_started")
}

func TestGameViewsKeepHandsPrivate(t *testing.T) {
	srv, db := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	joinerCookie, _ := createIdentity(t, srv)
	outsiderCookie, _ := createIdentity(t, srv)
	room := createRoom(t, srv, creatorCookie)
	if rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", joinerCookie); rec.Code != http.StatusOK {
		t.Fatalf("join status = %d: %s", rec.Code, rec.Body)
	}
	startGame(t, srv, room.ID, creatorCookie)

	state := loadPersistedState(t, db, room.ID)
	cookies := map[int]*http.Cookie{1: creatorCookie, 2: joinerCookie}

	for seat, cookie := range cookies {
		rec := doRequest(srv, http.MethodGet, "/api/rooms/"+room.ID+"/game", "", cookie)
		if rec.Code != http.StatusOK {
			t.Fatalf("seat %d game status = %d: %s", seat, rec.Code, rec.Body)
		}
		envelope := decodeGameEnvelope(t, rec)
		view := decodePlayerView(t, envelope)

		if view.ViewerSeat != seat {
			t.Errorf("seat %d viewerSeat = %d", seat, view.ViewerSeat)
		}
		wantHand := state.Players[seat-1].Hand
		if !sameCards(view.Hand, wantHand) {
			t.Errorf("seat %d hand = %v, want %v", seat, view.Hand, wantHand)
		}
		if envelope.Room.ViewerSeat == nil || *envelope.Room.ViewerSeat != seat {
			t.Errorf("seat %d room viewerSeat = %v", seat, envelope.Room.ViewerSeat)
		}
		if envelope.CanMove != (seat == state.ActiveSeat) {
			t.Errorf("seat %d canMove = %t, active seat is %d", seat, envelope.CanMove, state.ActiveSeat)
		}
		if body := rec.Body.String(); strings.Contains(body, cookie.Value) {
			t.Errorf("seat %d game response contains the identity token", seat)
		}
	}

	// A non-seated identity gets public state only.
	rec := doRequest(srv, http.MethodGet, "/api/rooms/"+room.ID+"/game", "", outsiderCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("outsider game status = %d: %s", rec.Code, rec.Body)
	}
	envelope := decodeGameEnvelope(t, rec)
	if envelope.CanMove {
		t.Error("outsider canMove = true")
	}
	if envelope.Room.ViewerSeat != nil {
		t.Errorf("outsider room viewerSeat = %v, want null", envelope.Room.ViewerSeat)
	}
	assertNoPrivateGameFields(t, envelope.Game)

	// The same is true for an anonymous visitor, and the view does not create an
	// identity.
	anon := doRequest(srv, http.MethodGet, "/api/rooms/"+room.ID+"/game", "", nil)
	if anon.Code != http.StatusOK {
		t.Fatalf("anonymous game status = %d: %s", anon.Code, anon.Body)
	}
	if cookies := anon.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("anonymous game view set %d cookies", len(cookies))
	}
	assertNoPrivateGameFields(t, decodeGameEnvelope(t, anon).Game)
}

func TestMoveAuthorizationAndTurn(t *testing.T) {
	srv, db := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	joinerCookie, _ := createIdentity(t, srv)
	outsiderCookie, _ := createIdentity(t, srv)
	room := setupStartedGame(t, srv, creatorCookie, joinerCookie)

	state := loadPersistedState(t, db, room.ID)
	cookies := map[int]*http.Cookie{1: creatorCookie, 2: joinerCookie}
	activeCookie := cookies[state.ActiveSeat]
	waitingCookie := cookies[3-state.ActiveSeat]

	activeView := fetchPlayerView(t, srv, room.ID, activeCookie)
	body := moveBody(legalViewMove(t, activeView))

	// A non-seated identity cannot move.
	rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", body, outsiderCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("outsider move status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body)
	}
	assertErrorCode(t, rec, "not_seated")

	// The waiting player cannot move.
	rec = doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", body, waitingCookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("out-of-turn move status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body)
	}
	assertErrorCode(t, rec, "not_your_turn")
	assertUnchangedState(t, db, room.ID, state.Version)

	// Malformed actions do not change the state.
	for _, invalid := range []string{
		`{"version":` + strconv.Itoa(state.Version) + `}`,
		`{"version":`,
		`{"version":999,"card":1,"target":2,"guess":2}`,
	} {
		rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", invalid, activeCookie)
		if rec.Code != http.StatusConflict && rec.Code != http.StatusBadRequest {
			t.Errorf("invalid move status = %d, want 400 or 409: %s", rec.Code, rec.Body)
		}
		assertUnchangedState(t, db, room.ID, state.Version)
	}

	// A valid move advances the version and becomes visible to the other player.
	rec = doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", body, activeCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid move status = %d: %s", rec.Code, rec.Body)
	}
	envelope := decodeGameEnvelope(t, rec)
	moved := decodePlayerView(t, envelope)
	if moved.Version != state.Version+1 {
		t.Errorf("version = %d, want %d", moved.Version, state.Version+1)
	}
	if moved.Phase == game.PhasePlaying && moved.ActiveSeat == activeView.ViewerSeat && !envelope.CanMove {
		t.Error("actor is active but canMove = false")
	}

	other := fetchPlayerView(t, srv, room.ID, waitingCookie)
	if other.Version != state.Version+1 {
		t.Errorf("other player version = %d, want %d", other.Version, state.Version+1)
	}
	if playCount(other.History) != playCount(activeView.History)+1 {
		t.Errorf("play events = %d, want %d", playCount(other.History), playCount(activeView.History)+1)
	}
}

func TestDuplicateMoveIsRejected(t *testing.T) {
	srv, db := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	joinerCookie, _ := createIdentity(t, srv)
	room := setupStartedGame(t, srv, creatorCookie, joinerCookie)

	state := loadPersistedState(t, db, room.ID)
	cookie := map[int]*http.Cookie{1: creatorCookie, 2: joinerCookie}[state.ActiveSeat]
	view := fetchPlayerView(t, srv, room.ID, cookie)
	body := moveBody(legalViewMove(t, view))

	first := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", body, cookie)
	if first.Code != http.StatusOK {
		t.Fatalf("first move status = %d: %s", first.Code, first.Body)
	}
	after := loadPersistedState(t, db, room.ID)

	replay := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", body, cookie)
	if replay.Code != http.StatusConflict {
		t.Fatalf("replay status = %d, want %d: %s", replay.Code, http.StatusConflict, replay.Body)
	}
	assertErrorCode(t, replay, "stale_version")
	assertUnchangedState(t, db, room.ID, after.Version)
	if playCount(after.History) != playCount(state.History)+1 {
		t.Errorf("play events = %d, want %d", playCount(after.History), playCount(state.History)+1)
	}
}

func TestConcurrentMovesApplyOnce(t *testing.T) {
	srv, db := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	joinerCookie, _ := createIdentity(t, srv)
	room := setupStartedGame(t, srv, creatorCookie, joinerCookie)

	before := loadPersistedState(t, db, room.ID)
	cookie := map[int]*http.Cookie{1: creatorCookie, 2: joinerCookie}[before.ActiveSeat]
	view := fetchPlayerView(t, srv, room.ID, cookie)
	body := moveBody(legalViewMove(t, view))

	const requests = 8
	codes := make([]int, requests)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", body, cookie).Code
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
		default:
			t.Errorf("concurrent move status = %d", code)
		}
	}
	if successes != 1 {
		t.Fatalf("successful moves = %d, want 1 (codes %v)", successes, codes)
	}

	after := loadPersistedState(t, db, room.ID)
	if after.Version != before.Version+1 {
		t.Errorf("version = %d, want %d", after.Version, before.Version+1)
	}
	if playCount(after.History) != playCount(before.History)+1 {
		t.Errorf("play events = %d, want %d", playCount(after.History), playCount(before.History)+1)
	}
}

func TestGameSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coverletter.db")
	open := func() (http.Handler, *sql.DB) {
		db, err := storage.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		return New(Config{
			Players: players.New(db),
			Rooms:   rooms.New(db),
			Matches: matches.New(db),
		}), db
	}

	srv, db := open()
	creatorCookie, _ := createIdentity(t, srv)
	joinerCookie, _ := createIdentity(t, srv)
	room := setupStartedGame(t, srv, creatorCookie, joinerCookie)
	state := loadPersistedState(t, db, room.ID)
	cookie := map[int]*http.Cookie{1: creatorCookie, 2: joinerCookie}[state.ActiveSeat]
	before := fetchPlayerView(t, srv, room.ID, cookie)
	seat := before.ViewerSeat
	if rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", moveBody(legalViewMove(t, before)), cookie); rec.Code != http.StatusOK {
		t.Fatalf("move before restart status = %d: %s", rec.Code, rec.Body)
	}
	moved := loadPersistedState(t, db, room.ID)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, restartedDB := open()
	t.Cleanup(func() { _ = restartedDB.Close() })
	after := fetchPlayerView(t, restarted, room.ID, cookie)
	if after.Version != moved.Version {
		t.Errorf("resumed version = %d, want %d", after.Version, moved.Version)
	}
	if after.ActiveSeat != moved.ActiveSeat {
		t.Errorf("resumed active seat = %d, want %d", after.ActiveSeat, moved.ActiveSeat)
	}
	if !sameCards(after.Hand, moved.Players[seat-1].Hand) {
		t.Errorf("resumed hand = %v, want %v", after.Hand, moved.Players[seat-1].Hand)
	}
	if len(after.History) != len(moved.History) {
		t.Errorf("resumed history length = %d, want %d", len(after.History), len(moved.History))
	}
}

func TestGameCompletionCompletesRoom(t *testing.T) {
	srv, db := newTestServer(t)
	creatorCookie, _ := createIdentity(t, srv)
	joinerCookie, _ := createIdentity(t, srv)
	room := setupStartedGame(t, srv, creatorCookie, joinerCookie)

	started := loadPersistedState(t, db, room.ID)
	before, final := advanceToFinalMove(t, started)
	savePersistedState(t, db, room.ID, before)

	cookie := map[int]*http.Cookie{1: creatorCookie, 2: joinerCookie}[final.Seat]
	rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/move", moveBody(final), cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("final move status = %d: %s", rec.Code, rec.Body)
	}
	envelope := decodeGameEnvelope(t, rec)
	if envelope.Room.State != rooms.StateCompleted {
		t.Errorf("room state = %q, want completed", envelope.Room.State)
	}
	if envelope.CanMove {
		t.Error("canMove = true for a completed match")
	}
	view := decodePlayerView(t, envelope)
	if view.Phase != game.PhaseMatchOver {
		t.Errorf("phase = %q, want match_over", view.Phase)
	}
	if view.MatchWinnerSeat < 1 || view.MatchWinnerSeat > 2 {
		t.Errorf("match winner = %d", view.MatchWinnerSeat)
	}
	winnerCallbacks := -1
	for _, player := range view.Players {
		if player.Seat == view.MatchWinnerSeat {
			winnerCallbacks = player.Callbacks
		}
	}
	if winnerCallbacks < view.Target {
		t.Errorf("winner callbacks = %d, target %d", winnerCallbacks, view.Target)
	}
	for _, player := range view.Players {
		if player.Seat != view.MatchWinnerSeat && player.Callbacks >= winnerCallbacks {
			t.Errorf("seat %d has %d callbacks, winner has %d", player.Seat, player.Callbacks, winnerCallbacks)
		}
	}

	var roomState string
	if err := db.QueryRow(`SELECT state FROM rooms WHERE id = ?`, room.ID).Scan(&roomState); err != nil {
		t.Fatal(err)
	}
	if roomState != string(rooms.StateCompleted) {
		t.Errorf("stored room state = %q, want completed", roomState)
	}

	// The other seat sees the finished match as well.
	otherCookie := map[int]*http.Cookie{1: creatorCookie, 2: joinerCookie}[3-final.Seat]
	other := fetchPlayerView(t, srv, room.ID, otherCookie)
	if other.Phase != game.PhaseMatchOver || other.MatchWinnerSeat != view.MatchWinnerSeat {
		t.Errorf("other view phase = %q, winner = %d", other.Phase, other.MatchWinnerSeat)
	}
}

func setupStartedGame(t *testing.T, srv http.Handler, creatorCookie, joinerCookie *http.Cookie) roomResponse {
	t.Helper()
	room := createRoom(t, srv, creatorCookie)
	if rec := doRequest(srv, http.MethodPost, "/api/rooms/"+room.ID+"/join", "", joinerCookie); rec.Code != http.StatusOK {
		t.Fatalf("join status = %d: %s", rec.Code, rec.Body)
	}
	startGame(t, srv, room.ID, creatorCookie)
	return room
}

func startGame(t *testing.T, srv http.Handler, roomID string, cookie *http.Cookie) {
	t.Helper()
	rec := doRequest(srv, http.MethodPost, "/api/rooms/"+roomID+"/start", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d: %s", rec.Code, rec.Body)
	}
}

func fetchPlayerView(t *testing.T, srv http.Handler, roomID string, cookie *http.Cookie) game.PlayerView {
	t.Helper()
	rec := doRequest(srv, http.MethodGet, "/api/rooms/"+roomID+"/game", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("game status = %d: %s", rec.Code, rec.Body)
	}
	return decodePlayerView(t, decodeGameEnvelope(t, rec))
}

func decodeGameEnvelope(t *testing.T, rec *httptest.ResponseRecorder) gameEnvelope {
	t.Helper()
	var envelope gameEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode game response: %v: %s", err, rec.Body)
	}
	return envelope
}

func decodePlayerView(t *testing.T, envelope gameEnvelope) game.PlayerView {
	t.Helper()
	var view game.PlayerView
	if err := json.Unmarshal(envelope.Game, &view); err != nil {
		t.Fatalf("decode player view: %v: %s", err, envelope.Game)
	}
	return view
}

func assertNoPrivateGameFields(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var gameView map[string]any
	if err := json.Unmarshal(raw, &gameView); err != nil {
		t.Fatalf("decode public game view: %v", err)
	}
	for _, field := range []string{"hand", "snapshots", "viewerSeat"} {
		if _, ok := gameView[field]; ok {
			t.Errorf("public game view contains %q", field)
		}
	}
	seats, ok := gameView["players"].([]any)
	if !ok {
		t.Fatalf("public game view has no players: %s", raw)
	}
	for _, seat := range seats {
		player, ok := seat.(map[string]any)
		if !ok {
			t.Fatalf("player entry is not an object: %v", seat)
		}
		for _, field := range []string{"hand", "snapshots"} {
			if _, ok := player[field]; ok {
				t.Errorf("public player contains %q", field)
			}
		}
	}
}

func moveBody(move game.Move) string {
	data, _ := json.Marshal(moveRequest{
		Version: move.Version,
		Card:    move.Card,
		Target:  move.Target,
		Guess:   move.Guess,
	})
	return string(data)
}

// legalViewMove chooses a legal move using only a player's private view.
func legalViewMove(t *testing.T, view game.PlayerView) game.Move {
	t.Helper()
	if len(view.Hand) == 0 {
		t.Fatal("active player has no cards")
	}
	if containsCard(view.Hand, game.NonCompete) &&
		(containsCard(view.Hand, game.CounterOffer) || containsCard(view.Hand, game.Reassignment)) {
		return game.Move{Version: view.Version, Card: game.NonCompete}
	}

	target := otherTarget(view)
	for _, card := range view.Hand {
		move := game.Move{Version: view.Version, Card: card}
		switch card {
		case game.ReferenceCheck:
			if target != 0 {
				move.Target = target
				move.Guess = game.DreamOffer
			}
		case game.InformationalInterview, game.SalaryComparison, game.CounterOffer:
			move.Target = target
		case game.Reassignment:
			move.Target = view.ViewerSeat
			if target != 0 {
				move.Target = target
			}
		}
		return move
	}
	t.Fatal("no legal move found")
	return game.Move{}
}

func otherTarget(view game.PlayerView) int {
	for _, player := range view.Players {
		if player.Seat != view.ViewerSeat && !player.Eliminated && !player.Protected {
			return player.Seat
		}
	}
	return 0
}

// advanceToFinalMove plays a match with a test-only driver that may inspect
// hands, returning the state just before the winning move and that move.
func advanceToFinalMove(t *testing.T, state game.State) (game.State, game.Move) {
	t.Helper()
	for turn := 0; turn < 20000; turn++ {
		if state.Phase != game.PhasePlaying {
			t.Fatalf("match left the playing phase at turn %d", turn)
		}
		move := legalStateMove(t, state)
		next, err := state.Apply(move)
		if err != nil {
			t.Fatalf("turn %d: applying %+v: %v", turn, move, err)
		}
		if next.Phase == game.PhaseMatchOver {
			return state, move
		}
		state = next
	}
	t.Fatal("match did not finish")
	return game.State{}, game.Move{}
}

func legalStateMove(t *testing.T, state game.State) game.Move {
	t.Helper()
	player := state.Players[state.ActiveSeat-1]
	if containsCard(player.Hand, game.NonCompete) &&
		(containsCard(player.Hand, game.CounterOffer) || containsCard(player.Hand, game.Reassignment)) {
		return game.Move{Seat: player.Seat, Version: state.Version, Card: game.NonCompete}
	}
	for _, card := range player.Hand {
		move := game.Move{Seat: player.Seat, Version: state.Version, Card: card}
		switch card {
		case game.ReferenceCheck:
			target := stateOtherTarget(state, player.Seat)
			if target == 0 {
				return move
			}
			move.Target = target
			move.Guess = game.DreamOffer
			if cards := state.Players[target-1].Hand; len(cards) > 0 && cards[0] != game.ReferenceCheck {
				move.Guess = cards[0]
			}
		case game.InformationalInterview, game.SalaryComparison, game.CounterOffer:
			target := stateOtherTarget(state, player.Seat)
			if target == 0 {
				return move
			}
			move.Target = target
		case game.Reassignment:
			move.Target = player.Seat
		}
		return move
	}
	t.Fatal("active player has no cards")
	return game.Move{}
}

func stateOtherTarget(state game.State, seat int) int {
	for _, player := range state.Players {
		if player.Seat != seat && !player.Eliminated && !player.Protected {
			return player.Seat
		}
	}
	return 0
}

func loadPersistedState(t *testing.T, db *sql.DB, roomID string) game.State {
	t.Helper()
	var encoded string
	var version int
	if err := db.QueryRow(`SELECT state, state_version FROM games WHERE room_id = ?`, roomID).Scan(&encoded, &version); err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err := json.Unmarshal([]byte(encoded), &state); err != nil {
		t.Fatal(err)
	}
	if state.Version != version {
		t.Fatalf("stored version = %d, column = %d", state.Version, version)
	}
	return state
}

func savePersistedState(t *testing.T, db *sql.DB, roomID string, state game.State) {
	t.Helper()
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE games SET state = ?, state_version = ? WHERE room_id = ?`, encoded, state.Version, roomID); err != nil {
		t.Fatal(err)
	}
}

func assertUnchangedState(t *testing.T, db *sql.DB, roomID string, wantVersion int) {
	t.Helper()
	state := loadPersistedState(t, db, roomID)
	if state.Version != wantVersion {
		t.Errorf("state version = %d, want unchanged %d", state.Version, wantVersion)
	}
}

func assertErrorCode(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var response errorResponse
	decodeJSON(t, rec, &response)
	if response.Error.Code != want {
		t.Errorf("error code = %q, want %q", response.Error.Code, want)
	}
}

func identityHash(t *testing.T, cookie *http.Cookie) string {
	t.Helper()
	hash, ok := auth.HashToken(cookie.Value)
	if !ok {
		t.Fatal("cookie is not a valid identity token")
	}
	return hex.EncodeToString(hash)
}

func playCount(events []game.Event) int {
	count := 0
	for _, event := range events {
		if event.Kind == "play" {
			count++
		}
	}
	return count
}

func sameCards(a, b []game.Card) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsCard(cards []game.Card, card game.Card) bool {
	for _, c := range cards {
		if c == card {
			return true
		}
	}
	return false
}
