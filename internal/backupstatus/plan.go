package backupstatus

import (
	"errors"
	"fmt"
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
	Missing   map[string]bool       // linked repositories the last good scan did not find
	Keys      map[string]Key        // every recovery key the store has held, by key ID
}

// Key is a recovery key as a restore needs it: which cards, and how many of them.
type Key struct {
	ID          string `json:"key_id"`
	Threshold   int    `json:"threshold"`
	TotalShares int    `json:"total_shares"`
	Status      string `json:"status"` // "current" | "previous" | "retired"
}

type Plan struct {
	Service     string    `json:"service_name"`
	Instance    string    `json:"instance,omitempty"`
	Repo        string    `json:"repo,omitempty"`
	RepoMissing bool      `json:"repo_missing,omitempty"` // Repo not found by the last good scan: no snapshot
	At          time.Time `json:"at"`
	Capsule     *Capsule  `json:"capsule,omitempty"`
	// RecoveryKey is the key Capsule is sealed to: restoring it takes Threshold of the
	// TotalShares custodian cards printed with this key ID.
	RecoveryKey *Key `json:"recovery_key,omitempty"`
	// Skipped are newer corrupt capsules passed over to reach Capsule, newest first.
	Skipped    []Capsule `json:"skipped"`
	Snapshot   *Snapshot `json:"snapshot,omitempty"`
	GapSeconds *int64    `json:"gap_seconds,omitempty"` // snapshot written minus capsule deposited
	Warnings   []string  `json:"warnings"`
	// Steps are the restore instructions, in order, as plain text. They describe the
	// documented offline restore (docs/RECOVERY-AND-SECURITY.md), not a command to paste.
	Steps []string `json:"steps"`
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
	// WarnOldKey and WarnUnknownKey are format strings; they name the key ID.
	WarnOldKey     = "This capsule is sealed to the rotated-out recovery key %s. It opens only with %d of the %d custodian cards printed with that key ID, not the current cards."
	WarnUnknownKey = "This capsule is sealed to recovery key %s, which this store has no record of. Find the custodian cards printed with that key ID."
)

// RestorePlan picks the capsule and snapshot to restore Service to as of At.
func RestorePlan(in PlanInput) (Plan, error) {
	at := in.At.UTC()
	if at.After(in.Now.Add(futureSlack)) {
		return Plan{}, ErrFutureAt
	}
	p := Plan{Service: in.Service, At: at, Skipped: []Capsule{}, Warnings: []string{}, Steps: []string{}}

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

	if p.Capsule != nil {
		if k, ok := in.Keys[p.Capsule.RecoveryKeyID]; ok {
			p.RecoveryKey = &k
			if k.Status != "current" {
				p.Warnings = append(p.Warnings, fmt.Sprintf(WarnOldKey, k.ID, k.Threshold, k.TotalShares))
			}
		} else {
			p.Warnings = append(p.Warnings, fmt.Sprintf(WarnUnknownKey, p.Capsule.RecoveryKeyID))
		}
	}

	p.RepoMissing = in.Missing[repo]
	if repo != "" && !p.RepoMissing {
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
	p.Steps = steps(p)
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

// steps is the restore that works today: download the capsule, check it, and open it with
// the product's own restore command and the custodian shares. KyRecovery runs none of it.
// ponytail: no single command is printed because no suite-wide restore tool takes a capsule
// yet; when KyQuickStart ships one, print its exact command here and test it against that CLI.
func steps(p Plan) []string {
	out := []string{}
	if c := p.Capsule; c != nil {
		cards := fmt.Sprintf("the custodian cards printed with recovery key %s", c.RecoveryKeyID)
		if k := p.RecoveryKey; k != nil {
			cards = fmt.Sprintf("%d of the %d custodian cards printed with recovery key %s", k.Threshold, k.TotalShares, k.ID)
		}
		out = append(out,
			fmt.Sprintf("Download capsule %s (needs an operator or admin) and check that its SHA-256, from sha256sum, is %s.", c.ID, c.Digest),
			fmt.Sprintf("On your workstation, run %s's own restore command against the downloaded file, as that product's restore guide describes. Type the shares from %s on its standard input, one per line. Never type shares into this page.", p.Service, cards),
			fmt.Sprintf("Check that the capsule ID and creation time the restore prints match %s and %s. Decrypting proves the capsule is authentic, not that it is the newest.", c.ID, c.CreatedAt.UTC().Format(time.RFC3339)),
		)
	}
	if s := p.Snapshot; s != nil {
		out = append(out, fmt.Sprintf("Restic snapshot %s in repository %s holds the restic side of this restore point. KyRecovery holds no restic password and cannot restore it: use the tool that made the backup, which holds the repository password.", s.ID, p.Repo))
	}
	return out
}
