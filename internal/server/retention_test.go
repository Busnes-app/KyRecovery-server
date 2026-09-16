package server_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

func TestRetentionPreviewAndPurge(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	request := func(method, path, body string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if authenticated {
			req.AddCookie(cookie)
		}
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		return rr
	}
	now := time.Now().UTC()
	dir := t.TempDir()
	for _, id := range []string{"old", "recent", "missing"} {
		deposited := now.Add(-48 * time.Hour)
		if id == "recent" {
			deposited = now
		}
		path := filepath.Join(dir, id+".kycap")
		if id != "missing" {
			if err := os.WriteFile(path, []byte("sealed"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := database.InsertCapsule(t.Context(), db.CapsuleRecord{ID: id, FilePath: path, DepositedAt: deposited, CreatedAt: now.Add(-365 * 24 * time.Hour), SizeBytes: 6, Status: "active"}); err != nil {
			t.Fatal(err)
		}
	}
	assertResponse := func(rr *httptest.ResponseRecorder, code int, body string) {
		t.Helper()
		if rr.Code != code || !strings.Contains(rr.Body.String(), body) {
			t.Fatalf("got %d %s; want %d containing %s", rr.Code, rr.Body.String(), code, body)
		}
	}
	assertResponse(request("GET", "/api/retention", "", true), 200, `"expired_count":0`)
	assertResponse(request("POST", "/api/retention/purge", `{"days":0}`, true), 200, `"purged_count":0`)
	for _, body := range []string{`{}`, `{"days":null}`, `{"days":-1}`, `{"days":36501}`, `{"days":1.5}`} {
		assertResponse(request("POST", "/api/retention", body, true), 400, "days")
	}
	if rr := request("POST", "/api/retention/purge", `{"days":1}`, false); rr.Code < 400 {
		t.Fatal("unauthenticated purge accepted")
	}
	assertResponse(request("POST", "/api/retention", `{"days":1}`, true), 200, `"expired_count":2`)
	if value, err := database.GetSetting(t.Context(), "capsule_retention_policy"); err != nil || value != `{"days":1,"weeks":0,"months":0}` {
		t.Fatalf("setting %q: %v", value, err)
	}
	assertResponse(request("POST", "/api/retention/purge", `{"days":2}`, true), 409, "changed")
	assertResponse(request("POST", "/api/retention/purge", `{"days":1}`, true), 200, `"purged_count":2`)
	assertResponse(request("POST", "/api/retention/purge", `{"days":1}`, true), 200, `"purged_count":0`)
	if _, err := os.Stat(filepath.Join(dir, "old.kycap")); !os.IsNotExist(err) {
		t.Fatalf("old file remains: %v", err)
	}
	caps, err := database.ListCapsules(t.Context())
	if err != nil || len(caps) != 1 || caps[0].ID != "recent" {
		t.Fatalf("remaining: %v %v", caps, err)
	}
	events, err := database.ListAuditEvents(t.Context(), 100)
	if err != nil {
		t.Fatal(err)
	}
	purged := 0
	for _, e := range events {
		if e.Action == "capsule_purged" {
			purged++
		}
	}
	if purged != 2 {
		t.Fatalf("purge events: %d", purged)
	}
}

func TestRetentionPermissionsAndFailedUnlink(t *testing.T) {
	srv, database := newTestServer(t)
	for _, role := range []string{"viewer", "operator"} {
		cookie := sessionCookie(t, database, role)
		for _, path := range []string{"/api/retention", "/api/retention/purge"} {
			req := httptest.NewRequest("POST", path, strings.NewReader(`{"days":1}`))
			req.AddCookie(cookie)
			rr := httptest.NewRecorder()
			srv.ServeHTTP(rr, req)
			if rr.Code != 403 {
				t.Fatalf("%s %s: %d", role, path, rr.Code)
			}
		}
	}
	cookie := sessionCookie(t, database, "admin")
	req := httptest.NewRequest("POST", "/api/retention", strings.NewReader(`{"days":1}`))
	req.AddCookie(cookie)
	req.Header.Set("Origin", "https://attacker.example")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Fatalf("cross-origin settings accepted: %d", rr.Code)
	}
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "child"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := database.InsertCapsule(t.Context(), db.CapsuleRecord{ID: "failed", FilePath: path, DepositedAt: time.Now().Add(-48 * time.Hour), CreatedAt: time.Now(), Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := database.SetSetting(t.Context(), "capsule_retention_policy", `{"days":1}`); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest("POST", "/api/retention/purge", strings.NewReader(`{"days":1}`))
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != 500 {
		t.Fatalf("unlink failure: %d %s", rr.Code, rr.Body.String())
	}
	rec, err := database.GetCapsule(t.Context(), "failed")
	if err != nil || rec == nil {
		t.Fatalf("lost retry row: %v", err)
	}
}

func TestTieredPurgeAndPoisonedLedger(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	now := time.Now().UTC()
	dir := t.TempDir()
	// Two receipts in the same UTC day/week/month outside the daily window.
	at := now.AddDate(0, 0, -20)
	at = time.Date(at.Year(), at.Month(), at.Day(), 12, 0, 0, 0, time.UTC)
	for i, id := range []string{"older", "newest"} {
		path := filepath.Join(dir, id+".kycap")
		if err := os.WriteFile(path, []byte("sealed"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := database.InsertCapsule(t.Context(), db.CapsuleRecord{ID: id, ServiceName: "notes", PairedAppID: "product", FilePath: path, DepositedAt: at.Add(time.Duration(i) * time.Minute), CreatedAt: now, Status: "active"}); err != nil {
			t.Fatal(err)
		}
	}
	call := func(s *server.Server, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		s.ServeHTTP(rr, req)
		return rr
	}
	policy := `{"days":14,"weeks":8,"months":12}`
	if rr := call(srv, "/api/retention", policy); rr.Code != 200 || !strings.Contains(rr.Body.String(), `"expired_count":1`) {
		t.Fatalf("preview: %d %s", rr.Code, rr.Body.String())
	}
	if rr := call(srv, "/api/retention/purge", `{"days":14}`); rr.Code != 409 {
		t.Fatalf("stale tier policy: %d", rr.Code)
	}
	if rr := call(srv, "/api/retention/purge", policy); rr.Code != 200 || !strings.Contains(rr.Body.String(), `"purged_count":1`) {
		t.Fatalf("purge: %d %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "newest.kycap")); err != nil {
		t.Fatal(err)
	}
	// Poison the ledger, then attempt a policy change and a purge with the saved policy.
	last, err := database.GetLastAuditEvent(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteAuditEventForTest(t.Context(), last.Seq); err != nil {
		t.Fatal(err)
	}
	poisoned, err := server.New(server.Config{DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(poisoned.Close)
	for _, path := range []string{"/api/retention", "/api/retention/purge"} {
		if rr := call(poisoned, path, policy); rr.Code != 503 {
			t.Fatalf("poisoned %s: %d", path, rr.Code)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "newest.kycap")); err != nil {
		t.Fatal(err)
	}
}
