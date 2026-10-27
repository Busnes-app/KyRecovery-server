package server_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

// TestMain makes the issuer client trust httptest's fixed TLS certificate. The client uses
// the system roots, which are read once, so this has to happen before any test runs.
func TestMain(m *testing.M) {
	idp := httptest.NewTLSServer(http.NotFoundHandler())
	dir, err := os.MkdirTemp("", "kyrecovery-ca")
	if err != nil {
		panic(err)
	}
	ca := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: idp.Certificate().Raw}), 0o600); err != nil {
		panic(err)
	}
	idp.Close()
	os.Setenv("SSL_CERT_FILE", ca)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// fakeKyIdentity is a TLS identity provider whose token endpoint returns the next ID token.
type fakeKyIdentity struct {
	key     *rsa.PrivateKey
	issuer  string
	idToken string
}

func newFakeKyIdentity(t *testing.T) *fakeKyIdentity {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeKyIdentity{key: key}
	idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": f.issuer, "authorization_endpoint": f.issuer + "/authorize",
				"token_endpoint": f.issuer + "/token", "jwks_uri": f.issuer + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{
				"kty": "RSA", "alg": "RS256", "kid": "k",
				"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB",
			}}})
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "a", "token_type": "Bearer", "id_token": f.idToken})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(idp.Close)
	f.issuer = idp.URL
	return f
}

