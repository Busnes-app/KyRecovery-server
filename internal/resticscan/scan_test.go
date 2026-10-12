package resticscan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func id(i int) string { return fmt.Sprintf("%064x", i) }

// put writes size bytes at base/rel and sets its mtime when mtime is non-zero.
func put(t *testing.T, base, rel string, size int, mtime time.Time) {
	t.Helper()
	p := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
}

// mkRepo creates a minimal repository at base/rel.
func mkRepo(t *testing.T, base, rel string) {
	t.Helper()
	put(t, base, rel+"/config", 1, time.Time{})
	put(t, base, rel+"/keys/"+id(1), 1, time.Time{})
}

func scan(t *testing.T, base string, lim Limits) ([]Repo, error) {
	t.Helper()
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return Scan(context.Background(), root, lim)
}

func paths(rs []Repo) string {
	var ps []string
	for _, r := range rs {
		ps = append(ps, r.Path)
	}
	return strings.Join(ps, ",")
}

func TestScanMeasuresARepository(t *testing.T) {
	base := t.TempDir()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	put(t, base, "alice/config", 1, time.Time{})
	put(t, base, "alice/keys/"+id(1), 1, time.Time{})
	put(t, base, "alice/keys/"+id(2), 1, time.Time{})
	put(t, base, "alice/keys/tmp-upload", 1, time.Time{})
	put(t, base, "alice/snapshots/"+id(9), 1, t1)
	put(t, base, "alice/snapshots/"+id(8), 1, t0)
	put(t, base, "alice/snapshots/"+strings.ToUpper(id(10)), 1, t0)
	put(t, base, "alice/snapshots/.rest-server-temp", 1, t0)
	put(t, base, "alice/data/ab/"+id(3), 10, time.Time{})
	put(t, base, "alice/data/ab/"+id(4), 5, time.Time{})
	put(t, base, "alice/data/ab/junk", 100, time.Time{})
	put(t, base, "alice/data/zz/"+id(5), 100, time.Time{})
	put(t, base, "alice/data/"+id(6), 100, time.Time{})
	put(t, base, "alice/locks/"+id(7), 1, t0)
	put(t, base, "alice/locks/"+id(8), 1, t1)
	put(t, base, "alice/locks/stale", 1, t1.Add(time.Hour))
	mkRepo(t, base, "bob")
	put(t, base, "stray/file", 1, time.Time{})

	rs, err := scan(t, base, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(rs); got != "alice,bob" {
		t.Fatalf("paths = %q", got)
	}
	a := rs[0]
	want := []Snapshot{{id(8), t0}, {id(9), t1}}
	if a.Err != "" || len(a.Snapshots) != 2 || a.Snapshots[0] != want[0] || a.Snapshots[1] != want[1] {
		t.Fatalf("alice = %+v", a)
	}
	if a.SizeBytes != 15 || a.KeyCount != 2 || a.LockSeenAt == nil || !a.LockSeenAt.Equal(t1) {
		t.Fatalf("alice = %+v", a)
	}
	if b := rs[1]; b.LockSeenAt != nil || b.KeyCount != 1 || b.Snapshots != nil || b.SizeBytes != 0 {
		t.Fatalf("bob = %+v", b)
	}
}

func TestScanPrivateReposLayout(t *testing.T) {
	base := t.TempDir()
	mkRepo(t, base, "bob/kynotes")
	mkRepo(t, base, "bob/kypass")
	mkRepo(t, base, "carol")
	mkRepo(t, base, "carol/inner") // inside a repo: not a repo of its own
	mkRepo(t, base, "dave/x/y")    // too deep

	rs, err := scan(t, base, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(rs); got != "bob/kynotes,bob/kypass,carol" {
		t.Fatalf("paths = %q", got)
	}
}

func TestScanDoesNotFollowSymlinks(t *testing.T) {
	base, outside := t.TempDir(), t.TempDir()
	mkRepo(t, outside, "eve")
	put(t, outside, "eve/snapshots/"+id(1), 1, time.Time{})
	mkRepo(t, base, "alice")
	mkRepo(t, base, "bob")
	for _, l := range [][2]string{
		{filepath.Join(outside, "eve"), "eve"},
		{filepath.Join(outside, "eve/snapshots"), "alice/snapshots"},
		{"alice", "mallory"},
	} {
		if err := os.Symlink(l[0], filepath.Join(base, l[1])); err != nil {
			t.Fatal(err)
		}
	}
	// bob/keys is now a symlink, so bob is no longer a repository.
	if err := os.RemoveAll(filepath.Join(base, "bob/keys")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../alice/keys", filepath.Join(base, "bob/keys")); err != nil {
		t.Fatal(err)
	}

	rs, err := scan(t, base, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(rs); got != "alice" {
		t.Fatalf("paths = %q", got)
	}
	if a := rs[0]; a.Err != "" || a.Snapshots != nil {
		t.Fatalf("alice = %+v", a)
	}
}

func skipIfRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
}

func TestScanFailsOnUnreadableRoot(t *testing.T) {
	skipIfRoot(t)
	base := t.TempDir()
	mkRepo(t, base, "alice")
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Chmod(base, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(base, 0o755) })

	if rs, err := Scan(context.Background(), root, DefaultLimits); err == nil {
		t.Fatalf("Scan = %+v, want error", rs)
	}
}

// An unreadable user directory would otherwise hide its repositories, which would then
// read as missing. Failing the scan keeps every earlier row.
func TestScanFailsOnUnreadableUserDirectory(t *testing.T) {
	skipIfRoot(t)
	base := t.TempDir()
	mkRepo(t, base, "alice/kynotes")
	mkRepo(t, base, "bob")
	dir := filepath.Join(base, "alice")
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	// 0: nothing inside can be inspected. 0100: config can be checked, the listing cannot.
	for _, mode := range []os.FileMode{0, 0o100} {
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		if rs, err := scan(t, base, DefaultLimits); err == nil {
			t.Fatalf("mode %o: Scan = %+v, want error", mode, rs)
		}
	}
}

func TestScanReportsAnUnreadableRepository(t *testing.T) {
	skipIfRoot(t)
	base := t.TempDir()
	mkRepo(t, base, "alice")
	put(t, base, "alice/data/ab/"+id(3), 10, time.Time{})
	put(t, base, "alice/snapshots/"+id(2), 1, time.Time{})
	mkRepo(t, base, "bob")
	put(t, base, "bob/snapshots/"+id(2), 1, time.Time{})
	snaps := filepath.Join(base, "alice/snapshots")
	if err := os.Chmod(snaps, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(snaps, 0o755) })

	rs, err := scan(t, base, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(rs); got != "alice,bob" {
		t.Fatalf("paths = %q", got)
	}
	a := rs[0]
	if a.Err == "" || a.Snapshots != nil || a.KeyCount != 0 || a.SizeBytes != 0 || a.LockSeenAt != nil {
		t.Fatalf("alice = %+v", a)
	}
	if strings.Contains(a.Err, base) {
		t.Fatalf("Err leaks absolute path: %q", a.Err)
	}
	if b := rs[1]; b.Err != "" || len(b.Snapshots) != 1 {
		t.Fatalf("bob = %+v", b)
	}
}

func TestScanBounds(t *testing.T) {
	base := t.TempDir()
	mkRepo(t, base, "alice")
	mkRepo(t, base, "bob")
	for _, lim := range []Limits{
		{MaxRepos: 1, MaxEntries: 1000},
		{MaxRepos: 10, MaxEntries: 3},
	} {
		if rs, err := scan(t, base, lim); !errors.Is(err, ErrLimit) {
			t.Errorf("%+v: Scan = %+v, %v; want ErrLimit", lim, rs, err)
		}
	}
	if _, err := scan(t, base, Limits{MaxRepos: 2, MaxEntries: 1000}); err != nil {
		t.Errorf("at the bound: %v", err)
	}
}

func TestScanStopsWhenCancelled(t *testing.T) {
	base := t.TempDir()
	mkRepo(t, base, "alice")
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rs, err := Scan(ctx, root, DefaultLimits); !errors.Is(err, context.Canceled) {
		t.Fatalf("Scan = %+v, %v; want context.Canceled", rs, err)
	}
}
