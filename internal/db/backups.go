package db

import (
	"context"
	"database/sql"
	"path"
	"time"

	"github.com/Busnes-app/kyrecovery-server/internal/backupstatus"
	"github.com/Busnes-app/kyrecovery-server/internal/resticscan"
)

// sampleRetention is how long daily restic samples are kept.
const sampleRetention = 400 * 24 * time.Hour

// RecordResticScanFailure notes a failed scan. Repository rows are left exactly as they were.
func (d *DB) RecordResticScanFailure(ctx context.Context, at time.Time, scanErr string) error {
	_, err := d.conn.ExecContext(ctx, `INSERT INTO restic_scan_state (singleton, last_attempt_at, last_error) VALUES (1, ?, ?)
		ON CONFLICT(singleton) DO UPDATE SET last_attempt_at = excluded.last_attempt_at, last_error = excluded.last_error`,
		at.UTC(), scanErr)
	return err
}

// ApplyResticScan stores a successful scan in one transaction. Repositories the scan did not
// find are marked missing; a repository it could not read keeps its earlier measurements.
func (d *DB) ApplyResticScan(ctx context.Context, at time.Time, repos []resticscan.Repo) error {
	at = at.UTC()
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	paired := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT service_name FROM paired_apps WHERE status = 'paired'`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		paired[n] = true
	}
	rows.Close()

	seen := map[string]bool{}
	for _, r := range repos {
		seen[r.Path] = true
		link := ""
		if paired[path.Base(r.Path)] {
			link = path.Base(r.Path)
		}
		// First sight sets the auto-link and the first good read sets the key baseline;
		// later scans change neither.
		if _, err := tx.ExecContext(ctx, `INSERT INTO restic_repos (path, service_name) VALUES (?, ?)
			ON CONFLICT(path) DO NOTHING`, r.Path, link); err != nil {
			return err
		}
		if r.Err != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE restic_repos SET scan_error = ?, missing_since = NULL WHERE path = ?`, r.Err, r.Path); err != nil {
				return err
			}
			continue
		}
		var last *time.Time
		if n := len(r.Snapshots); n > 0 {
			last = &r.Snapshots[n-1].WrittenAt
		}
		if _, err := tx.ExecContext(ctx, `UPDATE restic_repos SET snapshot_count = ?, last_snapshot_at = ?, size_bytes = ?,
			lock_seen_at = ?, key_count = ?, missing_since = NULL, scan_error = '', measured_at = ?,
			acked_key_count = CASE WHEN measured_at IS NULL THEN ? ELSE MIN(acked_key_count, ?) END WHERE path = ?`,
			len(r.Snapshots), last, r.SizeBytes, r.LockSeenAt, r.KeyCount, at, r.KeyCount, r.KeyCount, r.Path); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM restic_snapshots WHERE path = ?`, r.Path); err != nil {
			return err
		}
		for _, s := range r.Snapshots {
			if _, err := tx.ExecContext(ctx, `INSERT INTO restic_snapshots (path, id, written_at) VALUES (?, ?, ?)`, r.Path, s.ID, s.WrittenAt); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO restic_samples (path, day, size_bytes, snapshot_count) VALUES (?, ?, ?, ?)
			ON CONFLICT(path, day) DO UPDATE SET size_bytes = excluded.size_bytes, snapshot_count = excluded.snapshot_count`,
			r.Path, at.Format(time.DateOnly), r.SizeBytes, len(r.Snapshots)); err != nil {
			return err
		}
	}

	known, err := tx.QueryContext(ctx, `SELECT path FROM restic_repos WHERE missing_since IS NULL`)
	if err != nil {
		return err
	}
	var gone []string
	for known.Next() {
		var p string
		if err := known.Scan(&p); err != nil {
			known.Close()
			return err
		}
		if !seen[p] {
			gone = append(gone, p)
		}
	}
	known.Close()
	for _, p := range gone {
		if _, err := tx.ExecContext(ctx, `UPDATE restic_repos SET missing_since = ? WHERE path = ?`, at, p); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM restic_samples WHERE day < ?`, at.Add(-sampleRetention).Format(time.DateOnly)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO restic_scan_state (singleton, last_attempt_at, last_success_at, last_error) VALUES (1, ?, ?, '')
		ON CONFLICT(singleton) DO UPDATE SET last_attempt_at = excluded.last_attempt_at, last_success_at = excluded.last_success_at, last_error = ''`,
		at, at); err != nil {
		return err
	}
	return tx.Commit()
}

// ResticScanState returns the scanner's last attempt. Enabled is the caller's to set.
func (d *DB) ResticScanState(ctx context.Context) (backupstatus.ScanState, error) {
	var st backupstatus.ScanState
	err := d.conn.QueryRowContext(ctx, `SELECT last_attempt_at, last_success_at, last_error FROM restic_scan_state WHERE singleton = 1`).
		Scan(&st.LastAttemptAt, &st.LastSuccessAt, &st.LastError)
	if err == sql.ErrNoRows {
		return st, nil
	}
	return st, err
}

