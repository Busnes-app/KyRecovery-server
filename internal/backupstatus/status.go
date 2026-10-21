// Package backupstatus classifies backup evidence and plans restores. It is pure: rows and a
// clock in, verdicts out. It never touches the filesystem or the database.
package backupstatus

import (
	"slices"
	"sort"
	"time"
)

// State is one finding about a capsule instance or a restic repository.
type State string

const (
	Corrupt   State = "corrupt"
	ScanError State = "scan_error"
	Missing   State = "missing"
	Never     State = "never"
	Stale     State = "stale"
	KeyAdded  State = "key_added"
	OldLock   State = "old_lock"
	Pending   State = "pending" // paired within one interval, no capsule yet: not a finding
	OK        State = "ok"
)

// precedence orders states from worst to best. Activity is not a state, so it hides nothing.
var precedence = []State{Corrupt, ScanError, Missing, Never, Stale, KeyAdded, OldLock, Pending, OK}

func rank(s State) int { return slices.Index(precedence, s) }

func worst(states []State) State {
	w := OK
	for _, s := range states {
		if rank(s) < rank(w) {
			w = s
		}
	}
	return w
}

const (
	DefaultInterval = 26 * time.Hour
	MinInterval     = 15 * time.Minute
	lockStaleAfter  = 30 * time.Minute // restic refreshes locks every 5 min, treats 30 as stale
	scanFreshFor    = time.Hour
)

// Capsule is a stored capsule row. Instance is its paired_app_id.
type Capsule struct {
	ID       string `json:"id"`
	Instance string `json:"instance"`
	Service  string `json:"-"`
	Digest   string `json:"digest"`
	Status   string `json:"status"` // "active" | "corrupt"
	// RecoveryKeyID is the key the capsule is sealed to, as its manifest names it.
	RecoveryKeyID string     `json:"recovery_key_id"`
	CreatedAt     time.Time  `json:"created_at"`
	DepositedAt   time.Time  `json:"deposited_at"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
}

// Instance is a paired product. Only status "paired" creates an expectation.
type Instance struct {
	ID            string     `json:"id"`
	Service       string     `json:"-"`
	AppName       string     `json:"app_name"`
	Status        string     `json:"status"`
	LastDepositAt *time.Time `json:"last_deposit_at,omitempty"`
	PairedAt      *time.Time `json:"paired_at,omitempty"`
}

// Repo is a stored restic repository row.
type Repo struct {
	Path            string        `json:"path"`
	Service         string        `json:"service_name,omitempty"`
	Expected        time.Duration `json:"-"`
	SnapshotCount   int           `json:"snapshot_count"`
	LastSnapshotAt  *time.Time    `json:"last_snapshot_at,omitempty"`
	RecentSnapshots []time.Time   `json:"-"` // newest last, at most 10, for the cadence hint
	SizeBytes       int64         `json:"size_bytes"`
	LockSeenAt      *time.Time    `json:"lock_seen_at,omitempty"`
	KeyCount        int           `json:"key_count"`
	AckedKeyCount   int           `json:"acked_key_count"`
	MissingSince    *time.Time    `json:"missing_since,omitempty"`
	Err             string        `json:"scan_error,omitempty"`
	MeasuredAt      *time.Time    `json:"measured_at,omitempty"`
}

// ScanState is the restic scanner's last attempt. Enabled is false when no restic
// directory is configured.
type ScanState struct {
	Enabled       bool       `json:"enabled"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
}

// Fresh reports whether restic rows can be shown on their own authority.
func (s ScanState) Fresh(now time.Time) bool {
	return s.Enabled && s.LastError == "" && s.LastSuccessAt != nil && now.Sub(*s.LastSuccessAt) <= scanFreshFor
}

type Input struct {
	Now       time.Time
	Instances []Instance
	Capsules  []Capsule // extant capsules only
	Repos     []Repo
	Expected  map[string]time.Duration // per service
	Scan      ScanState
}

// Cadence is the observed median gap between recent backups.
type Cadence struct {
	MedianSeconds int64 `json:"median_seconds"`
	Mismatch      bool  `json:"mismatch"`
}

