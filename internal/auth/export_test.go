package auth

import (
	"net/http"
	"testing"
)

// Only compiled into auth's test variant. The production client retains its
// connect-time destination policy; external tests can trust a synthetic TLS IdP.
func UseIssuerClientForTest(t *testing.T, client *http.Client) {
	t.Helper()
	previous := httpClient
	httpClient = func() *http.Client { return client }
	t.Cleanup(func() { httpClient = previous })
}
