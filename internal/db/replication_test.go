package db_test

import (
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// A target's status is its last transfer only. The count of capsules with no successful
// copy is what says whether one capsule's failure was hidden by another's success.
func TestUnreplicatedCountsCapsulesWithoutACopy(t *testing.T) {
	ctx := t.Context()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.InsertReplicationTarget(ctx, db.ReplicationTargetRecord{
		ID: "t1", Name: "R2", Type: "s3", Endpoint: "https://r2.example", Bucket: "b", Region: "auto",
		Prefix: "capsules/", AutoSync: true, Status: "active", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b", "c"} {
		if err := database.InsertCapsule(ctx, db.CapsuleRecord{ID: id, ServiceName: "kynotes", FilePath: "/x/" + id,
			Digest: id, Status: "active", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	logAt := func(capsule, status string) {
		if err := database.InsertReplicationLog(ctx, db.ReplicationLogRecord{TargetID: "t1", CapsuleID: capsule, Status: status, CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	logAt("a", "failed") // a: failed, never copied
	logAt("b", "success")
	logAt("b", "failed") // b: a later failed retry does not remove the copy
	if err := database.SetCapsuleStatus(ctx, "c", "corrupt"); err != nil {
		t.Fatal(err) // c: corrupt capsules are never replicated, so they are not missing
	}
	counts, err := database.UnreplicatedCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts["t1"] != 1 {
		t.Fatalf("unreplicated = %d, want 1 (capsule a)", counts["t1"])
	}
}
