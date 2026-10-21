package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// A replication_targets table from before host_key existed must gain the
// column on open, and a target must round-trip through it.
func TestOpenAddsHostKeyToOldReplicationTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE replication_targets (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL, endpoint TEXT NOT NULL,
		bucket TEXT NOT NULL, region TEXT NOT NULL DEFAULT 'us-east-1', access_key TEXT NOT NULL,
		secret_key TEXT NOT NULL, prefix TEXT NOT NULL DEFAULT 'capsules/',
		auto_sync INTEGER NOT NULL DEFAULT 1, status TEXT NOT NULL DEFAULT 'active',
		last_sync_at DATETIME, created_at DATETIME NOT NULL)`)
	raw.Close()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ { // second open proves the migration is idempotent
		database, err := db.Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		database.Close()
	}

	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	rec := db.ReplicationTargetRecord{ID: "t1", Name: "n", Type: "sftp", Endpoint: "h", HostKey: "SHA256:abc", CreatedAt: time.Now()}
	if err := database.InsertReplicationTarget(ctx, rec); err != nil {
		t.Fatal(err)
	}
	got, err := database.GetReplicationTarget(ctx, "t1")
	if err != nil || got == nil || got.HostKey != "SHA256:abc" {
		t.Fatalf("host key did not round-trip: %+v err=%v", got, err)
	}
}

// A store from before rotation holds its key in the singleton recovery_key table. Opening
// it must carry that key over as the current one, once, and leave the old row for an older
// binary to find.
func TestOpenCarriesTheSingletonRecoveryKeyOver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE recovery_key (
		singleton INTEGER PRIMARY KEY CHECK (singleton = 1), key_id TEXT NOT NULL, public_key BLOB NOT NULL,
		threshold INTEGER NOT NULL, total_shares INTEGER NOT NULL, imported_by TEXT NOT NULL, imported_at DATETIME NOT NULL);
		INSERT INTO recovery_key VALUES (1, 'key-a', x'01', 3, 5, 'admin', '2026-01-02 03:04:05+00:00');`)
	raw.Close()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 2; i++ { // the second open proves the copy happens once
		database, err := db.Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		if i == 0 {
			if err := database.RotateRecoveryKey(ctx, "key-a", db.RecoveryKeyRecord{KeyID: "key-b", PublicKey: []byte{2}, Threshold: 2, TotalShares: 3, ImportedBy: "admin", ImportedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
		}
		database.Close()
	}
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	keys, err := database.ListRecoveryKeys(ctx)
	if err != nil || len(keys) != 2 {
		t.Fatalf("keys: %+v %v", keys, err)
	}
	got := map[string]db.RecoveryKeyRecord{}
	for _, k := range keys {
		got[k.KeyID] = k
	}
	if a := got["key-a"]; a.Status != "previous" || a.Threshold != 3 || a.TotalShares != 5 || a.ImportedBy != "admin" {
		t.Fatalf("carried-over key: %+v", a)
	}
	if cur, _ := database.GetRecoveryKey(ctx); cur == nil || cur.KeyID != "key-b" {
		t.Fatalf("current: %+v", cur)
	}
	if err := database.InsertRecoveryKey(ctx, db.RecoveryKeyRecord{KeyID: "key-c", PublicKey: []byte{3}, Threshold: 2, TotalShares: 2, ImportedAt: time.Now()}); err != db.ErrRecoveryKeyExists {
		t.Fatalf("first-import path on a keyed store: %v", err)
	}
}
