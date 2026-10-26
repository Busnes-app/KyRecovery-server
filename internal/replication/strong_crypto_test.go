package replication_test

import (
	"context"
	"crypto/ed25519"
	"crypto/fips140"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"github.com/Busnes-app/kyrecovery-server/internal/replication"
	"golang.org/x/crypto/ssh"
)

func TestSFTPCompatibilityKeyAndAlgorithmFloors(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		bits                   int
		kex, cipher, mac, host string
	}{
		{name: "strong", bits: 2048}, {name: "RSA1024", bits: 1024}, {name: "RSA2047", bits: 2047},
		{name: "SHA1 host signature", host: ssh.KeyAlgoRSA},
		{name: "SHA1 key exchange", kex: "diffie-hellman-group14-sha1"},
		{name: "CBC cipher", cipher: "aes128-cbc"}, {name: "96-bit MAC", cipher: ssh.CipherAES128CTR, mac: ssh.InsecureHMACSHA196},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var signer ssh.Signer
			var err error
			if tc.bits != 0 || tc.host != "" {
				bits := tc.bits
				if bits == 0 {
					bits = 2048
				}
				key, e := rsa.GenerateKey(rand.Reader, bits)
				if e != nil {
					t.Fatal(e)
				}
				signer, err = ssh.NewSignerFromKey(key)
			} else {
				_, key, e := ed25519.GenerateKey(rand.Reader)
				if e != nil {
					t.Fatal(e)
				}
				signer, err = ssh.NewSignerFromKey(key)
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.host != "" {
				signer, err = ssh.NewSignerWithAlgorithms(signer.(ssh.AlgorithmSigner), []string{tc.host})
				if err != nil {
					t.Fatal(err)
				}
			}
			cfg := &ssh.ServerConfig{NoClientAuth: true}
			cfg.AddHostKey(signer)
			if tc.kex != "" {
				cfg.KeyExchanges = []string{tc.kex}
			}
			if tc.cipher != "" {
				cfg.Ciphers = []string{tc.cipher}
			}
			if tc.mac != "" {
				cfg.MACs = []string{tc.mac}
			}
			addr, _ := serveSFTPServerWithConfig(t, cfg, nil)
			client := replication.NewSFTPClient(addr, "fixture", "unused", t.TempDir(), ssh.FingerprintSHA256(signer.PublicKey()))
			client.Timeout = 2 * time.Second
			err = client.TestConnection(context.Background())
			if tc.name == "strong" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatalf("accepted pinned %s peer", tc.name)
			}
		})
	}
}

func TestSFTPCompatibilityRefusesUndersizedPrivateKeyBeforeDial(t *testing.T) {
	for _, bits := range []int{1024, 2047} {
		t.Run(fmt.Sprint(bits), func(t *testing.T) {
			key, err := rsa.GenerateKey(rand.Reader, bits)
			if err != nil {
				t.Fatal(err)
			}
			secret := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
			c := replication.NewSFTPClient("127.0.0.1:1", "fixture", string(secret), "/retained", "SHA256:unused")
			c.Timeout = time.Second
			err = c.TestConnection(context.Background())
			if err == nil || !strings.Contains(err.Error(), "2048") {
				t.Fatalf("private key reached dial instead of policy refusal: %v", err)
			}
		})
	}
}

func TestStrongProfileRefusesIncompatibleTargetsWithoutRewritingRecords(t *testing.T) {
	if !fips140.Enabled() {
		return
	}
	for _, tc := range []struct{ kind, secret, want string }{{"smb", "unused", "SMB/NTLM"}, {"s3", "short", "16 bytes"}} {
		t.Run(tc.kind, func(t *testing.T) {
			database, mgr, capID := newCapsuleFixture(t)
			target := db.ReplicationTargetRecord{ID: "weak-target", Name: "retained", Type: tc.kind, Endpoint: "https://127.0.0.1:1", Bucket: "vault", SecretKey: tc.secret, Prefix: "retained/"}
			if err := database.InsertReplicationTarget(t.Context(), target); err != nil {
				t.Fatal(err)
			}
			if err := mgr.TestTarget(t.Context(), target); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("target reached network: %v", err)
			}
			rec, err := mgr.SyncCapsule(t.Context(), capID, target.ID)
			if err == nil || rec == nil || rec.Status != "failed" || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("false sync success: %+v %v", rec, err)
			}
			retained, err := database.GetReplicationTarget(t.Context(), target.ID)
			if err != nil || retained.Prefix != target.Prefix || retained.Endpoint != target.Endpoint || retained.SecretKey != target.SecretKey {
				t.Fatal("changed stored destination/credential")
			}
			capsule, err := database.GetCapsule(t.Context(), capID)
			if err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(capsule.FilePath); err != nil || string(data) != "mock-encrypted-capsule-content" {
				t.Fatal("changed source capsule")
			}
		})
	}
}

func TestSFTPCompatibilityCertificateCAFloor(t *testing.T) {
	for _, bits := range []int{1024, 2047, 2048} {
		t.Run(fmt.Sprint(bits), func(t *testing.T) {
			caKey, err := rsa.GenerateKey(rand.Reader, bits)
			if err != nil {
				t.Fatal(err)
			}
			ca, err := ssh.NewSignerFromKey(caKey)
			if err != nil {
				t.Fatal(err)
			}
			_, hostKey, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			host, err := ssh.NewSignerFromKey(hostKey)
			if err != nil {
				t.Fatal(err)
			}
			cert := &ssh.Certificate{Key: host.PublicKey(), CertType: ssh.HostCert, ValidPrincipals: []string{"127.0.0.1"}, ValidAfter: uint64(time.Now().Add(-time.Minute).Unix()), ValidBefore: uint64(time.Now().Add(time.Hour).Unix())}
			if err := cert.SignCert(rand.Reader, ca); err != nil {
				t.Fatal(err)
			}
			signer, err := ssh.NewCertSigner(cert, host)
			if err != nil {
				t.Fatal(err)
			}
			cfg := &ssh.ServerConfig{NoClientAuth: true}
			cfg.AddHostKey(signer)
			addr, _ := serveSFTPServerWithConfig(t, cfg, nil)
			client := replication.NewSFTPClient(addr, "fixture", "unused", t.TempDir(), ssh.FingerprintSHA256(cert))
			client.Timeout = 2 * time.Second
			err = client.TestConnection(t.Context())
			if bits < 2048 {
				if err == nil || !strings.Contains(err.Error(), "2048") {
					t.Fatalf("weak certificate CA accepted: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSFTPCompatibilityRSASigningDoesNotFallBackToSHA1(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	secret := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	host, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, algorithm := range []string{ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA} {
		t.Run(algorithm, func(t *testing.T) {
			cfg := &ssh.ServerConfig{PublicKeyAuthAlgorithms: []string{algorithm}, PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) { return nil, nil }}
			cfg.AddHostKey(host)
			addr, _ := serveSFTPServerWithConfig(t, cfg, nil)
			client := replication.NewSFTPClient(addr, "fixture", string(secret), t.TempDir(), ssh.FingerprintSHA256(host.PublicKey()))
			client.Timeout = 2 * time.Second
			err := client.TestConnection(t.Context())
			if algorithm == ssh.KeyAlgoRSA {
				if err == nil {
					t.Fatal("RSA authentication downgraded to SHA-1")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
