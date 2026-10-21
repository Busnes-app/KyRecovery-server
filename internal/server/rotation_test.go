package server_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Busnes-app/ky-primitives/recoverykey"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

func post(t *testing.T, srv *server.Server, cookie *http.Cookie, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func rotate(t *testing.T, srv *server.Server, cookie *http.Cookie, pub recoverykey.PublicKey, k, n int, replaces string) *httptest.ResponseRecorder {
	t.Helper()
	return post(t, srv, cookie, "/api/recovery-key/rotate", map[string]any{
		"public_key": base64.StdEncoding.EncodeToString(pub.Bytes()), "threshold": k, "total_shares": n, "replaces_key_id": replaces,
	})
}

type listedKey struct {
	KeyID          string `json:"key_id"`
	Status         string `json:"status"`
	Threshold      int    `json:"threshold"`
	TotalShares    int    `json:"total_shares"`
	CapsuleCount   int    `json:"capsule_count"`
	PinnedProducts []struct {
		ServiceName string `json:"service_name"`
	} `json:"pinned_products"`
}

func listKeys(t *testing.T, srv *server.Server, cookie *http.Cookie) map[string]listedKey {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/recovery-keys", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list keys: %d %s", rec.Code, rec.Body.String())
	}
	var keys []listedKey
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	out := map[string]listedKey{}
	for _, k := range keys {
		out[k.KeyID] = k
	}
	return out
}

