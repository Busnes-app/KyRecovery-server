package db_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// rawColumns reads what someone holding a copy of kyrecovery.db sees.
func rawColumns(t *testing.T, path string) (sessions, tokens, codes []string) {
	t.Helper()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	read := func(q string) []string {
		rows, err := raw.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		}
		return out
	}
	return read(`SELECT id FROM sessions`), read(`SELECT api_token FROM paired_apps`), read(`SELECT pairing_code FROM paired_apps`)
}

// Attack: copy the database, read the credential columns, and present what they hold.
// The columns must hold neither the session ID, nor the API token, nor the pairing code,
// and what they do hold must not work as any of them.
func TestStolenDatabaseYieldsNoUsableCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kyrecovery.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	now := time.Now().UTC()

	const sessionID = "0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := d.InsertSession(ctx, db.SessionRecord{ID: sessionID, UserID: "u1", Email: "a@example.com", Name: "A", Role: "admin", ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"111111", "222222"} {
		if err := d.InsertPairedApp(ctx, db.PairedAppRecord{ID: "pair-" + code, ServiceName: "kynotes", AppName: "n", PairingCode: code, Status: "pending", ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	claimed, err := d.ClaimPairingCode(ctx, "111111", "kynotes", "n", "key")
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed.APIToken) != len("kyrec_live_")+64 {
		t.Fatalf("claim must return the minted token once, got %q", claimed.APIToken)
	}

	sessions, tokens, codes := rawColumns(t, path)
	if len(sessions) != 1 || sessions[0] != sha256Hex(sessionID) {
		t.Fatalf("sessions.id is not the SHA-256 of the session ID: %v", sessions)
	}
	for _, v := range tokens {
		if v == claimed.APIToken {
			t.Fatal("paired_apps.api_token holds the API token as issued")
		}
	}
	for _, v := range codes {
		if v == "111111" || v == "222222" || v == sha256Hex("222222") {
			t.Fatalf("paired_apps.pairing_code holds the code or its unkeyed hash: %q", v)
		}
	}

	// Presenting the stolen values.
	if s, err := d.GetSession(ctx, sessions[0]); err != nil || s != nil {
		t.Fatalf("stored session hash accepted as a session ID: %+v %v", s, err)
	}
	for _, v := range tokens {
		if a, err := d.GetPairedAppByToken(ctx, v); err != nil || a != nil {
			t.Fatalf("stored token hash accepted as an API token: %+v %v", a, err)
		}
	}
	for _, v := range codes {
		if _, err := d.ClaimPairingCode(ctx, v, "evil", "evil", "key"); err == nil {
			t.Fatal("stored code hash accepted as a pairing code")
		}
	}

	// The real credentials still work, and nothing read back carries one.
	if s, err := d.GetSession(ctx, sessionID); err != nil || s == nil || s.ID != sessionID {
		t.Fatalf("session lookup by ID failed: %+v %v", s, err)
	}
	a, err := d.GetPairedAppByToken(ctx, claimed.APIToken)
	if err != nil || a == nil || a.ID != "pair-111111" || a.APIToken != "" || a.PairingCode != "" {
		t.Fatalf("token lookup: %+v %v", a, err)
	}
	if _, err := d.ClaimPairingCode(ctx, "222222", "kynotes", "m", "key"); err != nil {
		t.Fatalf("live code no longer claimable: %v", err)
	}
	list, err := d.ListPairedApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, app := range list {
		if app.APIToken != "" || app.PairingCode != "" {
			t.Fatalf("ListPairedApps returned a stored credential: %+v", app)
		}
	}

	// A password change keeps the caller's session and ends the others.
	const other = "fedcba9876543210fedcba9876543210fedcba9876543210"
	if err := d.InsertSession(ctx, db.SessionRecord{ID: other, UserID: "u1", Email: "a@example.com", Name: "A", Role: "admin", ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteUserSessionsExcept(ctx, "u1", sessionID); err != nil {
		t.Fatal(err)
	}
	if s, _ := d.GetSession(ctx, sessionID); s == nil {
		t.Fatal("DeleteUserSessionsExcept ended the kept session")
	}
	if s, _ := d.GetSession(ctx, other); s != nil {
		t.Fatal("DeleteUserSessionsExcept kept another session")
	}
	if err := d.DeleteSession(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	if s, _ := d.GetSession(ctx, sessionID); s != nil {
		t.Fatal("DeleteSession left the session")
	}
}

// A store written before hashing holds every credential as issued. Opening it must hash
// them in place, once, and leave every live session, paired product and pending code working.
func TestOpenHashesCredentialsStoredAsIssued(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kyrecovery.db")
	d, err := db.Open(path) // current schema, then rewound to a pre-hashing store below
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	const (
		sessionID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		token     = "kyrec_live_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		pending   = "kyrec_live_cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	)
	now := time.Now().UTC()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []struct {
		q    string
		args []any
	}{
		{`DROP TABLE IF EXISTS schema_migrations`, nil},
		{`INSERT INTO sessions (id, user_id, email, name, role, expires_at, created_at) VALUES (?, 'u1', 'a@example.com', 'A', 'admin', ?, ?)`, []any{sessionID, now.Add(time.Hour), now}},
		{`INSERT INTO paired_apps (id, service_name, app_name, api_token, pairing_code, status, expires_at, paired_at, created_at) VALUES ('p-old', 'kynotes', 'n', ?, '555555', 'paired', ?, ?, ?)`, []any{token, now, now, now}},
		{`INSERT INTO paired_apps (id, service_name, app_name, api_token, pairing_code, status, expires_at, created_at) VALUES ('p-pending', 'kynotes', 'n', ?, '666666', 'pending', ?, ?)`, []any{pending, now.Add(time.Hour), now}},
	} {
		if _, err := raw.Exec(stmt.q, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.q, err)
		}
	}
	raw.Close()

	// Opening twice proves the rewrite runs once: a second run would hash the hashes.
	for i := 0; i < 2; i++ {
		d, err := db.Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		d.Close()
	}

	sessions, tokens, codes := rawColumns(t, path)
	for _, v := range append(append(sessions, tokens...), codes...) {
		switch v {
		case sessionID, token, pending, "555555", "666666":
			t.Fatalf("credential %q still stored as issued", v)
		}
	}
	if len(sessions) != 1 || sessions[0] != sha256Hex(sessionID) {
		t.Fatalf("session not hashed once: %v", sessions)
	}

	d, err = db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if s, err := d.GetSession(ctx, sessionID); err != nil || s == nil || s.UserID != "u1" {
		t.Fatalf("existing session lost: %+v %v", s, err)
	}
	if a, err := d.GetPairedAppByToken(ctx, token); err != nil || a == nil || a.ID != "p-old" {
		t.Fatalf("existing product token lost: %+v %v", a, err)
	}
	// The pending row's token was never handed out; claiming its code mints a fresh one.
	claimed, err := d.ClaimPairingCode(ctx, "666666", "kynotes", "n", "key")
	if err != nil {
		t.Fatalf("existing pending code lost: %v", err)
	}
	if claimed.APIToken == pending {
		t.Fatal("claim handed out the token stored before hashing")
	}
	if a, _ := d.GetPairedAppByToken(ctx, pending); a != nil {
		t.Fatal("pre-hashing token of a pending row authenticates")
	}
	if a, err := d.GetPairedAppByToken(ctx, claimed.APIToken); err != nil || a == nil || a.ID != "p-pending" {
		t.Fatalf("minted token does not authenticate: %+v %v", a, err)
	}
}
