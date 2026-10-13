package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

type InstallerDirectory struct {
	Issuer, Subject, Event, Version, Digest string
	Active, Admin                           bool
}
type directoryRevision struct {
	Revision uint64
	Digest   string
}

func (d *DB) InspectInstallerDirectory(ctx context.Context, event InstallerDirectory) (bool, error) {
	revision, err := strconv.ParseUint(event.Version, 10, 64)
	if err != nil || revision == 0 || strconv.FormatUint(revision, 10) != event.Version {
		return false, errors.New("invalid directory revision")
	}
	b, managed, err := d.InstallerConfiguration(ctx)
	if err != nil || !managed || b.SSO == nil || b.SSO.IssuerURL != event.Issuer {
		return false, errors.New("directory issuer not configured")
	}
	id := InstallerIdentityID(event.Issuer, event.Subject)
	g, assigned, err := d.InstallerGrant(ctx, id)
	if err != nil || !assigned || g.SetupID != b.SetupID {
		return false, errors.New("directory subject not assigned")
	}
	switch event.Event {
	case "user.created", "user.updated", "user.deleted", "user.deactivated":
	default:
		return false, errors.New("unsupported directory event")
	}
	raw, err := d.GetSetting(ctx, "installer.directory."+id)
	if err != nil {
		return false, err
	}
	if raw == "" {
		return true, nil
	}
	var prior directoryRevision
	if json.Unmarshal([]byte(raw), &prior) != nil {
		return false, errors.New("invalid retained directory revision")
	}
	if prior.Revision > revision || prior.Revision == revision && prior.Digest != event.Digest {
		return false, errors.New("stale or conflicting directory revision")
	}
	return prior.Revision != revision, nil
}

func (d *DB) ApplyInstallerDirectoryAudit(ctx context.Context, event InstallerDirectory, ar AuditRecord, count uint64, hash string) error {
	changed, err := d.InspectInstallerDirectory(ctx, event)
	if err != nil {
		return err
	}
	if !changed {
		return errors.New("directory state already applied")
	}
	id := InstallerIdentityID(event.Issuer, event.Subject)
	g, _, err := d.InstallerGrant(ctx, id)
	if err != nil {
		return err
	}
	g.Active = (event.Event == "user.created" || event.Event == "user.updated") && event.Active && event.Admin
	revision, _ := strconv.ParseUint(event.Version, 10, 64)
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw string
	err = tx.QueryRowContext(ctx, "SELECT value FROM system_settings WHERE key=?", "installer.directory."+id).Scan(&raw)
	if err == nil {
		var prior directoryRevision
		if json.Unmarshal([]byte(raw), &prior) != nil || prior.Revision >= revision {
			return errors.New("directory revision changed")
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	value, _ := json.Marshal(g)
	result, err := tx.ExecContext(ctx, "UPDATE system_settings SET value=?,updated_at=? WHERE key=?", string(value), time.Now().UTC(), "installer.identity."+id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("directory identity changed")
	}
	if !g.Active {
		if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=?", id); err != nil {
			return err
		}
	}
	value, _ = json.Marshal(directoryRevision{revision, event.Digest})
	if _, err = tx.ExecContext(ctx, `INSERT INTO system_settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, "installer.directory."+id, string(value), time.Now().UTC()); err != nil {
		return err
	}
	if err = insertAuditAndAnchor(ctx, tx, ar, count, hash); err != nil {
		return err
	}
	return tx.Commit()
}
