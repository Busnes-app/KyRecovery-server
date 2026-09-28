package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

type healthResponse struct {
	Schema  string `json:"schema"`
	Service string `json:"service"`
	Status  string `json:"status"`
	Time    string `json:"time"`
	Checks  []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"checks"`
}

func getHealth(t *testing.T, srv http.Handler) (*httptest.ResponseRecorder, healthResponse) {
	t.Helper()
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var got healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode health: %v; body=%q", err, w.Body.String())
	}
	return w, got
}

func TestHealthzPublicBeforeCeremonyAndCached(t *testing.T) {
	srv, database := newTestServer(t)
	w, got := getHealth(t, srv)
	if w.Code != http.StatusOK || got.Schema != "ky.health/1" || got.Service != "kyrecovery" || got.Status != "ok" || got.Time == "" {
		t.Fatalf("health=%d %+v", w.Code, got)
	}
	if len(got.Checks) != 2 || got.Checks[0].Name != "database" || got.Checks[0].Status != "ok" || got.Checks[1].Name != "audit" || got.Checks[1].Status != "ok" {
		t.Fatalf("checks=%+v", got.Checks)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache header=%q", w.Header().Get("Cache-Control"))
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	cached, _ := getHealth(t, srv)
	if cached.Body.String() != w.Body.String() {
		t.Fatalf("repeated request did not reuse result: %q != %q", cached.Body.String(), w.Body.String())
	}
}

func TestHealthzClosedDatabaseIsDownWithoutDetails(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	ledger := audit.NewLedger(database)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	fresh, err := server.New(server.Config{DataDir: t.TempDir()}, database, ledger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fresh.Close)
	w, got := getHealth(t, fresh)
	if w.Code != http.StatusServiceUnavailable || got.Status != "down" || got.Checks[0].Status != "down" {
		t.Fatalf("health=%d %+v", w.Code, got)
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "closed") || strings.Contains(w.Body.String(), "/tmp/") || got.Checks[0].Reason != "" {
		t.Fatalf("health leaked database error or path: %q", w.Body.String())
	}
}

func TestReadinessStillRequiresSession(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/readiness", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous readiness=%d", w.Code)
	}
}
