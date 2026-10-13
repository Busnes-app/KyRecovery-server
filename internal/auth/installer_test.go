package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstallerVerifiedOIDCSubjectAndNoFallback(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	idp := httptest.NewTLSServer(mux)
	defer idp.Close()
	original := httpClient
	httpClient = func() *http.Client { return idp.Client() }
	defer func() { httpClient = original }()
	enc := base64.RawURLEncoding.EncodeToString
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": idp.URL, "authorization_endpoint": idp.URL + "/authorize", "token_endpoint": idp.URL + "/token", "jwks_uri": idp.URL + "/jwks", "userinfo_endpoint": idp.URL + "/userinfo", "id_token_signing_alg_values_supported": []string{"RS256"}})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "fixture", "alg": "RS256", "use": "sig", "n": enc(key.N.Bytes()), "e": enc(big.NewInt(int64(key.E)).Bytes())}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		code := r.FormValue("code")
		if code == "missing-token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fixture"})
			return
		}
		header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "fixture"})
		claims, _ := json.Marshal(map[string]any{"iss": idp.URL, "sub": code, "aud": "recovery", "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": "fixture-nonce", "roles": []string{"admin"}, "email": "owner@example.test"})
		payload := enc(header) + "." + enc(claims)
		sum := sha256.Sum256([]byte(payload))
		signature, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		if e != nil {
			t.Error(e)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fixture", "id_token": payload + "." + enc(signature)})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		t.Error("managed login used userinfo fallback")
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": "owner", "role": "admin"})
	})
	database, err := db.Open(filepath.Join(t.TempDir(), "recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dir := t.TempDir()
	client := filepath.Join(dir, "client")
	directory := filepath.Join(dir, "directory")
	for _, p := range []string{client, directory} {
		if err := os.WriteFile(p, []byte("private-test-credential"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b := applysetup.Common{Schema: applysetup.Schema, Service: "kyrecovery", SetupID: "12345678-1234-1234-1234-123456789abc", SSO: &applysetup.SSO{IssuerURL: idp.URL, ClientID: "recovery", ClientSecretFile: client, DirectorySecretFile: directory, RedirectURI: "https://recovery.example/api/auth/callback"}, Admins: []applysetup.Identity{{Issuer: idp.URL, Subject: "owner", Username: "owner-admin"}}}
	if err = audit.NewLedger(database).ApplyInstaller(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(OIDCConfig{}, database)
	for _, code := range []string{"owner", "daily", "unassigned", "missing-token"} {
		profile, e := mgr.ExchangeCode(t.Context(), code, "verifier", "fixture-nonce")
		if code == "owner" {
			if e != nil || profile.Role != RoleAdmin || profile.Issuer != idp.URL || profile.Subject != code {
				t.Fatal("owner profile", profile, e)
			}
		} else if e == nil {
			t.Fatal("unassigned or unverified authority", code, profile)
		}
	}
}
