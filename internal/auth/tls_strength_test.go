package auth_test

import (
	"crypto/fips140"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/auth"
)

func TestStrongTLSRejectsWeakTrustedRoot(t *testing.T) {
	rootKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	root := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	root, err = x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, root, &leafKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	var issuer string
	var calls atomic.Int32
	idp := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys"})
	}))
	idp.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{leafDER, rootDER}, PrivateKey: leafKey}}}
	idp.StartTLS()
	defer idp.Close()
	issuer = idp.URL
	roots := x509.NewCertPool()
	roots.AddCert(root)
	client := idp.Client()
	client.Transport.(*http.Transport).TLSClientConfig.RootCAs = roots
	auth.UseIssuerClientForTest(t, client)
	mgr := auth.NewManager(auth.OIDCConfig{Enabled: true, IssuerURL: issuer, ClientID: "client"}, nil)
	_, err = mgr.BuildAuthURL(t.Context(), "state", "nonce", "challenge")
	if (err != nil) != fips140.Enabled() {
		t.Fatalf("mode=%v weak-root result=%v", fips140.Enabled(), err)
	}
	if fips140.Enabled() && calls.Load() != 0 {
		t.Fatal("discovery crossed weak-root TLS connection")
	}
}
