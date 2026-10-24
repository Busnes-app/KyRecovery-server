package server

import (
	"path/filepath"
	"strings"
	"testing"
)

// FuzzCapsuleNames checks the names a paired product chooses for itself. An accepted capsule
// ID becomes exactly one file directly in the capsule directory; an accepted service name
// is also an accepted ID and fits the "cap-<service>-<UnixNano>" shape a sealer mints.
func FuzzCapsuleNames(f *testing.F) {
	for _, s := range []string{
		"cap-" + strings.Repeat("s", 64) + "-1788490198114479660", "kysignon", "ky-notes", "ky_notes.v2", "A1",
		"", "../../etc/passwd", "a/b", "..", "cap id", "cap\\id", "x..y/../z", "-leading", "cap\x00",
		strings.Repeat("a", 129), strings.Repeat("s", 65),
		// Edge cases: a trailing newline (regexp $ is end of text), dots, case-only differences.
		"cap-a\n", ".hidden", "a.", "CON", "cap-A.kycap",
	} {
		f.Add(s)
	}
	root := filepath.Join(string(filepath.Separator), "srv", "kyrecovery")
	s := &Server{cfg: Config{DataDir: root}}
	f.Fuzz(func(t *testing.T, name string) {
		if validCapsuleID(name) {
			p := s.capsulePath(name)
			if filepath.Dir(p) != filepath.Join(root, "capsules") || filepath.Base(p) != name+".kycap" {
				t.Fatalf("capsule ID %q is stored at %q, outside %s/capsules", name, p, root)
			}
			if strings.ContainsAny(name, "/\\\x00\n") || len(name) > 128 {
				t.Fatalf("capsule ID %q accepted", name)
			}
		}
		if validServiceName(name) {
			if !validCapsuleID(name) {
				t.Fatalf("service name %q is not a valid capsule ID", name)
			}
			if minted := "cap-" + name + "-1788490198114479660"; !validCapsuleID(minted) {
				t.Fatalf("service name %q accepted but its capsule ID %q is refused", name, minted)
			}
		}
	})
}
