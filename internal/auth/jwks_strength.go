package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	josejson "github.com/go-jose/go-jose/v4/json"
)

// This client belongs only to the upstream OIDC verifier's JWKS fetches. Keep
// discovery, caching, rotation, signatures and claims in coreos/go-oidc.
type strongJWKSTransport struct{ base http.RoundTripper }

func (t strongJWKSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		resp.Body.Close()
		return nil, errors.New("OIDC JWKS redirects are refused")
	}
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}
	defer resp.Body.Close()
	const maxJWKSBytes = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxJWKSBytes {
		return nil, errors.New("OIDC JWKS exceeds 1 MiB")
	}
	var set struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, errors.New("invalid OIDC JWKS")
	}
	for _, rawKey := range set.Keys {
		// Match go-oidc's envelope parsing and go-jose's case-sensitive JWK
		// members, so the floor checks the modulus the verifier actually uses.
		var key struct {
			Type string `json:"kty"`
			N    string `json:"n"`
		}
		if err := josejson.Unmarshal(rawKey, &key); err != nil {
			return nil, errors.New("invalid OIDC JWK")
		}
		if key.Type != "RSA" {
			continue
		}
		n, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil || new(big.Int).SetBytes(n).BitLen() < 2048 {
			return nil, errors.New("OIDC JWKS RSA keys must be at least 2048 bits")
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	return resp, nil
}

// spacedTransport refuses a request made within every of the previous one, so a token
// naming an unknown key cannot make each anonymous request fetch the JWKS. go-oidc keeps
// its cached keys when a refresh fails.
type spacedTransport struct {
	base  http.RoundTripper
	every time.Duration
	mu    sync.Mutex
	last  time.Time
}

func (t *spacedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	if !t.last.IsZero() && time.Since(t.last) < t.every {
		t.mu.Unlock()
		return nil, errors.New("OIDC JWKS was fetched moments ago")
	}
	t.last = time.Now()
	t.mu.Unlock()
	return t.base.RoundTrip(req)
}
