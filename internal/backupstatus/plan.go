package backupstatus

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Snapshot is a restic snapshot file: its ID and when rest-server wrote it.
type Snapshot struct {
	ID        string    `json:"id"`
	WrittenAt time.Time `json:"written_at"`
}

type PlanInput struct {
	Now, At   time.Time
	Service   string
	Instance  string                // required when Instances has more than one
	Repo      string                // required when Snapshots has more than one key
	Instances []Instance            // every instance of Service that holds capsules or is paired
	Capsules  []Capsule             // extant capsules of Service
	Snapshots map[string][]Snapshot // per linked repository path
}

type Plan struct {
	Service  string    `json:"service_name"`
	Instance string    `json:"instance,omitempty"`
	Repo     string    `json:"repo,omitempty"`
	At       time.Time `json:"at"`
	Capsule  *Capsule  `json:"capsule,omitempty"`
	// Skipped are newer corrupt capsules passed over to reach Capsule, newest first.
	Skipped    []Capsule `json:"skipped"`
	Snapshot   *Snapshot `json:"snapshot,omitempty"`
	GapSeconds *int64    `json:"gap_seconds,omitempty"` // snapshot written minus capsule deposited
	Warnings   []string  `json:"warnings"`
	Command    string    `json:"command,omitempty"`
}

// AmbiguousError asks the caller to choose rather than have the plan pick arbitrarily.
type AmbiguousError struct {
	Field   string
	Choices []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%s is required: one of %s", e.Field, strings.Join(e.Choices, ", "))
}

var ErrFutureAt = errors.New("at is in the future")

const (
	futureSlack = 5 * time.Minute
	mismatchGap = time.Hour

	WarnPairing  = "Capsule and snapshot are paired by write time; that does not establish they are consistent with each other."
	WarnMismatch = "Capsule and snapshot were written more than an hour apart."
)

// shellSafe admits the characters a quoted command argument may hold. Every segment of a
// repository path must pass separately.
var shellSafe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func safe(s string) bool {
	for _, seg := range strings.Split(s, "/") {
		if !shellSafe.MatchString(seg) || strings.Contains(seg, "..") {
			return false
		}
	}
	return true
}

// RestorePlan picks the capsule and snapshot to restore Service to as of At.
func RestorePlan(in PlanInput) (Plan, error) {
	at := in.At.UTC()
	if at.After(in.Now.Add(futureSlack)) {
		return Plan{}, ErrFutureAt
	}
	p := Plan{Service: in.Service, At: at, Skipped: []Capsule{}, Warnings: []string{}}

	inst, err := choose("instance", in.Instance, idsOf(in.Instances))
	if err != nil {
		return Plan{}, err
	}
	repos := make([]string, 0, len(in.Snapshots))
	for r := range in.Snapshots {
		repos = append(repos, r)
	}
	repo, err := choose("repo", in.Repo, repos)
	if err != nil {
		return Plan{}, err
	}
	p.Instance, p.Repo = inst, repo

	var caps []Capsule
	for _, c := range in.Capsules {
		if c.Instance == inst && !c.DepositedAt.After(at) {
			caps = append(caps, c)
		}
	}
	sortCapsules(caps)
	for i := len(caps) - 1; i >= 0; i-- {
		if caps[i].Status == "corrupt" {
			p.Skipped = append(p.Skipped, caps[i])
			continue
		}
		c := caps[i]
		p.Capsule = &c
		break
	}

	if repo != "" {
		// With a capsule, pair by write time; without one (restic-only), take the newest as of At.
		target, bound := at, true
		if p.Capsule != nil {
			target, bound = p.Capsule.DepositedAt, false
		}
		p.Snapshot = nearest(in.Snapshots[repo], target, bound)
	}
	if p.Capsule != nil && p.Snapshot != nil {
		gap := p.Snapshot.WrittenAt.Sub(p.Capsule.DepositedAt)
		secs := int64(gap / time.Second)
		p.GapSeconds = &secs
		p.Warnings = append(p.Warnings, WarnPairing)
		if gap > mismatchGap || gap < -mismatchGap {
			p.Warnings = append(p.Warnings, WarnMismatch)
		}
	}
	if p.Capsule != nil || p.Snapshot != nil {
		if commandSafe(p) {
			p.Command = command(p)
		} else {
			p.Warnings = append(p.Warnings, "A name here cannot be passed safely on a command line; no command is shown.")
		}
	}
	return p, nil
}

func idsOf(insts []Instance) []string {
	out := make([]string, 0, len(insts))
	for _, i := range insts {
		out = append(out, i.ID)
	}
	return out
}

// choose returns want if it is a choice, the only choice when want is empty, or an error.
func choose(field, want string, choices []string) (string, error) {
	sort.Strings(choices)
	if want != "" {
		for _, c := range choices {
			if c == want {
				return want, nil
			}
		}
		return "", &AmbiguousError{Field: field, Choices: choices}
	}
	switch len(choices) {
	case 0:
		return "", nil
	case 1:
		return choices[0], nil
	}
	return "", &AmbiguousError{Field: field, Choices: choices}
}

// nearest is the snapshot written closest to target; ties go to the earlier, then lower ID.
// When bound is set, only snapshots written at or before target count.
func nearest(snaps []Snapshot, target time.Time, bound bool) *Snapshot {
	var best *Snapshot
	var bestD time.Duration
	for i := range snaps {
		s := snaps[i]
		if bound && s.WrittenAt.After(target) {
			continue
		}
		d := s.WrittenAt.Sub(target)
		if d < 0 {
			d = -d
		}
		if best == nil || d < bestD || (d == bestD && (s.WrittenAt.Before(best.WrittenAt) ||
			(s.WrittenAt.Equal(best.WrittenAt) && s.ID < best.ID))) {
			best, bestD = &s, d
		}
	}
	return best
}

// command is the pinned KyQuickStart restore contract. Every argument has passed safe.
func command(p Plan) string {
	b := "kyquickstart restore '" + p.Service + "'"
	if p.Capsule != nil {
		b += " --capsule '" + p.Capsule.ID + "'"
	}
	if p.Snapshot != nil {
		b += " --repo '" + p.Repo + "' --snapshot '" + p.Snapshot.ID + "'"
	}
	return b
}

func commandSafe(p Plan) bool {
	if !safe(p.Service) || (p.Capsule != nil && !safe(p.Capsule.ID)) {
		return false
	}
	return p.Snapshot == nil || (safe(p.Repo) && safe(p.Snapshot.ID))
}
