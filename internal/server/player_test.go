package server

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/taolicus/coverletter/internal/auth"
	"github.com/taolicus/coverletter/internal/players"
)

func TestGetPlayerCreatesIdentity(t *testing.T) {
	srv, db := newTestServer(t)
	rec := doRequest(srv, http.MethodGet, "/api/player", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != auth.CookieName {
		t.Errorf("cookie name = %q, want %q", cookie.Name, auth.CookieName)
	}
	if !cookie.HttpOnly {
		t.Error("cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q, want /", cookie.Path)
	}
	if cookie.MaxAge <= 0 {
		t.Errorf("MaxAge = %d, want a persistent cookie", cookie.MaxAge)
	}
	if cookie.Secure {
		t.Error("cookie is Secure over plain HTTP")
	}

	var player players.Player
	decodeJSON(t, rec, &player)
	if player.ID == 0 {
		t.Error("player id is zero")
	}
	if player.DisplayName != "" {
		t.Errorf("displayName = %q, want empty", player.DisplayName)
	}
	if strings.Contains(rec.Body.String(), cookie.Value) {
		t.Error("response body contains the raw identity token")
	}
	hash, ok := auth.HashToken(cookie.Value)
	if !ok {
		t.Fatal("cookie is not a valid token")
	}
	if strings.Contains(rec.Body.String(), hex.EncodeToString(hash)) {
		t.Error("response body contains the token hash")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("player rows = %d, want 1", count)
	}
}

func TestGetPlayerReusesIdentity(t *testing.T) {
	srv, db := newTestServer(t)
	first := doRequest(srv, http.MethodGet, "/api/player", "", nil)
	cookie := first.Result().Cookies()[0]
	var player players.Player
	decodeJSON(t, first, &player)

	second := doRequest(srv, http.MethodGet, "/api/player", "", cookie)
	if second.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", second.Code, http.StatusOK, second.Body)
	}
	if cookies := second.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("got %d replacement cookies, want none", len(cookies))
	}
	var again players.Player
	decodeJSON(t, second, &again)
	if again.ID != player.ID {
		t.Errorf("player id = %d, want %d", again.ID, player.ID)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("player rows = %d, want 1", count)
	}
}

func TestGetPlayerReplacesUnknownToken(t *testing.T) {
	srv, db := newTestServer(t)
	unknown, err := auth.NewToken()
	if err != nil {
		t.Fatal(err)
	}

	rec := doRequest(srv, http.MethodGet, "/api/player", "", &http.Cookie{
		Name:  auth.CookieName,
		Value: unknown,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1 replacement", len(cookies))
	}
	if cookies[0].Value == unknown {
		t.Error("unknown token was reused")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("player rows = %d, want 1", count)
	}
}

func TestUpdatePlayer(t *testing.T) {
	srv, db := newTestServer(t)
	first := doRequest(srv, http.MethodGet, "/api/player", "", nil)
	cookie := first.Result().Cookies()[0]
	var created players.Player
	decodeJSON(t, first, &created)

	rec := doRequest(srv, http.MethodPatch, "/api/player", `{"displayName":"  Ada Lovelace  "}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}
	var updated players.Player
	decodeJSON(t, rec, &updated)
	if updated.ID != created.ID {
		t.Errorf("player id = %d, want %d", updated.ID, created.ID)
	}
	if updated.DisplayName != "Ada Lovelace" {
		t.Errorf("displayName = %q, want %q", updated.DisplayName, "Ada Lovelace")
	}

	again := doRequest(srv, http.MethodGet, "/api/player", "", cookie)
	var persisted players.Player
	decodeJSON(t, again, &persisted)
	if persisted.DisplayName != "Ada Lovelace" {
		t.Errorf("persisted displayName = %q, want %q", persisted.DisplayName, "Ada Lovelace")
	}

	var stored string
	if err := db.QueryRow(`SELECT display_name FROM players WHERE id = ?`, created.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "Ada Lovelace" {
		t.Errorf("stored displayName = %q, want %q", stored, "Ada Lovelace")
	}
}

func TestUpdatePlayerRejectsInvalidNames(t *testing.T) {
	srv, db := newTestServer(t)
	first := doRequest(srv, http.MethodGet, "/api/player", "", nil)
	cookie := first.Result().Cookies()[0]
	rec := doRequest(srv, http.MethodPatch, "/api/player", `{"displayName":"Original"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("set up display name: status = %d: %s", rec.Code, rec.Body)
	}

	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: `{"displayName":""}`},
		{name: "whitespace", body: `{"displayName":"   "}`},
		{name: "too long", body: `{"displayName":"` + strings.Repeat("x", 33) + `"}`},
		{name: "control character", body: `{"displayName":"Ada\u0000Lovelace"}`},
		{name: "newline", body: `{"displayName":"Ada\nLovelace"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRequest(srv, http.MethodPatch, "/api/player", tt.body, cookie)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body)
			}
			var response errorResponse
			decodeJSON(t, rec, &response)
			if response.Error.Code != "invalid_name" {
				t.Errorf("error code = %q, want invalid_name", response.Error.Code)
			}
		})
	}

	var stored string
	if err := db.QueryRow(`SELECT display_name FROM players`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "Original" {
		t.Errorf("displayName changed to %q", stored)
	}
}

func TestInvalidUpdateDoesNotCreatePlayer(t *testing.T) {
	srv, db := newTestServer(t)
	rec := doRequest(srv, http.MethodPatch, "/api/player", `{"displayName":""}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM players`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("player rows = %d, want 0", count)
	}
}

func TestPlayerMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doRequest(srv, http.MethodPost, "/api/player", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusMethodNotAllowed, rec.Body)
	}
	if got := rec.Header().Get("Allow"); got != "GET, PATCH" {
		t.Errorf("Allow = %q, want %q", got, "GET, PATCH")
	}
	var response errorResponse
	decodeJSON(t, rec, &response)
	if response.Error.Code != "method_not_allowed" {
		t.Errorf("error code = %q, want method_not_allowed", response.Error.Code)
	}
}

func TestPlayerCookieIsSecureBehindHTTPSProxy(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/player", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	if !cookies[0].Secure {
		t.Error("cookie is not Secure when X-Forwarded-Proto is https")
	}
}

func doRequest(srv http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, value any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), value); err != nil {
		t.Fatalf("decode response: %v: %s", err, rec.Body)
	}
}
