package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

func hexID(i int) string { return fmt.Sprintf("%064x", i) }

func writeFile(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// backupFixture is a store with one paired kynotes instance holding one fresh capsule, and a
// restic directory with a matching repository written ten minutes after it.
type backupFixture struct {
	srv       *Server
	db        *db.DB
	resticDir string
	snapshot  string
}

func newBackupFixture(t *testing.T) *backupFixture {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	dir := t.TempDir()
	now := time.Now().UTC()

	repo := filepath.Join(dir, "kynotes", "main") // rest-server user kynotes
	writeFile(t, filepath.Join(repo, "config"), now)
	writeFile(t, filepath.Join(repo, "keys", hexID(1)), now)
	writeFile(t, filepath.Join(repo, "snapshots", hexID(2)), now.Add(-50*time.Minute))
	writeFile(t, filepath.Join(repo, "data", "ab", hexID(3)), now)

	ctx := t.Context()
	if err := database.InsertPairedApp(ctx, db.PairedAppRecord{ID: "app-1", ServiceName: "kynotes", AppName: "notes", APIToken: "tok",
		PairingCode: "123456", Status: "paired", ExpiresAt: now, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := database.InsertCapsule(ctx, db.CapsuleRecord{ID: "cap-kynotes-1", ServiceName: "kynotes", FilePath: "/x", Digest: "d",
		Status: "active", CreatedAt: now.Add(-time.Hour), DepositedAt: now.Add(-time.Hour), PairedAppID: "app-1"}); err != nil {
		t.Fatal(err)
	}
	return &backupFixture{db: database, resticDir: dir, snapshot: hexID(2), srv: fixtureServer(t, database, dir)}
}

func fixtureServer(t *testing.T, database *db.DB, resticDir string) *Server {
	t.Helper()
	srv, err := New(Config{DataDir: t.TempDir(), ResticDir: resticDir}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func cookieFor(t *testing.T, database *db.DB, role string) *http.Cookie {
	t.Helper()
	id := fmt.Sprintf("sess-%s-%d", role, time.Now().UnixNano())
	if err := database.InsertSession(t.Context(), db.SessionRecord{ID: id, UserID: "u-" + role, Email: role + "@example.com",
		Role: role, ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: auth.SessionCookieName, Value: id}
}

func call(t *testing.T, srv *Server, c *http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if c != nil {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

type healthBody struct {
	Status string `json:"status"`
	Checks []struct {
		Name, Status, Reason string
	} `json:"checks"`
}

func backupCheck(t *testing.T, srv *Server) (healthBody, string) {
	t.Helper()
	rr := call(t, srv, nil, http.MethodGet, "/healthz", nil)
	var h healthBody
	if err := json.Unmarshal(rr.Body.Bytes(), &h); err != nil {
		t.Fatal(err)
	}
	return h, rr.Body.String()
}

func reason(h healthBody) string {
	for _, c := range h.Checks {
		if c.Name == "backups" {
			return c.Status + "/" + c.Reason
		}
	}
	return "absent"
}

func TestBackupStatusPlanAndHealth(t *testing.T) {
	f := newBackupFixture(t)
	f.srv.scanRestic(t.Context())

	h, body := backupCheck(t, f.srv)
	if reason(h) != "ok/" {
		t.Fatalf("backups check = %s; body %s", reason(h), body)
	}
	for _, leak := range []string{"kynotes", "main", "cap-"} {
		if strings.Contains(body, leak) {
			t.Fatalf("public health leaks %q: %s", leak, body)
		}
	}

	viewer := cookieFor(t, f.db, auth.RoleViewer)
	rr := call(t, f.srv, viewer, http.MethodGet, "/api/backups/status", nil)
	var st struct {
		Report struct {
			Services []struct {
				Name           string `json:"service_name"`
				State          string `json:"state"`
				ExpectsCapsule bool   `json:"expects_capsule"`
				ExpectsRestic  bool   `json:"expects_restic"`
			} `json:"services"`
		} `json:"report"`
		Samples []db.ResticSample `json:"samples"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &st); err != nil || rr.Code != http.StatusOK {
		t.Fatalf("status %d %s", rr.Code, rr.Body.String())
	}
	s := st.Report.Services
	if len(s) != 1 || s[0].Name != "kynotes" || s[0].State != "ok" || !s[0].ExpectsCapsule || !s[0].ExpectsRestic || len(st.Samples) != 1 {
		t.Fatalf("status = %s", rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), f.resticDir) || strings.Contains(rr.Body.String(), `"/x"`) {
		t.Fatalf("status leaks a server path: %s", rr.Body.String())
	}

	rr = call(t, f.srv, viewer, http.MethodGet, "/api/backups/restore-plan?service=kynotes", nil)
	var plan struct {
		Command    string `json:"command"`
		GapSeconds int64  `json:"gap_seconds"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || rr.Code != http.StatusOK {
		t.Fatalf("plan %d %s", rr.Code, rr.Body.String())
	}
	want := "kyquickstart restore 'kynotes' --capsule 'cap-kynotes-1' --snapshot 'kynotes/main:" + f.snapshot + "'"
	if plan.Command != want || plan.GapSeconds != 600 {
		t.Fatalf("plan = %s", rr.Body.String())
	}

	for _, q := range []string{"", "?service=kynotes&at=2026-10-10T00:00:00", "?service=kynotes&at=2999-01-01T00:00:00Z"} {
		if rr := call(t, f.srv, viewer, http.MethodGet, "/api/backups/restore-plan"+q, nil); rr.Code != http.StatusBadRequest {
			t.Fatalf("plan%s = %d, want 400", q, rr.Code)
		}
	}
}

// A scan that cannot read the root must not empty the inventory or read as healthy.
func TestFailedScanKeepsRowsAndDegrades(t *testing.T) {
	f := newBackupFixture(t)
	f.srv.scanRestic(t.Context())
	if err := os.Rename(f.resticDir, f.resticDir+"-gone"); err != nil {
		t.Fatal(err)
	}
	f.srv.scanRestic(t.Context())

	repos, err := f.db.ListResticRepos(t.Context())
	if err != nil || len(repos) != 1 || repos[0].MissingSince != nil || repos[0].SnapshotCount != 1 {
		t.Fatalf("repos after failed scan = %+v, %v", repos, err)
	}
	h, _ := backupCheck(t, fixtureServer(t, f.db, f.resticDir))
	if reason(h) != "degraded/backup_scan_failed" {
		t.Fatalf("backups check = %s", reason(h))
	}
}

func TestBackupAdminWrites(t *testing.T) {
	f := newBackupFixture(t)
	f.srv.scanRestic(t.Context())
	admin := cookieFor(t, f.db, auth.RoleAdmin)

	cases := []struct {
		path string
		body any
		want int
	}{
		{"/api/backups/repos/link", map[string]string{"repo_path": "kynotes/main", "service_name": "vaultwarden"}, http.StatusOK},
		{"/api/backups/repos/link", map[string]string{"repo_path": "nope", "service_name": "x"}, http.StatusNotFound},
		{"/api/backups/repos/link", map[string]string{"repo_path": "kynotes/main", "service_name": "../x"}, http.StatusBadRequest},
		{"/api/backups/expectations", map[string]any{"service_name": "kynotes", "expected_interval_seconds": 3600}, http.StatusOK},
		{"/api/backups/expectations", map[string]any{"repo_path": "kynotes/main", "expected_interval_seconds": 7200}, http.StatusOK},
		{"/api/backups/expectations", map[string]any{"service_name": "kynotes", "expected_interval_seconds": 60}, http.StatusBadRequest},
		{"/api/backups/expectations", map[string]any{"service_name": "kynotes", "repo_path": "kynotes/main", "expected_interval_seconds": 3600}, http.StatusBadRequest},
		{"/api/backups/repos/ack-key", map[string]string{"repo_path": "kynotes/main"}, http.StatusOK},
	}
	for _, tc := range cases {
		if rr := call(t, f.srv, admin, http.MethodPost, tc.path, tc.body); rr.Code != tc.want {
			t.Fatalf("POST %s %v = %d %s, want %d", tc.path, tc.body, rr.Code, rr.Body.String(), tc.want)
		}
	}
	repos, _ := f.db.ListResticRepos(t.Context())
	if repos[0].Service != "vaultwarden" || repos[0].Expected != 2*time.Hour {
		t.Fatalf("repo = %+v", repos[0])
	}
	events, _ := f.db.ListAuditEvents(t.Context(), 50)
	seen := map[string]int{}
	for _, e := range events {
		seen[e.Action]++
	}
	if seen["restic_repo_linked"] != 1 || seen["backup_expectation_set"] != 2 || seen["restic_key_acknowledged"] != 1 {
		t.Fatalf("audit events = %v", seen)
	}

	if rr := call(t, f.srv, cookieFor(t, f.db, auth.RoleOperator), http.MethodPost, "/api/backups/repos/link",
		map[string]string{"repo_path": "kynotes/main", "service_name": "x"}); rr.Code != http.StatusForbidden {
		t.Fatalf("operator link = %d", rr.Code)
	}
}

// An admin change that cannot be audited is not made.
func TestBackupAdminWriteFailsClosedOnAudit(t *testing.T) {
	f := newBackupFixture(t)
	f.srv.scanRestic(t.Context())
	admin := cookieFor(t, f.db, auth.RoleAdmin)
	if rr := call(t, f.srv, admin, http.MethodPost, "/api/backups/repos/ack-key", map[string]string{"repo_path": "kynotes/main"}); rr.Code != http.StatusOK {
		t.Fatalf("ack = %d", rr.Code)
	}
	last, err := f.db.GetLastAuditEvent(t.Context())
	if err != nil || last == nil {
		t.Fatal(err)
	}
	if err := f.db.DeleteAuditEventForTest(t.Context(), last.Seq); err != nil {
		t.Fatal(err)
	}
	poisoned := fixtureServer(t, f.db, f.resticDir)

	rr := call(t, poisoned, admin, http.MethodPost, "/api/backups/repos/link", map[string]string{"repo_path": "kynotes/main", "service_name": "other"})
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("link on poisoned ledger = %d %s", rr.Code, rr.Body.String())
	}
	if repos, _ := f.db.ListResticRepos(t.Context()); repos[0].Service != "kynotes" {
		t.Fatalf("unaudited link applied: %q", repos[0].Service)
	}
}

// A product paired moments ago has had no chance to deposit; it must not page anyone yet.
func TestFreshPairingDoesNotDegradeHealth(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	now := time.Now().UTC()
	if err := database.InsertPairedApp(t.Context(), db.PairedAppRecord{ID: "app-1", ServiceName: "generic", AppName: "notes",
		APIToken: "tok", PairingCode: "123456", Status: "pending", ExpiresAt: now.Add(time.Hour), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	// The real claim path, which is what records paired_at.
	if _, err := database.ClaimPairingCode(t.Context(), "123456", "kynotes", "notes", "key"); err != nil {
		t.Fatal(err)
	}
	h, body := backupCheck(t, fixtureServer(t, database, ""))
	if reason(h) != "ok/" {
		t.Fatalf("backups check = %s; body %s", reason(h), body)
	}
}

// The audit logger binds stderr at init, so the failing scan runs in a child process.
func TestScanFailureLogsStructuredError(t *testing.T) {
	if os.Getenv("KY_SCAN_LOG_CHILD") == "1" {
		database, err := db.Open(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		fixtureServer(t, database, filepath.Join(t.TempDir(), "absent")).scanRestic(t.Context())
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestScanFailureLogsStructuredError$")
	cmd.Env = append(os.Environ(), "KY_SCAN_LOG_CHILD=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("child: %v; stderr %s", err, stderr.String())
	}
	for _, line := range strings.Split(stderr.String(), "\n") {
		var e audit.LogEntry
		if json.Unmarshal([]byte(line), &e) == nil && e.Level == "ERROR" && e.Action == "restic_scan" && e.Error != "" {
			return
		}
	}
	t.Fatalf("no structured restic_scan error on stderr: %q", stderr.String())
}
