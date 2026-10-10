// Package resticscan reads restic repository metadata from a rest-server data directory.
// It lists directories and stats files; it never reads file contents.
package resticscan

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"time"
)

// Snapshot is one file in a repository's snapshots/ directory.
type Snapshot struct {
	ID        string    // the 64-hex file name
	WrittenAt time.Time // file mtime, UTC
}

// Repo is one repository's metadata. Err is set when this repository could not be read;
// its measurements are then zero and must not replace earlier ones.
type Repo struct {
	Path       string     // slash-separated, relative to the root, e.g. "alice" or "alice/kynotes"
	Snapshots  []Snapshot // sorted by WrittenAt, then ID
	SizeBytes  int64      // total bytes of admitted files under data/
	LockSeenAt *time.Time // newest admitted lock mtime, nil if none
	KeyCount   int
	Err        string
}

type Limits struct{ MaxRepos, MaxEntries int }

var DefaultLimits = Limits{MaxRepos: 1000, MaxEntries: 2_000_000}

var ErrLimit = errors.New("resticscan: scan bound exceeded")

// errSkip marks a path that is not a real directory (a symlink, a file); it reads as absent.
var errSkip = errors.New("not a directory")

type scanner struct {
	ctx     context.Context
	root    *os.Root
	lim     Limits
	entries int
}

// Scan walks root. Any error return means the whole scan failed and the result must be discarded.
func Scan(ctx context.Context, root *os.Root, lim Limits) ([]Repo, error) {
	s := &scanner{ctx: ctx, root: root, lim: lim}
	top, err := s.list(".")
	if err != nil {
		return nil, err
	}
	var repos []Repo
	for _, u := range top {
		if !u.IsDir() {
			continue
		}
		paths := []string{u.Name()}
		isTop, err := s.isRepo(u.Name())
		if err != nil {
			return nil, err
		}
		if !isTop {
			paths = nil
			sub, err := s.list(u.Name())
			if err != nil && !absent(err) {
				return nil, err
			}
			for _, e := range sub {
				p := u.Name() + "/" + e.Name()
				if !e.IsDir() {
					continue
				}
				ok, err := s.isRepo(p)
				if err != nil {
					return nil, err
				}
				if ok {
					paths = append(paths, p)
				}
			}
		}
		for _, p := range paths {
			r, ok, err := s.repo(p)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if repos = append(repos, r); len(repos) > s.lim.MaxRepos {
				return nil, fmt.Errorf("%w: more than %d repositories", ErrLimit, s.lim.MaxRepos)
			}
		}
	}
	slices.SortFunc(repos, func(a, b Repo) int { return cmp.Compare(a.Path, b.Path) })
	return repos, nil
}

func (s *scanner) fatal(err error) bool {
	return err != nil && (errors.Is(err, ErrLimit) || s.ctx.Err() != nil)
}

// isRepo reports whether p holds a regular config and a keys directory. A path that cannot
// be inspected fails the scan: guessing "not a repository" would later read as missing.
func (s *scanner) isRepo(p string) (bool, error) {
	c, err := s.root.Lstat(p + "/config")
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	k, err := s.root.Lstat(p + "/keys")
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return c.Mode().IsRegular() && k.IsDir(), nil
}

// repo measures p. ok is false when p stopped being a repository during the read.
func (s *scanner) repo(p string) (r Repo, ok bool, err error) {
	r = Repo{Path: p}
	if err := s.measure(&r); err != nil {
		if s.fatal(err) {
			return Repo{}, false, err
		}
		r = Repo{Path: p, Err: err.Error()}
	}
	still, err := s.isRepo(p)
	return r, still, err
}

func (s *scanner) measure(r *Repo) error {
	snaps, err := s.files(r.Path + "/snapshots")
	if err != nil {
		return err
	}
	for _, fi := range snaps {
		r.Snapshots = append(r.Snapshots, Snapshot{ID: fi.Name(), WrittenAt: fi.ModTime().UTC()})
	}
	slices.SortFunc(r.Snapshots, func(a, b Snapshot) int {
		return cmp.Or(a.WrittenAt.Compare(b.WrittenAt), cmp.Compare(a.ID, b.ID))
	})

	keys, err := s.files(r.Path + "/keys")
	if err != nil {
		return err
	}
	r.KeyCount = len(keys)

	locks, err := s.files(r.Path + "/locks")
	if err != nil {
		return err
	}
	for _, fi := range locks {
		if t := fi.ModTime().UTC(); r.LockSeenAt == nil || t.After(*r.LockSeenAt) {
			r.LockSeenAt = &t
		}
	}

	data, err := s.list(r.Path + "/data")
	if err != nil && !absent(err) {
		return err
	}
	for _, e := range data {
		if !e.IsDir() || !isHex(e.Name(), 2) {
			continue
		}
		packs, err := s.files(r.Path + "/data/" + e.Name())
		if err != nil {
			return err
		}
		for _, fi := range packs {
			r.SizeBytes += fi.Size()
		}
	}
	return nil
}

// files returns the admitted files in dir. A missing dir has none.
func (s *scanner) files(dir string) ([]fs.FileInfo, error) {
	es, err := s.list(dir)
	if absent(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []fs.FileInfo
	for _, e := range es {
		if !e.Type().IsRegular() || !isHex(e.Name(), 64) {
			continue
		}
		fi, err := s.root.Lstat(dir + "/" + e.Name())
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if fi.Mode().IsRegular() {
			out = append(out, fi)
		}
	}
	return out, nil
}

// list reads dir, counting every entry against MaxEntries.
func (s *scanner) list(dir string) ([]fs.DirEntry, error) {
	f, err := s.openDir(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var all []fs.DirEntry
	for {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		es, err := f.ReadDir(1024)
		if s.entries += len(es); s.entries > s.lim.MaxEntries {
			return nil, fmt.Errorf("%w: more than %d directory entries", ErrLimit, s.lim.MaxEntries)
		}
		all = append(all, es...)
		if err == io.EOF {
			return all, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// openDir opens dir only if it is a real directory, and still the same one once open.
func (s *scanner) openDir(dir string) (*os.File, error) {
	fi, err := s.root.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errSkip
	}
	f, err := s.root.Open(dir)
	if err != nil {
		return nil, err
	}
	if st, err := f.Stat(); err != nil || !os.SameFile(fi, st) {
		f.Close()
		return nil, errSkip
	}
	return f, nil
}

func absent(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, errSkip)
}

func isHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
