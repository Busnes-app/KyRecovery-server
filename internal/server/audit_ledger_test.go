package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Busnes-app/ky-primitives/recoverykey"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

// A ledger that cannot append is not a quiet degradation for a blind store: the audit
// trail is most of its evidence that a deposit happened. Deposits stop, and both the
// verify endpoint and readiness say why.
func TestPoisonedLedgerRefusesDepositsAndIsVisible(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	k, err := recoverykey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	importKey(t, srv, cookie, k.Public(), 3, 5)
	token, _ := pairProduct(t, srv, cookie, "kynotes")

	// Someone with database access removes the tail of the log. The anchor still counts it.
	last, err := database.GetLastAuditEvent(t.Context())
	if err != nil || last == nil {
		t.Fatalf("last audit event: %v %v", last, err)
	}
	if err := database.DeleteAuditEventForTest(t.Context(), last.Seq); err != nil {
		t.Fatal(err)
	}

	poisoned, err := server.New(server.Config{Port: 8097, DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(poisoned.Close)

	if rec := deposit(poisoned, token, sealFor(t, k.Public(), "kynotes")); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("deposit onto a poisoned ledger: %d %s", rec.Code, rec.Body.String())
	}

	verify := ask(t, poisoned, cookie, http.MethodPost, "/api/audit/verify")
	if verify["append_disabled"] != true {
		t.Fatalf("audit verify does not report the latched ledger: %v", verify)
	}
	if verify["error"] == nil {
		t.Fatalf("audit verify reports no reason: %v", verify)
	}
	ready := ask(t, poisoned, cookie, http.MethodGet, "/api/readiness")
	if ready["audit_append_disabled"] != true {
		t.Fatalf("readiness does not report the latched ledger: %v", ready)
	}
	health, got := getHealth(t, poisoned)
	if health.Code != http.StatusOK || got.Status != "degraded" || len(got.Checks) != 3 || got.Checks[0].Status != "ok" || got.Checks[1].Name != "audit" || got.Checks[1].Status != "degraded" || got.Checks[1].Reason != "append_disabled" {
		t.Fatalf("poisoned health=%d %+v", health.Code, got)
	}
	if strings.Contains(health.Body.String(), "audit log") || strings.Contains(health.Body.String(), "anchor") {
		t.Fatalf("public health leaked ledger detail: %q", health.Body.String())
	}
}

func ask(t *testing.T, srv *server.Server, cookie *http.Cookie, method, path string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("%s %s: %d %s", method, path, rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return out
}

// Sealed bytes leaving the store is the event that most needs a record. If the ledger
// cannot append it, the download does not happen: an unlogged copy of a capsule is exactly
// what an operator would later need the log to rule out.
func TestPoisonedLedgerRefusesDownloads(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	k, err := recoverykey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	importKey(t, srv, cookie, k.Public(), 3, 5)
	token, _ := pairProduct(t, srv, cookie, "kynotes")
	if rec := deposit(srv, token, sealFor(t, k.Public(), "kynotes")); rec.Code != http.StatusCreated {
		t.Fatalf("deposit: %d %s", rec.Code, rec.Body.String())
	}
	caps, _ := database.ListCapsules(t.Context())
	id := caps[0].ID

	last, err := database.GetLastAuditEvent(t.Context())
	if err != nil || last == nil {
		t.Fatalf("last audit event: %v %v", last, err)
	}
	if err := database.DeleteAuditEventForTest(t.Context(), last.Seq); err != nil {
		t.Fatal(err)
	}
	poisoned, err := server.New(server.Config{Port: 8098, DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(poisoned.Close)

	req := httptest.NewRequest(http.MethodGet, "/api/capsules/"+id+"/download", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	poisoned.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("download onto a poisoned ledger: %d, want 503", rr.Code)
	}
	if rr.Header().Get("Content-Disposition") != "" {
		t.Fatal("the refused download still started sending a capsule")
	}
}

func TestFailedAuditVerifyWarnsUntilSuccessfulVerifyButAcceptsDeposits(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	k, _ := recoverykey.Generate()
	importKey(t, srv, cookie, k.Public(), 3, 5)
	token, _ := pairProduct(t, srv, cookie, "kynotes")
	// Remove an interior row, keeping the current tail and anchor intact.
	var removed db.AuditRecord
	for event, err := range database.IterAuditEvents(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		removed = event
		break
	}
	if err := database.DeleteAuditEventForTest(t.Context(), removed.Seq); err != nil {
		t.Fatal(err)
	}
	if got := ask(t, srv, cookie, http.MethodPost, "/api/audit/verify"); got["valid"] != false || got["append_disabled"] == true {
		t.Fatalf("verify: %v", got)
	}
	fresh := func() *server.Server {
		s, err := server.New(server.Config{DataDir: t.TempDir()}, database, audit.NewLedger(database))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(s.Close)
		return s
	}
	srv = fresh() // failed evidence survives construction/restart
	ready := ask(t, srv, cookie, http.MethodGet, "/api/readiness")
	if ready["audit_chain_broken"] != true || ready["audit_append_disabled"] != false {
		t.Fatalf("readiness: %v", ready)
	}
	w, health := getHealth(t, srv)
	if w.Code != 200 || health.Checks[1].Status != "degraded" || health.Checks[1].Reason != "chain_broken" {
		t.Fatalf("health: %d %+v", w.Code, health)
	}
	if rr := deposit(srv, token, sealFor(t, k.Public(), "kynotes")); rr.Code != http.StatusCreated {
		t.Fatalf("deposit refused after failed verification: %d %s", rr.Code, rr.Body.String())
	}
	if ready := ask(t, srv, cookie, http.MethodGet, "/api/readiness"); ready["audit_chain_broken"] != true {
		t.Fatal("deposit cleared warning")
	}
	count, hash, _, err := database.GetAuditAnchor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := database.InsertAuditEventAndAnchor(t.Context(), removed, count, hash); err != nil {
		t.Fatal(err)
	}
	if got := ask(t, srv, cookie, http.MethodPost, "/api/audit/verify"); got["valid"] != true {
		t.Fatalf("repair verification: %v", got)
	}
	srv = fresh()
	if ready := ask(t, srv, cookie, http.MethodGet, "/api/readiness"); ready["audit_chain_broken"] != false {
		t.Fatalf("warning not cleared: %v", ready)
	}
	w, health = getHealth(t, srv)
	if w.Code != 200 || health.Checks[1].Status != "ok" {
		t.Fatalf("repaired health: %d %+v", w.Code, health)
	}
}
