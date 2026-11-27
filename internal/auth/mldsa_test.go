package auth_test

import (
	"context"
	"crypto"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/auth"
)

func signMLDSA65JWT(t *testing.T, priv *mldsa.PrivateKey, header map[string]string, claims map[string]any) string {
	t.Helper()
	h, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	c, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signingInput := b64u(h) + "." + b64u(c)
	sig, err := priv.Sign(rand.Reader, []byte(signingInput), crypto.Hash(0))
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + b64u(sig)
}

func TestOIDCVerifiesMLDSA65IDAndLogoutTokens(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	mldsaKey, err := mldsa.GenerateKey(mldsa.MLDSA65())
	if err != nil {
		t.Fatal(err)
	}
	otherMLDSA, err := mldsa.GenerateKey(mldsa.MLDSA65())
	if err != nil {
		t.Fatal(err)
	}

	const clientID = "kyrecovery"
	const nonce = "nonce-mldsa-65"
	tokens := map[string]string{}

	var srvURL string
	mux := http.NewServeMux()
	srv := httptest.NewTLSServer(mux)
	srvURL = srv.URL
	auth.UseIssuerClientForTest(t, srv.Client())
	t.Cleanup(srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                srvURL,
			"authorization_endpoint":                srvURL + "/oauth/authorize",
			"token_endpoint":                        srvURL + "/oauth/token",
			"userinfo_endpoint":                     srvURL + "/oauth/userinfo",
			"jwks_uri":                              srvURL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256", "ML-DSA-65"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA", "kid": "rsa-1", "alg": "RS256", "use": "sig",
					"n": b64u(rsaKey.N.Bytes()), "e": b64u(big.NewInt(int64(rsaKey.E)).Bytes()),
				},
				{
					"kty": "AKP", "kid": "mldsa-1", "alg": "ML-DSA-65", "use": "sig",
					"pub": b64u(mldsaKey.PublicKey().Bytes()),
				},
			},
		})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		idTok, ok := tokens[r.FormValue("code")]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     idTok,
		})
	})

	idClaims := map[string]any{
		"iss":            srvURL,
		"aud":            clientID,
		"sub":            "pq-user",
		"email":          "pq@example.com",
		"email_verified": true,
		"name":           "PQ Operator",
		"nonce":          nonce,
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
	}
	validID := signMLDSA65JWT(t, mldsaKey, map[string]string{"alg": "ML-DSA-65", "typ": "JWT", "kid": "mldsa-1"}, idClaims)
	tokens["valid-mldsa"] = validID
	tokens["wrong-key"] = signMLDSA65JWT(t, otherMLDSA, map[string]string{"alg": "ML-DSA-65", "typ": "JWT", "kid": "mldsa-1"}, idClaims)
	tokens["missing-kid"] = signMLDSA65JWT(t, mldsaKey, map[string]string{"alg": "ML-DSA-65", "typ": "JWT"}, idClaims)
	tokens["rsa-kid-mismatch"] = signMLDSA65JWT(t, mldsaKey, map[string]string{"alg": "ML-DSA-65", "typ": "JWT", "kid": "rsa-1"}, idClaims)

	parts := strings.Split(validID, ".")
	tamperedSig := parts[2][:len(parts[2])-2] + "AA"
	if tamperedSig == parts[2] {
		tamperedSig = parts[2][:len(parts[2])-2] + "BB"
	}
	tokens["tampered"] = parts[0] + "." + parts[1] + "." + tamperedSig

	mgr := auth.NewManager(auth.OIDCConfig{
		Enabled:     true,
		IssuerURL:   srvURL,
		ClientID:    clientID,
		RedirectURL: "https://recovery.example/api/auth/callback",
	}, nil)

	ctx := context.Background()

	// 1. Valid ML-DSA-65 ID token verifies via ExchangeCode.
	info, err := mgr.ExchangeCode(ctx, "valid-mldsa", "verifier", nonce)
	if err != nil {
		t.Fatalf("ExchangeCode(valid-mldsa) failed: %v", err)
	}
	if info.Subject != "pq-user" || info.Email != "pq@example.com" {
		t.Fatalf("unexpected UserInfo: %+v", info)
	}

	// 2. Tampered, wrong-key, missing-kid, and cross-algorithm kid tokens fail.
	for _, code := range []string{"tampered", "wrong-key", "missing-kid", "rsa-kid-mismatch"} {
		if _, err := mgr.ExchangeCode(ctx, code, "verifier", nonce); err == nil {
			t.Fatalf("ExchangeCode(%s) unexpectedly succeeded", code)
		}
	}

	// 3. Valid ML-DSA-65 back-channel logout token verifies via VerifyLogoutToken.
	logoutClaims := map[string]any{
		"iss":    srvURL,
		"aud":    clientID,
		"sub":    "pq-user",
		"sid":    "sid-pq",
		"jti":    "jti-pq",
		"iat":    time.Now().Unix(),
		"exp":    time.Now().Add(2 * time.Minute).Unix(),
		"events": map[string]any{logoutEvent: map[string]any{}},
	}
	logoutTok := signMLDSA65JWT(t, mldsaKey, map[string]string{"alg": "ML-DSA-65", "typ": "logout+jwt", "kid": "mldsa-1"}, logoutClaims)
	got, err := mgr.VerifyLogoutToken(ctx, logoutTok)
	if err != nil || got.SessionID != "sid-pq" || got.Subject != "pq-user" {
		t.Fatalf("VerifyLogoutToken(ML-DSA-65): got=%+v err=%v", got, err)
	}
}
