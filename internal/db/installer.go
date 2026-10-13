package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Busnes-app/ky-primitives/applysetup"
)

const InstallerConfigKey = "installer.config"

var ErrInstallerMissing = errors.New("installer configuration absent")

type InstallerGrant struct {
	SetupID, Issuer, Subject, Username string
	Active                             bool
}

func InstallerIdentityID(issuer, subject string) string {
	sum := sha256.Sum256([]byte(issuer + "\x00" + subject))
	return "installer-" + hex.EncodeToString(sum[:])
}

func (d *DB) InstallerConfiguration(ctx context.Context) (applysetup.Common, bool, error) {
	raw, err := d.GetSetting(ctx, InstallerConfigKey)
	if err != nil {
		return applysetup.Common{}, true, err
	}
	if raw == "" {
		return applysetup.Common{}, false, nil
	}
	var b applysetup.Common
	if err = applysetup.DecodeRaw([]byte(raw), &b); err != nil {
		return b, true, err
	}
	if err = b.Validate("kyrecovery", "/api/auth/callback"); err != nil {
		return b, true, err
	}
	return b, true, nil
}

func (d *DB) InstallerGrant(ctx context.Context, id string) (InstallerGrant, bool, error) {
	raw, err := d.GetSetting(ctx, "installer.identity."+id)
	if err != nil {
		return InstallerGrant{}, true, err
	}
	if raw == "" {
		return InstallerGrant{}, false, nil
	}
	var g InstallerGrant
	if json.Unmarshal([]byte(raw), &g) != nil || g.SetupID == "" || g.Issuer == "" || g.Subject == "" || id != InstallerIdentityID(g.Issuer, g.Subject) {
		return g, true, errors.New("invalid installer identity grant")
	}
	return g, true, nil
}

func ValidateInstaller(b applysetup.Common) error {
	if err := b.Validate("kyrecovery", "/api/auth/callback"); err != nil {
		return err
	}
	if b.SSO == nil || len(b.Admins) == 0 || len(b.Everyday) != 0 || b.Backup != nil || b.SSO.SCIMTokenFile != "" || b.SSO.DirectorySecretFile == "" || strings.TrimRight(b.SSO.IssuerURL, "/") != b.SSO.IssuerURL {
		return errors.New("KyRecovery requires exact administrators and signed directory; ordinary users and backup enrollment unsupported")
	}
	for _, id := range b.Admins {
		if id.Issuer != b.SSO.IssuerURL {
			return errors.New("administrator issuer differs")
		}
	}
	return nil
}

func (d *DB) InspectInstaller(ctx context.Context, b applysetup.Common) (applysetup.Report, error) {
	r := applysetup.NewReport(b)
	if err := ValidateInstaller(b); err != nil {
		return r, err
	}
	retained, managed, err := d.InstallerConfiguration(ctx)
	if err != nil {
		return r, err
	}
	if !managed {
		return r, ErrInstallerMissing
	}
	desired, _ := json.Marshal(b)
	actual, _ := json.Marshal(retained)
	if string(desired) != string(actual) {
		return r, errors.New("installer configuration ownership or values conflict")
	}
	r.Present = append(r.Present, applysetup.Object{Kind: "configuration", Name: "sso"})
	for _, id := range b.Admins {
		key := InstallerIdentityID(id.Issuer, id.Subject)
		g, found, err := d.InstallerGrant(ctx, key)
		if err != nil {
			return r, err
		}
		if !found {
			return r, ErrInstallerMissing
		}
		if g.SetupID != b.SetupID || g.Issuer != id.Issuer || g.Subject != id.Subject || g.Username != id.Username || !g.Active {
			return r, errors.New("installer administrator drift or revocation")
		}
		r.Present = append(r.Present, applysetup.Object{Kind: "identity", Name: id.Username, ID: key})
	}
	return r, nil
}

// ApplyInstallerAudit commits authority, the keyed event and its anchor together.
// The running authentication manager reads the owned configuration at each boundary.
func (d *DB) ApplyInstallerAudit(ctx context.Context, b applysetup.Common, ar AuditRecord, count uint64, hash string) error {
	if err := ValidateInstaller(b); err != nil {
		return err
	}
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM system_settings WHERE key=? OR key LIKE 'installer.identity.%' OR (key LIKE 'sso.%' AND value NOT IN ('','false','0'))", InstallerConfigKey).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return errors.New("existing SSO or installer ownership conflicts")
	}
	put := func(key, value string) error {
		_, e := tx.ExecContext(ctx, "INSERT INTO system_settings(key,value,updated_at) VALUES(?,?,?)", key, value, time.Now().UTC())
		return e
	}
	raw, _ := json.Marshal(b)
	if err = put(InstallerConfigKey, string(raw)); err != nil {
		return err
	}
	for _, id := range b.Admins {
		key := InstallerIdentityID(id.Issuer, id.Subject)
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE id=?", key).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			return errors.New("existing local identity collision")
		}
		value, _ := json.Marshal(InstallerGrant{b.SetupID, id.Issuer, id.Subject, id.Username, true})
		if err = put("installer.identity."+key, string(value)); err != nil {
			return err
		}
	}
	// Sessions from before activation carry no issuer binding, so the managed grant
	// checks would skip them. End them all; local credentials sign in again.
	if _, err = tx.ExecContext(ctx, "DELETE FROM sessions"); err != nil {
		return err
	}
	if err = insertAuditAndAnchor(ctx, tx, ar, count, hash); err != nil {
		return err
	}
	return tx.Commit()
}

func insertAuditAndAnchor(ctx context.Context, tx *sql.Tx, ar AuditRecord, count uint64, hash string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events (`+auditCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, ar.Seq, ar.PrevHash, ar.EventHash, ar.Action, ar.Actor, ar.TargetID, ar.DetailsJSON, ar.CreatedAt); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_anchor (singleton,count,hash) VALUES (1,?,?) ON CONFLICT(singleton) DO UPDATE SET count=excluded.count,hash=excluded.hash`, count, hash)
	return err
}
