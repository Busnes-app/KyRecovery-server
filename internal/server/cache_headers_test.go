package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// index.html names the running build's scripts, so browsers revalidate it: a tab opened after an
// upgrade loads the new build. Static files keep their headers.
func TestIndexRevalidatesAndStaticFilesKeepCaching(t *testing.T) {
	srv, _ := newTestServer(t)
	for path, want := range map[string]string{"/": "no-cache", "/static/js/app.js": "", "/favicon.svg": "public, max-age=86400"} {
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != want {
			t.Errorf("%s: status %d, Cache-Control %q, want %q", path, w.Code, w.Header().Get("Cache-Control"), want)
		}
	}
}
