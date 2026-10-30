package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
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
	if got := rec.Body.String(); got != string(index) {
		t.Errorf("body does not match web/index.html")
	}
}
