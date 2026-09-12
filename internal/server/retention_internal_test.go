package server

import (
	"github.com/Busness-app/kyrecovery-server/internal/audit"
	"runtime"
	"testing"
	"time"

	"github.com/Busness-app/kyrecovery-server/internal/db"
)

func TestExpiryBoundary(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		at   time.Time
		days int
		want bool
	}{
		{now.Add(-24 * time.Hour), 1, true},
		{now.Add(-24*time.Hour + time.Nanosecond), 1, false},
		{now.Add(-48 * time.Hour), 0, false},
		{time.Time{}, 1, false},
		{now.Add(time.Hour), 1, false},
	} {
		if got := len(retentionCandidates([]db.CapsuleRecord{{ID: "one", DepositedAt: tc.at}}, retentionPolicy{Days: tc.days}, now)) == 1; got != tc.want {
			t.Errorf("%v days=%d: %v", tc.at, tc.days, got)
		}
	}
}

func TestTieredRetentionPerProduct(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	rec := func(id, app string, at time.Time) db.CapsuleRecord {
		return db.CapsuleRecord{ID: id, PairedAppID: app, ServiceName: "notes", DepositedAt: at}
	}
	caps := []db.CapsuleRecord{
		rec("daily1", "a", now.Add(-time.Hour)), rec("daily2", "a", now.Add(-2*time.Hour)),
		rec("weekly-new", "a", time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)),
		rec("weekly-old", "a", time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)),
		rec("other-app", "b", time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)),
		rec("monthly-new", "a", time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)),
		rec("monthly-old", "a", time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)),
		rec("too-old", "a", now.AddDate(-2, 0, 0)),
	}
	policy := retentionPolicy{Days: 14, Weeks: 8, Months: 12}
	for i := 0; i < 2; i++ {
		expired := retentionCandidates(caps, policy, now)
		got := map[string]bool{}
		for _, c := range expired {
			got[c.ID] = true
		}
		if len(got) != 3 || !got["weekly-old"] || !got["monthly-old"] || !got["too-old"] {
			t.Fatalf("expired: %v", got)
		}
		// Input order cannot change the survivors; DB ordering uses untrusted creation time.
		for l, r := 0, len(caps)-1; l < r; l, r = l+1, r-1 {
			caps[l], caps[r] = caps[r], caps[l]
		}
	}
}

func TestRetentionWaitsForFailedPublish(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s, err := New(Config{DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	release := s.idLocks.acquire("pending")
	if err := database.InsertCapsule(t.Context(), db.CapsuleRecord{ID: "pending", CreatedAt: time.Now(), DepositedAt: time.Now(), Status: "active"}); err != nil {
		release()
		t.Fatal(err)
	}
	done := make(chan []db.CapsuleRecord, 1)
	go func() { caps, _ := s.retentionCapsules(t.Context()); done <- caps }()
	deadline := time.Now().Add(time.Second)
	for {
		s.idLocks.mu.Lock()
		refs := s.idLocks.m["pending"].refs
		s.idLocks.mu.Unlock()
		if refs == 2 {
			break
		}
		if time.Now().After(deadline) {
			release()
			t.Fatal("snapshot did not wait for publish")
		}
		runtime.Gosched()
	}
	if err := database.DeleteCapsule(t.Context(), "pending"); err != nil {
		release()
		t.Fatal(err)
	}
	release()
	select {
	case caps := <-done:
		if len(caps) != 0 {
			t.Fatal("failed publish considered a survivor")
		}
	case <-time.After(time.Second):
		t.Fatal("snapshot stuck after publish rollback")
	}
}

func TestMonthlyRetentionClampsMonthEnd(t *testing.T) {
	now := time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC)
	caps := []db.CapsuleRecord{{ID: "feb", DepositedAt: time.Date(2026, 2, 28, 13, 0, 0, 0, time.UTC)}}
	if got := retentionCandidates(caps, retentionPolicy{Days: 1, Months: 1}, now); len(got) != 0 {
		t.Fatal("February survivor lost to month normalization")
	}
}