type InstanceStatus struct {
	Instance
	State        State    `json:"state"`
	CapsuleCount int      `json:"capsule_count"`
	Newest       *Capsule `json:"newest,omitempty"`
	Cadence      *Cadence `json:"cadence,omitempty"`
}

type RepoStatus struct {
	Repo
	State           State    `json:"state"`
	Flags           []State  `json:"flags"`
	Activity        bool     `json:"activity"`
	ExpectedSeconds int64    `json:"expected_seconds"`
	Cadence         *Cadence `json:"cadence,omitempty"`
}

type ServiceStatus struct {
	Name            string           `json:"service_name"`
	State           State            `json:"state"`
	Flags           []State          `json:"flags"`
	ExpectsCapsule  bool             `json:"expects_capsule"`
	ExpectsRestic   bool             `json:"expects_restic"`
	ExpectedSeconds int64            `json:"expected_seconds"`
	Instances       []InstanceStatus `json:"instances"`
	Repos           []RepoStatus     `json:"repos"`
}

type Report struct {
	Services   []ServiceStatus `json:"services"`
	Unassigned []RepoStatus    `json:"unassigned"`
	Counts     map[State]int   `json:"counts"`
	Worst      State           `json:"worst"`
	ScanFresh  bool            `json:"scan_fresh"`
}

// Evaluate classifies every service a paired product or a repository link names.
func Evaluate(in Input) Report {
	fresh := in.Scan.Fresh(in.Now)
	svcs := map[string]*ServiceStatus{}
	get := func(name string) *ServiceStatus {
		if s := svcs[name]; s != nil {
			return s
		}
		exp := in.Expected[name]
		if exp == 0 {
			exp = DefaultInterval
		}
		s := &ServiceStatus{Name: name, ExpectedSeconds: int64(exp / time.Second), Instances: []InstanceStatus{}, Repos: []RepoStatus{}}
		svcs[name] = s
		return s
	}

	byInstance := map[string][]Capsule{}
	for _, c := range in.Capsules {
		byInstance[c.Instance] = append(byInstance[c.Instance], c)
	}
	for _, inst := range in.Instances {
		if inst.Status != "paired" {
			continue
		}
		s := get(inst.Service)
		s.ExpectsCapsule = true
		s.Instances = append(s.Instances, classifyInstance(inst, byInstance[inst.ID], time.Duration(s.ExpectedSeconds)*time.Second, in.Now))
	}

	rep := Report{Unassigned: []RepoStatus{}, Counts: map[State]int{}, ScanFresh: fresh}
	for _, r := range in.Repos {
		if r.Service == "" {
			rep.Unassigned = append(rep.Unassigned, classifyRepo(r, false, 0, true, in.Now))
			continue
		}
		s := get(r.Service)
		s.ExpectsRestic = true
		exp := r.Expected
		if exp == 0 {
			exp = time.Duration(s.ExpectedSeconds) * time.Second
		}
		s.Repos = append(s.Repos, classifyRepo(r, true, exp, fresh, in.Now))
	}

	var all []State
	for _, s := range svcs {
		var states []State
		for _, i := range s.Instances {
			states = append(states, i.State)
		}
		for _, r := range s.Repos {
			states = append(states, r.Flags...)
		}
		s.Flags = sortedSet(states)
		s.State = worst(s.Flags)
		rep.Counts[s.State]++
		all = append(all, s.State)
		rep.Services = append(rep.Services, *s)
	}
	for _, r := range rep.Unassigned {
		all = append(all, r.State)
	}
	sort.Slice(rep.Services, func(i, j int) bool { return rep.Services[i].Name < rep.Services[j].Name })
	if rep.Services == nil {
		rep.Services = []ServiceStatus{}
	}
	rep.Worst = worst(all)
	return rep
}

