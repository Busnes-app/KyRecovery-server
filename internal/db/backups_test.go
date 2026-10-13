package db_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/backupstatus"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/resticscan"
)

func openMem(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func repoRow(t *testing.T, d *db.DB, path string) backupstatus.Repo {
	t.Helper()
	rs, err := d.ListResticRepos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if r.Path == path {
			return r
		}
	}
	t.Fatalf("repo %q not stored", path)
	return backupstatus.Repo{}
}

func scanned(path string, keys int, snaps ...time.Time) resticscan.Repo {
	r := resticscan.Repo{Path: path, KeyCount: keys, SizeBytes: 100}
	for i, s := range snaps {
		r.Snapshots = append(r.Snapshots, resticscan.Snapshot{ID: fmt.Sprintf("%064x", i), WrittenAt: s})
	}
	return r
}

func TestKeyBaselineAcrossScans(t *testing.T) {
	ctx, d := context.Background(), openMem(t)
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	apply := func(i, keys int) backupstatus.Repo {
		t.Helper()
		if err := d.ApplyResticScan(ctx, t0.Add(time.Duration(i)*time.Hour), []resticscan.Repo{scanned("u/r", keys)}); err != nil {
			t.Fatal(err)
		}
		return repoRow(t, d, "u/r")
	}

	if r := apply(0, 2); r.AckedKeyCount != 2 {
		t.Fatalf("first sight baseline = %d, want 2", r.AckedKeyCount)
	}
	// An added key stays flagged across later scans until acknowledged.
	for i := 1; i <= 3; i++ {
		if r := apply(i, 3); r.KeyCount != 3 || r.AckedKeyCount != 2 {
			t.Fatalf("scan %d: keys %d baseline %d", i, r.KeyCount, r.AckedKeyCount)
		}
	}
	if ok, err := d.AckResticKeys(ctx, "u/r"); !ok || err != nil {
		t.Fatalf("ack = %v, %v", ok, err)
	}
	if r := repoRow(t, d, "u/r"); r.AckedKeyCount != 3 {
		t.Fatalf("baseline after ack = %d", r.AckedKeyCount)
	}
	// Removal lowers the baseline, so the same key coming back alerts again.
	if r := apply(4, 2); r.AckedKeyCount != 2 {
		t.Fatalf("baseline after removal = %d", r.AckedKeyCount)
	}
	if r := apply(5, 3); r.KeyCount <= r.AckedKeyCount {
		t.Fatalf("re-added key not flagged: %+v", r)
	}
}

