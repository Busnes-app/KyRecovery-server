package auth

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Busnes-app/ky-primitives/pqtls"
)

func TestOutboundPQTLSHandshake(t *testing.T) {
	serverStateCh := make(chan tls.ConnectionState, 1)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil {
			select {
			case serverStateCh <- *r.TLS:
			default:
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13}
	srv.StartTLS()
	defer srv.Close()

	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())

	client := *oidcHTTPClient()
	switch tr := client.Transport.(type) {
	case *http.Transport:
		cloned := cloneTransportWithRoots(tr, roots)
		defer cloned.CloseIdleConnections()
		client.Transport = cloned
	case httpsOIDCTransport:
		base, ok := tr.base.(*http.Transport)
		if !ok {
			t.Fatalf("unexpected wrapped transport type %T", tr.base)
		}
		cloned := cloneTransportWithRoots(base, roots)
		defer cloned.CloseIdleConnections()
		client.Transport = httpsOIDCTransport{base: cloned}
	default:
		t.Fatalf("unexpected transport type %T", client.Transport)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("outbound TLS request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if resp.TLS == nil {
		t.Fatal("missing client TLS connection state")
	}
	pqtls.RequireHybrid(t, *resp.TLS)

	select {
	case serverState := <-serverStateCh:
		pqtls.RequireHybrid(t, serverState)
	default:
		t.Fatal("server did not record TLS connection state")
	}
}

func cloneTransportWithRoots(base *http.Transport, roots *x509.CertPool) *http.Transport {
	cloned := base.Clone()
	if cloned.TLSClientConfig == nil {
		cloned.TLSClientConfig = &tls.Config{RootCAs: roots}
	} else {
		cloned.TLSClientConfig = cloned.TLSClientConfig.Clone()
		cloned.TLSClientConfig.RootCAs = roots
	}
	return cloned
}