func auditActions(t *testing.T, database *db.DB) map[string][]string {
	t.Helper()
	events, err := database.ListAuditEvents(t.Context(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, e := range events {
		out[e.Action] = append(out[e.Action], e.TargetID)
	}
	return out
}

// The whole rotation: a second ceremony's key becomes current beside the first, products
// learn it only by re-pairing, capsules stay recorded against the key they were sealed to,
// the restore plan names that key's cards, and the old key retires only once nothing
// retained needs it.
func TestRecoveryKeyRotation(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	a, _ := recoverykey.Generate() // test-only: the private halves never leave this test
	b, _ := recoverykey.Generate()
	idA, idB := a.Public().ID(), b.Public().ID()

	if rec := importKey(t, srv, cookie, a.Public(), 3, 5); rec.Code != http.StatusCreated {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	oldToken, _ := pairProduct(t, srv, cookie, "kynotes")
	if rec := deposit(srv, oldToken, sealFor(t, a.Public(), "kynotes")); rec.Code != http.StatusCreated {
		t.Fatalf("deposit to A: %d %s", rec.Code, rec.Body.String())
	}

	// The import endpoint stays single-shot; it points at rotation instead of replacing.
	if rec := importKey(t, srv, cookie, b.Public(), 2, 3); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "/api/recovery-key/rotate") {
		t.Fatalf("second import: %d %s", rec.Code, rec.Body.String())
	}
	// A rotation must name the key it replaces, and it must be the current one.
	if rec := rotate(t, srv, cookie, b.Public(), 2, 3, "deadbeef"); rec.Code != http.StatusConflict {
		t.Fatalf("rotate naming the wrong key: %d %s", rec.Code, rec.Body.String())
	}
	if rec := rotate(t, srv, cookie, a.Public(), 2, 3, idA); rec.Code != http.StatusConflict {
		t.Fatalf("rotate to the key already held: %d %s", rec.Code, rec.Body.String())
	}
	// Shares are never accepted, on rotation as on import.
	rec := post(t, srv, cookie, "/api/recovery-key/rotate", map[string]any{
		"public_key": base64.StdEncoding.EncodeToString(b.Public().Bytes()), "threshold": 2, "total_shares": 3,
		"replaces_key_id": idA, "shares": []string{"ky2-x"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("rotate with shares: %d", rec.Code)
	}

	rec = rotate(t, srv, cookie, b.Public(), 2, 3, idA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("rotate: %d %s", rec.Code, rec.Body.String())
	}
	var rotated struct {
		KeyID    string `json:"key_id"`
		Replaces string `json:"replaces_key_id"`
		Repair   []struct {
			ServiceName string `json:"service_name"`
		} `json:"products_to_repair"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &rotated)
	if rotated.KeyID != idB || rotated.Replaces != idA || len(rotated.Repair) != 1 || rotated.Repair[0].ServiceName != "kynotes" {
		t.Fatalf("rotate response: %s", rec.Body.String())
	}

	// Both keys are stored, each with its ID; exactly one is current.
	keys := listKeys(t, srv, cookie)
	if len(keys) != 2 || keys[idA].Status != "previous" || keys[idB].Status != "current" {
		t.Fatalf("keys after rotation: %+v", keys)
	}
	if keys[idB].Threshold != 2 || keys[idB].TotalShares != 3 || keys[idA].Threshold != 3 || keys[idA].TotalShares != 5 {
		t.Fatalf("topologies: %+v", keys)
	}
	if keys[idA].CapsuleCount != 1 || len(keys[idA].PinnedProducts) != 1 {
		t.Fatalf("A's dependents: %+v", keys[idA])
	}
	if cur := ask(t, srv, cookie, http.MethodGet, "/api/recovery-key"); cur["key_id"] != idB {
		t.Fatalf("current key: %v", cur["key_id"])
	}
	// A stale tab that still thinks A is current is refused.
	c, _ := recoverykey.Generate()
	if rec := rotate(t, srv, cookie, c.Public(), 2, 3, idA); rec.Code != http.StatusConflict {
		t.Fatalf("stale rotate: %d %s", rec.Code, rec.Body.String())
	}

	// A product that has not re-paired is refused, told why, and its old capsule is untouched.
	rec = deposit(srv, oldToken, sealFor(t, a.Public(), "kynotes"))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "rotated out") {
		t.Fatalf("deposit to the rotated-out key: %d %s", rec.Code, rec.Body.String())
	}
	// Re-pairing hands out the new key, and its deposits are recorded against it.
	newToken, claim := pairProduct(t, srv, cookie, "kynotes")
	if claim["recovery_public_key"] != base64.StdEncoding.EncodeToString(b.Public().Bytes()) || claim["threshold"] != float64(2) {
		t.Fatalf("re-pair handed out %v %v", claim["recovery_public_key"], claim["threshold"])
	}
	if rec := deposit(srv, newToken, sealFor(t, b.Public(), "kynotes")); rec.Code != http.StatusCreated {
		t.Fatalf("deposit to B: %d %s", rec.Code, rec.Body.String())
	}
	caps, err := database.ListCapsules(t.Context())
	if err != nil || len(caps) != 2 {
		t.Fatalf("capsules: %d %v", len(caps), err)
	}
	var capA string
	sealedTo := map[string]string{}
	for _, c := range caps {
		sealedTo[c.PairedAppID] = c.RecoveryKeyID
		if c.RecoveryKeyID == idA {
			capA = c.ID
		}
	}
	if len(sealedTo) != 2 || capA == "" {
		t.Fatalf("capsules are not recorded against the keys they were sealed to: %+v", sealedTo)
	}

	// The restore plan for the old product names the old key and its cards.
	oldApp, _ := database.GetPairedAppByToken(t.Context(), oldToken)
	newApp, _ := database.GetPairedAppByToken(t.Context(), newToken)
	plan := ask(t, srv, cookie, http.MethodGet, "/api/backups/restore-plan?service=kynotes&instance="+url.QueryEscape(oldApp.ID))
	key, _ := plan["recovery_key"].(map[string]any)
	if key["key_id"] != idA || key["status"] != "previous" || key["threshold"] != float64(3) || key["total_shares"] != float64(5) {
		t.Fatalf("plan for the old capsule names %v", plan["recovery_key"])
	}
	if !strings.Contains(strings.Join(anyStrings(plan["warnings"]), " "), "3 of the 5 custodian cards printed with that key ID") {
		t.Fatalf("plan warnings do not name the old cards: %v", plan["warnings"])
	}
	plan = ask(t, srv, cookie, http.MethodGet, "/api/backups/restore-plan?service=kynotes&instance="+url.QueryEscape(newApp.ID))
	if key, _ := plan["recovery_key"].(map[string]any); key["key_id"] != idB || key["status"] != "current" {
		t.Fatalf("plan for the new capsule names %v", plan["recovery_key"])
	}

	// Retiring is refused for the current key, and for A while its capsule is retained.
	if rec := post(t, srv, cookie, "/api/recovery-key/retire", map[string]string{"key_id": idB}); rec.Code != http.StatusConflict {
		t.Fatalf("retire current: %d %s", rec.Code, rec.Body.String())
	}
	rec = post(t, srv, cookie, "/api/recovery-key/retire", map[string]string{"key_id": idA})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), capA) {
		t.Fatalf("retire with a dependent capsule: %d %s", rec.Code, rec.Body.String())
	}
	if keys := listKeys(t, srv, cookie); keys[idA].Status != "previous" {
		t.Fatalf("refused retire changed the key: %+v", keys[idA])
	}
	// The SQL guard holds even if a caller skips the handler's check.
	if err := database.RetireRecoveryKey(t.Context(), idA, "test", oldApp.CreatedAt); err == nil {
		t.Fatal("db retired a key a capsule still names")
	}

	// Once the capsule has gone (retention), the key retires.
	if err := database.DeleteCapsule(t.Context(), capA); err != nil {
		t.Fatal(err)
	}
	rec = post(t, srv, cookie, "/api/recovery-key/retire", map[string]string{"key_id": idA})
	if rec.Code != http.StatusOK {
		t.Fatalf("retire: %d %s", rec.Code, rec.Body.String())
	}
	if keys := listKeys(t, srv, cookie); keys[idA].Status != "retired" || keys[idB].Status != "current" {
		t.Fatalf("keys after retire: %+v", keys)
	}
	if rec := post(t, srv, cookie, "/api/recovery-key/retire", map[string]string{"key_id": idA}); rec.Code != http.StatusConflict {
		t.Fatalf("second retire: %d", rec.Code)
	}
	// A retired key is never brought back.
	if rec := rotate(t, srv, cookie, a.Public(), 3, 5, idB); rec.Code != http.StatusConflict {
		t.Fatalf("rotate back to a retired key: %d %s", rec.Code, rec.Body.String())
	}

	// Every step is in the ledger, against the key it concerns, requests before results.
	got := auditActions(t, database)
	for action, target := range map[string]string{
		"recovery_key_imported":             idA,
		"recovery_key_rotation_requested":   idB,
		"recovery_key_rotated":              idB,
		"recovery_key_retirement_requested": idA,
		"recovery_key_retired":              idA,
	} {
		if len(got[action]) != 1 || got[action][0] != target {
			t.Errorf("audit %s: %v, want one event for %s", action, got[action], target)
		}
	}
	if v := ask(t, srv, cookie, http.MethodPost, "/api/audit/verify"); v["valid"] != true {
		t.Fatalf("ledger does not verify: %v", v)
	}
}

func anyStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// Rotation and retirement are recorded before they are made. A ledger that cannot append
// refuses both, and nothing changes.
func TestRotationAndRetireFailClosedOnAudit(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	a, _ := recoverykey.Generate()
	b, _ := recoverykey.Generate()
	importKey(t, srv, cookie, a.Public(), 3, 5)

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

	if rec := rotate(t, poisoned, cookie, b.Public(), 2, 3, a.Public().ID()); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("rotate on a poisoned ledger: %d %s", rec.Code, rec.Body.String())
	}
	if cur, _ := database.GetRecoveryKey(t.Context()); cur == nil || cur.KeyID != a.Public().ID() {
		t.Fatalf("unaudited rotation changed the current key: %+v", cur)
	}
}

// A rotation request from another origin is refused before the handler: the ceremony page
// posts same-origin, and nothing else should be able to replace the key in an admin's name.
func TestRotateRefusesCrossOrigin(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	a, _ := recoverykey.Generate()
	b, _ := recoverykey.Generate()
	importKey(t, srv, cookie, a.Public(), 3, 5)
	raw, _ := json.Marshal(map[string]any{
		"public_key": base64.StdEncoding.EncodeToString(b.Public().Bytes()), "threshold": 2, "total_shares": 3, "replaces_key_id": a.Public().ID(),
	})
	for _, path := range []string{"/api/recovery-key/rotate", "/api/recovery-key/retire"} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("cross-site %s: %d", path, rec.Code)
		}
	}
	if cur, _ := database.GetRecoveryKey(t.Context()); cur.KeyID != a.Public().ID() {
		t.Fatal("cross-site rotation changed the current key")
	}
}

// Only admins rotate or retire; a viewer may list.
func TestRotationIsAdminOnly(t *testing.T) {
	srv, admin, database := newAdminServer(t)
	a, _ := recoverykey.Generate()
	b, _ := recoverykey.Generate()
	importKey(t, srv, admin, a.Public(), 3, 5)
	for _, role := range []string{"viewer", "operator"} {
		c := sessionCookie(t, database, role)
		if rec := rotate(t, srv, c, b.Public(), 2, 3, a.Public().ID()); rec.Code != http.StatusForbidden {
			t.Fatalf("%s rotate: %d", role, rec.Code)
		}
		if rec := post(t, srv, c, "/api/recovery-key/retire", map[string]string{"key_id": a.Public().ID()}); rec.Code != http.StatusForbidden {
			t.Fatalf("%s retire: %d", role, rec.Code)
		}
	}
	listKeys(t, srv, sessionCookie(t, database, "viewer"))
}