func classifyInstance(inst Instance, caps []Capsule, exp time.Duration, now time.Time) InstanceStatus {
	sortCapsules(caps)
	st := InstanceStatus{Instance: inst, CapsuleCount: len(caps)}
	var times []time.Time
	for _, c := range caps {
		times = append(times, c.DepositedAt)
	}
	st.Cadence = cadence(times, exp)
	switch {
	case len(caps) == 0 && inst.PairedAt != nil && now.Sub(*inst.PairedAt) <= exp:
		st.State = Pending
	case len(caps) == 0:
		st.State = Never
	case caps[len(caps)-1].Status == "corrupt":
		st.State = Corrupt
	case overdue(caps[len(caps)-1].DepositedAt, exp, now):
		st.State = Stale
	default:
		st.State = OK
	}
	if len(caps) > 0 {
		n := caps[len(caps)-1]
		st.Newest = &n
	}
	return st
}

// classifyRepo judges one repository. An unassigned repository raises only key and read
// findings: nothing expects it to be fresh.
func classifyRepo(r Repo, linked bool, exp time.Duration, fresh bool, now time.Time) RepoStatus {
	var f []State
	if r.Err != "" || (linked && !fresh) {
		f = append(f, ScanError)
	}
	if linked {
		switch {
		case r.MissingSince != nil:
			f = append(f, Missing)
		case r.SnapshotCount == 0 || r.LastSnapshotAt == nil:
			f = append(f, Never)
		case overdue(*r.LastSnapshotAt, exp, now):
			f = append(f, Stale)
		}
	}
	if r.KeyCount > r.AckedKeyCount {
		f = append(f, KeyAdded)
	}
	activity := false
	if r.LockSeenAt != nil {
		if now.Sub(*r.LockSeenAt) <= lockStaleAfter {
			activity = true
		} else if linked {
			f = append(f, OldLock)
		}
	}
	flags := sortedSet(f)
	st := RepoStatus{Repo: r, Flags: flags, State: worst(flags), Activity: activity, ExpectedSeconds: int64(exp / time.Second)}
	if linked {
		st.Cadence = cadence(r.RecentSnapshots, exp)
	}
	return st
}

func overdue(last time.Time, exp time.Duration, now time.Time) bool {
	return now.Sub(last) > exp+exp/10
}

// cadence is the median gap across the last 10 times. Fewer than 3 points say nothing.
func cadence(times []time.Time, exp time.Duration) *Cadence {
	if len(times) > 10 {
		times = times[len(times)-10:]
	}
	if len(times) < 3 {
		return nil
	}
	gaps := make([]time.Duration, 0, len(times)-1)
	for i := 1; i < len(times); i++ {
		gaps = append(gaps, times[i].Sub(times[i-1]))
	}
	slices.Sort(gaps)
	m := gaps[len(gaps)/2]
	if len(gaps)%2 == 0 {
		m = (gaps[len(gaps)/2-1] + m) / 2
	}
	return &Cadence{MedianSeconds: int64(m / time.Second), Mismatch: m > 2*exp || 2*m < exp}
}

func sortedSet(states []State) []State {
	out := []State{}
	for _, s := range precedence {
		if s != OK && slices.Contains(states, s) {
			out = append(out, s)
		}
	}
	return out
}

// sortCapsules orders oldest first by deposit time, then ID.
func sortCapsules(caps []Capsule) {
	sort.Slice(caps, func(i, j int) bool {
		if !caps[i].DepositedAt.Equal(caps[j].DepositedAt) {
			return caps[i].DepositedAt.Before(caps[j].DepositedAt)
		}
		return caps[i].ID < caps[j].ID
	})
}

// Reason is the fixed health reason code for a state, empty for states that are not findings.
func Reason(s State) string {
	return map[State]string{
		Corrupt:   "backup_corrupt",
		ScanError: "backup_scan_failed",
		Missing:   "backup_missing",
		Never:     "backup_never",
		Stale:     "backup_stale",
		KeyAdded:  "restic_key_added",
		OldLock:   "restic_old_lock",
	}[s]
}

// Reasons lists every reason code, for declaring them once at startup.
func Reasons() []string {
	var out []string
	for _, s := range precedence {
		if r := Reason(s); r != "" {
			out = append(out, r)
		}
	}
	return out
}
