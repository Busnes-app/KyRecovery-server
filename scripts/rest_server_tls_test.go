package scripts

import (
	"crypto/fips140"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/ky-primitives/pqtls"
)

// This digest's linux/amd64 binary reports go1.26.10 (see STRONG-CRYPTO.md).
// A new digest needs fresh binary inspection and a real-container handshake.
const verifiedRestServerImage = "restic/rest-server:0.15.0@sha256:5922067eb6f1ed8a632bc56101677ab4968a3b88a28806423e95ebb5eeecdebb"

func TestRestServerComposeTLS(t *testing.T) {
	src, err := os.ReadFile("../docker-compose.restic.yml")
	if err != nil {
		t.Fatal(err)
	}
	// Scope checks to active rest-server settings, never comments or KyRecovery.
	_, service, ok := strings.Cut(string(src), "\n  rest-server:\n")
	if !ok {
		t.Fatal("missing rest-server service")
	}
	service, _, _ = strings.Cut(service, "\n  kyrecovery:")
	image := regexp.MustCompile(`(?m)^    image: (\S+)$`).FindStringSubmatch(service)
	if len(image) != 2 || image[1] != verifiedRestServerImage {
		t.Fatal("rest-server must pin the digest verified with Go >= 1.24; inspect the replacement binary and update the evidence")
	}
	options := regexp.MustCompile(`(?m)^      OPTIONS: >-\n((?:        [^\n]*\n)+)`).FindStringSubmatch(service)
	want := "--private-repos --append-only --group-accessible-repos --tls --tls-cert /etc/rest-server/tls/cert.pem --tls-key /etc/rest-server/tls/key.pem"
	if len(options) != 2 || strings.Join(strings.Fields(options[1]), " ") != want {
		t.Fatal("rest-server OPTIONS must retain TLS, certificate paths and repository protections without disabling overrides")
	}
}

// CI invokes this in separate default and fips140=on processes. Neither side
// sets CurvePreferences: this proves the Go defaults used by restic/rest-server.
func TestRestServerDefaultTLSHandshake(t *testing.T) {
	serverState := make(chan tls.ConnectionState, 1)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverState <- *r.TLS
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{}
	srv.StartTLS()
	defer srv.Close()
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	probeRestServerTLS(t, srv.URL, roots, http.StatusOK)
	pqtls.RequireHybrid(t, <-serverState)
}

// Opt-in real-image check, run by verify-restic-overlay.sh against its owned
// container. Ordinary go test needs neither Docker nor external network access.
func TestRestServerContainerTLSHandshake(t *testing.T) {
	endpoint := os.Getenv("KYRECOVERY_REST_SERVER_TEST_URL")
	if endpoint == "" {
		t.Skip("run scripts/verify-restic-overlay.sh for the real-image probe")
	}
	cert, err := os.ReadFile(os.Getenv("KYRECOVERY_REST_SERVER_TEST_CA"))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(cert) {
		t.Fatal("invalid probe CA")
	}
	probeRestServerTLS(t, endpoint, roots, http.StatusOK)
}

func probeRestServerTLS(t *testing.T, endpoint string, roots *x509.CertPool, status int) {
	t.Helper()
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("alice", "alice-pass") // disposable overlay fixture only
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != status || resp.TLS == nil {
		t.Fatalf("status = %d, TLS = %v", resp.StatusCode, resp.TLS)
	}
	pqtls.RequireHybrid(t, *resp.TLS)
	if resp.TLS.Version != tls.VersionTLS13 {
		t.Fatalf("TLS version = %#x, want TLS 1.3", resp.TLS.Version)
	}
	t.Logf("client fips140=%v: TLS 1.3, curve=%v", fips140.Enabled(), resp.TLS.CurveID)
}
