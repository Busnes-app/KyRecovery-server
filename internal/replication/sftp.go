package replication

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// defaultTransferBudget bounds one whole Put or TestConnection, dial included.
// A target that accepts the TCP connection and then stalls must not pin a
// goroutine and the capsule's file handle forever.
const defaultTransferBudget = 5 * time.Minute

// UnknownHostKeyError is returned when a target has no pinned host key. It
// carries the server's fingerprint so the operator can pin it deliberately.
type UnknownHostKeyError struct{ Fingerprint string }

func (e *UnknownHostKeyError) Error() string {
	return "host key not pinned; server presented " + e.Fingerprint
}

// SFTPClient preserves absolute-directory targets unsupported by offsite v0.1.0.
// Relative-directory targets use the library. It uploads capsules over SSH. The server's host key must match the
// pinned SHA256 fingerprint; a blank pin never connects.
type SFTPClient struct {
	Addr    string // host or host:port, port defaults to 22
	User    string
	Secret  string // password, or a PEM private key
	Dir     string
	HostKey string        // "SHA256:..." as printed by ssh-keygen -l
	Timeout time.Duration // whole-operation budget; zero means defaultTransferBudget
}

func NewSFTPClient(addr, user, secret, dir, hostKey string) *SFTPClient {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "22")
	}
	return &SFTPClient{Addr: addr, User: user, Secret: secret, Dir: strings.TrimSuffix(dir, "/"), HostKey: hostKey}
}

// auth picks key or password by PEM structure, never by prefix: a private key
// with stray leading whitespace must not be sent to the server as a password.
func (c *SFTPClient) auth() (ssh.AuthMethod, error) {
	if block, _ := pem.Decode([]byte(c.Secret)); block != nil {
		signer, err := ssh.ParsePrivateKey([]byte(c.Secret))
		if err != nil {
			return nil, fmt.Errorf("private key: %w", err)
		}
		if err := checkSSHKey(signer.PublicKey()); err != nil {
			return nil, err
		}
		if signer.PublicKey().Type() == ssh.KeyAlgoRSA {
			algorithmSigner, ok := signer.(ssh.AlgorithmSigner)
			if !ok {
				return nil, fmt.Errorf("RSA signer does not support SHA-2")
			}
			signer, err = ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256})
			if err != nil {
				return nil, fmt.Errorf("RSA signing policy: %w", err)
			}
		}
		return ssh.PublicKeys(signer), nil
	}
	return ssh.Password(c.Secret), nil
}

// dial returns a connected SFTP session and a cleanup that closes it. The
// connection is torn down when ctx ends, which unblocks any stalled I/O.
func (c *SFTPClient) dial(ctx context.Context) (*sftp.Client, func(), error) {
	auth, err := c.auth()
	if err != nil {
		return nil, nil, err
	}
	algorithms := ssh.SupportedAlgorithms()
	cfg := &ssh.ClientConfig{
		Config:            ssh.Config{KeyExchanges: algorithms.KeyExchanges, Ciphers: algorithms.Ciphers, MACs: algorithms.MACs},
		HostKeyAlgorithms: algorithms.HostKeys,
		User:              c.User,
		Auth:              []ssh.AuthMethod{auth},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			if err := checkSSHKey(key); err != nil {
				return err
			}
			fp := ssh.FingerprintSHA256(key)
			if c.HostKey == "" {
				return &UnknownHostKeyError{Fingerprint: fp}
			}
			if fp != c.HostKey {
				return fmt.Errorf("host key mismatch: pinned %s, server presented %s", c.HostKey, fp)
			}
			return nil
		},
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return nil, nil, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	cleanup := func() { stop(); conn.Close() }

	sconn, chans, reqs, err := ssh.NewClientConn(conn, c.Addr, cfg)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	client, err := sftp.NewClient(ssh.NewClient(sconn, chans, reqs))
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return client, func() { client.Close(); cleanup() }, nil
}

func (c *SFTPClient) budget(ctx context.Context) (context.Context, context.CancelFunc) {
	t := c.Timeout
	if t == 0 {
		t = defaultTransferBudget
	}
	return context.WithTimeout(ctx, t)
}

