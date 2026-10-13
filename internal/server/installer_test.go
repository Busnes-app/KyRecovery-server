package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Busnes-app/ky-primitives/applysetup"
	"github.com/Busnes-app/ky-primitives/syncauth"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func recoveryInstallerBundle(t *testing.T) applysetup.Common {
	t.Helper()
	dir := t.TempDir()
	client := filepath.Join(dir, "client")
	directory := filepath.Join(dir, "directory")
	for _, p := range []string{client, directory} {
		if err := os.WriteFile(p, []byte("private-disposable-credential"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return applysetup.Common{Schema: applysetup.Schema, Service: "kyrecovery", SetupID: "12345678-1234-1234-1234-123456789abc", SSO: &applysetup.SSO{IssuerURL: "https://identity.example", ClientID: "recovery", ClientSecretFile: client, DirectorySecretFile: directory, RedirectURI: "https://recovery.example/api/auth/callback"}, Admins: []applysetup.Identity{{Issuer: "https://identity.example", Subject: "owner", Username: "owner-admin"}}}
}
func TestInstallerAtomicLiveReadOnlyAndDirectory(t *testing.T) {
	s, d := newTestServer(t)
	ctx := context.Background()
	b := recoveryInstallerBundle(t)
	raw, _ := json.Marshal(b)
	before, _, _, err := d.GetAuditAnchor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyInstaller(ctx, raw, true); err == nil {
		t.Fatal("missing check passed")
	}
	after, _, _, _ := d.GetAuditAnchor(ctx)
	if after != before {
		t.Fatal("check wrote audit")
	}
	r, err := s.ApplyInstaller(ctx, raw, false)
	if err != nil || len(r.Created) != 2 || len(r.Pending) != 0 {
		t.Fatal("create", r, err)
	}
	count, _, _, _ := d.GetAuditAnchor(ctx)
	if count != before+1 {
		t.Fatal("missing atomic audit")
	}
	for _, check := range []bool{true, false} {
		r, err = s.ApplyInstaller(ctx, raw, check)
		if err != nil || len(r.Created) != 0 || len(r.Present) != 2 {
			t.Fatal("resume", r, err)
		}
	}
	after, _, _, _ = d.GetAuditAnchor(ctx)
	if after != count {
		t.Fatal("resume audited")
	}
	mgr := auth.NewManager(auth.OIDCConfig{}, d)
	if !mgr.IsEnabled() || mgr.GetConfig().IssuerURL != b.SSO.IssuerURL {
		t.Fatal("running SSO not activated")
	}
	cookie, err := mgr.CreateSession(ctx, &auth.UserInfo{Issuer: b.SSO.IssuerURL, Subject: "owner", Role: "viewer"}, false)
	if err != nil {
		t.Fatal(err)
	}
	request := func(path string) int {
		q := httptest.NewRequest(http.MethodGet, path, nil)
		q.AddCookie(cookie)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, q)
		return w.Code
	}
	if code := request("/admin/ceremony"); code != 200 {
		t.Fatal("owner not admin", code)
	}
	if key, err := d.GetRecoveryKey(ctx); err != nil || key != nil {
		t.Fatal("installer bypassed ceremony")
	}
	if _, err = mgr.CreateSession(ctx, &auth.UserInfo{Issuer: b.SSO.IssuerURL, Subject: "unassigned", Role: "admin"}, false); err == nil {
		t.Fatal("global claim granted unassigned subject")
	}
	if err = mgr.SaveConfig(ctx, auth.OIDCConfig{}); err == nil {
		t.Fatal("native configuration overwrote ownership")
	}
	if err = d.SetSetting(ctx, "sso.issuer_url", "https://foreign.example"); err == nil {
		t.Fatal("native setter overwrote ownership")
	}
	body := `{"id":"owner","active":false,"meta":{"version":"W/\"2\""}}`
	send := func(body string, signed bool) int {
		q := httptest.NewRequest(http.MethodPost, "/api/installer/directory", strings.NewReader(body))
		if signed {
			h, err := syncauth.Sign([]byte("private-disposable-credential"), time.Now(), "user.updated", "fixture", []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			h.Apply(q)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, q)
		return w.Code
	}
	if code := send(body, false); code != 401 {
		t.Fatal("unsigned directory accepted", code)
	}
	if code := send(body, true); code != 204 {
		t.Fatal("offboard", code)
	}
	count, _, _, _ = d.GetAuditAnchor(ctx)
	if code := send(body, true); code != 204 {
		t.Fatal("duplicate", code)
	}
	after, _, _, _ = d.GetAuditAnchor(ctx)
	if after != count {
		t.Fatal("duplicate audit")
	}
	if code := request("/admin/ceremony"); code != 403 {
		t.Fatal("session retained", code)
	}
	if _, err = s.ApplyInstaller(ctx, raw, false); err == nil {
		t.Fatal("setup reactivated revoked admin")
	}
	on := `{"id":"owner","active":true,"roles":[{"value":"kyrecovery.admin"}],"meta":{"version":"W/\"1\""}}`
	if code := send(on, true); code != 422 {
		t.Fatal("stale state activated", code)
	}
}
func TestInstallerCollisionRollsBackWithAudit(t *testing.T) {
	s, d := newTestServer(t)
	ctx := context.Background()
	b := recoveryInstallerBundle(t)
	b.Admins = append(b.Admins, applysetup.Identity{Issuer: b.SSO.IssuerURL, Subject: "collision", Username: "second-admin"})
	id := db.InstallerIdentityID(b.SSO.IssuerURL, "collision")
	if err := d.InsertUser(ctx, db.UserRecord{ID: id, Username: "legacy", PasswordHash: "preserved", Role: "admin", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	count, _, _, _ := d.GetAuditAnchor(ctx)
	raw, _ := json.Marshal(b)
	if _, err := s.ApplyInstaller(ctx, raw, false); err == nil {
		t.Fatal("collision adopted")
	}
	if _, managed, err := d.InstallerConfiguration(ctx); err != nil || managed {
		t.Fatal("partial config")
	}
	if _, assigned, err := d.InstallerGrant(ctx, db.InstallerIdentityID(b.SSO.IssuerURL, "owner")); err != nil || assigned {
		t.Fatal("partial owner")
	}
	after, _, _, _ := d.GetAuditAnchor(ctx)
	if after != count {
		t.Fatal("rolled-back grant advanced keyed audit")
	}
	legacy, err := d.GetUserByID(ctx, id)
	if err != nil || legacy.PasswordHash != "preserved" {
		t.Fatal("legacy account changed")
	}
}

func TestInstallerNetworkTrust(t *testing.T) {
	d, e := db.Open(":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	s, e := server.New(server.Config{DataDir: t.TempDir(), TrustedProxies: []netip.Prefix{netip.MustParsePrefix("192.0.2.7/32")}}, d, audit.NewLedger(d))
	if e != nil {
		t.Fatal(e)
	}
	for _, trusted := range []bool{false, true} {
		r := httptest.NewRequest(http.MethodGet, "/api/installer/network", nil)
		r.RemoteAddr = "198.51.100.8:42"
		if trusted {
			r.RemoteAddr = "192.0.2.7:42"
		}
		r.Header.Set("X-Forwarded-For", "203.0.113.6")
		r.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		var report map[string]any
		if e = json.Unmarshal(w.Body.Bytes(), &report); e != nil {
			t.Fatal(e)
		}
		ip, scheme := "198.51.100.8", "http"
		if trusted {
			ip, scheme = "203.0.113.6", "https"
		}
		if w.Code != 200 || len(report) != 3 || report["clientIp"] != ip || report["scheme"] != scheme || report["proxyHeadersTrusted"] != trusted || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("network", w.Body)
		}
	}
}

// A built-in SSO session from before managed setup carries no issuer binding, so the
// managed grant checks would skip it. Activation must end it; local login still works.
func TestInstallerActivationEndsPriorSessions(t *testing.T) {
	s, d := newTestServer(t)
	ctx := context.Background()
	mgr := auth.NewManager(auth.OIDCConfig{}, d)
	adminPass, _, err := mgr.EnsureAdminUser(ctx, "CorrectHorseBatteryStaple1!")
	if err != nil {
		t.Fatal(err)
	}
	prior, err := mgr.CreateSession(ctx, &auth.UserInfo{Issuer: "https://identity.example", Subject: "owner", Role: auth.RoleAdmin}, false)
	if err != nil {
		t.Fatal(err)
	}
	request := func(path string, c *http.Cookie) int {
		q := httptest.NewRequest(http.MethodGet, path, nil)
		q.AddCookie(c)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, q)
		return w.Code
	}
	if code := request("/api/readiness", prior); code != 200 {
		t.Fatal("fixture session unusable before setup", code)
	}
	raw, _ := json.Marshal(recoveryInstallerBundle(t))
	if _, err = s.ApplyInstaller(ctx, raw, false); err != nil {
		t.Fatal(err)
	}
	if code := request("/api/readiness", prior); code != 401 {
		t.Fatal("pre-setup SSO session survived managed activation", code)
	}
	// A native callback that read "unmanaged" before activation inserts afterwards.
	now := time.Now().UTC()
	late := db.SessionRecord{ID: "late-native-callback", UserID: "owner", Role: auth.RoleAdmin, NativeSSO: true, ExpiresAt: now.Add(time.Hour), CreatedAt: now}
	if err = d.InsertSession(ctx, late); err == nil {
		t.Fatal("native SSO session inserted after managed activation")
	}
	if code := request("/api/readiness", &http.Cookie{Name: auth.SessionCookieName, Value: late.ID}); code != 401 {
		t.Fatal("late native session usable", code)
	}
	// A provider that returns no ID token yields a userinfo profile without an issuer;
	// it is still a native login and must be fenced the same way.
	if _, err = mgr.CreateSession(ctx, &auth.UserInfo{Subject: "former-admin", Role: auth.RoleAdmin}, false); !errors.Is(err, db.ErrSessionRefused) {
		t.Fatal("issuer-less native session created after managed activation", err)
	}
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": adminPass})
	q := httptest.NewRequest(http.MethodPost, "/api/auth/login/local", strings.NewReader(string(body)))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, q)
	if w.Code != 200 || len(w.Result().Cookies()) == 0 {
		t.Fatal("local login after setup", w.Code, w.Body.String())
	}
	if code := request("/api/readiness", w.Result().Cookies()[0]); code != 200 {
		t.Fatal("fresh local session refused", code)
	}
}
