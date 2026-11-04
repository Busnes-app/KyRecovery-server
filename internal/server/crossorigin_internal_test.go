package server

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// Every cookie-authenticated write in apiPolicy is refused when the browser says it came
// from another origin, same-site included: SameSite=Lax still sends the session cookie
// from a sibling subdomain. Routes with their own credential are left to it.
func TestEveryCookieWriteRefusesCrossOrigin(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s, err := New(Config{DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"POST /api/auth/sso/config", "POST /api/recovery-key", "POST /api/replication/targets",
		"POST /api/auth/password", "POST /api/pairing/generate", "DELETE /api/replication/targets/x"}
	for key := range apiPolicy {
		paths = append(paths, key)
	}
	for _, key := range paths {
		method, path, _ := strings.Cut(key, " ")
		if method == "GET" {
			continue
		}
		if method == "*" {
			method = http.MethodPost
		}
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Sec-Fetch-Site", "same-site")
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		refused := rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "cross-origin")
		if refused == notCookieAuthenticated[path] {
			t.Errorf("%s %s from another origin: %d %s", method, path, rec.Code, rec.Body.String())
		}
	}
	// The dashboard itself is same-origin and goes on to the role check.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/sso/config", strings.NewReader(`{}`))
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "cross-origin") {
		t.Fatalf("same-origin write refused: %d %s", rec.Code, rec.Body.String())
	}
}

// A reverse proxy that is not listed in KYRECOVERY_TRUSTED_PROXIES collapses every client
// onto its address. The operator is told once, with the value to set.
func TestUntrustedProxyIsReportedOnce(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s, err := New(Config{DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	log.SetOutput(&out)
	defer log.SetOutput(os.Stderr)
	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/api/installer/network", nil)
		req.RemoteAddr = "172.18.0.1:40000"
		req.Header.Set("X-Forwarded-For", "198.51.100.9")
		s.ServeHTTP(httptest.NewRecorder(), req)
	}
	if got := strings.Count(out.String(), "KYRECOVERY_TRUSTED_PROXIES=172.18.0.1"); got != 1 {
		t.Fatalf("warned %d times, want once: %q", got, out.String())
	}
}
