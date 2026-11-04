package server

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Unauthenticated pairing claims are guessable (6 digits), so attempts are capped per
// source address and per code for the lifetime of a pairing window.
const (
	claimWindow          = 15 * time.Minute
	claimAttemptsPerIP   = 10
	claimAttemptsPerCode = 5
	// The per-code count is keyed on the guess, so it cannot stop enumeration, and one
	// IPv6 /64 or a botnet has many addresses. Failed claims from everywhere share one
	// budget: at 100 per window, a pending code survives a window with odds of 9,000 to 1.
	claimFailuresGlobal = 100
	claimGlobalKey      = "claim-global"
)

// limitAddr is the address a limit is charged to: an IPv6 client controls its whole /64,
// so it is one source; IPv4 addresses stand alone.
func limitAddr(ip string) string {
	addr := net.ParseIP(ip)
	if addr == nil || addr.To4() != nil {
		return ip
	}
	return addr.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// rateLimiter is a fixed-window attempt counter.
// ponytail: per-process only; move the counts to SQLite or Redis if KyRecovery is ever load-balanced.
type rateLimiter struct {
	mu     sync.Mutex
	window time.Duration
	counts map[string]*windowCount
}

type windowCount struct {
	n       int
	resetAt time.Time
}

func newRateLimiter(window time.Duration) *rateLimiter {
	return &rateLimiter{window: window, counts: make(map[string]*windowCount)}
}

// exceeded reports whether key has already used up its failure budget for this window.
func (l *rateLimiter) exceeded(key string, limit int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	c, ok := l.counts[key]
	return ok && !now.After(c.resetAt) && c.n >= limit
}

// record charges one attempt to key. Pairing and sign-in charge only failures, so a
// host that legitimately pairs many services cannot lock itself out; backup pushes
// charge every attempt because the cost there is storage, not guessing.
func (l *rateLimiter) record(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.counts) > 1024 {
		for k, c := range l.counts {
			if now.After(c.resetAt) {
				delete(l.counts, k)
			}
		}
	}

	c, ok := l.counts[key]
	if !ok || now.After(c.resetAt) {
		c = &windowCount{resetAt: now.Add(l.window)}
		l.counts[key] = c
	}
	c.n++
}

// clientIP returns the source address of r, ignoring forwarding headers a caller can forge.
// ponytail: add trusted-proxy handling only when KyRecovery is deployed behind one.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
