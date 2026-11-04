package replication_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/replication"
)

func TestLocalReplication(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	defer database.Close()

	ledger := audit.NewLedger(database)
	mgr := replication.NewManager(database, ledger)

	// Create a dummy capsule file
	tempDir := t.TempDir()
	capFilePath := filepath.Join(tempDir, "cap-test-123.kycap")
	if err := os.WriteFile(capFilePath, []byte("mock-encrypted-capsule-content"), 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	capRec := db.CapsuleRecord{
		ID:          "cap-test-123",
		ServiceName: "kysignon",
		FilePath:    capFilePath,
		SizeBytes:   30,
		Digest:      fmt.Sprintf("%x", sha256.Sum256([]byte("mock-encrypted-capsule-content"))),
		PayloadHash: "abc123hash",
		Threshold:   2,
		TotalShares: 3,
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}
	if err := database.InsertCapsule(ctx, capRec); err != nil {
		t.Fatalf("InsertCapsule failed: %v", err)
	}

	// 1. Create a local replication target
	offsiteDir := filepath.Join(tempDir, "offsite-vault")
	target := db.ReplicationTargetRecord{
		ID:        "target-local-01",
		Name:      "Local Offsite Vault",
		Type:      "local",
		Endpoint:  offsiteDir,
		AutoSync:  true,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}
	if err := database.InsertReplicationTarget(ctx, target); err != nil {
		t.Fatalf("InsertReplicationTarget failed: %v", err)
	}

	// 2. Test connectivity
	if err := mgr.TestTarget(ctx, target); err != nil {
		t.Fatalf("TestTarget failed: %v", err)
	}

	// 3. Replicate capsule
	logRec, err := mgr.SyncCapsule(ctx, "cap-test-123", "target-local-01")
	if err != nil {
		t.Fatalf("SyncCapsule failed: %v", err)
	}
	if logRec.Status != "success" || logRec.BytesTransferred != 30 {
		t.Fatalf("unexpected log record: %+v", logRec)
	}

	// 4. Verify file was copied to offsite destination
	replicatedPath := filepath.Join(offsiteDir, "cap-test-123.kycap")
	data, err := os.ReadFile(replicatedPath)
	if err != nil || string(data) != "mock-encrypted-capsule-content" {
		t.Fatalf("replicated file content mismatch or not found: %v", err)
	}
}

// A replica may be the last good copy: a capsule marked corrupt, or whose bytes no longer
// match the recorded digest, is never written over it, and the refusal is a failed sync.
func TestReplicationNeverSendsBadBytes(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mgr := replication.NewManager(database, audit.NewLedger(database))
	dir := t.TempDir()
	good := []byte("mock-encrypted-capsule-content")
	offsiteDir := filepath.Join(dir, "offsite")
	if err := database.InsertReplicationTarget(ctx, db.ReplicationTargetRecord{ID: "t1", Name: "local", Type: "local", Endpoint: offsiteDir, Status: "active", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, status string
		onDisk     []byte
	}{
		{"cap-rotted", "active", []byte("mock-encrypted-capsule-CONTENT")},
		{"cap-flagged", "corrupt", good},
		{"cap-missing", "active", nil},
	} {
		path := filepath.Join(dir, tc.id+".kycap")
		if tc.onDisk != nil {
			if err := os.WriteFile(path, tc.onDisk, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := database.InsertCapsule(ctx, db.CapsuleRecord{ID: tc.id, ServiceName: "kynotes", FilePath: path, SizeBytes: int64(len(good)),
			Digest: fmt.Sprintf("%x", sha256.Sum256(good)), Status: tc.status, CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
		logRec, err := mgr.SyncCapsule(ctx, tc.id, "t1")
		if err == nil || logRec == nil || logRec.Status != "failed" {
			t.Fatalf("%s: err=%v log=%+v", tc.id, err, logRec)
		}
		if _, err := os.Stat(filepath.Join(offsiteDir, tc.id+".kycap")); err == nil {
			t.Fatalf("%s: bad bytes reached the replica", tc.id)
		}
		if target, _ := database.GetReplicationTarget(ctx, "t1"); target.Status != "failed" {
			t.Fatalf("%s: target status %q", tc.id, target.Status)
		}
	}
}
