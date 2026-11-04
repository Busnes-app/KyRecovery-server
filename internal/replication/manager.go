package replication

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Busnes-app/ky-primitives/offsite"
	"github.com/Busnes-app/kyrecovery-server/internal/audit"
	"github.com/Busnes-app/kyrecovery-server/internal/db"
)

// Manager coordinates replication of encrypted capsules to offsite storage targets.
type Manager struct {
	db     *db.DB
	ledger *audit.Ledger
}

// NewManager creates a new replication manager.
func NewManager(database *db.DB, ledger *audit.Ledger) *Manager {
	return &Manager{
		db:     database,
		ledger: ledger,
	}
}

// SyncCapsule replicates a single capsule to a specific target.
func (m *Manager) SyncCapsule(ctx context.Context, capsuleID, targetID string) (*db.ReplicationLogRecord, error) {
	startedAt := time.Now().UTC()

	target, err := m.db.GetReplicationTarget(ctx, targetID)
	if err != nil || target == nil {
		return nil, fmt.Errorf("replication target not found: %s", targetID)
	}

	capRec, err := m.db.GetCapsule(ctx, capsuleID)
	if err != nil || capRec == nil {
		return nil, fmt.Errorf("capsule not found: %s", capsuleID)
	}

	sizeBytes, transferErr := m.sendVerified(ctx, *target, capRec)

	completedAt := time.Now().UTC()
	durationMs := completedAt.Sub(startedAt).Milliseconds()

	status := "success"
	errMsg := ""
	if transferErr != nil {
		status = "failed"
		errMsg = transferErr.Error()
	}

	logRec := db.ReplicationLogRecord{
		TargetID:         target.ID,
		CapsuleID:        capRec.ID,
		BytesTransferred: sizeBytes,
		DurationMs:       durationMs,
		Status:           status,
		ErrorMessage:     errMsg,
		CreatedAt:        completedAt,
	}

	// A caller that hung up cancelled the transfer, not the record of it.
	done := context.WithoutCancel(ctx)
	_ = m.db.InsertReplicationLog(done, logRec)
	_ = m.db.UpdateReplicationTargetLastSync(done, target.ID, status)

	if m.ledger != nil {
		_, _ = m.ledger.Record(done, "capsule_replicated", "replication-daemon", capRec.ID, map[string]interface{}{
			"target_id":   target.ID,
			"target_name": target.Name,
			"status":      status,
			"bytes":       sizeBytes,
			"duration_ms": durationMs,
		})
	}

	if transferErr != nil {
		return &logRec, transferErr
	}
	return &logRec, nil
}

// sendVerified transfers the stored bytes only when they are still the bytes that were
// deposited. A replica may be the last good copy, so a corrupt or changed local file must
// never be written over it. Every failure here is a failed sync, logged like one.
func (m *Manager) sendVerified(ctx context.Context, target db.ReplicationTargetRecord, capRec *db.CapsuleRecord) (int64, error) {
	if capRec.Status == "corrupt" {
		return 0, fmt.Errorf("capsule %s is marked corrupt; not replicating it", capRec.ID)
	}
	capFile, err := os.Open(capRec.FilePath)
	if err != nil {
		return 0, fmt.Errorf("failed opening capsule file: %w", err)
	}
	defer capFile.Close()
	h := sha256.New()
	size, err := io.Copy(h, capFile)
	if err != nil {
		return 0, fmt.Errorf("failed reading capsule file: %w", err)
	}
	if size != capRec.SizeBytes || hex.EncodeToString(h.Sum(nil)) != capRec.Digest {
		return size, fmt.Errorf("capsule %s no longer matches its recorded digest; not replicating it", capRec.ID)
	}
	if _, err := capFile.Seek(0, io.SeekStart); err != nil {
		return size, fmt.Errorf("failed rewinding capsule file: %w", err)
	}
	return size, m.transfer(ctx, target, capRec, capFile, size)
}

// SyncAllAutoTargets replicates a capsule across all targets configured for auto-sync.
func (m *Manager) SyncAllAutoTargets(ctx context.Context, capsuleID string) []db.ReplicationLogRecord {
	targets, err := m.db.ListReplicationTargets(ctx)
	if err != nil {
		return nil
	}

	var logs []db.ReplicationLogRecord
	for _, t := range targets {
		if t.AutoSync && t.Status != "disabled" {
			logRec, _ := m.SyncCapsule(ctx, capsuleID, t.ID)
			if logRec != nil {
				logs = append(logs, *logRec)
			}
		}
	}
	return logs
}

// TestTarget validates connectivity and credentials for a target.
func (m *Manager) TestTarget(ctx context.Context, target db.ReplicationTargetRecord) error {
	if err := checkTargetCrypto(target); err != nil {
		return err
	}
	l, err := targetLocation(target, "connectivity-test")
	if err != nil {
		return err
	}
	if l.absoluteSFTP {
		return NewSFTPClient(target.Endpoint, target.AccessKey, target.SecretKey, target.Prefix, target.HostKey).TestConnection(ctx)
	}
	if l.virtualS3 {
		return NewS3Client(target.Endpoint, target.Bucket, target.Region, target.AccessKey, target.SecretKey).TestConnection(ctx)
	}
	t, err := offsite.Parse(l.config)
	if err != nil {
		return err
	}
	return t.Test(ctx)
}
