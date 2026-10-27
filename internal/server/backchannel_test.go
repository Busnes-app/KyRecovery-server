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
	"sync"
	"sync/atomic"
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
	// discoveries and jwksFetches count the provider's outbound fetches.
	discoveries, jwksFetches atomic.Int32
	// beforeDiscovery, when set, runs before the discovery document is served.
	beforeDiscovery func()
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
			f.discoveries.Add(1)
			if f.beforeDiscovery != nil {
				f.beforeDiscovery()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": f.issuer, "authorization_endpoint": f.issuer + "/authorize",
				"token_endpoint": f.issuer + "/token", "jwks_uri": f.issuer + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			f.jwksFetches.Add(1)
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
	claims["exp"] = time.Now().Add(2 * time.Minute).Unix()
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = time.Now().Unix()
	}
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
	return ssoLoginAt(t, srv, f, sub, sid, time.Now())
}

// ssoLoginAt is ssoLogin with an ID token the provider issued at iat.
func ssoLoginAt(t *testing.T, srv *server.Server, f *fakeKyIdentity, sub, sid string, iat time.Time) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/login", nil))
	loc, err := url.Parse(w.Header().Get("Location"))
	if w.Code != http.StatusFound || err != nil {
		t.Fatalf("login redirect: %d %s", w.Code, w.Body.String())
	}
	q := loc.Query()
	f.idToken = f.sign(t, "", map[string]any{"sub": sub, "sid": sid, "nonce": q.Get("nonce"), "email": sub + "@example.com", "iat": iat.Unix()})
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

// newSSOServer is a server whose SSO issuer is issuer.
func newSSOServer(t *testing.T, issuer string) *server.Server {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	srv, err := server.New(server.Config{Port: 8095, DataDir: t.TempDir(), Auth: auth.OIDCConfig{
		Enabled: true, IssuerURL: issuer, ClientID: "kyrecovery", RedirectURL: "http://localhost:8095/api/auth/callback",
	}}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv
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

	// A subject-wide logout fences a callback already holding an ID token issued before
	// it, even for a subject with no session here yet; a later login still works.
	carolAt := time.Now().Add(-30 * time.Second)
	carolLogout := f.sign(t, "logout+jwt", map[string]any{"sub": "carol", "jti": "carol", "iat": carolAt.Unix(),
		"events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}})
	if w := postLogout(srv, carolLogout); w.Code != http.StatusOK {
		t.Fatalf("subject-wide logout of an unprovisioned subject: got %d", w.Code)
	}
	if w := ssoLoginAt(t, srv, f, "carol", "sid-c1", time.Now().Add(-time.Minute)); w.Code != http.StatusUnauthorized || issuedCookie(w) != nil {
		t.Fatalf("pending callback survived a subject-wide logout: %d", w.Code)
	}
	if w := ssoLoginAt(t, srv, f, "carol", "sid-c2", carolAt); w.Code != http.StatusUnauthorized || issuedCookie(w) != nil {
		t.Fatalf("callback from the logout's own second survived it: %d", w.Code)
	}
	if w := ssoLoginAt(t, srv, f, "carol", "sid-c3", time.Now().Add(5*time.Second)); w.Code != http.StatusFound || !signedIn(srv, issuedCookie(w)) {
		t.Fatalf("login issued after the subject-wide logout was refused: %d", w.Code)
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

// TestBackchannelLogoutFetchesNothingPerRequest: the route is anonymous and not rate
// limited, so neither forged tokens nor a failing issuer may cost an outbound fetch each.
func TestBackchannelLogoutFetchesNothingPerRequest(t *testing.T) {
	f := newFakeKyIdentity(t)
	var failedDiscoveries atomic.Int32
	failing := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failedDiscoveries.Add(1)
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	// forged carries valid claims under a key ID the provider never published.
	forged := func(i int) string {
		h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "unknown-" + string(rune('a'+i%26)), "typ": "logout+jwt"})
		c, _ := json.Marshal(map[string]any{"iss": f.issuer, "aud": "kyrecovery", "sub": "alice", "iat": time.Now().Unix(),
			"exp": time.Now().Add(time.Minute).Unix(), "events": map[string]any{"http://schemas.openid.net/event/backchannel-logout": map[string]any{}}})
		return base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c) + "." +
			base64.RawURLEncoding.EncodeToString(make([]byte, 256))
	}

	const n = 20
	srv := newSSOServer(t, f.issuer)
	for i := range n {
		if w := postLogout(srv, forged(i)); w.Code != http.StatusBadRequest {
			t.Fatalf("forged logout %d: got %d", i, w.Code)
		}
	}
	if d, k := f.discoveries.Load(), f.jwksFetches.Load(); d > 1 || k > 1 {
		t.Fatalf("%d forged logouts made %d discoveries and %d JWKS fetches, want at most 1 each", n, d, k)
	}
	// The keys already fetched still verify a genuine logout.
	if w := postLogout(srv, f.logoutToken(t, "alice", "")); w.Code != http.StatusOK {
		t.Fatalf("genuine logout after forged ones: got %d", w.Code)
	}

	down := newSSOServer(t, failing.URL)
	for i := range n {
		if w := postLogout(down, f.logoutToken(t, "alice", "")); w.Code != http.StatusBadRequest {
			t.Fatalf("logout with a failing issuer %d: got %d", i, w.Code)
		}
	}
	if d := failedDiscoveries.Load(); d > 1 {
		t.Fatalf("%d logouts retried a failing discovery %d times, want at most 1", n, d)
	}
}

// TestCancelledDiscoveryIsNotCachedAsIssuerFailure: the first request to need the provider
// runs the shared discovery. Its caller hanging up must not block logins and logouts for
// the failure-retry interval while the issuer is healthy.
func TestCancelledDiscoveryIsNotCachedAsIssuerFailure(t *testing.T) {
	f := newFakeKyIdentity(t)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.beforeDiscovery = func() {
		once.Do(func() {
			close(started)
			<-release
		})
	}
	srv := newSSOServer(t, f.issuer)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/auth/backchannel-logout",
			strings.NewReader(url.Values{"logout_token": {f.logoutToken(t, "alice", "")}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		srv.ServeHTTP(httptest.NewRecorder(), req)
	}()
	<-started
	cancel()
	// A discovery bound to the request gives up on cancel; give it the chance before the
	// issuer answers, so the test catches one.
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	<-done

	w := ssoLogin(t, srv, f, "alice", "sid-1")
	if w.Code != http.StatusFound || issuedCookie(w) == nil {
		t.Fatalf("login after a cancelled discovery: got %d %s", w.Code, w.Body.String())
	}
	if w := postLogout(srv, f.logoutToken(t, "alice", "sid-1")); w.Code != http.StatusOK {
		t.Fatalf("logout after a cancelled discovery: got %d", w.Code)
	}
}
