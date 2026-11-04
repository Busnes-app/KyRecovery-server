package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

func newTestServer(t *testing.T) (*server.Server, *db.DB) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	srv, err := server.New(server.Config{Port: 8099, DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	t.Cleanup(srv.Close)
	return srv, database
}

// sessionCookie mints a session with the given role, standing in for a user the
// identity provider placed in that role.
func sessionCookie(t *testing.T, database *db.DB, role string) *http.Cookie {
	t.Helper()
	id := fmt.Sprintf("sess-%s-%d", role, time.Now().UnixNano())
	err := database.InsertSession(t.Context(), db.SessionRecord{
		ID:        id,
		UserID:    "usr-" + role,
		Email:     role + "@example.com",
		Name:      role,
		Role:      role,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("InsertSession failed: %v", err)
	}
	return &http.Cookie{Name: auth.SessionCookieName, Value: id}
}

// TestRoleEnforcementOverHTTP proves the policy table is actually applied by the
// handler chain: a viewer cannot start a replication sync, an operator cannot
// rotate the trust configuration, and an anonymous caller cannot do either.
func TestRoleEnforcementOverHTTP(t *testing.T) {
	srv, database := newTestServer(t)

	viewer := sessionCookie(t, database, auth.RoleViewer)
	operator := sessionCookie(t, database, auth.RoleOperator)
	admin := sessionCookie(t, database, auth.RoleAdmin)

	routes := []struct {
		name, method, path, body string
		minRole                  string
	}{
		{"custodian create", http.MethodPost, "/api/custodians", `{"name":"C","email":"c@example.invalid"}`, auth.RoleOperator},
		{"audit verify", http.MethodPost, "/api/audit/verify", `{}`, auth.RoleOperator},
		{"replication sync", http.MethodPost, "/api/replication/sync", `{"capsule_id":"cap-x"}`, auth.RoleOperator},
		{"pairing generate", http.MethodPost, "/api/pairing/generate", `{}`, auth.RoleAdmin},
		{"pairing revoke", http.MethodPost, "/api/pairing/revoke", `{"id":"pair-x"}`, auth.RoleAdmin},
		{"pairing clear", http.MethodPost, "/api/pairing/clear", `{"id":"pair-x"}`, auth.RoleAdmin},
		{"replication target create", http.MethodPost, "/api/replication/targets", `{"type":"local","endpoint":"/tmp/x"}`, auth.RoleAdmin},
		{"replication target delete", http.MethodDelete, "/api/replication/targets/target-1", ``, auth.RoleAdmin},
		{"sso config write", http.MethodPost, "/api/auth/sso/config", `{"enabled":false}`, auth.RoleAdmin},
		{"sso connection test", http.MethodPost, "/api/auth/sso/test", `{"issuer_url":"https://example.invalid"}`, auth.RoleAdmin},
	}

	roles := []struct {
		name   string
		cookie *http.Cookie
	}{
		{auth.RoleViewer, viewer},
		{auth.RoleOperator, operator},
		{auth.RoleAdmin, admin},
	}

	for _, rt := range routes {
		// Anonymous callers never get past authentication.
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: anonymous expected 401, got %d", rt.name, rec.Code)
		}

		for _, role := range roles {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
			req.AddCookie(role.cookie)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			allowed := auth.RoleRank(role.name) >= auth.RoleRank(rt.minRole)
			if allowed && rec.Code == http.StatusForbidden {
				t.Errorf("%s: %s should be permitted but got 403", rt.name, role.name)
			}
			if !allowed && rec.Code != http.StatusForbidden {
				t.Errorf("%s: %s should be denied but got %d", rt.name, role.name, rec.Code)
			}
		}
	}
}

// TestSSOTestEndpointIsNotAnOpenSSRFPrimitive proves an unauthenticated caller
// cannot make the server reach an arbitrary URL: the target is never contacted.
func TestSSOTestEndpointIsNotAnOpenSSRFPrimitive(t *testing.T) {
	srv, database := newTestServer(t)

	var reached bool
	var mu sync.Mutex
	internalService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reached = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer internalService.Close()

	body := fmt.Sprintf(`{"issuer_url":%q}`, internalService.URL)

	for _, tc := range []struct {
		name   string
		cookie *http.Cookie
		want   int
	}{
		{"anonymous", nil, http.StatusUnauthorized},
		{"viewer", sessionCookie(t, database, auth.RoleViewer), http.StatusForbidden},
		{"operator", sessionCookie(t, database, auth.RoleOperator), http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/sso/test", strings.NewReader(body))
		if tc.cookie != nil {
			req.AddCookie(tc.cookie)
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: expected %d, got %d: %s", tc.name, tc.want, rec.Code, rec.Body.String())
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if reached {
		t.Fatal("a non-admin caller made the server issue an outbound request")
	}
}

func TestLocalLoginIsThrottled(t *testing.T) {
	srv, database := newTestServer(t)
	authMgr := auth.NewManager(auth.OIDCConfig{}, database)
	adminPass, _, err := authMgr.EnsureAdminUser(t.Context(), "CorrectHorseBatteryStaple1!")
	if err != nil {
		t.Fatalf("EnsureAdminUser failed: %v", err)
	}

	login := func(addr, username, password string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"username": username, "password": password})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login/local", bytes.NewReader(body))
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	// Past the account cap the guesser's address gets the same 401 as a wrong password,
	// for the right password too, so the cap reveals nothing and still stops guessing.
	const guesser = "203.0.113.9:5555"
	for i := 0; i < 6; i++ {
		if code := login(guesser, "admin", "wrong-password").Code; code != http.StatusUnauthorized {
			t.Fatalf("failed sign-in %d: %d", i, code)
		}
	}
	if code := login(guesser, "admin", adminPass).Code; code != http.StatusUnauthorized {
		t.Fatalf("the account cap should apply to the guessing address, got %d", code)
	}

	// The cap is per account and address: the real admin elsewhere still signs in,
	// including from another address in an IPv6 /64 the guesser does not control.
	if code := login("198.51.100.4:6000", "admin", adminPass).Code; code != http.StatusOK {
		t.Fatalf("the admin is locked out by someone else's guesses: %d", code)
	}

	// The address cap still ends a guesser's run across many account names, and an IPv6
	// client's whole /64 counts as one address.
	throttled := false
	for i := 0; i < 25; i++ {
		addr := fmt.Sprintf("[2001:db8:1:2::%x]:5555", i+1)
		if login(addr, fmt.Sprintf("user%d", i), "wrong").Code == http.StatusTooManyRequests {
			throttled = true
			break
		}
	}
	if !throttled {
		t.Fatal("guesses rotating addresses within one /64 were never throttled")
	}
}

// A failed sign-in's claimed name is bounded before it reaches the append-only ledger.
func TestFailedLoginRecordsABoundedName(t *testing.T) {
	srv, database := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"username": strings.Repeat("x", 100000), "password": "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/local", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("login: %d", rec.Code)
	}
	events, _ := database.ListAuditEvents(t.Context(), 10)
	found := false
	for _, e := range events {
		if e.Action == "user_login_failed" {
			found = true
			if len(e.DetailsJSON) > 512 {
				t.Fatalf("ledger row of %d bytes for one failed sign-in", len(e.DetailsJSON))
			}
		}
	}
	if !found {
		t.Fatal("the failed sign-in was not recorded")
	}
}

func TestLoginDoesNotEchoSessionToken(t *testing.T) {
	srv, database := newTestServer(t)
	authMgr := auth.NewManager(auth.OIDCConfig{}, database)
	adminPass, _, err := authMgr.EnsureAdminUser(t.Context(), "CorrectHorseBatteryStaple1!")
	if err != nil {
		t.Fatalf("EnsureAdminUser failed: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": adminPass})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/local", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Value == "" {
		t.Fatal("expected a session cookie")
	}
	if !cookies[0].HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if strings.Contains(rec.Body.String(), cookies[0].Value) {
		t.Fatal("the session token was echoed into the response body, exposing it to scripts")
	}
}

// TestSessionCookieIsSecureOverTLS proves a session established over HTTPS is
// marked Secure and can therefore never be replayed over plaintext HTTP.
func TestSessionCookieIsSecureOverTLS(t *testing.T) {
	srv, database := newTestServer(t)
	authMgr := auth.NewManager(auth.OIDCConfig{}, database)
	adminPass, _, err := authMgr.EnsureAdminUser(t.Context(), "CorrectHorseBatteryStaple1!")
	if err != nil {
		t.Fatalf("EnsureAdminUser failed: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": adminPass})
	req := httptest.NewRequest(http.MethodPost, "https://recovery.example/api/auth/login/local", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || !cookies[0].Secure {
		t.Fatalf("a session created over HTTPS must set Secure: %+v", cookies)
	}
}

// GetSession accepts a session as a bearer token, so logout must end that session too,
// not only a cookie's.
func TestBearerLogoutEndsTheSession(t *testing.T) {
	srv, database := newTestServer(t)
	id := sessionCookie(t, database, auth.RoleViewer).Value
	call := func(path string) int {
		method := http.MethodGet
		if path == "/api/auth/logout" {
			method = http.MethodPost
		}
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+id)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := call("/api/readiness"); code != http.StatusOK {
		t.Fatalf("bearer session before logout: %d", code)
	}
	if code := call("/api/auth/logout"); code != http.StatusOK {
		t.Fatalf("logout: %d", code)
	}
	if code := call("/api/readiness"); code != http.StatusUnauthorized {
		t.Fatalf("bearer session after logout: %d, want 401", code)
	}
}
