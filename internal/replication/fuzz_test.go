package replication

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// safeDir reports whether dir is "" or a relative slash path with no empty, dot or dot-dot
// segment and no backslash or NUL: a directory that cannot climb out of its share.
func safeDir(dir string) bool {
	if dir == "" {
		return true
	}
	for _, part := range strings.Split(dir, "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00") {
			return false
		}
	}
	return true
}

// FuzzParseSMBEndpoint checks the endpoint forms an operator pastes (\\host\share\dir,
// //host/share, smb://host:port/share/dir). An accepted endpoint yields a host:port with no
// path or credentials, a share that names one share, and a directory that stays inside it;
// parsing the result again changes nothing. A refused one yields nothing.
func FuzzParseSMBEndpoint(f *testing.F) {
	for _, s := range [][3]string{
		{`\\nas\Public\Vault`, "", ""},
		{"127.0.0.1:1445", "vault", "kyrecovery-test-1/"},
		{"host", "share", ""},
		{"smb://host/share/dir", "", ""},
		{"//host/share", "", "nested/dir"},
		{"smb://user:password@host/share", "", ""},
		// Edge cases: IPv6, explicit share overriding the path, climbing, empty pieces.
		{"[2001:db8::1]:445", "share", ""},
		{"2001:db8::1", "share", ""},
		{"SMB://host/a/b/c", "other", "d"},
		{"host", "..", ""},
		{"host", "share", "../../etc"},
		{"host", "share", "a//b"},
		{"host", "sh\x00are", ""},
		{"smb://", "share", ""},
		{"host/share/./x", "", ""},
		{":445", "share", ""},
	} {
		f.Add(s[0], s[1], s[2])
	}
	f.Fuzz(func(t *testing.T, endpoint, share, dir string) {
		addr, gotShare, gotDir, err := ParseSMBEndpoint(endpoint, share, dir)
		if err != nil {
			if addr != "" || gotShare != "" || gotDir != "" {
				t.Fatalf("ParseSMBEndpoint(%q, %q, %q) failed (%v) but returned %q %q %q", endpoint, share, dir, err, addr, gotShare, gotDir)
			}
			return
		}
		if strings.Contains(endpoint, "@") {
			t.Fatalf("endpoint %q with userinfo accepted", endpoint)
		}
		if _, _, err := net.SplitHostPort(addr); err != nil || strings.ContainsAny(addr, "/\\@") {
			t.Fatalf("ParseSMBEndpoint(%q) addr %q is not a bare host:port", endpoint, addr)
		}
		if gotShare == "" || gotShare == "." || gotShare == ".." || strings.ContainsAny(gotShare, "/\\\x00") {
			t.Fatalf("ParseSMBEndpoint(%q, %q) share %q is unsafe", endpoint, share, gotShare)
		}
		if !safeDir(gotDir) {
			t.Fatalf("ParseSMBEndpoint(%q, %q, %q) dir %q leaves the share", endpoint, share, dir, gotDir)
		}
		a2, s2, d2, err := ParseSMBEndpoint(addr, gotShare, gotDir)
		if err != nil || a2 != addr || s2 != gotShare || d2 != gotDir {
			t.Fatalf("reparse of %q %q %q gave %q %q %q, %v", addr, gotShare, gotDir, a2, s2, d2, err)
		}
	})
}

const (
	fuzzAccess = "AKIAXW7ACCESSKEY"
	fuzzSecret = "zq9SecretKeyValue"
)

