package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	josejson "github.com/go-jose/go-jose/v4/json"
)

const (
	// algMLDSA65 is the RFC 9964 / JOSE identifier for ML-DSA-65.
	algMLDSA65 = "ML-DSA-65"
	// maxJWKSCacheTTL bounds how long cached JWKS keys stay valid before a forced refresh.
	maxJWKSCacheTTL = 5 * time.Minute
)

// supportedOIDCSigningAlgs is the asymmetric signing algorithm allowlist accepted from
// OIDC providers. Symmetric MACs and alg:none are excluded.
var supportedOIDCSigningAlgs = []jose.SignatureAlgorithm{
	jose.RS256, jose.RS384, jose.RS512,
	jose.PS256, jose.PS384, jose.PS512,
	jose.ES256, jose.ES384, jose.ES512,
	jose.EdDSA,
	jose.SignatureAlgorithm(algMLDSA65),
}

// This client belongs only to the upstream OIDC verifier's JWKS fetches.
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
// naming an unknown key cannot make each anonymous request fetch the JWKS.
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

// remoteJWKSKeySet implements oidc.KeySet using go-jose/v4 directly so RFC 9964
// kty:"AKP" / alg:"ML-DSA-65" keys work alongside classical RSA, ECDSA, and EdDSA keys
// (coreos/go-oidc v3.21.0 drops ML-DSA-65 keys in its built-in RemoteKeySet).
type remoteJWKSKeySet struct {
	jwksURL string
	client  *http.Client

	mu        sync.RWMutex
	keys      []jose.JSONWebKey
	fetchedAt time.Time
	inflight  *jwksRefreshCall
}

type jwksRefreshCall struct {
	done chan struct{}
	keys []jose.JSONWebKey
	err  error
}

func newRemoteJWKSKeySet(client *http.Client, jwksURL string) *remoteJWKSKeySet {
	return &remoteJWKSKeySet{
		jwksURL: jwksURL,
		client:  client,
	}
}

func (k *remoteJWKSKeySet) VerifySignature(ctx context.Context, rawJWT string) ([]byte, error) {
	jws, err := jose.ParseSigned(rawJWT, supportedOIDCSigningAlgs)
	if err != nil {
		return nil, fmt.Errorf("malformed jwt: %w", err)
	}
	if len(jws.Signatures) != 1 {
		return nil, errors.New("jwt must carry exactly one signature")
	}
	hdr := jws.Signatures[0].Protected
	if hdr.Algorithm == algMLDSA65 && hdr.KeyID == "" {
		return nil, errors.New("ML-DSA-65 token missing kid header")
	}

	keys, fresh := k.cachedKeys()
	if fresh {
		if payload, ok := verifyWithJWKSet(jws, keys); ok {
			return payload, nil
		}
	}

	keys, err = k.refresh(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching OIDC JWKS: %w", err)
	}
	if payload, ok := verifyWithJWKSet(jws, keys); ok {
		return payload, nil
	}
	return nil, errors.New("failed to verify id token signature")
}

func (k *remoteJWKSKeySet) cachedKeys() ([]jose.JSONWebKey, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if len(k.keys) == 0 || time.Since(k.fetchedAt) >= maxJWKSCacheTTL {
		return nil, false
	}
	return k.keys, true
}

func (k *remoteJWKSKeySet) refresh(ctx context.Context) ([]jose.JSONWebKey, error) {
	k.mu.Lock()
	if call := k.inflight; call != nil {
		k.mu.Unlock()
		select {
		case <-call.done:
			return call.keys, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &jwksRefreshCall{done: make(chan struct{})}
	k.inflight = call
	k.mu.Unlock()

	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), oidcDiscoveryTimeout)
	defer cancel()

	keys, err := k.fetchKeys(fetchCtx)

	k.mu.Lock()
	if err == nil {
		k.keys = keys
		k.fetchedAt = time.Now()
	}
	k.inflight = nil
	call.keys = keys
	call.err = err
	close(call.done)
	k.mu.Unlock()

	return keys, err
}

func (k *remoteJWKSKeySet) fetchKeys(ctx context.Context) ([]jose.JSONWebKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := k.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}
	const maxJWKSBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxJWKSBytes {
		return nil, errors.New("OIDC JWKS exceeds 1 MiB")
	}
	var set jose.JSONWebKeySet
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("decoding OIDC JWKS: %w", err)
	}
	return set.Keys, nil
}

func verifyWithJWKSet(jws *jose.JSONWebSignature, keys []jose.JSONWebKey) ([]byte, bool) {
	hdr := jws.Signatures[0].Protected
	for _, key := range keys {
		if !key.Valid() {
			continue
		}
		if key.Use != "" && key.Use != "sig" {
			continue
		}
		if hdr.KeyID != "" && key.KeyID != hdr.KeyID {
			continue
		}
		if key.Algorithm != "" && key.Algorithm != hdr.Algorithm {
			continue
		}
		if payload, err := jws.Verify(&key); err == nil {
			return payload, true
		}
	}
	return nil, false
}

// allowedOIDCSigningAlgs returns the intersection of supportedOIDCSigningAlgs and the
// provider's discovered id_token_signing_alg_values_supported (plus RS256 and ML-DSA-65
// fallback when the discovery list is omitted).
func allowedOIDCSigningAlgs(discovered []string) []string {
	supported := make(map[string]struct{}, len(supportedOIDCSigningAlgs))
	for _, a := range supportedOIDCSigningAlgs {
		supported[string(a)] = struct{}{}
	}
	if len(discovered) == 0 {
		return []string{string(jose.RS256), algMLDSA65}
	}
	seen := make(map[string]struct{}, len(discovered))
	algs := make([]string, 0, len(discovered))
	for _, a := range discovered {
		if _, ok := supported[a]; !ok {
			continue
		}
		if _, dup := seen[a]; dup {
			continue
		}
		seen[a] = struct{}{}
		algs = append(algs, a)
	}
	if len(algs) == 0 {
		return []string{string(jose.RS256)}
	}
	return algs
}