// Reconcile abandoned compatibility uploads after their maximum expected lifetime.
// Keep at least the production budget even when a caller uses a shorter timeout,
// plus a grace minute for timestamp precision and teardown. Recent files may belong
// to another concurrent sync and must remain untouched. Housekeeping is best-effort:
// drop-box targets can allow writes without listing or deleting existing files.
func (c *SFTPClient) reapStaging(client *sftp.Client) {
	entries, err := client.ReadDir(c.Dir)
	if err != nil {
		audit.Log().Error("sftp_staging_cleanup", "replication-daemon", "", "Could not list staging files; continuing transfer", err)
		return
	}
	cutoff := time.Now().Add(-max(defaultTransferBudget, c.Timeout) - time.Minute)
	for _, entry := range entries {
		if entry.Mode().IsRegular() && strings.HasPrefix(entry.Name(), ".ky-offsite-compat-") && entry.ModTime().Before(cutoff) {
			if err := client.Remove(path.Join(c.Dir, entry.Name())); err != nil && !os.IsNotExist(err) {
				audit.Log().Error("sftp_staging_cleanup", "replication-daemon", "", "Could not remove expired staging file; continuing transfer", err)
			}
		}
	}
}

// Put streams data to Dir/name, creating Dir if needed. The bytes land in a
// temporary name and are renamed into place, so an aborted transfer never
// replaces a complete replica with a short one.
func (c *SFTPClient) Put(ctx context.Context, name string, data io.Reader) error {
	ctx, cancel := c.budget(ctx)
	defer cancel()
	client, cleanup, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := client.MkdirAll(c.Dir); err != nil {
		return err
	}
	c.reapStaging(client)
	final := path.Join(c.Dir, name)
	tmp := path.Join(path.Dir(final), ".ky-offsite-compat-"+rand.Text())
	f, err := client.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	if _, err := f.ReadFrom(data); err != nil {
		f.Close()
		client.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		client.Remove(tmp)
		return err
	}
	if err := client.PosixRename(tmp, final); err != nil {
		client.Remove(tmp)
		return err
	}
	return nil
}

// TestConnection authenticates and proves Dir is writable.
func (c *SFTPClient) TestConnection(ctx context.Context) error {
	ctx, cancel := c.budget(ctx)
	defer cancel()
	client, cleanup, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := client.MkdirAll(c.Dir); err != nil {
		return err
	}
	c.reapStaging(client)
	probe := path.Join(c.Dir, ".kyrecovery-ping")
	f, err := client.Create(probe)
	if err != nil {
		return err
	}
	f.Close()
	return client.Remove(probe)
}

// Absolute-directory targets retain this compatibility transport until the
// shared offsite API can represent them. Keep its key policy as strict as the
// shared client; a pinned fingerprint does not waive the key-size requirement.
func checkSSHKey(key ssh.PublicKey) error {
	if cert, ok := key.(*ssh.Certificate); ok {
		if !slices.Contains(ssh.SupportedAlgorithms().PublicKeyAuths, cert.Signature.Format) {
			return fmt.Errorf("SSH certificate uses an unsupported signature algorithm")
		}
		if err := checkSSHKey(cert.SignatureKey); err != nil {
			return err
		}
		key = cert.Key
	}
	public, ok := key.(ssh.CryptoPublicKey)
	if !ok {
		return fmt.Errorf("unsupported SSH key type %q", key.Type())
	}
	switch k := public.CryptoPublicKey().(type) {
	case *rsa.PublicKey:
		if k.N.BitLen() < 2048 {
			return fmt.Errorf("RSA SSH keys must be at least 2048 bits")
		}
	case *ecdsa.PublicKey:
		if k.Curve.Params().BitSize < 224 {
			return fmt.Errorf("EC SSH keys must be at least 224 bits")
		}
	case ed25519.PublicKey:
	default:
		return fmt.Errorf("unsupported SSH key type %q", key.Type())
	}
	return nil
}
