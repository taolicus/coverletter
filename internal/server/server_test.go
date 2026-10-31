package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/taolicus/coverletter/web"
)

func TestRootServesEmbeddedPage(t *testing.T) {
	index, err := fs.ReadFile(web.Files, "index.html")
	if err != nil {
		t.Fatal(err)
	}

	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "text/html; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}
	if got := rec.Body.String(); got != string(index) {
		t.Errorf("body does not match web/index.html")
	}
}

func TestClientRoutesServeAppShell(t *testing.T) {
	index, err := fs.ReadFile(web.Files, "index.html")
	if err != nil {
		t.Fatal(err)
	}

	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/room/abcdefghij", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != string(index) {
		t.Errorf("body does not match web/index.html")
	}
}

func TestServesEmbeddedAssets(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
	}{
		{path: "/style.css", contentType: "text/css; charset=utf-8"},
		{path: "/app.js", contentType: "text/javascript; charset=utf-8"},
	}

	srv, _ := newTestServer(t)
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			want, err := fs.ReadFile(web.Files, strings.TrimPrefix(tt.path, "/"))
			if err != nil {
				t.Fatal(err)
			}

			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.contentType {
				t.Errorf("Content-Type = %q, want %q", got, tt.contentType)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
			}
			if got := rec.Body.String(); got != string(want) {
				t.Errorf("body does not match web/%s", strings.TrimPrefix(tt.path, "/"))
			}
		})
	}
}
