package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/Busnes-app/ky-primitives/health"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/backupstatus"
	"github.com/Busnes-app/kyrecovery-server/internal/resticscan"
)

const (
	resticScanEvery    = 15 * time.Minute
	resticScanDeadline = 2 * time.Minute
	sampleWindow       = 90 * 24 * time.Hour
	maxInterval        = 400 * 24 * time.Hour
)

// backupReasons are the health reason codes, declared once.
var backupReasons = func() map[backupstatus.State]health.Reason {
	m := map[backupstatus.State]health.Reason{}
	for _, s := range []backupstatus.State{backupstatus.Corrupt, backupstatus.ScanError, backupstatus.Missing,
		backupstatus.Never, backupstatus.Stale, backupstatus.KeyAdded, backupstatus.OldLock} {
		m[s] = health.DeclareReason(backupstatus.Reason(s))
	}
	return m
}()

// backupHealth degrades with the worst stored finding. It reads the database only; the
// scanner is the only thing that touches the restic directory.
func (s *Server) backupHealth(ctx context.Context) error {
	in, err := s.backupInput(ctx)
	if err != nil {
		return err
	}
	if r, ok := backupReasons[backupstatus.Evaluate(in).Worst]; ok {
		return health.Degrade(r)
	}
	return nil
}

func (s *Server) backupInput(ctx context.Context) (backupstatus.Input, error) {
	in := backupstatus.Input{Now: time.Now().UTC()}
	apps, err := s.db.ListPairedApps(ctx)
	if err != nil {
		return in, err
	}
	for _, a := range apps {
		in.Instances = append(in.Instances, backupstatus.Instance{ID: a.ID, Service: a.ServiceName, AppName: a.AppName, Status: a.Status, LastDepositAt: a.LastBackupAt, PairedAt: a.PairedAt})
	}
	caps, err := s.db.ListCapsules(ctx)
	if err != nil {
		return in, err
	}
	for _, c := range caps {
		in.Capsules = append(in.Capsules, backupstatus.Capsule{ID: c.ID, Instance: c.PairedAppID, Digest: c.Digest, Status: c.Status,
			CreatedAt: c.CreatedAt, DepositedAt: c.DepositedAt, VerifiedAt: c.VerifiedAt})
	}
	if in.Repos, err = s.db.ListResticRepos(ctx); err != nil {
		return in, err
	}
	if in.Expected, err = s.db.BackupExpectations(ctx); err != nil {
		return in, err
	}
	if in.Scan, err = s.db.ResticScanState(ctx); err != nil {
		return in, err
	}
	in.Scan.Enabled = s.cfg.ResticDir != ""
	return in, nil
}

// runResticScanner scans on start and every resticScanEvery. One goroutine, so scans never overlap.
func (s *Server) runResticScanner(ctx context.Context) {
	if s.cfg.ResticDir == "" {
		return
	}
	t := time.NewTicker(resticScanEvery)
	defer t.Stop()
	for {
		s.scanRestic(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// scanRestic opens the root afresh each time, so a mount that appears later is picked up.
func (s *Server) scanRestic(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, resticScanDeadline)
	defer cancel()
	now := time.Now().UTC()
	repos, err := func() ([]resticscan.Repo, error) {
		root, err := os.OpenRoot(s.cfg.ResticDir)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		return resticscan.Scan(ctx, root, resticscan.DefaultLimits)
	}()
	if err == nil {
		err = s.db.ApplyResticScan(ctx, now, repos)
	}
	if err != nil {
		audit.Log().Error("restic_scan", "restic-scanner", "", "restic scan failed", err)
		if err := s.db.RecordResticScanFailure(context.WithoutCancel(ctx), now, err.Error()); err != nil {
			audit.Log().Error("restic_scan", "restic-scanner", "", "recording scan failure failed", err)
		}
	}
}

func (s *Server) handleBackupStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	in, err := s.backupInput(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading backup status")
		return
	}
	samples, err := s.db.ListResticSamples(ctx, in.Now.Add(-sampleWindow))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading backup status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"report":                   backupstatus.Evaluate(in),
		"scan":                     in.Scan,
		"samples":                  samples,
		"default_interval_seconds": int64(backupstatus.DefaultInterval / time.Second),
	})
}

