package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// A store from before back-channel logout holds SSO sessions with no sid, so no logout
// can reach them. Upgrading must sign out those SSO sessions once, keep local password
// sessions, and never repeat the delete on a later start.
func TestUpgradeSignsOutSSOSessionsNoLogoutCanReach(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kyrecovery.db")
	d, err := db.Open(path) // current schema, then rewound to the pre-sid store below
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	now := time.Now().UTC()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []struct {
		q    string
		args []any
	}{
		{`DROP TABLE sso_logouts`, nil},
		{`DROP TABLE sso_subject_logouts`, nil},
		{`ALTER TABLE sessions DROP COLUMN sso_sid`, nil},
		{`ALTER TABLE sessions DROP COLUMN sso_subject`, nil},
		{`ALTER TABLE sessions DROP COLUMN sso_issuer`, nil},
		{`INSERT INTO users (id, username, password_hash, email, name, role, created_at) VALUES ('usr-admin-001', 'admin', 'x', 'a@example.com', 'A', 'admin', ?)`, []any{now}},
		{`INSERT INTO sessions (id, user_id, email, name, role, expires_at, created_at) VALUES (?, 'usr-admin-001', 'a@example.com', 'A', 'admin', ?, ?)`, []any{sha256Hex("local"), now.Add(time.Hour), now}},
		{`INSERT INTO sessions (id, user_id, email, name, role, expires_at, created_at) VALUES (?, 'oidc-sub', 'b@example.com', 'B', 'operator', ?, ?)`, []any{sha256Hex("native"), now.Add(time.Hour), now}},
		{`INSERT INTO sessions (id, user_id, email, name, role, expires_at, created_at, identity_issuer, identity_subject) VALUES (?, 'installer-x', 'c@example.com', 'C', 'admin', ?, ?, 'https://id.example', 'c')`, []any{sha256Hex("managed"), now.Add(time.Hour), now}},
	} {
		if _, err := raw.Exec(stmt.q, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.q, err)
		}
	}
	raw.Close()

	ctx := context.Background()
	open := func() *db.DB {
		t.Helper()
		d, err := db.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	want := func(d *db.DB, id string, alive bool) {
		t.Helper()
		s, err := d.GetSession(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if (s != nil) != alive {
			t.Fatalf("session %q alive = %v, want %v", id, s != nil, alive)
		}
	}

	d = open()
	want(d, "local", true)
	want(d, "native", false)
	want(d, "managed", false)
	// A sid-less SSO login made after the upgrade is reachable by subject; a second
	// start must not delete it.
	if err := d.InsertSession(ctx, db.SessionRecord{ID: "after", UserID: "oidc-sub", Role: "operator", NativeSSO: true,
		SSOIssuer: "https://id.example", SSOSubject: "oidc-sub", ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	d.Close()

	d = open()
	defer d.Close()
	want(d, "local", true)
	want(d, "after", true)
}