func (f *fakeKyIdentity) sign(t *testing.T, typ string, claims map[string]any) string {
	t.Helper()
	claims["iss"], claims["aud"] = f.issuer, "kyrecovery"
	claims["iat"], claims["exp"] = time.Now().Unix(), time.Now().Add(2*time.Minute).Unix()
	header := map[string]string{"alg": "RS256", "kid": "k"}
	if typ != "" {
		header["typ"] = typ
	}
	h, _ := json.Marshal(header)
	c, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	digest := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (f *fakeKyIdentity) logoutToken(t *testing.T, sub, sid string) string {
	claims := map[string]any{"sub": sub, "jti": sid + sub, "events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}}
	if sid != "" {
		claims["sid"] = sid
	}
	return f.sign(t, "logout+jwt", claims)
}

// ssoLogin runs the KyIdentity login and callback for sub/sid and returns the callback response.
func ssoLogin(t *testing.T, srv *server.Server, f *fakeKyIdentity, sub, sid string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/login", nil))
	loc, err := url.Parse(w.Header().Get("Location"))
	if w.Code != http.StatusFound || err != nil {
		t.Fatalf("login redirect: %d %s", w.Code, w.Body.String())
	}
	q := loc.Query()
	f.idToken = f.sign(t, "", map[string]any{"sub": sub, "sid": sid, "nonce": q.Get("nonce"), "email": sub + "@example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=c&state="+q.Get("state"), nil)
	for _, c := range w.Result().Cookies() {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func issuedCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			return c
		}
	}
	return nil
}

func signedIn(srv *server.Server, c *http.Cookie) bool {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(c)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var me struct{ Authenticated bool }
	return json.Unmarshal(w.Body.Bytes(), &me) == nil && me.Authenticated
}

func postLogout(srv *server.Server, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/backchannel-logout", strings.NewReader(url.Values{"logout_token": {token}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestBackchannelLogoutEndsTheSSOLogin(t *testing.T) {
	f := newFakeKyIdentity(t)
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	srv, err := server.New(server.Config{Port: 8095, DataDir: t.TempDir(), Auth: auth.OIDCConfig{
		Enabled: true, IssuerURL: f.issuer, ClientID: "kyrecovery", RedirectURL: "http://localhost:8095/api/auth/callback",
	}}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	ctx := context.Background()

	ended := issuedCookie(ssoLogin(t, srv, f, "alice", "sid-a1"))
	kept := issuedCookie(ssoLogin(t, srv, f, "alice", "sid-a2"))
	if ended == nil || kept == nil || !signedIn(srv, ended) || !signedIn(srv, kept) {
		t.Fatal("SSO login did not issue working sessions")
	}
	// The same sid from another issuer names another login.
	foreign := db.SessionRecord{ID: "foreign-issuer-session", UserID: "alice", Role: auth.RoleViewer, SSOIssuer: "https://other.example",
		SSOSubject: "alice", SSOSID: "sid-a1", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now().Add(-time.Hour)}
	if err := database.InsertSession(ctx, foreign); err != nil {
		t.Fatal(err)
	}

	// An ID token is not a logout token, even when signed by the provider.
	if w := postLogout(srv, f.idToken); w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_request") {
		t.Fatalf("ID token as logout token: got %d %s", w.Code, w.Body.String())
	}
	// A valid token in a body over 16 KiB is refused unread.
	big := url.Values{"logout_token": {f.logoutToken(t, "alice", "sid-a1")}, "pad": {strings.Repeat("a", 16<<10)}}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/backchannel-logout", strings.NewReader(big.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	if srv.ServeHTTP(w, req); w.Code != http.StatusBadRequest || !signedIn(srv, ended) {
		t.Fatalf("oversized body: got %d", w.Code)
	}
	w = postLogout(srv, f.logoutToken(t, "alice", "sid-a1"))
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("logout: got %d %q %s", w.Code, w.Header().Get("Cache-Control"), w.Body.String())
	}
	if signedIn(srv, ended) {
		t.Fatal("session survived its back-channel logout")
	}
	if !signedIn(srv, kept) {
		t.Fatal("logout of one login ended another login of the same user")
	}
	if s, err := database.GetSession(ctx, foreign.ID); err != nil || s == nil {
		t.Fatalf("logout ended another issuer's login with the same sid: %v", err)
	}
	// A replay is harmless and still acknowledged.
	if w := postLogout(srv, f.logoutToken(t, "alice", "sid-a1")); w.Code != http.StatusOK {
		t.Fatalf("replayed logout: got %d", w.Code)
	}

	// A logout that lands before the callback finishes fences that login.
	if w := postLogout(srv, f.logoutToken(t, "alice", "sid-late")); w.Code != http.StatusOK {
		t.Fatalf("early logout: got %d", w.Code)
	}
	if w := ssoLogin(t, srv, f, "alice", "sid-late"); w.Code != http.StatusUnauthorized || issuedCookie(w) != nil {
		t.Fatalf("callback revived an ended login: %d", w.Code)
	}

	// A subject-wide token ends the user's logins issued by then, and nobody else's.
	bob := issuedCookie(ssoLogin(t, srv, f, "bob", "sid-b1"))
	later := db.SessionRecord{ID: "alice-after-logout", UserID: "alice", Role: auth.RoleViewer, SSOIssuer: f.issuer,
		SSOSubject: "alice", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now().Add(time.Minute)}
	if err := database.InsertSession(ctx, later); err != nil {
		t.Fatal(err)
	}
	if w := postLogout(srv, f.logoutToken(t, "alice", "")); w.Code != http.StatusOK {
		t.Fatalf("subject-wide logout: got %d", w.Code)
	}
	if signedIn(srv, kept) || !signedIn(srv, bob) {
		t.Fatal("subject-wide logout ended the wrong sessions")
	}
	if s, err := database.GetSession(ctx, later.ID); err != nil || s == nil {
		t.Fatalf("subject-wide logout ended a login issued after it: %v", err)
	}
	if s, err := database.GetSession(ctx, foreign.ID); err != nil || s == nil {
		t.Fatalf("subject-wide logout ended another issuer's login: %v", err)
	}
	if w := postLogout(srv, f.logoutToken(t, "nobody", "")); w.Code != http.StatusOK {
		t.Fatalf("logout for an unknown subject: got %d", w.Code)
	}

	events, err := database.ListAuditEvents(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	var audited bool
	for _, e := range events {
		audited = audited || (e.Action == "user_logged_out_sso" && e.Actor == f.issuer)
	}
	if !audited {
		t.Fatal("back-channel logout was not audited")
	}
}
