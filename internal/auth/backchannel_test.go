package auth_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/auth"
)

const logoutEvent = "http://schemas.openid.net/event/backchannel-logout"

func TestVerifyLogoutToken(t *testing.T) {
	iss := newFakeIssuer(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	mgr := auth.NewManager(auth.OIDCConfig{Enabled: true, IssuerURL: iss.URL, ClientID: "kyrecovery"}, nil)
	ctx := context.Background()

	valid := func() map[string]any {
		return map[string]any{
			"iss": iss.URL, "aud": "kyrecovery", "sub": "subject", "sid": "sid-1", "jti": "jti-1",
			"iat": time.Now().Unix(), "exp": time.Now().Add(2 * time.Minute).Unix(),
			"events": map[string]any{logoutEvent: map[string]any{}},
		}
	}
	logoutHeader := func() map[string]string {
		return map[string]string{"alg": "RS256", "kid": "test-key", "typ": "logout+jwt"}
	}

	got, err := mgr.VerifyLogoutToken(ctx, signJWT(t, iss.key, logoutHeader(), valid()))
	if err != nil || got.SessionID != "sid-1" || got.Subject != "subject" || got.Issuer != iss.URL || got.IssuedAt.IsZero() {
		t.Fatalf("valid token: claims=%+v error=%v", got, err)
	}
	subOnly := valid()
	delete(subOnly, "sid")
	if got, err := mgr.VerifyLogoutToken(ctx, signJWT(t, iss.key, map[string]string{"alg": "RS256", "kid": "test-key"}, subOnly)); err != nil || got.SessionID != "" || got.Subject != "subject" {
		t.Fatalf("untyped subject-wide token: claims=%+v error=%v", got, err)
	}

	type mutate func(h map[string]string, c map[string]any) *rsa.PrivateKey
	cases := map[string]mutate{
		"foreign key":    func(h map[string]string, c map[string]any) *rsa.PrivateKey { return other },
		"wrong audience": func(h map[string]string, c map[string]any) *rsa.PrivateKey { c["aud"] = "other"; return iss.key },
		"wrong issuer": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["iss"] = "https://evil.example"
			return iss.key
		},
		"expired": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["exp"] = time.Now().Add(-time.Minute).Unix()
			return iss.key
		},
		"no exp":           func(h map[string]string, c map[string]any) *rsa.PrivateKey { delete(c, "exp"); return iss.key },
		"no iat":           func(h map[string]string, c map[string]any) *rsa.PrivateKey { delete(c, "iat"); return iss.key },
		"nonce (ID token)": func(h map[string]string, c map[string]any) *rsa.PrivateKey { c["nonce"] = "n"; return iss.key },
		"no events":        func(h map[string]string, c map[string]any) *rsa.PrivateKey { delete(c, "events"); return iss.key },
		"other event": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["events"] = map[string]any{"x": map[string]any{}}
			return iss.key
		},
		"event not object": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["events"] = map[string]any{logoutEvent: "x"}
			return iss.key
		},
		"event null": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			c["events"] = map[string]any{logoutEvent: nil}
			return iss.key
		},
		"no sid or sub": func(h map[string]string, c map[string]any) *rsa.PrivateKey {
			delete(c, "sid")
			delete(c, "sub")
			return iss.key
		},
		"JWT typ":  func(h map[string]string, c map[string]any) *rsa.PrivateKey { h["typ"] = "JWT"; return iss.key },
		"alg none": func(h map[string]string, c map[string]any) *rsa.PrivateKey { h["alg"] = "none"; return iss.key },
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			h, c := logoutHeader(), valid()
			assertRejected(t, mgr, signJWT(t, m(h, c), h, c))
		})
	}
	idToken := iss.sign(t, iss.key, map[string]any{"iss": iss.URL, "aud": "kyrecovery", "sub": "subject", "sid": "sid-1", "nonce": "n",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()})
	for name, raw := range map[string]string{"garbage": "not.a.jwt", "empty": "", "two parts": "a.b", "ID token": idToken} {
		t.Run(name, func(t *testing.T) { assertRejected(t, mgr, raw) })
	}
}

func assertRejected(t *testing.T, mgr *auth.Manager, raw string) {
	t.Helper()
	if got, err := mgr.VerifyLogoutToken(context.Background(), raw); !errors.Is(err, auth.ErrInvalidLogoutToken) || got != nil {
		t.Fatalf("accepted: claims=%+v error=%v", got, err)
	}
}

func signJWT(t *testing.T, key *rsa.PrivateKey, header map[string]string, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(header)
	c, _ := json.Marshal(claims)
	input := b64u(h) + "." + b64u(c)
	digest := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + b64u(sig)
}