// FuzzTargetLocation maps stored replication targets to the URL and object name the
// offsite library gets. For every target ValidateTarget accepts: the object name is a safe
// relative path ending in .kycap, the URL carries no credentials, query or fragment, and
// for SMB the share and directory the library re-reads from the URL are the ones the
// stored fields named, so a target cannot validate as one share and write to another.
func FuzzTargetLocation(f *testing.F) {
	types := []string{"local", "s3", "sftp", "smb"}
	for _, s := range []struct {
		typ                          uint8
		endpoint, bucket, prefix, id string
	}{
		{1, "", "vault", "", "cap-KySignOn-1"},
		{1, "", "vault", "/nested/vault/", "cap-KySignOn-1"},
		{2, "host", "", "user/vault", "cap-KySignOn-1"},
		{2, "host:2222", "", "/absolute/vault", "cap-KySignOn-1"},
		{1, "https://vault.example", "vault", "", "cap-KySignOn-1"},
		{3, `\\nas\Public\Vault`, "", "", "cap-KySignOn-1"},
		{0, "/tmp/a #?b", "", "", "cap-KySignOn-1"},
		{3, "host", "share", "", "cap-Notes-1"},
		{1, "http://host", "", "", "cap-one"},
		{1, "https://user:secret@host", "", "", "cap-one"},
		{1, "https://host?secret=x", "", "", "cap-one"},
		// Edge cases: climbing prefixes and IDs, odd shares, unknown types.
		{1, "", "vault", "../", "cap-one"},
		{1, "", "vault", "a/../b/", "cap-one"},
		{3, "host", "sh%2Fare", "a?b#c", "cap-one"},
		{3, "[2001:db8::1]:445", "share", "x", "cap-one"},
		{2, "[::1]", "", "", ".."},
		{4, "host", "", "", "cap-one"},
	} {
		f.Add(s.typ, s.endpoint, s.bucket, s.prefix, s.id)
	}
	f.Fuzz(func(t *testing.T, typ uint8, endpoint, bucket, prefix, id string) {
		rec := db.ReplicationTargetRecord{Endpoint: endpoint, Bucket: bucket, Prefix: prefix, AccessKey: fuzzAccess, SecretKey: fuzzSecret, Type: fmt.Sprint(typ)}
		if int(typ) < len(types) {
			rec.Type = types[typ]
		}
		l, err := targetLocation(rec, id)
		if err != nil {
			return
		}
		if !replicaID.MatchString(id) || strings.Contains(endpoint, "@") {
			t.Fatalf("targetLocation accepted id %q endpoint %q", id, endpoint)
		}
		if !strings.HasSuffix(l.name, ".kycap") || strings.HasPrefix(l.name, "/") || !safeDir(l.name) {
			t.Fatalf("object name %q is not a safe relative path", l.name)
		}
		switch rec.Type {
		case "s3":
			if l.name != strings.TrimPrefix(prefix, "/")+id+".kycap" {
				t.Fatalf("s3 object name %q for prefix %q id %q", l.name, prefix, id)
			}
		case "smb":
			if l.name != fmt.Sprintf("kycap-v1-%x.kycap", sha256.Sum256([]byte(id))) {
				t.Fatalf("smb object name %q is not the hashed name of %q", l.name, id)
			}
		}
		if ValidateTarget(rec) != nil {
			return
		}
		u, err := url.Parse(l.config.URL)
		if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			t.Fatalf("validated target has URL %q (%v)", l.config.URL, err)
		}
		for _, cred := range []string{fuzzAccess, fuzzSecret} {
			if !strings.Contains(endpoint+bucket+prefix, cred) && strings.Contains(l.config.URL, cred) {
				t.Fatalf("a credential leaked into the URL %q", l.config.URL)
			}
		}
		if rec.Type == "smb" {
			addr, share, dir, _ := ParseSMBEndpoint(endpoint, bucket, prefix)
			a2, s2, d2, err := ParseSMBEndpoint(u.Host+u.Path, "", "")
			if err != nil || a2 != addr || s2 != share || d2 != dir {
				t.Fatalf("stored %q %q %q validates as %q %q %q but the URL %q reads as %q %q %q (%v)",
					endpoint, bucket, prefix, addr, share, dir, l.config.URL, a2, s2, d2, err)
			}
		}
	})
}
