package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Busness-app/kyrecovery-server/internal/db"
)

const retentionSetting = "capsule_retention_policy"

type retentionPolicy struct {
	Days   int `json:"days"`
	Weeks  int `json:"weeks"`
	Months int `json:"months"`
}

func (p retentionPolicy) valid() bool {
	return p.Days >= 0 && p.Days <= 36500 && p.Weeks >= 0 && p.Weeks <= 5200 && p.Months >= 0 && p.Months <= 1200
}

type retentionRequest struct {
	Days   *int `json:"days"`
	Weeks  int  `json:"weeks"`
	Months int  `json:"months"`
}

func readRetentionRequest(r *http.Request) (retentionPolicy, error) {
	var req retentionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Days == nil {
		return retentionPolicy{}, fmt.Errorf("days is required")
	}
	p := retentionPolicy{Days: *req.Days, Weeks: req.Weeks, Months: req.Months}
	if !p.valid() {
		return p, fmt.Errorf("invalid retention policy")
	}
	return p, nil
}

type retentionPreview struct {
	retentionPolicy
	ExpiredCount int   `json:"expired_count"`
	ExpiredBytes int64 `json:"expired_bytes"`
}

func (s *Server) retention(ctx context.Context) (retentionPolicy, error) {
	var p retentionPolicy
	value, err := s.db.GetSetting(ctx, retentionSetting)
	if err != nil || value == "" {
		return p, err
	}
	if err := json.Unmarshal([]byte(value), &p); err != nil || !p.valid() {
		return p, fmt.Errorf("invalid retention setting")
	}
	return p, nil
}

// Retention windows overlap: keep all recent deposits PLUS the newest receipt in
// each UTC ISO week/month within the respective lookback. Horizons are measured
// from now, not added together. Ties use ID so preview and purge are deterministic.
func retentionCandidates(caps []db.CapsuleRecord, p retentionPolicy, now time.Time) []db.CapsuleRecord {
	if p.Days == 0 {
		return nil
	}
	now = now.UTC()
	dailyCutoff := now.Add(-time.Duration(p.Days) * 24 * time.Hour)
	weeklyCutoff := now.AddDate(0, 0, -7*p.Weeks)
	// Clamp month ends: one month before March 31 is February's last day.
	monthStart := time.Date(now.Year(), now.Month()-time.Month(p.Months), 1, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), time.UTC)
	lastDay := monthStart.AddDate(0, 1, -1).Day()
	monthlyCutoff := monthStart.AddDate(0, 0, min(now.Day(), lastDay)-1)
	keep := make(map[string]bool)
	type bucket struct{ app, service, period string }
	newest := make(map[bucket]db.CapsuleRecord)
	selectNewest := func(rec db.CapsuleRecord, period string) {
		key := bucket{rec.PairedAppID, rec.ServiceName, period}
		prev, ok := newest[key]
		if !ok || rec.DepositedAt.After(prev.DepositedAt) || (rec.DepositedAt.Equal(prev.DepositedAt) && rec.ID > prev.ID) {
			newest[key] = rec
		}
	}
	for _, rec := range caps {
		at := rec.DepositedAt.UTC()
		if at.IsZero() || at.After(dailyCutoff) {
			keep[rec.ID] = true
		}
		if p.Weeks > 0 && !at.Before(weeklyCutoff) {
			year, week := at.ISOWeek()
			selectNewest(rec, fmt.Sprintf("w%d-%d", year, week))
		}
		if p.Months > 0 && !at.Before(monthlyCutoff) {
			selectNewest(rec, "m"+at.Format("2006-01"))
		}
	}
	for _, rec := range newest {
		keep[rec.ID] = true
	}
	var expired []db.CapsuleRecord
	for _, rec := range caps {
		if !keep[rec.ID] {
			expired = append(expired, rec)
		}
	}
	return expired
}

// Wait for in-flight publishes before choosing weekly/monthly survivors. A row
// awaiting publication must not displace a completed backup if its publish fails.
func (s *Server) retentionCapsules(ctx context.Context) ([]db.CapsuleRecord, error) {
	caps, err := s.db.ListCapsules(ctx)
	if err != nil {
		return nil, err
	}
	stable := make([]db.CapsuleRecord, 0, len(caps))
	for _, candidate := range caps {
		release := s.idLocks.acquire(candidate.ID)
		rec, err := s.db.GetCapsule(ctx, candidate.ID)
		release()
		if err != nil {
			return nil, err
		}
		if rec != nil {
			stable = append(stable, *rec)
		}
	}
	return stable, nil
}

