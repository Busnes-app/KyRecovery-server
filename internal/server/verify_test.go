package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/ky-primitives/capsule"
	"github.com/Busnes-app/ky-primitives/recoverykey"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

func TestVerifyDetectsAFlippedByte(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	k, _ := recoverykey.Generate()
	importKey(t, srv, cookie, k.Public(), 3, 5)
	token, _ := pairProduct(t, srv, cookie, "kynotes")
	raw := sealFor(t, k.Public(), "kynotes")
	rec := deposit(srv, token, raw)
	if rec.Code != http.StatusCreated {
		t.Fatalf("deposit: %d", rec.Code)
	}
	caps, _ := database.ListCapsules(t.Context())
	id := caps[0].ID

	verify := func() (int, string) {
		req := httptest.NewRequest(http.MethodPost, "/api/capsules/"+id+"/verify", nil)
		req.AddCookie(cookie)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		return rr.Code, rr.Body.String()
	}
	if code, body := verify(); code != http.StatusOK || !strings.Contains(body, `"valid":true`) {
		t.Fatalf("intact: %d %s", code, body)
	}

	data, _ := os.ReadFile(caps[0].FilePath)
	data[len(data)/2] ^= 0x01
	if err := os.WriteFile(caps[0].FilePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if code, body := verify(); code != http.StatusOK || !strings.Contains(body, `"valid":false`) {
		t.Fatalf("flipped: %d %s", code, body)
	}
	after, _ := database.GetCapsule(t.Context(), id)
	if after.Status != "corrupt" {
		t.Fatalf("status %q", after.Status)
	}
}

// A row whose file is gone (a crash-orphaned insert, or media loss) is the strongest
// possible evidence of corruption: it must be flagged, not treated as an error the sweep
// cannot make progress past.
func TestVerifyOfAMissingFileFlagsCorrupt(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	k, _ := recoverykey.Generate()
	importKey(t, srv, cookie, k.Public(), 3, 5)
	raw := sealFor(t, k.Public(), "kynotes")
	m, err := capsule.ReadUnverifiedManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	rec := db.CapsuleRecord{
		ID: m.CapsuleID, ServiceName: m.ServiceName, AppVersion: m.AppVersion,
		FilePath: filepath.Join(t.TempDir(), "gone.kycap"), SizeBytes: int64(len(raw)),
		Digest: "deadbeef", PayloadHash: m.PayloadHash, Threshold: m.Threshold, TotalShares: m.TotalShares,
		RecoveryKeyID: m.RecoveryKeyID, EncapsulatedKey: m.EncapsulatedKey,
		CreatedAt: m.CreatedAt, DepositedAt: time.Now().UTC(), Status: "active",
	}
	if err := database.InsertCapsule(t.Context(), rec); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/capsules/"+rec.ID+"/verify", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"valid":false`) {
		t.Fatalf("verify of a missing file: %d %s, want 200 valid:false", rr.Code, rr.Body.String())
	}

	after, err := database.GetCapsule(t.Context(), rec.ID)
	if err != nil || after == nil {
		t.Fatalf("GetCapsule: %v", err)
	}
	if after.Status != "corrupt" {
		t.Fatalf("status %q, want corrupt", after.Status)
	}

	events, err := database.ListAuditEvents(t.Context(), 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Action == "capsule_corrupt" && e.TargetID == rec.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("no capsule_corrupt audit event for the missing-file capsule")
	}
}

// An on-demand verify must attribute the audit event to whoever asked for it, not to the
// unattended sweep: "who checked this just now" and "the nightly job found this" are
// different questions.
func TestVerifyRecordsTheCallerAsActor(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	k, _ := recoverykey.Generate()
	importKey(t, srv, cookie, k.Public(), 3, 5)
	token, _ := pairProduct(t, srv, cookie, "kynotes")
	raw := sealFor(t, k.Public(), "kynotes")
	rec := deposit(srv, token, raw)
	if rec.Code != http.StatusCreated {
		t.Fatalf("deposit: %d", rec.Code)
	}
	caps, _ := database.ListCapsules(t.Context())
	id := caps[0].ID

	req := httptest.NewRequest(http.MethodPost, "/api/capsules/"+id+"/verify", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rr.Code, rr.Body.String())
	}

	events, err := database.ListAuditEvents(t.Context(), 50)
	if err != nil {
		t.Fatal(err)
	}
	var actor string
	for _, e := range events {
		if e.Action == "capsule_verified" && e.TargetID == id {
			actor = e.Actor
		}
	}
	if actor == "" || actor == "integrity-sweep" {
		t.Fatalf("capsule_verified actor = %q, want the admin caller", actor)
	}
}