func (s *Server) handleRestorePlan(w http.ResponseWriter, r *http.Request) {
	ctx, q := r.Context(), r.URL.Query()
	service := q.Get("service")
	if !validServiceName(service) {
		writeError(w, http.StatusBadRequest, "service is required")
		return
	}
	in := backupstatus.PlanInput{Now: time.Now().UTC(), Service: service, Instance: q.Get("instance"), Repo: q.Get("repo"),
		Snapshots: map[string][]backupstatus.Snapshot{}}
	in.At = in.Now
	if at := q.Get("at"); at != "" {
		t, err := time.Parse(time.RFC3339, at)
		if err != nil {
			writeError(w, http.StatusBadRequest, "at must be RFC 3339 with an offset")
			return
		}
		in.At = t
	}

	all, err := s.backupInput(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading backup status")
		return
	}
	holds := map[string]bool{}
	for _, c := range all.Capsules {
		holds[c.Instance] = true
	}
	ofService := map[string]bool{}
	for _, i := range all.Instances {
		ofService[i.ID] = i.Service == service
	}
	for _, c := range all.Capsules {
		if ofService[c.Instance] {
			in.Capsules = append(in.Capsules, c)
		}
	}
	// Revoked instances stay restorable: their capsules are still the data.
	for _, i := range all.Instances {
		if i.Service == service && (i.Status == "paired" || holds[i.ID]) {
			in.Instances = append(in.Instances, i)
		}
	}
	for _, repo := range all.Repos {
		if repo.Service != service {
			continue
		}
		if in.Snapshots[repo.Path], err = s.db.ListResticSnapshots(ctx, repo.Path); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed reading snapshots")
			return
		}
	}

	plan, err := backupstatus.RestorePlan(in)
	var amb *backupstatus.AmbiguousError
	switch {
	case errors.As(err, &amb):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": amb.Error(), "field": amb.Field, "choices": amb.Choices})
	case errors.Is(err, backupstatus.ErrFutureAt):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "Failed planning restore")
	default:
		writeJSON(w, http.StatusOK, plan)
	}
}

// auditFirst records an admin change before it is made. If the record cannot be written,
// the change is refused: an unaudited configuration change is not one this store makes.
func (s *Server) auditFirst(w http.ResponseWriter, r *http.Request, action, target string, details map[string]any) bool {
	if _, err := s.ledger.Record(r.Context(), action, s.actor(r), target, details); err != nil {
		writeError(w, http.StatusServiceUnavailable, "Audit ledger unavailable; change not made")
		return false
	}
	return true
}

func (s *Server) handleBackupExpectations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		ServiceName string `json:"service_name"`
		RepoPath    string `json:"repo_path"`
		Seconds     int64  `json:"expected_interval_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.ServiceName == "") == (req.RepoPath == "") {
		writeError(w, http.StatusBadRequest, "exactly one of service_name and repo_path is required")
		return
	}
	interval := time.Duration(req.Seconds) * time.Second
	if interval < backupstatus.MinInterval || interval > maxInterval {
		writeError(w, http.StatusBadRequest, "expected_interval_seconds must be between 900 and 34560000")
		return
	}
	ctx := r.Context()
	if req.ServiceName != "" && !validServiceName(req.ServiceName) {
		writeError(w, http.StatusBadRequest, "invalid service_name")
		return
	}
	if req.RepoPath != "" && !s.repoKnown(w, r, req.RepoPath) {
		return
	}
	if !s.auditFirst(w, r, "backup_expectation_set", req.ServiceName+req.RepoPath, map[string]any{
		"service_name": req.ServiceName, "repo_path": req.RepoPath, "expected_interval_seconds": req.Seconds}) {
		return
	}
	var err error
	if req.ServiceName != "" {
		err = s.db.SetServiceExpectation(ctx, req.ServiceName, interval)
	} else {
		_, err = s.db.SetRepoExpectation(ctx, req.RepoPath, interval)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed saving expectation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleResticLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		RepoPath    string `json:"repo_path"`
		ServiceName string `json:"service_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RepoPath == "" {
		writeError(w, http.StatusBadRequest, "repo_path is required")
		return
	}
	if req.ServiceName != "" && !validServiceName(req.ServiceName) {
		writeError(w, http.StatusBadRequest, "invalid service_name")
		return
	}
	if !s.repoKnown(w, r, req.RepoPath) {
		return
	}
	if !s.auditFirst(w, r, "restic_repo_linked", req.RepoPath, map[string]any{"service_name": req.ServiceName}) {
		return
	}
	if _, err := s.db.LinkResticRepo(r.Context(), req.RepoPath, req.ServiceName); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed linking repository")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleResticAckKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		RepoPath string `json:"repo_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RepoPath == "" {
		writeError(w, http.StatusBadRequest, "repo_path is required")
		return
	}
	if !s.repoKnown(w, r, req.RepoPath) {
		return
	}
	if !s.auditFirst(w, r, "restic_key_acknowledged", req.RepoPath, nil) {
		return
	}
	if _, err := s.db.AckResticKeys(r.Context(), req.RepoPath); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed acknowledging keys")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) repoKnown(w http.ResponseWriter, r *http.Request, repoPath string) bool {
	ok, err := s.db.ResticRepoExists(r.Context(), repoPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading repositories")
		return false
	}
	if !ok {
		writeError(w, http.StatusNotFound, "unknown repository")
	}
	return ok
}
