package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Busnes-app/kyrecovery-server/internal/appversion"
	"github.com/Busnes-app/kyrecovery-server/internal/auth"
)

// The overview shows custodian_count as a directory size. The reconstruction
// quorum comes only from the recovery-key record, so readiness must not start
// carrying threshold or total_shares, which a caption could mistake for one.
func TestReadinessReportsCustodianCountNotQuorum(t *testing.T) {
	srv, database := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/readiness", nil)
	req.AddCookie(sessionCookie(t, database, auth.RoleViewer))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["custodian_count"]; !ok {
		t.Fatal("custodian_count missing")
	}
	for _, k := range []string{"threshold", "total_shares", "quorum"} {
		if _, ok := body[k]; ok {
			t.Fatalf("readiness must not report %q; the quorum belongs to /api/recovery-key", k)
		}
	}
	if body["version"] != appversion.String() {
		t.Fatalf("version = %v, want %q for the About dialog", body["version"], appversion.String())
	}
}

// The About dialog embeds the licence verbatim; it must not drift from LICENSE.txt.
func TestAboutDialogCarriesTheLicence(t *testing.T) {
	licence, err := os.ReadFile("../../LICENSE.txt")
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	want := `aria-label="MIT License">` + strings.TrimRight(string(licence), "\n") + "</textarea>"
	if !strings.Contains(string(page), want) {
		t.Fatal("index.html's About dialog does not carry LICENSE.txt verbatim")
	}
}
