package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Busnes-app/ky-primitives/recoverykey"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// Attempts held open together are charged one by one: no more than the budget pass,
// however many arrive at once.
func TestTakeChargesConcurrentAttemptsAtomically(t *testing.T) {
	l := newRateLimiter(time.Minute)
	now := time.Now()
	var passed atomic.Int32
	var wg sync.WaitGroup
	for range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.take("k", 10, now) {
				passed.Add(1)
			}
		}()
	}
	wg.Wait()
	if passed.Load() != 10 {
		t.Fatalf("%d attempts passed a budget of 10", passed.Load())
	}
	l.refund("k")
	if !l.take("k", 10, now) || l.take("k", 10, now) {
		t.Fatal("a refund should give back exactly one attempt")
	}
}

func TestLimitAddrGroupsIPv6By64(t *testing.T) {
	if limitAddr("2001:db8:1:2::1") != limitAddr("2001:db8:1:2:ffff::9") {
		t.Fatal("two addresses in one /64 are separate sources")
	}
	if limitAddr("2001:db8:1:2::1") == limitAddr("2001:db8:1:3::1") {
		t.Fatal("two /64s are one source")
	}
	if limitAddr("203.0.113.9") != "203.0.113.9" {
		t.Fatal("IPv4 changed")
	}
}

// A failed claim is counted against its code, but under a hash: an unauthenticated
// caller must not be able to park a megabyte-long guess in the limiter per attempt.
func TestClaimLimiterKeysAreBounded(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s, err := New(Config{DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	k, err := recoverykey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	pub := k.Public()
	if err := database.InsertRecoveryKey(t.Context(), db.RecoveryKeyRecord{
		KeyID: pub.ID(), PublicKey: pub.Bytes(), Threshold: 3, TotalShares: 5, ImportedBy: "test", ImportedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/pairing/claim",
		strings.NewReader(`{"pairing_code":"`+strings.Repeat("7", 512<<10)+`"}`))
	req.RemoteAddr = "203.0.113.7:4000"
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("claim with a long code: %d, want 400", rec.Code)
	}
	s.claimLimit.mu.Lock()
	defer s.claimLimit.mu.Unlock()
	if len(s.claimLimit.counts) == 0 {
		t.Fatal("the failed claim was not counted")
	}
	for key := range s.claimLimit.counts {
		if len(key) > 128 {
			t.Fatalf("limiter holds a %d-byte key", len(key))
		}
	}
}
