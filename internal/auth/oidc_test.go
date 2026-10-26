package auth_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/auth"
)

// fakeIssuer is a minimal OIDC provider: discovery, JWKS and a token endpoint
// that hands back whichever ID token the test asked for by authorization code.
type fakeIssuer struct {
	*httptest.Server
	key          *rsa.PrivateKey
	tokens       map[string]string
	fetches      atomic.Int32
	authEndpoint string
	jwkBefore    string
	jwkAfter     string
}

func newFakeIssuer(t *testing.T, bits ...int) *fakeIssuer {
	t.Helper()
	size := 2048
	if len(bits) > 0 {
		size = bits[0]
	}
	key, err := rsa.GenerateKey(rand.Reader, size)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}

	iss := &fakeIssuer{key: key, tokens: map[string]string{}}
	mux := http.NewServeMux()
	iss.Server = httptest.NewTLSServer(mux)
	auth.UseIssuerClientForTest(t, iss.Client())
	t.Cleanup(iss.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"issuer":                                iss.URL,
			"authorization_endpoint":                authorizationEndpoint(iss),
			"token_endpoint":                        iss.URL + "/oauth/token",
			"userinfo_endpoint":                     iss.URL + "/oauth/userinfo",
			"jwks_uri":                              iss.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		iss.fetches.Add(1)
		jwk, err := json.Marshal(map[string]string{
			"kty": "RSA", "kid": "test-key", "alg": "RS256", "use": "sig",
			"n": b64u(key.N.Bytes()), "e": b64u(big.NewInt(int64(key.E)).Bytes()),
		})
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = fmt.Fprintf(w, `{"keys":[{%s%s%s}]}`, iss.jwkBefore, jwk[1:len(jwk)-1], iss.jwkAfter)
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		idToken, ok := iss.tokens[r.FormValue("code")]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     idToken,
		})
	})
	return iss
}

func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// sign produces an RS256 JWT with the issuer's key.
func (f *fakeIssuer) sign(t *testing.T, key *rsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": "test-key"})
	payload, _ := json.Marshal(claims)
	signingInput := b64u(header) + "." + b64u(payload)

	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}
	return signingInput + "." + b64u(sig)
}

// unsigned mimics the token an attacker crafts when claims are read without
// verifying the signature at all.
func unsigned(claims map[string]interface{}) string {
	header, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	return b64u(header) + "." + b64u(payload) + "."
}

func TestExchangeCodeVerifiesIDToken(t *testing.T) {
	iss := newFakeIssuer(t)
	const clientID = "kyrecovery"
	const nonce = "nonce-abc123"

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}

	base := func() map[string]interface{} {
		return map[string]interface{}{
			"iss":   iss.URL,
			"aud":   clientID,
			"sub":   "user-1",
			"email": "operator@example.com",
			"name":  "Ops Person",
			"nonce": nonce,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
		}
	}
	withClaim := func(k string, v interface{}) map[string]interface{} {
		c := base()
		c[k] = v
		return c
	}

	// An attacker-controlled token endpoint claiming to be an admin.
	forged := base()
	forged["role"] = "admin"

	iss.tokens["valid"] = iss.sign(t, iss.key, base())
	iss.tokens["unsigned"] = unsigned(forged)
	iss.tokens["wrong-key"] = iss.sign(t, otherKey, forged)
	iss.tokens["wrong-audience"] = iss.sign(t, iss.key, withClaim("aud", "some-other-client"))
	iss.tokens["wrong-issuer"] = iss.sign(t, iss.key, withClaim("iss", "https://evil.example"))
	iss.tokens["expired"] = iss.sign(t, iss.key, withClaim("exp", time.Now().Add(-time.Hour).Unix()))
	iss.tokens["replayed-nonce"] = iss.sign(t, iss.key, withClaim("nonce", "a-different-login"))
	iss.tokens["no-nonce"] = iss.sign(t, iss.key, withClaim("nonce", ""))

	mgr := auth.NewManager(auth.OIDCConfig{
		Enabled:     true,
		IssuerURL:   iss.URL,
		ClientID:    clientID,
		RedirectURL: "http://localhost:8095/api/auth/callback",
	}, nil)

	ctx := context.Background()

	// The honest path still works.
	info, err := mgr.ExchangeCode(ctx, "valid", "verifier", nonce)
	if err != nil {
		t.Fatalf("valid ID token was rejected: %v", err)
	}
	if info.Subject != "user-1" || info.Email != "operator@example.com" {
		t.Fatalf("unexpected profile: %+v", info)
	}
	// A token with no role claim must not silently become an operator.
	if info.Role != auth.RoleViewer {
		t.Fatalf("expected least-privilege role for a token with no role claim, got %q", info.Role)
	}

	for _, code := range []string{"unsigned", "wrong-key", "wrong-audience", "wrong-issuer", "expired", "replayed-nonce", "no-nonce"} {
		if info, err := mgr.ExchangeCode(ctx, code, "verifier", nonce); err == nil {
			t.Errorf("%s ID token was accepted and produced role %q", code, info.Role)
		}
	}
}

// TestExchangeCodeRejectsMissingIDTokenWithoutUserinfo proves a failed token
// validation is never papered over by a second, unverified profile source.
func TestExchangeCodeRejectsMissingIDTokenWithoutUserinfo(t *testing.T) {
	iss := newFakeIssuer(t)
	iss.tokens["no-id-token"] = "" // token endpoint returns an empty id_token

	mgr := auth.NewManager(auth.OIDCConfig{
		Enabled: true, IssuerURL: iss.URL, ClientID: "kyrecovery",
	}, nil)

	if _, err := mgr.ExchangeCode(context.Background(), "no-id-token", "verifier", "nonce"); err == nil {
		t.Fatal("expected failure when the issuer returns no id_token and no working userinfo endpoint")
	}
}

