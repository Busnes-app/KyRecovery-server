package server_test

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

func TestClearRevokedPairingPreservesBackups(t *testing.T) {
	srv, database := newTestServer(t)
	admin := sessionCookie(t, database, auth.RoleAdmin)
	for _, status := range []string{"revoked", "paired", "pending"} {
		if err := database.InsertPairedApp(t.Context(), db.PairedAppRecord{ID: status, Status: status, APIToken: "token-" + status, PairingCode: status, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(t.TempDir(), "cap-one.kycap")
	if err := os.WriteFile(file, []byte("sealed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := database.InsertCapsule(t.Context(), db.CapsuleRecord{ID: "cap-one", PairedAppID: "revoked", FilePath: file, CreatedAt: time.Now(), DepositedAt: time.Now(), Status: "active"}); err != nil {
		t.Fatal(err)
	}
	call := func(id, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/pairing/clear", strings.NewReader(`{"id":"`+id+`"}`))
		r.AddCookie(admin)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		return w
	}
	if w := call("revoked", "https://attacker.example"); w.Code != 403 {
		t.Fatalf("cross-origin: %d", w.Code)
	}
	for _, id := range []string{"paired", "pending", "absent"} {
		if w := call(id, ""); w.Code != 409 {
			t.Fatalf("%s: %d %s", id, w.Code, w.Body.String())
		}
	}
	if w := call("revoked", ""); w.Code != 200 {
		t.Fatalf("clear: %d %s", w.Code, w.Body.String())
	}
	if w := call("revoked", ""); w.Code != 409 {
		t.Fatalf("repeat: %d", w.Code)
	}
	apps, err := database.ListPairedApps(t.Context())
	if err != nil || len(apps) != 2 {
		t.Fatalf("apps: %v %v", apps, err)
	}
	for _, app := range apps {
		if app.Status == "revoked" {
			t.Fatal("revoked row remains")
		}
	}
	if app, err := database.GetPairedAppByToken(t.Context(), "token-revoked"); err != nil || app != nil {
		t.Fatalf("cleared token authorized: %v", err)
	}
	cap, err := database.GetCapsule(t.Context(), "cap-one")
	if err != nil || cap == nil || cap.PairedAppID != "revoked" {
		t.Fatal("capsule metadata changed")
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "sealed" {
		t.Fatal("capsule bytes changed")
	}
	events, err := database.ListAuditEvents(t.Context(), 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Action == "pairing_cleared" {
			found = true
			if e.Actor != "admin@example.com" {
				t.Fatalf("actor: %q", e.Actor)
			}
		}
	}
	if !found {
		t.Fatal("missing clear audit event")
	}
}

func TestClearPairingRefusesPoisonedLedger(t *testing.T) {
	_, database := newTestServer(t)
	admin := sessionCookie(t, database, auth.RoleAdmin)
	if err := database.InsertPairedApp(t.Context(), db.PairedAppRecord{ID: "revoked", Status: "revoked", APIToken: "revoked-token", CreatedAt: time.Now(), ExpiresAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	ledger := audit.NewLedger(database)
	event, err := ledger.Record(t.Context(), "fixture", "admin", "revoked", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DeleteAuditEventForTest(t.Context(), event.Seq); err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(server.Config{DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	r := httptest.NewRequest("POST", "/api/pairing/clear", strings.NewReader(`{"id":"revoked"}`))
	r.AddCookie(admin)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatalf("poisoned ledger: %d", w.Code)
	}
	apps, err := database.ListPairedApps(t.Context())
	if err != nil || len(apps) != 1 {
		t.Fatal("unrecordable clear deleted row")
	}
}