// ListResticRepos returns every known repository with its newest ten snapshot times.
func (d *DB) ListResticRepos(ctx context.Context) ([]backupstatus.Repo, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT path, service_name, expected_interval_seconds, snapshot_count, last_snapshot_at,
		size_bytes, lock_seen_at, key_count, acked_key_count, missing_since, scan_error, measured_at FROM restic_repos ORDER BY path`)
	if err != nil {
		return nil, err
	}
	var out []backupstatus.Repo
	for rows.Next() {
		var r backupstatus.Repo
		var secs int64
		if err := rows.Scan(&r.Path, &r.Service, &secs, &r.SnapshotCount, &r.LastSnapshotAt, &r.SizeBytes, &r.LockSeenAt,
			&r.KeyCount, &r.AckedKeyCount, &r.MissingSince, &r.Err, &r.MeasuredAt); err != nil {
			rows.Close()
			return nil, err
		}
		r.Expected = time.Duration(secs) * time.Second
		out = append(out, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		recent, err := d.conn.QueryContext(ctx, `SELECT written_at FROM (SELECT written_at FROM restic_snapshots WHERE path = ?
			ORDER BY written_at DESC LIMIT 10) ORDER BY written_at`, out[i].Path)
		if err != nil {
			return nil, err
		}
		for recent.Next() {
			var t time.Time
			if err := recent.Scan(&t); err != nil {
				recent.Close()
				return nil, err
			}
			out[i].RecentSnapshots = append(out[i].RecentSnapshots, t)
		}
		recent.Close()
	}
	return out, nil
}

// ListResticSnapshots returns one repository's snapshots, oldest first.
func (d *DB) ListResticSnapshots(ctx context.Context, repoPath string) ([]backupstatus.Snapshot, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT id, written_at FROM restic_snapshots WHERE path = ? ORDER BY written_at, id`, repoPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []backupstatus.Snapshot{}
	for rows.Next() {
		var s backupstatus.Snapshot
		if err := rows.Scan(&s.ID, &s.WrittenAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ResticSample is one day's size and snapshot count for a repository.
type ResticSample struct {
	Path          string `json:"path"`
	Day           string `json:"day"`
	SizeBytes     int64  `json:"size_bytes"`
	SnapshotCount int    `json:"snapshot_count"`
}

// ListResticSamples returns every sample on or after since, by path then day.
func (d *DB) ListResticSamples(ctx context.Context, since time.Time) ([]ResticSample, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT path, day, size_bytes, snapshot_count FROM restic_samples WHERE day >= ? ORDER BY path, day`,
		since.UTC().Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ResticSample{}
	for rows.Next() {
		var s ResticSample
		if err := rows.Scan(&s.Path, &s.Day, &s.SizeBytes, &s.SnapshotCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// BackupExpectations returns the admin-set interval per service.
func (d *DB) BackupExpectations(ctx context.Context) (map[string]time.Duration, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT service_name, expected_interval_seconds FROM backup_expectations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Duration{}
	for rows.Next() {
		var n string
		var secs int64
		if err := rows.Scan(&n, &secs); err != nil {
			return nil, err
		}
		out[n] = time.Duration(secs) * time.Second
	}
	return out, rows.Err()
}

// SetServiceExpectation sets a service's expected backup interval.
func (d *DB) SetServiceExpectation(ctx context.Context, service string, interval time.Duration) error {
	_, err := d.conn.ExecContext(ctx, `INSERT INTO backup_expectations (service_name, expected_interval_seconds) VALUES (?, ?)
		ON CONFLICT(service_name) DO UPDATE SET expected_interval_seconds = excluded.expected_interval_seconds`,
		service, int64(interval/time.Second))
	return err
}

// SetRepoExpectation sets one repository's interval. It reports false for an unknown path.
func (d *DB) SetRepoExpectation(ctx context.Context, repoPath string, interval time.Duration) (bool, error) {
	return d.updateRepo(ctx, `UPDATE restic_repos SET expected_interval_seconds = ? WHERE path = ?`, int64(interval/time.Second), repoPath)
}

// LinkResticRepo assigns a repository to a service; "" unlinks it.
func (d *DB) LinkResticRepo(ctx context.Context, repoPath, service string) (bool, error) {
	return d.updateRepo(ctx, `UPDATE restic_repos SET service_name = ? WHERE path = ?`, service, repoPath)
}

// AckResticKeys accepts the repository's current key count as its baseline.
func (d *DB) AckResticKeys(ctx context.Context, repoPath string) (bool, error) {
	return d.updateRepo(ctx, `UPDATE restic_repos SET acked_key_count = key_count WHERE path = ?`, repoPath)
}

// ResticRepoExists reports whether the scanner has ever seen repoPath.
func (d *DB) ResticRepoExists(ctx context.Context, repoPath string) (bool, error) {
	var n int
	err := d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM restic_repos WHERE path = ?`, repoPath).Scan(&n)
	return n == 1, err
}

func (d *DB) updateRepo(ctx context.Context, q string, args ...any) (bool, error) {
	res, err := d.conn.ExecContext(ctx, q, args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