func TestTestSSOConnectionRequiresDiscovery(t *testing.T) {
	mgr := auth.NewManager(auth.OIDCConfig{}, nil)
	ctx := context.Background()

	// A server that answers, but publishes no OIDC metadata, is not a usable issuer.
	bare := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer bare.Close()
	if err := mgr.TestSSOConnection(ctx, bare.URL); err == nil {
		t.Fatal("a plain HTTP responder was reported as a working KySignOn issuer")
	}

	// Schemes that are not http(s) never reach the network.
	for _, target := range []string{"file:///etc/passwd", "gopher://127.0.0.1:70/x", "ftp://internal/", ""} {
		if err := mgr.TestSSOConnection(ctx, target); err == nil {
			t.Errorf("issuer URL %q should have been rejected", target)
		}
	}

	// Link-local addresses (cloud metadata) are refused at dial time.
	err := mgr.TestSSOConnection(ctx, "https://169.254.169.254")
	if err == nil || !strings.Contains(fmt.Sprint(err), "169.254.169.254") {
		t.Fatalf("cloud metadata address should be blocked, got %v", err)
	}

	iss := newFakeIssuer(t)
	if err := mgr.TestSSOConnection(ctx, iss.URL); err != nil {
		t.Fatalf("a real issuer should pass: %v", err)
	}
}

func TestOIDCSigningKeyFloor(t *testing.T) {
	for _, bits := range []int{1024, 2047, 2048} {
		t.Run(fmt.Sprint(bits), func(t *testing.T) {
			iss := newFakeIssuer(t, bits)
			iss.tokens["fixture"] = iss.sign(t, iss.key, map[string]any{"iss": iss.URL, "aud": "recovery", "sub": "subject", "nonce": "nonce", "exp": time.Now().Add(time.Hour).Unix()})
			for _, variant := range []struct{ name, before, after string }{
				{name: "canonical"},
				{name: "uppercase type", after: `,"KTY":"EC"`},
				{name: "mixed-case type", after: `,"Kty":"EC"`},
				{name: "uppercase modulus", after: `,"N":"` + b64u(new(big.Int).Lsh(big.NewInt(1), 2047).Bytes()) + `"`},
				{name: "uppercase short modulus", after: `,"N":"AQ"`},
				{name: "aliases before canonical", before: `"KTY":"EC","N":"AQ",`},
				{name: "type and modulus aliases", after: `,"KTY":"EC","N":"AQ"`},
			} {
				t.Run(variant.name, func(t *testing.T) {
					iss.jwkBefore, iss.jwkAfter = variant.before, variant.after
					fetches := iss.fetches.Load()
					mgr := auth.NewManager(auth.OIDCConfig{Enabled: true, IssuerURL: iss.URL, ClientID: "recovery"}, nil)
					for i := 0; i < 2; i++ {
						got, err := mgr.ExchangeCode(t.Context(), "fixture", "verifier", "nonce")
						if bits < 2048 {
							if err == nil || got != nil {
								t.Fatalf("accepted RSA-%d token: %+v", bits, got)
							}
							if !strings.Contains(err.Error(), "2048") {
								t.Fatalf("wrong refusal: %v", err)
							}
							return
						}
						if err != nil || got.Subject != "subject" {
							t.Fatalf("strong ID token: %+v %v", got, err)
						}
					}
					if delta := iss.fetches.Load() - fetches; delta != 1 {
						t.Fatalf("JWKS cache lost: %d fetches", delta)
					}
				})
			}
		})
	}
}

func TestMissingIDTokenDoesNotUseUserinfo(t *testing.T) {
	iss := newFakeIssuer(t)
	iss.tokens["missing"] = ""
	var calls atomic.Int32
	iss.Config.Handler.(*http.ServeMux).HandleFunc("/oauth/userinfo", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{"sub": "subject", "role": "admin"})
	})
	mgr := auth.NewManager(auth.OIDCConfig{Enabled: true, IssuerURL: iss.URL, ClientID: "recovery"}, nil)
	got, err := mgr.ExchangeCode(t.Context(), "missing", "verifier", "nonce")
	if err == nil || got != nil || calls.Load() != 0 {
		t.Fatalf("missing token reached userinfo or authenticated: %+v %v calls=%d", got, err, calls.Load())
	}
}

func authorizationEndpoint(iss *fakeIssuer) string {
	if iss.authEndpoint != "" {
		return iss.authEndpoint
	}
	return iss.URL + "/oauth/authorize"
}

func TestAuthorizationEndpointRetainsQuery(t *testing.T) {
	iss := newFakeIssuer(t)
	iss.authEndpoint = iss.URL + "/authorize?audience=retained&state=untrusted-default"
	mgr := auth.NewManager(auth.OIDCConfig{Enabled: true, IssuerURL: iss.URL, ClientID: "recovery"}, nil)
	target, err := mgr.BuildAuthURL(t.Context(), "actual-state", "actual-nonce", "challenge")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("audience") != "retained" || u.Query().Get("state") != "actual-state" || u.Query().Get("nonce") != "actual-nonce" {
		t.Fatalf("authorization parameters changed: %v", u.Query())
	}
}
