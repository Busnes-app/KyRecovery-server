package replication

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/Busnes-app/kyrecovery-server/internal/db"
	"golang.org/x/crypto/ssh"
)

// The dashboard's secret field is a password input, which strips line breaks. A pasted
// private key must still be used as a key, and a broken one never sent as a password.
func TestFlattenedPrivateKeyIsStillAKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.NewReplacer("\n", "", "\r", "").Replace(string(pem.EncodeToMemory(block)))
	if _, err := ssh.ParsePrivateKey([]byte(sshSecret(flat))); err != nil {
		t.Fatalf("flattened key does not parse after sshSecret: %v", err)
	}
	if _, err := NewSFTPClient("example.test", "ky", flat, "/vault", "SHA256:x").auth(); err != nil {
		t.Fatalf("absolute-SFTP auth with a flattened key: %v", err)
	}
	target := db.ReplicationTargetRecord{Type: "sftp", Endpoint: "example.test", AccessKey: "ky", Prefix: "vault", SecretKey: flat}
	l, err := targetLocation(target, "cap-1")
	if err != nil || l.config.Secret != sshSecret(flat) {
		t.Fatalf("shared SFTP location: %v", err)
	}
	if got := sshSecret("correct horse battery staple"); got != "correct horse battery staple" {
		t.Fatalf("a password changed: %q", got)
	}
	target.SecretKey = flat[:len(flat)/2]
	if _, err := targetLocation(target, "cap-1"); err == nil {
		t.Fatal("a truncated key was accepted, so it would be sent as a password")
	}
}
