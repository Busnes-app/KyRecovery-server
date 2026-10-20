package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/server"
)

// An authorization error from the IdP is reported as such, never as a failed token exchange,
// and the IdP's free text is not reflected.
func TestCallbackReportsProviderError(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	srv, err := server.New(server.Config{Port: 8095, DataDir: t.TempDir()}, database, audit.NewLedger(database))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ query, want string }{
		{"error=access_denied&error_description=%3Cscript%3E", "access_denied"},
		{"error=server_error", "server_error"},
		{"error=%3Cb%3Ebad%3C%2Fb%3E", "identity provider returned an error"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=s&"+tc.query, nil)
		for _, c := range []string{"kyrec_state=s", "kyrec_pkce=p", "kyrec_nonce=n"} {
			name, value, _ := strings.Cut(c, "=")
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		body := rec.Body.String()
		if rec.Code != http.StatusUnauthorized || !strings.Contains(body, tc.want) || strings.Contains(body, "token exchange") || strings.Contains(body, "script") || strings.Contains(body, "<b>") {
			t.Errorf("%s: got %d %s", tc.query, rec.Code, body)
		}
	}
}
