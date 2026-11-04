package server

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
