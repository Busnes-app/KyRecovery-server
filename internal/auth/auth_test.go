package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := auth.HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !auth.VerifyPassword("correct-horse-battery-staple", hash) {
		t.Fatalf("VerifyPassword rejected the password it was hashed from")
	}
	if auth.VerifyPassword("wrong-password", hash) {
		t.Fatalf("VerifyPassword accepted the wrong password")
	}
	if auth.VerifyPassword("correct-horse-battery-staple", "not a phc string") {
		t.Fatalf("VerifyPassword accepted a malformed stored hash")
	}
}

func TestPKCEGeneration(t *testing.T) {
	verifier, challenge, err := auth.GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE failed: %v", err)
	}
	if len(verifier) == 0 || len(challenge) == 0 {
		t.Fatalf("empty verifier or challenge")
	}
}

func TestSessionManagement(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer database.Close()

	mgr := auth.NewManager(auth.OIDCConfig{
		Enabled:    true,
		AdminEmail: "admin@kyrecovery.local",
	}, database)

	user := &auth.UserInfo{
		Subject: "usr-001",
		Email:   "admin@kyrecovery.local",
		Name:    "System Administrator",
		Role:    "admin",
	}

	cookie, err := mgr.CreateSession(ctx, user, true)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if cookie.Name != auth.SessionCookieName || cookie.Value == "" {
		t.Fatalf("unexpected cookie: %+v", cookie)
	}

	// Read session back
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(cookie)

	session, err := mgr.GetSession(ctx, req)
	if err != nil || session == nil {
		t.Fatalf("GetSession failed: %v, session=%+v", err, session)
	}
	if session.Email != "admin@kyrecovery.local" || session.Role != "admin" {
		t.Fatalf("session data mismatch: %+v", session)
	}

	// Invalidate session
	clearCookie := mgr.InvalidateSession(ctx, req)
	if clearCookie.MaxAge != -1 {
		t.Fatalf("expected clear cookie with MaxAge -1")
	}

	// Verify session deleted
	sessionAfter, err := mgr.GetSession(ctx, req)
	if err != nil || sessionAfter != nil {
		t.Fatalf("expected nil session after deletion")
	}
}

// An admin who clears the SSO admin email in Settings must not see the KY_ADMIN_EMAIL
// default promote that address again after a restart.
func TestClearedAdminEmailSurvivesRestart(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	env := auth.OIDCConfig{AdminEmail: "former-admin@example.com"}
	m := auth.NewManager(env, database)
	if err := m.SaveConfig(t.Context(), auth.OIDCConfig{IssuerURL: "https://id.example", ClientID: "c"}); err != nil {
		t.Fatal(err)
	}
	if got := auth.NewManager(env, database).GetConfig().AdminEmail; got != "" {
		t.Fatalf("admin email after restart = %q, want the saved empty value", got)
	}
	if got := auth.NewManager(env, nil).GetConfig().AdminEmail; got != env.AdminEmail {
		t.Fatalf("without saved settings the default applies; got %q", got)
	}
}

// A user demoted at the identity provider signs in again: the older session that still
// carries the admin role ends, and a second session with the same role is left alone.
func TestSSORoleChangeEndsSessionsWithTheOldRole(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	m := auth.NewManager(auth.OIDCConfig{}, database)
	login := func(role string) string {
		c, err := m.CreateSession(t.Context(), &auth.UserInfo{Issuer: "https://id.example", Subject: "s1", Role: role}, false)
		if err != nil {
			t.Fatal(err)
		}
		return c.Value
	}
	admin := login(auth.RoleAdmin)
	viewer1 := login(auth.RoleViewer)
	viewer2 := login(auth.RoleViewer)
	for id, want := range map[string]bool{admin: false, viewer1: true, viewer2: true} {
		if s, _ := database.GetSession(t.Context(), id); (s != nil) != want {
			t.Fatalf("session %s alive = %v, want %v", id[:6], s != nil, want)
		}
	}
}

// Callbacks can finish out of order. An admin sign-in whose ID token predates the viewer
// sign-in that already completed is the stale grant: it is refused and the viewer stays.
func TestStaleSSOCallbackCannotRestoreTheOldRole(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	m := auth.NewManager(auth.OIDCConfig{}, database)
	now := time.Now().UTC()
	viewer, err := m.CreateSession(t.Context(), &auth.UserInfo{Issuer: "https://id.example", Subject: "s1", Role: auth.RoleViewer, IssuedAt: now}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateSession(t.Context(), &auth.UserInfo{Issuer: "https://id.example", Subject: "s1", Role: auth.RoleAdmin, IssuedAt: now.Add(-time.Minute)}, false); err == nil {
		t.Fatal("a stale admin callback created a session")
	}
	if s, _ := database.GetSession(t.Context(), viewer.Value); s == nil {
		t.Fatal("the stale callback ended the newer viewer session")
	}
}