func (s *Server) handleRetention(w http.ResponseWriter, r *http.Request) {
	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()
	ctx := r.Context()
	switch r.Method {
	case http.MethodPost:
		policy, err := readRetentionRequest(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Use integer days (0–36500), weeks (0–5200), months (0–1200); days 0 keeps forever")
			return
		}
		if _, err := s.ledger.Record(ctx, "retention_change_requested", s.actor(r), retentionSetting, map[string]interface{}{"policy": policy}); err != nil {
			writeError(w, http.StatusServiceUnavailable, "Audit ledger unavailable; retention unchanged")
			return
		}
		value, _ := json.Marshal(policy)
		if err := s.db.SetSetting(ctx, retentionSetting, string(value)); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed saving retention")
			return
		}
	case http.MethodGet:
	default:
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	policy, err := s.retention(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading retention")
		return
	}
	caps, err := s.retentionCapsules(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed listing capsules")
		return
	}
	preview := retentionPreview{retentionPolicy: policy}
	now := time.Now().UTC()
	for _, rec := range retentionCandidates(caps, policy, now) {
		preview.ExpiredCount++
		preview.ExpiredBytes += rec.SizeBytes
	}
	writeJSON(w, http.StatusOK, preview)
}

// Callers hold retentionMu across policy changes and purges; each candidate is re-read under
// the same ID lock used by deposits and verification. Failed deletes retain their row
// so the next purge can retry, including after a crash between unlink and row deletion.
func (s *Server) purgeExpired(ctx context.Context, policy retentionPolicy, now time.Time, actor string) (int, error) {
	caps, err := s.retentionCapsules(ctx)
	if err != nil {
		return 0, err
	}
	purged := 0
	// ponytail: scan the existing capsule list; use a deposited_at index and batches if the catalog outgrows memory.
	for _, candidate := range retentionCandidates(caps, policy, now) {
		deleted, err := func() (bool, error) {
			defer s.idLocks.acquire(candidate.ID)()
			rec, err := s.db.GetCapsule(ctx, candidate.ID)
			if err != nil {
				return false, err
			}
			if rec == nil || !rec.DepositedAt.Equal(candidate.DepositedAt) {
				return false, nil
			}
			details := map[string]interface{}{"digest": rec.Digest, "size_bytes": rec.SizeBytes, "deposited_at": rec.DepositedAt, "retention_policy": policy}
			if _, err := s.ledger.Record(ctx, "capsule_purge_requested", actor, rec.ID, details); err != nil {
				return false, err
			}
			if err := os.Remove(rec.FilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return false, err
			}
			// Finish bookkeeping even if the requesting browser disconnects after unlink.
			finishCtx := context.WithoutCancel(ctx)
			if err := s.db.DeleteCapsule(finishCtx, rec.ID); err != nil {
				return false, err
			}
			_, err = s.ledger.Record(finishCtx, "capsule_purged", actor, rec.ID, details)
			return true, err
		}()
		if deleted {
			purged++
		}
		if err != nil {
			return purged, err
		}
	}
	return purged, nil
}

func (s *Server) handleRetentionPurge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	// Require the policy the admin previewed, so another admin's edit cannot widen a purge.
	requested, err := readRetentionRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Preview retention and supply days, weeks and months")
		return
	}
	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()
	policy, err := s.retention(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed reading retention")
		return
	}
	if policy != requested {
		writeError(w, http.StatusConflict, "Retention changed; refresh the preview")
		return
	}
	if err := s.ledger.Healthy(); err != nil {
		writeError(w, http.StatusServiceUnavailable, "Audit ledger unavailable; purge refused")
		return
	}
	count, err := s.purgeExpired(r.Context(), policy, time.Now().UTC(), s.actor(r))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"purged_count": count, "error": "Purge stopped; some expired capsules may remain. Retry after checking storage and audit health."})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"purged_count": count})
}