func TestBaselineWaitsForFirstGoodRead(t *testing.T) {
	ctx, d := context.Background(), openMem(t)
	now := time.Now().UTC()
	if err := d.ApplyResticScan(ctx, now, []resticscan.Repo{{Path: "u/r", Err: "permission denied"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyResticScan(ctx, now.Add(time.Minute), []resticscan.Repo{scanned("u/r", 2)}); err != nil {
		t.Fatal(err)
	}
	if r := repoRow(t, d, "u/r"); r.AckedKeyCount != 2 {
		t.Fatalf("baseline = %d, want 2 from the first good read", r.AckedKeyCount)
	}
}

func TestScanErrorKeepsMeasurementsAndMissingNeedsSuccess(t *testing.T) {
	ctx, d := context.Background(), openMem(t)
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := d.ApplyResticScan(ctx, t0, []resticscan.Repo{scanned("u/a", 1, t0), scanned("u/b", 1)}); err != nil {
		t.Fatal(err)
	}

	if err := d.ApplyResticScan(ctx, t0.Add(time.Hour), []resticscan.Repo{{Path: "u/a", Err: "boom"}, scanned("u/b", 1)}); err != nil {
		t.Fatal(err)
	}
	a := repoRow(t, d, "u/a")
	if a.Err != "boom" || a.SnapshotCount != 1 || a.MeasuredAt == nil || !a.MeasuredAt.Equal(t0) {
		t.Fatalf("errored repo lost its measurements: %+v", a)
	}

	// A failed scan changes no row and marks nothing missing.
	if err := d.RecordResticScanFailure(ctx, t0.Add(2*time.Hour), "root unreadable"); err != nil {
		t.Fatal(err)
	}
	if b := repoRow(t, d, "u/b"); b.MissingSince != nil {
		t.Fatalf("failed scan marked b missing")
	}
	st, err := d.ResticScanState(ctx)
	if err != nil || st.LastError != "root unreadable" || !st.LastSuccessAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("state = %+v, %v", st, err)
	}

	// Only a successful scan that does not find b marks it missing; finding it again clears it.
	if err := d.ApplyResticScan(ctx, t0.Add(3*time.Hour), []resticscan.Repo{scanned("u/a", 1, t0)}); err != nil {
		t.Fatal(err)
	}
	if b := repoRow(t, d, "u/b"); b.MissingSince == nil {
		t.Fatalf("b not marked missing")
	}
	if a := repoRow(t, d, "u/a"); a.Err != "" {
		t.Fatalf("good read did not clear the error")
	}
	if err := d.ApplyResticScan(ctx, t0.Add(4*time.Hour), []resticscan.Repo{scanned("u/a", 1), scanned("u/b", 1)}); err != nil {
		t.Fatal(err)
	}
	if b := repoRow(t, d, "u/b"); b.MissingSince != nil {
		t.Fatalf("b still missing after reappearing")
	}
	if st, _ := d.ResticScanState(ctx); st.LastError != "" {
		t.Fatalf("success did not clear the error: %+v", st)
	}
}

// Auto-link follows the rest-server user (first segment), the identity KyQuickStart issued to
// one app. A repository name another app chose must not claim a service.
func TestAutoLinkOnlyToPairedServiceOnFirstSight(t *testing.T) {
	ctx, d := context.Background(), openMem(t)
	now := time.Now().UTC()
	app := db.PairedAppRecord{ID: "p1", ServiceName: "kynotes", AppName: "n", APIToken: "t", PairingCode: "1", Status: "paired", ExpiresAt: now, CreatedAt: now}
	if err := d.InsertPairedApp(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyResticScan(ctx, now, []resticscan.Repo{scanned("kynotes", 1), scanned("kynotes/files", 1), scanned("mallory/kynotes", 1), scanned("alice/other", 1)}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"kynotes": "kynotes", "kynotes/files": "kynotes", "mallory/kynotes": "", "alice/other": ""} {
		if r := repoRow(t, d, path); r.Service != want {
			t.Fatalf("%s auto-linked to %q, want %q", path, r.Service, want)
		}
	}
	// An admin unlink survives later scans.
	if _, err := d.LinkResticRepo(ctx, "kynotes/files", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyResticScan(ctx, now.Add(time.Minute), []resticscan.Repo{scanned("kynotes/files", 1)}); err != nil {
		t.Fatal(err)
	}
	if r := repoRow(t, d, "kynotes/files"); r.Service != "" {
		t.Fatalf("rescan re-linked: %q", r.Service)
	}
}

func TestSnapshotsSamplesAndRecent(t *testing.T) {
	ctx, d := context.Background(), openMem(t)
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var snaps []time.Time
	for i := 0; i < 12; i++ {
		snaps = append(snaps, t0.Add(time.Duration(i)*time.Hour))
	}
	old := t0.Add(-500 * 24 * time.Hour)
	if err := d.ApplyResticScan(ctx, old, []resticscan.Repo{scanned("u/r", 1)}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyResticScan(ctx, t0.Add(12*time.Hour), []resticscan.Repo{scanned("u/r", 1, snaps...)}); err != nil {
		t.Fatal(err)
	}
	r := repoRow(t, d, "u/r")
	if r.SnapshotCount != 12 || len(r.RecentSnapshots) != 10 || !r.RecentSnapshots[9].Equal(snaps[11]) || !r.LastSnapshotAt.Equal(snaps[11]) {
		t.Fatalf("repo = %+v", r)
	}
	all, err := d.ListResticSnapshots(ctx, "u/r")
	if err != nil || len(all) != 12 {
		t.Fatalf("snapshots = %d, %v", len(all), err)
	}
	samples, err := d.ListResticSamples(ctx, old.Add(-time.Hour))
	if err != nil || len(samples) != 1 || samples[0].Day != "2026-10-01" {
		t.Fatalf("samples = %+v, %v (the 500-day-old one must be pruned)", samples, err)
	}
}

func TestCapsuleVerifiedAt(t *testing.T) {
	ctx, d := context.Background(), openMem(t)
	if err := d.InsertCapsule(ctx, db.CapsuleRecord{ID: "c1", ServiceName: "s", Status: "corrupt"}); err != nil {
		t.Fatal(err)
	}
	if c, _ := d.GetCapsule(ctx, "c1"); c.VerifiedAt != nil {
		t.Fatalf("new capsule claims verification")
	}
	at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	if err := d.MarkCapsuleVerified(ctx, "c1", at); err != nil {
		t.Fatal(err)
	}
	c, _ := d.GetCapsule(ctx, "c1")
	if c.Status != "active" || c.VerifiedAt == nil || !c.VerifiedAt.Equal(at) {
		t.Fatalf("capsule = %+v", c)
	}
}

// A capsules table from before verified_at must gain it on open and keep its rows.
func TestOpenAddsVerifiedAtToOldCapsules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE capsules (id TEXT PRIMARY KEY, service_name TEXT NOT NULL, app_name TEXT NOT NULL DEFAULT '',
		app_version TEXT NOT NULL DEFAULT '', file_path TEXT NOT NULL, size_bytes INTEGER NOT NULL, digest TEXT NOT NULL,
		payload_hash TEXT NOT NULL, threshold INTEGER NOT NULL, total_shares INTEGER NOT NULL, recovery_key_id TEXT NOT NULL,
		encapsulated_key TEXT NOT NULL, created_at DATETIME NOT NULL, deposited_at DATETIME NOT NULL,
		paired_app_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'active');
		INSERT INTO capsules VALUES ('old', 's', '', '', '/x', 1, 'd', 'p', 2, 3, 'k', 'e', '2026-01-01 00:00:00', '2026-01-01 00:00:00', 'p1', 'active');`)
	raw.Close()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		d, err := db.Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		c, err := d.GetCapsule(context.Background(), "old")
		d.Close()
		if err != nil || c == nil || c.VerifiedAt != nil {
			t.Fatalf("open %d: capsule = %+v, %v", i, c, err)
		}
	}
}
