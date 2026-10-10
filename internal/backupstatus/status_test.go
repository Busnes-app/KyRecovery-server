package backupstatus

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) time.Time   { return now.Add(-d) }
func agoP(d time.Duration) *time.Time { t := ago(d); return &t }

func freshScan() ScanState {
	return ScanState{Enabled: true, LastSuccessAt: agoP(10 * time.Minute)}
}

func paired(id, svc string) Instance { return Instance{ID: id, Service: svc, Status: "paired"} }

func capsule(id, inst string, age time.Duration, status string) Capsule {
	return Capsule{ID: id, Instance: inst, Status: status, DepositedAt: ago(age)}
}

func linked(path, svc string, last time.Duration) Repo {
	return Repo{Path: path, Service: svc, SnapshotCount: 1, LastSnapshotAt: agoP(last), KeyCount: 1, AckedKeyCount: 1}
}

func service(t *testing.T, r Report, name string) ServiceStatus {
	t.Helper()
	for _, s := range r.Services {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("service %q not in report", name)
	return ServiceStatus{}
}

func TestCapsuleStates(t *testing.T) {
	cases := []struct {
		name string
		caps []Capsule
		want State
	}{
		{"never", nil, Never},
		{"ok", []Capsule{capsule("c1", "a", time.Hour, "active")}, OK},
		{"stale past interval plus ten percent", []Capsule{capsule("c1", "a", 29*time.Hour, "active")}, Stale},
		{"within ten percent", []Capsule{capsule("c1", "a", 28*time.Hour, "active")}, OK},
		{"newest corrupt beats older good", []Capsule{
			capsule("c1", "a", 2*time.Hour, "active"), capsule("c2", "a", time.Hour, "corrupt")}, Corrupt},
		{"older corrupt does not matter", []Capsule{
			capsule("c1", "a", 2*time.Hour, "corrupt"), capsule("c2", "a", time.Hour, "active")}, OK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Evaluate(Input{Now: now, Instances: []Instance{paired("a", "kynotes")}, Capsules: tc.caps})
			if got := service(t, r, "kynotes").State; got != tc.want {
				t.Fatalf("state = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestFreshPairingIsPendingForOneInterval(t *testing.T) {
	for _, tc := range []struct {
		pairedAgo time.Duration
		want      State
	}{{time.Hour, Pending}, {29 * time.Hour, Never}} {
		inst := paired("a", "kynotes")
		inst.PairedAt = agoP(tc.pairedAgo)
		r := Evaluate(Input{Now: now, Instances: []Instance{inst}})
		if got := service(t, r, "kynotes").State; got != tc.want || (tc.want == Pending && r.Worst != Pending) {
			t.Fatalf("paired %v ago: state = %s worst = %s, want %s", tc.pairedAgo, got, r.Worst, tc.want)
		}
	}
}

// Retention can delete every capsule while paired_apps.last_backup_at stays set. Only extant
// capsules count, so the service must read never, not ok.
func TestRetentionEmptiedServiceIsNever(t *testing.T) {
	inst := paired("a", "kynotes")
	inst.LastDepositAt = agoP(time.Hour)
	r := Evaluate(Input{Now: now, Instances: []Instance{inst}})
	if got := service(t, r, "kynotes").State; got != Never {
		t.Fatalf("state = %s, want never", got)
	}
}

func TestPendingAndRevokedCreateNoExpectation(t *testing.T) {
	r := Evaluate(Input{Now: now, Instances: []Instance{
		{ID: "p", Service: "a", Status: "pending"}, {ID: "r", Service: "b", Status: "revoked"}}})
	if len(r.Services) != 0 || r.Worst != OK {
		t.Fatalf("services = %+v, worst = %s", r.Services, r.Worst)
	}
}

func TestChannelsExpectedOnlyWhenPresent(t *testing.T) {
	r := Evaluate(Input{
		Now:       now,
		Instances: []Instance{paired("a", "capsule-only")},
		Capsules:  []Capsule{capsule("c1", "a", time.Hour, "active")},
		Repos:     []Repo{linked("u/vault", "restic-only", time.Hour)},
		Scan:      freshScan(),
	})
	c, rs := service(t, r, "capsule-only"), service(t, r, "restic-only")
	if c.State != OK || !c.ExpectsCapsule || c.ExpectsRestic {
		t.Fatalf("capsule-only = %+v", c)
	}
	if rs.State != OK || rs.ExpectsCapsule || !rs.ExpectsRestic {
		t.Fatalf("restic-only = %+v", rs)
	}
}

func TestWorstInstanceAndRepoWin(t *testing.T) {
	r := Evaluate(Input{
		Now:       now,
		Instances: []Instance{paired("a", "s"), paired("b", "s")},
		Capsules:  []Capsule{capsule("c1", "a", time.Hour, "active"), capsule("c2", "b", 40*time.Hour, "active")},
		Repos:     []Repo{linked("u/one", "s", time.Hour), linked("u/two", "s", time.Hour)},
		Scan:      freshScan(),
	})
	s := service(t, r, "s")
	if s.State != Stale || len(s.Instances) != 2 || len(s.Repos) != 2 {
		t.Fatalf("service = %+v", s)
	}
}

func TestRepoStatesAndPrecedence(t *testing.T) {
	base := linked("u/r", "s", time.Hour)
	cases := []struct {
		name   string
		mod    func(*Repo)
		scan   ScanState
		want   State
		flags  []State
		active bool
	}{
		{"ok", func(*Repo) {}, freshScan(), OK, []State{}, false},
		{"never", func(r *Repo) { r.SnapshotCount, r.LastSnapshotAt = 0, nil }, freshScan(), Never, []State{Never}, false},
		{"stale", func(r *Repo) { r.LastSnapshotAt = agoP(30 * time.Hour) }, freshScan(), Stale, []State{Stale}, false},
		{"per-repo override", func(r *Repo) { r.Expected = time.Hour; r.LastSnapshotAt = agoP(2 * time.Hour) }, freshScan(), Stale, []State{Stale}, false},
		{"missing", func(r *Repo) { r.MissingSince = agoP(time.Hour) }, freshScan(), Missing, []State{Missing}, false},
		{"read error keeps other flags", func(r *Repo) { r.Err = "permission denied"; r.KeyCount = 2 }, freshScan(), ScanError, []State{ScanError, KeyAdded}, false},
		{"old lock", func(r *Repo) { r.LockSeenAt = agoP(31 * time.Minute) }, freshScan(), OldLock, []State{OldLock}, false},
		{"activity hides nothing", func(r *Repo) { r.LockSeenAt = agoP(time.Minute); r.LastSnapshotAt = agoP(30 * time.Hour) }, freshScan(), Stale, []State{Stale}, true},
		{"stale scan", func(*Repo) {}, ScanState{Enabled: true, LastSuccessAt: agoP(2 * time.Hour)}, ScanError, []State{ScanError}, false},
		{"failed latest scan", func(*Repo) {}, ScanState{Enabled: true, LastSuccessAt: agoP(time.Minute), LastError: "x"}, ScanError, []State{ScanError}, false},
		{"scanner disabled", func(*Repo) {}, ScanState{}, ScanError, []State{ScanError}, false},
		{"corrupt outranks all", nil, freshScan(), Corrupt, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			in := Input{Now: now, Repos: []Repo{r}, Scan: tc.scan}
			if tc.mod == nil {
				in.Instances = []Instance{paired("a", "s")}
				in.Capsules = []Capsule{capsule("c", "a", time.Hour, "corrupt")}
				in.Repos[0].Err = "x"
			} else {
				tc.mod(&in.Repos[0])
			}
			s := service(t, Evaluate(in), "s")
			if s.State != tc.want {
				t.Fatalf("state = %s, want %s (flags %v)", s.State, tc.want, s.Flags)
			}
			if tc.flags != nil {
				rs := s.Repos[0]
				if !slices.Equal(rs.Flags, tc.flags) || rs.Activity != tc.active {
					t.Fatalf("flags = %v activity = %v, want %v %v", rs.Flags, rs.Activity, tc.flags, tc.active)
				}
			}
		})
	}
}

func TestUnassignedRaisesOnlyKeyAndReadFindings(t *testing.T) {
	r := Evaluate(Input{Now: now, Scan: ScanState{}, Repos: []Repo{
		{Path: "u/old", SnapshotCount: 0, MissingSince: agoP(time.Hour)},
	}})
	if r.Worst != OK || r.Unassigned[0].State != OK {
		t.Fatalf("worst = %s, unassigned = %+v", r.Worst, r.Unassigned)
	}
	r = Evaluate(Input{Now: now, Repos: []Repo{{Path: "u/new", KeyCount: 2, AckedKeyCount: 1}}})
	if r.Worst != KeyAdded {
		t.Fatalf("worst = %s, want key_added", r.Worst)
	}
}

func TestCadence(t *testing.T) {
	daily := []time.Time{ago(72 * time.Hour), ago(48 * time.Hour), ago(24 * time.Hour)}
	if c := cadence(daily, 26*time.Hour); c == nil || c.MedianSeconds != 86400 || c.Mismatch {
		t.Fatalf("daily = %+v", c)
	}
	if c := cadence(daily[:2], 26*time.Hour); c != nil {
		t.Fatalf("two points = %+v, want nil", c)
	}
	if c := cadence(daily, 6*time.Hour); c == nil || !c.Mismatch {
		t.Fatalf("daily against 6h = %+v, want mismatch", c)
	}
}

func TestReasonsAreDeclarable(t *testing.T) {
	for _, r := range Reasons() {
		for _, c := range r {
			if !(c >= 'a' && c <= 'z' || c == '_' || c >= '0' && c <= '9') || len(r) > 64 {
				t.Fatalf("reason %q breaks the health grammar", r)
			}
		}
	}
	if len(Reasons()) != len(precedence)-2 { // ok and pending are not findings
		t.Fatalf("every non-ok state needs a reason")
	}
}

func snap(id string, age time.Duration) Snapshot { return Snapshot{ID: id, WrittenAt: ago(age)} }

func TestRestorePlan(t *testing.T) {
	hexA, hexB := strings.Repeat("a", 64), strings.Repeat("b", 64)
	base := PlanInput{
		Now: now, At: now, Service: "kynotes",
		Instances: []Instance{paired("i1", "kynotes")},
		Capsules: []Capsule{
			capsule("cap-old", "i1", 48*time.Hour, "active"),
			capsule("cap-new", "i1", 24*time.Hour, "active"),
		},
		Snapshots: map[string][]Snapshot{"u/kynotes": {snap(hexA, 46*time.Hour), snap(hexB, 24*time.Hour-10*time.Minute)}},
	}

	p, err := RestorePlan(base)
	if err != nil {
		t.Fatal(err)
	}
	if p.Capsule.ID != "cap-new" || p.Snapshot.ID != hexB || *p.GapSeconds != 600 {
		t.Fatalf("plan = %+v", p)
	}
	want := "kyquickstart restore 'kynotes' --capsule 'cap-new' --repo 'u/kynotes' --snapshot '" + hexB + "'"
	if p.Command != want {
		t.Fatalf("command = %q", p.Command)
	}
	if !slices.Contains(p.Warnings, WarnPairing) || slices.Contains(p.Warnings, WarnMismatch) {
		t.Fatalf("warnings = %v", p.Warnings)
	}

	t.Run("at selects older capsule, UTC", func(t *testing.T) {
		in := base
		in.At = ago(30 * time.Hour).In(time.FixedZone("x", 5*3600))
		p, _ := RestorePlan(in)
		if p.Capsule.ID != "cap-old" || p.Snapshot.ID != hexA || p.At.Location() != time.UTC {
			t.Fatalf("plan = %+v", p)
		}
		if !slices.Contains(p.Warnings, WarnMismatch) {
			t.Fatalf("1h gap must warn: %v", p.Warnings)
		}
	})
	t.Run("corrupt newest is reported and skipped", func(t *testing.T) {
		in := base
		in.Capsules = append([]Capsule{capsule("cap-bad", "i1", time.Hour, "corrupt")}, base.Capsules...)
		p, _ := RestorePlan(in)
		if p.Capsule.ID != "cap-new" || len(p.Skipped) != 1 || p.Skipped[0].ID != "cap-bad" {
			t.Fatalf("plan = %+v", p)
		}
	})
	t.Run("deposit ties break on ID", func(t *testing.T) {
		in := base
		in.Capsules = []Capsule{capsule("cap-b", "i1", time.Hour, "active"), capsule("cap-a", "i1", time.Hour, "active")}
		p, _ := RestorePlan(in)
		if p.Capsule.ID != "cap-b" {
			t.Fatalf("capsule = %s, want the higher ID as newest", p.Capsule.ID)
		}
	})
	t.Run("snapshot ties go earlier", func(t *testing.T) {
		in := base
		in.Snapshots = map[string][]Snapshot{"u/kynotes": {snap(hexB, 23*time.Hour), snap(hexA, 25*time.Hour)}}
		p, _ := RestorePlan(in)
		if p.Snapshot.ID != hexA {
			t.Fatalf("snapshot = %s, want the earlier one", p.Snapshot.ID)
		}
	})
	t.Run("before first backup", func(t *testing.T) {
		in := base
		in.At = ago(100 * time.Hour)
		p, err := RestorePlan(in)
		if err != nil || p.Capsule != nil || p.Command != "" {
			t.Fatalf("plan = %+v, err = %v", p, err)
		}
	})
	t.Run("future", func(t *testing.T) {
		in := base
		in.At = now.Add(time.Hour)
		if _, err := RestorePlan(in); !errors.Is(err, ErrFutureAt) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("ambiguous instance and repo", func(t *testing.T) {
		in := base
		in.Instances = append(in.Instances, paired("i2", "kynotes"))
		var amb *AmbiguousError
		if _, err := RestorePlan(in); !errors.As(err, &amb) || amb.Field != "instance" {
			t.Fatalf("err = %v", err)
		}
		in.Instance = "i1"
		in.Snapshots = map[string][]Snapshot{"u/a": nil, "u/b": nil}
		if _, err := RestorePlan(in); !errors.As(err, &amb) || amb.Field != "repo" {
			t.Fatalf("err = %v", err)
		}
		in.Repo = "u/zzz"
		if _, err := RestorePlan(in); !errors.As(err, &amb) {
			t.Fatalf("unknown repo err = %v", err)
		}
	})
	t.Run("restic-only takes newest snapshot as of at", func(t *testing.T) {
		in := base
		in.Instances, in.Capsules = nil, nil
		in.At = ago(30 * time.Hour)
		p, _ := RestorePlan(in)
		if p.Snapshot == nil || p.Snapshot.ID != hexA || p.GapSeconds != nil {
			t.Fatalf("plan = %+v", p)
		}
		if p.Command != "kyquickstart restore 'kynotes' --repo 'u/kynotes' --snapshot '"+hexA+"'" {
			t.Fatalf("command = %q", p.Command)
		}
	})
	t.Run("unsafe name gets no command", func(t *testing.T) {
		in := base
		in.Snapshots = map[string][]Snapshot{"u/it's": {snap(hexA, 24*time.Hour)}}
		p, _ := RestorePlan(in)
		if p.Command != "" || len(p.Warnings) == 0 {
			t.Fatalf("plan = %+v", p)
		}
	})
}