// A file that cannot be read for any reason other than being gone says nothing about the
// bytes. A dropped mount or a lost permission must not be laundered into a "corrupt"
// verdict across the whole store, so the row is left alone and the caller gets an error.
func TestVerifyOfAnUnreadableFileDoesNotFlagCorrupt(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	k, _ := recoverykey.Generate()
	importKey(t, srv, cookie, k.Public(), 3, 5)
	token, _ := pairProduct(t, srv, cookie, "kynotes")
	raw := sealFor(t, k.Public(), "kynotes")
	if rec := deposit(srv, token, raw); rec.Code != http.StatusCreated {
		t.Fatalf("deposit: %d", rec.Code)
	}
	caps, _ := database.ListCapsules(t.Context())
	id, path := caps[0].ID, caps[0].FilePath

	// A directory opens fine and fails on read: an I/O failure that is not absence.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/capsules/"+id+"/verify", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("verify of an unreadable file: %d %s, want 500", rr.Code, rr.Body.String())
	}
	after, err := database.GetCapsule(t.Context(), id)
	if err != nil || after == nil {
		t.Fatalf("GetCapsule: %v", err)
	}
	if after.Status == "corrupt" {
		t.Fatal("a transient read failure marked the capsule corrupt")
	}
}

// An SSO user without a verified email is recorded by subject, never by the display name
// the identity provider lets them type, which could be the local admin's address.
func TestActorIsNeverTheDisplayName(t *testing.T) {
	srv, cookie, database := newAdminServer(t)
	k, _ := recoverykey.Generate()
	importKey(t, srv, cookie, k.Public(), 3, 5)
	token, _ := pairProduct(t, srv, cookie, "kynotes")
	if rec := deposit(srv, token, sealFor(t, k.Public(), "kynotes")); rec.Code != http.StatusCreated {
		t.Fatalf("deposit: %d", rec.Code)
	}
	caps, _ := database.ListCapsules(t.Context())
	const sid = "sso-operator-session-0123456789abcdef0123"
	if err := database.InsertSession(t.Context(), db.SessionRecord{ID: sid, UserID: "idp-subject-7", Name: "admin@kyrecovery.local",
		Role: "operator", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(), NativeSSO: true}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/capsules/"+caps[0].ID+"/verify", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sid})
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("verify: %d %s", rr.Code, rr.Body.String())
	}
	events, _ := database.ListAuditEvents(t.Context(), 50)
	for _, e := range events {
		if e.Action == "capsule_verified" && e.Actor != "idp-subject-7" {
			t.Fatalf("capsule_verified actor = %q, want the subject", e.Actor)
		}
	}
}

func TestCapsuleVerifyRequiresProtectedOperatorPost(t *testing.T) {
	srv, admin, database := newAdminServer(t)
	k, _ := recoverykey.Generate()
	importKey(t, srv, admin, k.Public(), 3, 5)
	token, _ := pairProduct(t, srv, admin, "kynotes")
	if got := deposit(srv, token, sealFor(t, k.Public(), "kynotes")); got.Code != http.StatusCreated {
		t.Fatal(got.Body.String())
	}
	caps, _ := database.ListCapsules(t.Context())
	url := "/api/capsules/" + caps[0].ID + "/verify"
	for _, tc := range []struct {
		method, role string
		cross        bool
		want         int
	}{
		{http.MethodGet, auth.RoleViewer, false, 405},
		{http.MethodGet, auth.RoleOperator, false, 405},
		{http.MethodPost, auth.RoleViewer, false, 403},
		{http.MethodPost, auth.RoleOperator, true, 403},
		{http.MethodPost, auth.RoleAdmin, true, 403},
		{http.MethodPost, auth.RoleOperator, false, 200},
		{http.MethodPost, auth.RoleAdmin, false, 200},
	} {
		req := httptest.NewRequest(tc.method, url, nil)
		req.AddCookie(sessionCookie(t, database, tc.role))
		if tc.cross {
			req.Header.Set("Origin", "https://sibling.example")
			req.Header.Set("Sec-Fetch-Site", "same-site")
		}
		before, _ := database.ListAuditEvents(t.Context(), 100)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("%s %s cross=%v: %d %s", tc.method, tc.role, tc.cross, rr.Code, rr.Body.String())
		}
		after, _ := database.ListAuditEvents(t.Context(), 100)
		if tc.want != 200 && len(after) != len(before) {
			t.Fatal("refused verification appended an audit event")
		}
	}
}
