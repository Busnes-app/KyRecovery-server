package db

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"iter"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Busnes-app/kyrecovery-server/internal/secrets"
)

// CapsuleRecord stores metadata for a stored capsule. Every field but Digest, FilePath,
// DepositedAt, PairedAppID and Status is copied from the manifest as the sealer attested
// it: the store cannot open the container, so it records rather than verifies.
type CapsuleRecord struct {
	ID              string    `json:"id"`
	ServiceName     string    `json:"service_name"`
	AppName         string    `json:"app_name"`
	AppVersion      string    `json:"app_version"`
	FilePath        string    `json:"-"`
	SizeBytes       int64     `json:"size_bytes"`
	Digest          string    `json:"digest"`       // SHA-256 hex of the container as deposited
	PayloadHash     string    `json:"payload_hash"` // from the manifest, sealer-attested
	Threshold       int       `json:"threshold"`
	TotalShares     int       `json:"total_shares"`
	RecoveryKeyID   string    `json:"recovery_key_id"`
	EncapsulatedKey string    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`   // from the manifest
	DepositedAt     time.Time `json:"deposited_at"` // kyrecovery's clock
	PairedAppID     string    `json:"paired_app_id"`
	Status          string    `json:"status"` // "active" | "corrupt"
	// VerifiedAt is the last time the stored bytes matched Digest; nil means stored, never checked.
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

// CustodianRecord represents a designated recovery custodian.
type CustodianRecord struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuditRecord represents one link of the tamper-evident audit chain. CreatedAt is
// the RFC3339Nano string that was hashed into the record, stored verbatim: the
// hashed bytes and the stored bytes are the same bytes.
type AuditRecord struct {
	Seq         uint64 `json:"sequence_num"`
	PrevHash    string `json:"prev_hash"`
	EventHash   string `json:"event_hash"`
	Action      string `json:"action"`
	Actor       string `json:"actor"`
	TargetID    string `json:"target_id"`
	DetailsJSON string `json:"details_json"`
	CreatedAt   string `json:"created_at"`
}

// PairedAppRecord represents a connected Ky Suite app or busnes.app service.
// APIToken and PairingCode are credentials, so they never serialise by default:
// the pairing code is returned only by the admin route that generates it, and the
// token only by the claim that mints it. The store keeps only their hashes, so a
// record read back from SQLite carries neither: both are set only on the record
// returned by the call that created them.
type PairedAppRecord struct {
	ID           string     `json:"id"`
	ServiceName  string     `json:"service_name"`
	AppName      string     `json:"app_name"`
	APIToken     string     `json:"-"`
	PairingCode  string     `json:"-"`
	Status       string     `json:"status"` // "pending", "paired", "revoked"
	ExpiresAt    time.Time  `json:"expires_at"`
	PairedAt     *time.Time `json:"paired_at,omitempty"`
	LastBackupAt *time.Time `json:"last_backup_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	// RecoveryKeyID is the key this product was handed at pairing.
	RecoveryKeyID string `json:"recovery_key_id,omitempty"`
}

// RecoveryKeyRecord is a suite recovery public key this store has held. Exactly one is
// "current": the key handed to products at pairing and the only one deposits may be sealed
// to. A rotated-out key is "previous" while capsules sealed to it are retained, and
// "retired" once the operator has said its cards may be destroyed. The private half never
// existed here.
type RecoveryKeyRecord struct {
	KeyID       string     `json:"key_id"`
	PublicKey   []byte     `json:"-"`
	Threshold   int        `json:"threshold"`
	TotalShares int        `json:"total_shares"`
	Status      string     `json:"status"` // "current" | "previous" | "retired"
	ImportedBy  string     `json:"imported_by"`
	ImportedAt  time.Time  `json:"imported_at"`
	RetiredBy   string     `json:"retired_by,omitempty"`
	RetiredAt   *time.Time `json:"retired_at,omitempty"`
}

var ErrRecoveryKeyExists = errors.New("a recovery key is already imported")

// ErrRecoveryKeyKnown means a rotation named a key this store already holds, in any state.
// A key is never brought back: its cards may already be destroyed.
var ErrRecoveryKeyKnown = errors.New("this recovery key is already known to the store")

// ErrStaleRotation means the rotation named a key to replace that is no longer current.
var ErrStaleRotation = errors.New("the key to replace is not the current recovery key")

// ErrRetireRefused means the key is current, already retired, unknown, or still has capsules.
var ErrRetireRefused = errors.New("recovery key cannot be retired")

// ErrCapsuleExists reports that a capsule row with this ID is already stored. The row is the
// mutual-exclusion primitive for a deposit: whoever inserts it owns the file path.
var ErrCapsuleExists = errors.New("a capsule with this ID is already stored")

// ErrSessionRefused means the session lost its authority before it could be stored.
var ErrSessionRefused = errors.New("session authority unavailable")

// ErrSSOSessionEnded means the identity provider already ended the login being signed in.
var ErrSSOSessionEnded = errors.New("SSO session already ended")

// UserRecord represents a local user account.
type UserRecord struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// SessionRecord represents an authenticated user session. ID is the bearer
// credential the session cookie carries, so it is never serialised into a response.
// SQLite holds only its SHA-256 (see tokenHash); every method here takes the raw ID.
type SessionRecord struct {
	// NativeSSO marks a built-in OIDC login; it is refused once managed setup exists.
	NativeSSO       bool      `json:"-"`
	IdentityIssuer  string    `json:"-"`
	IdentitySubject string    `json:"-"`
	ID              string    `json:"-"`
	UserID          string    `json:"user_id"`
	Email           string    `json:"email"`
	Name            string    `json:"name"`
	Role            string    `json:"role"`
	ExpiresAt       time.Time `json:"expires_at"`
	CreatedAt       time.Time `json:"created_at"`
	// SSOIssuer, SSOSubject and SSOSID name the provider login, for back-channel logout.
	SSOIssuer  string `json:"-"`
	SSOSubject string `json:"-"`
	SSOSID     string `json:"-"`
}

// ReplicationTargetRecord defines an offsite backup target (S3, Cloudflare R2, MinIO, Local).
type ReplicationTargetRecord struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"` // "s3", "local"
	Endpoint   string     `json:"endpoint"`
	Bucket     string     `json:"bucket"`
	Region     string     `json:"region"`
	AccessKey  string     `json:"access_key"`
	SecretKey  string     `json:"secret_key,omitempty"`
	Prefix     string     `json:"prefix"`
	HostKey    string     `json:"host_key"` // sftp: pinned SHA256 fingerprint
	AutoSync   bool       `json:"auto_sync"`
	Status     string     `json:"status"` // "active", "disabled", "error"
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ReplicationLogRecord tracks an individual capsule transfer to a remote target.
type ReplicationLogRecord struct {
	ID               int64     `json:"id"`
	TargetID         string    `json:"target_id"`
	CapsuleID        string    `json:"capsule_id"`
	BytesTransferred int64     `json:"bytes_transferred"`
	DurationMs       int64     `json:"duration_ms"`
	Status           string    `json:"status"` // "success", "failed"
	ErrorMessage     string    `json:"error_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// DB wraps the SQLite database handle.
type DB struct {
	conn *sql.DB
	keys *secrets.Keyring
}

// Open initializes or connects to SQLite at dbPath.
func Open(dbPath string) (*DB, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", dbPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite single-writer safety

	// Credentials are sealed with a key held outside the database, so a stolen
	// copy of the database file is not a copy of the offsite storage credentials.
	keyDir := ""
	if dbPath != ":memory:" {
		keyDir = filepath.Dir(dbPath)
	}
	keys, err := secrets.Load(keyDir)
	if err != nil {
		conn.Close()
		return nil, err
	}

	database := &DB{conn: conn, keys: keys}
	if err := database.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return database, nil
}

// Keyring returns the server keyring protecting at-rest secrets.
func (d *DB) Keyring() *secrets.Keyring { return d.keys }

// PingContext checks whether the database can answer a bounded request.
func (d *DB) PingContext(ctx context.Context) error { return d.conn.PingContext(ctx) }

// Close closes the database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		email TEXT NOT NULL,
		name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'operator',
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS capsules (
		id TEXT PRIMARY KEY,
		service_name TEXT NOT NULL,
		app_name TEXT NOT NULL DEFAULT '',
		app_version TEXT NOT NULL DEFAULT '',
		file_path TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		digest TEXT NOT NULL,
		payload_hash TEXT NOT NULL,
		threshold INTEGER NOT NULL,
		total_shares INTEGER NOT NULL,
		recovery_key_id TEXT NOT NULL,
		encapsulated_key TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		deposited_at DATETIME NOT NULL,
		paired_app_id TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'active'
	);

	CREATE TABLE IF NOT EXISTS custodians (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		fingerprint TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS audit_events (
		seq INTEGER PRIMARY KEY,
		prev_hash TEXT NOT NULL,
		event_hash TEXT NOT NULL,
		action TEXT NOT NULL,
		actor TEXT NOT NULL,
		target_id TEXT NOT NULL,
		details_json TEXT NOT NULL,
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS audit_anchor (
		singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
		count INTEGER NOT NULL,
		hash TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS paired_apps (
		id TEXT PRIMARY KEY,
		service_name TEXT NOT NULL,
		app_name TEXT NOT NULL,
		api_token TEXT NOT NULL UNIQUE,
		pairing_code TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		recovery_key_id TEXT NOT NULL DEFAULT '',
		expires_at DATETIME NOT NULL,
		paired_at DATETIME,
		last_backup_at DATETIME,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		email TEXT NOT NULL,
		name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'operator',
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL
	);

	-- Back-channel logout fences a provider login's sid so a callback in flight cannot revive it.
	CREATE TABLE IF NOT EXISTS sso_logouts (
		issuer TEXT NOT NULL,
		sid TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		PRIMARY KEY (issuer, sid)
	);

	-- A subject-wide logout fences that subject's logins whose ID token predates cutoff.
	CREATE TABLE IF NOT EXISTS sso_subject_logouts (
		issuer TEXT NOT NULL,
		subject TEXT NOT NULL,
		cutoff DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		PRIMARY KEY (issuer, subject)
	);

	CREATE TABLE IF NOT EXISTS replication_targets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		bucket TEXT NOT NULL,
		region TEXT NOT NULL DEFAULT 'us-east-1',
		access_key TEXT NOT NULL,
		secret_key TEXT NOT NULL,
		prefix TEXT NOT NULL DEFAULT 'capsules/',
		host_key TEXT NOT NULL DEFAULT '',
		auto_sync INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'active',
		last_sync_at DATETIME,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS replication_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_id TEXT NOT NULL,
		capsule_id TEXT NOT NULL,
		bytes_transferred INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL,
		status TEXT NOT NULL,
		error_message TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL,
		FOREIGN KEY (target_id) REFERENCES replication_targets(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS recovery_key (
		singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
		key_id TEXT NOT NULL,
		public_key BLOB NOT NULL,
		threshold INTEGER NOT NULL,
		total_shares INTEGER NOT NULL,
		imported_by TEXT NOT NULL,
		imported_at DATETIME NOT NULL
	);

	-- recovery_key above is the single-key table from before rotation. It is copied into
	-- recovery_keys once and never written again; it stays so that an older binary started
	-- on a migrated database still finds the key it pinned instead of offering a fresh import.
	CREATE TABLE IF NOT EXISTS recovery_keys (
		key_id TEXT PRIMARY KEY,
		public_key BLOB NOT NULL,
		threshold INTEGER NOT NULL,
		total_shares INTEGER NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('current', 'previous', 'retired')),
		imported_by TEXT NOT NULL,
		imported_at DATETIME NOT NULL,
		retired_by TEXT NOT NULL DEFAULT '',
		retired_at DATETIME
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_recovery_keys_one_current ON recovery_keys(status) WHERE status = 'current';
	INSERT INTO recovery_keys (key_id, public_key, threshold, total_shares, status, imported_by, imported_at)
		SELECT key_id, public_key, threshold, total_shares, 'current', imported_by, imported_at FROM recovery_key
		WHERE NOT EXISTS (SELECT 1 FROM recovery_keys);

	CREATE TABLE IF NOT EXISTS restic_scan_state (
		singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
		last_attempt_at DATETIME,
		last_success_at DATETIME,
		last_error TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS restic_repos (
		path TEXT PRIMARY KEY,
		service_name TEXT NOT NULL DEFAULT '',
		expected_interval_seconds INTEGER NOT NULL DEFAULT 0,
		snapshot_count INTEGER NOT NULL DEFAULT 0,
		last_snapshot_at DATETIME,
		size_bytes INTEGER NOT NULL DEFAULT 0,
		lock_seen_at DATETIME,
		key_count INTEGER NOT NULL DEFAULT 0,
		acked_key_count INTEGER NOT NULL DEFAULT 0,
		missing_since DATETIME,
		scan_error TEXT NOT NULL DEFAULT '',
		measured_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS restic_snapshots (
		path TEXT NOT NULL REFERENCES restic_repos(path) ON DELETE CASCADE,
		id TEXT NOT NULL,
		written_at DATETIME NOT NULL,
		PRIMARY KEY (path, id)
	);

	CREATE TABLE IF NOT EXISTS restic_samples (
		path TEXT NOT NULL REFERENCES restic_repos(path) ON DELETE CASCADE,
		day TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		snapshot_count INTEGER NOT NULL,
		PRIMARY KEY (path, day)
	);

	CREATE TABLE IF NOT EXISTS backup_expectations (
		service_name TEXT PRIMARY KEY,
		expected_interval_seconds INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_capsules_service ON capsules(service_name);
	CREATE INDEX IF NOT EXISTS idx_capsules_recovery_key ON capsules(recovery_key_id);
	CREATE INDEX IF NOT EXISTS idx_paired_code ON paired_apps(pairing_code);
	CREATE INDEX IF NOT EXISTS idx_paired_token ON paired_apps(api_token);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_repl_logs_target ON replication_logs(target_id);
	`

	if _, err := d.conn.Exec(schema); err != nil {
		return err
	}
	if err := d.addColumn("replication_targets", "host_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	for _, column := range []string{"identity_issuer", "identity_subject", "sso_issuer", "sso_subject", "sso_sid"} {
		if err := d.addColumn("sessions", column, "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	if err := d.addColumn("capsules", "verified_at", "DATETIME"); err != nil {
		return err
	}
	return d.hashStoredTokens()
}

// tokenHash is how a bearer credential is stored: the hex SHA-256 of the token, as
// KyIdentity stores its session tokens. Session IDs (192 bits) and API tokens (256 bits)
// are random, so a fast unsalted hash is enough: a copy of the database yields only
// values that are useless as credentials and cannot be reversed.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// codeHash is how a pairing code is stored. A six-digit code has under 20 bits, so a
// plain SHA-256 of it is reversed by trying all 900,000 codes; the hash is keyed with a
// keyring key instead, which a copy of the database without secret.key does not hold.
func (d *DB) codeHash(code string) string {
	mac := hmac.New(sha256.New, d.keys.PairingCodeKey())
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// newAPIToken mints a product's 256-bit deposit token.
func newAPIToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed generating api token: %w", err)
	}
	return "kyrec_live_" + hex.EncodeToString(b), nil
}

// tokenHashMigration names the one-time rewrite of credentials stored as issued.
const tokenHashMigration = "hash-stored-tokens-v1"

// hashStoredTokens replaces every session ID, API token and pairing code that a store
// written before hashing kept as issued with its hash, once, in one transaction. Existing
// sessions and paired products keep working, because lookups hash what they are given. The
// marker row is written first, so a second process opening the same database waits for the
// write lock and then sees the marker instead of hashing the hashes.
func (d *DB) hashStoredTokens() error {
	if _, err := d.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`); err != nil {
		return err
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT OR IGNORE INTO schema_migrations (name, applied_at) VALUES (?, ?)`, tokenHashMigration, time.Now().UTC())
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return nil // already applied
	}

	type row struct{ id, token, code string }
	var sessions []string
	rows, err := tx.Query(`SELECT id FROM sessions`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		sessions = append(sessions, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	var apps []row
	rows, err = tx.Query(`SELECT id, api_token, pairing_code FROM paired_apps`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var a row
		if err := rows.Scan(&a.id, &a.token, &a.code); err != nil {
			rows.Close()
			return err
		}
		apps = append(apps, a)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, id := range sessions {
		if _, err := tx.Exec(`UPDATE sessions SET id = ? WHERE id = ?`, tokenHash(id), id); err != nil {
			return err
		}
	}
	for _, a := range apps {
		if _, err := tx.Exec(`UPDATE paired_apps SET api_token = ?, pairing_code = ? WHERE id = ?`,
			tokenHash(a.token), d.codeHash(a.code), a.id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if len(sessions)+len(apps) > 0 {
		log.Printf("db: hashed %d stored session IDs and %d product tokens and pairing codes", len(sessions), len(apps))
	}
	return nil
}

// addColumn adds a column to a table created by an older schema, once.
func (d *DB) addColumn(table, column, def string) error {
	rows, err := d.conn.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	_, err = d.conn.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, def))
	return err
}

// InsertCapsule saves a new capsule record.
func (d *DB) InsertCapsule(ctx context.Context, c CapsuleRecord) error {
	q := `INSERT INTO capsules (id, service_name, app_name, app_version, file_path, size_bytes, digest, payload_hash, threshold, total_shares, recovery_key_id, encapsulated_key, created_at, deposited_at, paired_app_id, status)
	      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := d.conn.ExecContext(ctx, q, c.ID, c.ServiceName, c.AppName, c.AppVersion, c.FilePath, c.SizeBytes,
		c.Digest, c.PayloadHash, c.Threshold, c.TotalShares, c.RecoveryKeyID, c.EncapsulatedKey,
		c.CreatedAt.UTC(), c.DepositedAt.UTC(), c.PairedAppID, c.Status)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return ErrCapsuleExists
	}
	return err
}

// MarkCapsuleVerified records that the stored bytes matched the digest at t.
func (d *DB) MarkCapsuleVerified(ctx context.Context, id string, t time.Time) error {
	_, err := d.conn.ExecContext(ctx, `UPDATE capsules SET status = 'active', verified_at = ? WHERE id = ?`, t.UTC(), id)
	return err
}

// SetCapsuleStatus updates a capsule's status ("active" or "corrupt").
func (d *DB) SetCapsuleStatus(ctx context.Context, id, status string) error {
	_, err := d.conn.ExecContext(ctx, `UPDATE capsules SET status = ? WHERE id = ?`, status, id)
	return err
}

// DeleteCapsule removes a capsule row. It is the rollback for a publish that claimed the row
// and then failed to put the file in place.
func (d *DB) DeleteCapsule(ctx context.Context, id string) error {
	_, err := d.conn.ExecContext(ctx, `DELETE FROM capsules WHERE id = ?`, id)
	return err
}

// GetCapsule retrieves a capsule by ID.
func (d *DB) GetCapsule(ctx context.Context, id string) (*CapsuleRecord, error) {
	q := `SELECT id, service_name, app_name, app_version, file_path, size_bytes, digest, payload_hash, threshold, total_shares, recovery_key_id, encapsulated_key, created_at, deposited_at, paired_app_id, status, verified_at FROM capsules WHERE id = ?`
	row := d.conn.QueryRowContext(ctx, q, id)
	var c CapsuleRecord
	err := row.Scan(&c.ID, &c.ServiceName, &c.AppName, &c.AppVersion, &c.FilePath, &c.SizeBytes, &c.Digest, &c.PayloadHash, &c.Threshold, &c.TotalShares, &c.RecoveryKeyID, &c.EncapsulatedKey, &c.CreatedAt, &c.DepositedAt, &c.PairedAppID, &c.Status, &c.VerifiedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListCapsules returns all active capsules.
func (d *DB) ListCapsules(ctx context.Context) ([]CapsuleRecord, error) {
	q := `SELECT id, service_name, app_name, app_version, file_path, size_bytes, digest, payload_hash, threshold, total_shares, recovery_key_id, encapsulated_key, created_at, deposited_at, paired_app_id, status, verified_at FROM capsules ORDER BY created_at DESC`
	rows, err := d.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []CapsuleRecord
	for rows.Next() {
		var c CapsuleRecord
		if err := rows.Scan(&c.ID, &c.ServiceName, &c.AppName, &c.AppVersion, &c.FilePath, &c.SizeBytes, &c.Digest, &c.PayloadHash, &c.Threshold, &c.TotalShares, &c.RecoveryKeyID, &c.EncapsulatedKey, &c.CreatedAt, &c.DepositedAt, &c.PairedAppID, &c.Status, &c.VerifiedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// InsertCustodian adds a custodian.
func (d *DB) InsertCustodian(ctx context.Context, c CustodianRecord) error {
	q := `INSERT INTO custodians (id, name, email, fingerprint, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := d.conn.ExecContext(ctx, q, c.ID, c.Name, c.Email, c.Fingerprint, c.CreatedAt.UTC())
	return err
}

// ListCustodians returns all registered custodians.
func (d *DB) ListCustodians(ctx context.Context) ([]CustodianRecord, error) {
	q := `SELECT id, name, email, fingerprint, created_at FROM custodians ORDER BY name ASC`
	rows, err := d.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []CustodianRecord
	for rows.Next() {
		var c CustodianRecord
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Fingerprint, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

const auditCols = `seq, prev_hash, event_hash, action, actor, target_id, details_json, created_at`

func scanAuditEvent(sc interface{ Scan(...any) error }) (AuditRecord, error) {
	var ar AuditRecord
	err := sc.Scan(&ar.Seq, &ar.PrevHash, &ar.EventHash, &ar.Action, &ar.Actor, &ar.TargetID, &ar.DetailsJSON, &ar.CreatedAt)
	return ar, err
}

// GetLastAuditEvent returns the latest audit record or nil if empty.
func (d *DB) GetLastAuditEvent(ctx context.Context) (*AuditRecord, error) {
	row := d.conn.QueryRowContext(ctx, `SELECT `+auditCols+` FROM audit_events ORDER BY seq DESC LIMIT 1`)
	ar, err := scanAuditEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ar, nil
}

// InsertAuditEventAndAnchor stores an event and the anchor it produced in one
// transaction: a record without its anchor, or an anchor without its record,
// leaves the chain unable to say where its end is.
func (d *DB) InsertAuditEventAndAnchor(ctx context.Context, ar AuditRecord, count uint64, hash string) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := insertAuditAndAnchor(ctx, tx, ar, count, hash); err != nil {
		return err
	}
	return tx.Commit()
}

// GetAuditAnchor returns the stored chain length and head. found is false before
// the first event is recorded.
func (d *DB) GetAuditAnchor(ctx context.Context) (count uint64, hash string, found bool, err error) {
	row := d.conn.QueryRowContext(ctx, `SELECT count, hash FROM audit_anchor WHERE singleton = 1`)
	err = row.Scan(&count, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	return count, hash, true, nil
}

// IterAuditEvents yields the whole log ascending. Verification cannot page: a
// fixed window reports a gap on a healthy chain once the log outgrows it.
func (d *DB) IterAuditEvents(ctx context.Context) iter.Seq2[AuditRecord, error] {
	return func(yield func(AuditRecord, error) bool) {
		rows, err := d.conn.QueryContext(ctx, `SELECT `+auditCols+` FROM audit_events ORDER BY seq ASC`)
		if err != nil {
			yield(AuditRecord{}, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			ar, err := scanAuditEvent(rows)
			if !yield(ar, err) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(AuditRecord{}, err)
		}
	}
}

// ListAuditEvents returns the most recent audit events, newest first, for display.
func (d *DB) ListAuditEvents(ctx context.Context, limit int) ([]AuditRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.conn.QueryContext(ctx, `SELECT `+auditCols+` FROM audit_events ORDER BY seq DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AuditRecord
	for rows.Next() {
		ar, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, ar)
	}
	return list, rows.Err()
}

// DeleteAuditEventForTest removes one audit event. It exists for tests that need
// a damaged log; nothing in production calls it.
// DeleteAuditAnchorForTest removes the anchor row, as a database-level attacker would.
func (d *DB) DeleteAuditAnchorForTest(ctx context.Context) error {
	_, err := d.conn.ExecContext(ctx, `DELETE FROM audit_anchor WHERE singleton = 1`)
	return err
}

func (d *DB) DeleteAuditEventForTest(ctx context.Context, seq uint64) error {
	_, err := d.conn.ExecContext(ctx, `DELETE FROM audit_events WHERE seq = ?`, seq)
	return err
}

// pairedAppColumns never includes api_token or pairing_code: they hold hashes, and a hash
// copied into APIToken or PairingCode would read as the credential it is not.
const pairedAppColumns = `id, service_name, app_name, status, recovery_key_id, expires_at, paired_at, last_backup_at, created_at`

// InsertPairedApp stores a pairing record, keeping only the hashes of its pairing code and
// API token. A record with no API token (a pending code) gets the hash of a random value
// nobody holds; ClaimPairingCode replaces it with the hash of the token it mints.
func (d *DB) InsertPairedApp(ctx context.Context, app PairedAppRecord) error {
	token := app.APIToken
	if token == "" {
		var err error
		if token, err = newAPIToken(); err != nil {
			return err
		}
	}
	q := `INSERT INTO paired_apps (id, service_name, app_name, api_token, pairing_code, status, recovery_key_id, expires_at, created_at)
	      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := d.conn.ExecContext(ctx, q, app.ID, app.ServiceName, app.AppName, tokenHash(token), d.codeHash(app.PairingCode), app.Status,
		app.RecoveryKeyID, app.ExpiresAt.UTC(), app.CreatedAt.UTC())
	return err
}

// GetPairedAppByCode finds a pending pairing code.
func (d *DB) GetPairedAppByCode(ctx context.Context, code string) (*PairedAppRecord, error) {
	q := `SELECT ` + pairedAppColumns + `
	      FROM paired_apps WHERE pairing_code = ?`
	row := d.conn.QueryRowContext(ctx, q, d.codeHash(code))
	var a PairedAppRecord
	err := row.Scan(&a.ID, &a.ServiceName, &a.AppName, &a.Status, &a.RecoveryKeyID, &a.ExpiresAt, &a.PairedAt, &a.LastBackupAt, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetPairedAppByToken finds an active paired app by its API bearer token. The lookup is
// by the token's hash: the database never compares against, or holds, the token itself.
func (d *DB) GetPairedAppByToken(ctx context.Context, token string) (*PairedAppRecord, error) {
	q := `SELECT ` + pairedAppColumns + `
	      FROM paired_apps WHERE api_token = ? AND status = 'paired'`
	row := d.conn.QueryRowContext(ctx, q, tokenHash(token))
	var a PairedAppRecord
	err := row.Scan(&a.ID, &a.ServiceName, &a.AppName, &a.Status, &a.RecoveryKeyID, &a.ExpiresAt, &a.PairedAt, &a.LastBackupAt, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ClaimPairingCode consumes a pairing code and marks it paired with specific app name/service.
// It mints the product's API token here and stores only its hash, so the returned record is
// the one and only place the token exists in plaintext.
func (d *DB) ClaimPairingCode(ctx context.Context, code, serviceName, appName, recoveryKeyID string) (*PairedAppRecord, error) {
	app, err := d.GetPairedAppByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, errors.New("invalid or expired pairing code")
	}
	if app.Status != "pending" {
		return nil, errors.New("pairing code already consumed")
	}
	if time.Now().After(app.ExpiresAt) {
		return nil, errors.New("pairing code expired")
	}

	// The status and expiry guards live in the UPDATE so two concurrent claims of the same
	// code cannot both mint a token; the checks above only shape the error message.
	token, err := newAPIToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	q := `UPDATE paired_apps SET status = 'paired', service_name = ?, app_name = ?, paired_at = ?, recovery_key_id = ?, api_token = ?
	      WHERE id = ? AND status = 'pending' AND expires_at > ?`
	res, err := d.conn.ExecContext(ctx, q, serviceName, appName, now, recoveryKeyID, tokenHash(token), app.ID, now)
	if err != nil {
		return nil, err
	}
	if rows, err := res.RowsAffected(); err != nil {
		return nil, err
	} else if rows == 0 {
		return nil, errors.New("pairing code already consumed")
	}

	app.Status = "paired"
	app.APIToken = token
	app.ServiceName = serviceName
	app.AppName = appName
	app.PairedAt = &now
	app.RecoveryKeyID = recoveryKeyID
	return app, nil
}

// UpdateAppLastBackup marks the latest backup time for a paired app.
func (d *DB) UpdateAppLastBackup(ctx context.Context, id string) error {
	now := time.Now().UTC()
	q := `UPDATE paired_apps SET last_backup_at = ? WHERE id = ?`
	_, err := d.conn.ExecContext(ctx, q, now, id)
	return err
}

// ListPairedApps returns all registered and pending paired applications.
func (d *DB) ListPairedApps(ctx context.Context) ([]PairedAppRecord, error) {
	q := `SELECT ` + pairedAppColumns + `
	      FROM paired_apps ORDER BY created_at DESC`
	rows, err := d.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []PairedAppRecord
	for rows.Next() {
		var a PairedAppRecord
		if err := rows.Scan(&a.ID, &a.ServiceName, &a.AppName, &a.Status, &a.RecoveryKeyID, &a.ExpiresAt, &a.PairedAt, &a.LastBackupAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

// RevokePairedApp revokes an active paired application token.
func (d *DB) RevokePairedApp(ctx context.Context, id string) error {
	q := `UPDATE paired_apps SET status = 'revoked' WHERE id = ?`
	_, err := d.conn.ExecContext(ctx, q, id)
	return err
}

// ClearRevokedPairedApp removes only an already-revoked registration. Capsule
// metadata and audit history are independent records and remain intact.
func (d *DB) ClearRevokedPairedApp(ctx context.Context, id string) (bool, error) {
	result, err := d.conn.ExecContext(ctx, `DELETE FROM paired_apps WHERE id = ? AND status = 'revoked'`, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// InsertRecoveryKey pins the first suite recovery public key as current. It succeeds only
// on a store that has never held a key; every later key arrives through RotateRecoveryKey.
func (d *DB) InsertRecoveryKey(ctx context.Context, k RecoveryKeyRecord) error {
	q := `INSERT INTO recovery_keys (key_id, public_key, threshold, total_shares, status, imported_by, imported_at)
	      SELECT ?, ?, ?, ?, 'current', ?, ? WHERE NOT EXISTS (SELECT 1 FROM recovery_keys)`
	res, err := d.conn.ExecContext(ctx, q, k.KeyID, k.PublicKey, k.Threshold, k.TotalShares, k.ImportedBy, k.ImportedAt.UTC())
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrRecoveryKeyExists
	}
	return nil
}

// RotateRecoveryKey makes k the current key and demotes replaces to previous, in one
// transaction. replaces must be the current key at commit, so two ceremony tabs racing each
// other cannot both win, and k must be a key this store has never held.
func (d *DB) RotateRecoveryKey(ctx context.Context, replaces string, k RecoveryKeyRecord) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE recovery_keys SET status = 'previous' WHERE key_id = ? AND status = 'current'`, replaces)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrStaleRotation
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO recovery_keys (key_id, public_key, threshold, total_shares, status, imported_by, imported_at)
	      VALUES (?, ?, ?, ?, 'current', ?, ?)`, k.KeyID, k.PublicKey, k.Threshold, k.TotalShares, k.ImportedBy, k.ImportedAt.UTC())
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return ErrRecoveryKeyKnown
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// RetireRecoveryKey marks a previous key retired. The guards live in the UPDATE: a current
// key, or one any stored capsule names, is never retired, whatever the caller checked first.
func (d *DB) RetireRecoveryKey(ctx context.Context, keyID, by string, at time.Time) error {
	res, err := d.conn.ExecContext(ctx, `UPDATE recovery_keys SET status = 'retired', retired_by = ?, retired_at = ?
	      WHERE key_id = ? AND status = 'previous' AND NOT EXISTS (SELECT 1 FROM capsules WHERE recovery_key_id = ?)`,
		by, at.UTC(), keyID, keyID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrRetireRefused
	}
	return nil
}

const recoveryKeyColumns = `key_id, public_key, threshold, total_shares, status, imported_by, imported_at, retired_by, retired_at`

func scanRecoveryKey(row interface{ Scan(...any) error }) (RecoveryKeyRecord, error) {
	var k RecoveryKeyRecord
	err := row.Scan(&k.KeyID, &k.PublicKey, &k.Threshold, &k.TotalShares, &k.Status, &k.ImportedBy, &k.ImportedAt, &k.RetiredBy, &k.RetiredAt)
	return k, err
}

// GetRecoveryKey returns the current recovery key, or (nil, nil) if none has been imported.
func (d *DB) GetRecoveryKey(ctx context.Context) (*RecoveryKeyRecord, error) {
	return d.getRecoveryKey(ctx, `SELECT `+recoveryKeyColumns+` FROM recovery_keys WHERE status = 'current'`)
}

// GetRecoveryKeyByID returns any key this store has held, or (nil, nil).
func (d *DB) GetRecoveryKeyByID(ctx context.Context, keyID string) (*RecoveryKeyRecord, error) {
	return d.getRecoveryKey(ctx, `SELECT `+recoveryKeyColumns+` FROM recovery_keys WHERE key_id = ?`, keyID)
}

func (d *DB) getRecoveryKey(ctx context.Context, q string, args ...any) (*RecoveryKeyRecord, error) {
	k, err := scanRecoveryKey(d.conn.QueryRowContext(ctx, q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ListRecoveryKeys returns every key this store has held, newest first.
func (d *DB) ListRecoveryKeys(ctx context.Context) ([]RecoveryKeyRecord, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT `+recoveryKeyColumns+` FROM recovery_keys ORDER BY imported_at DESC, key_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []RecoveryKeyRecord
	for rows.Next() {
		k, err := scanRecoveryKey(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, k)
	}
	return list, rows.Err()
}

// InsertSession stores a new user session.
func (d *DB) InsertSession(ctx context.Context, s SessionRecord) error {
	q := `INSERT INTO sessions (id, user_id, email, name, role, expires_at, created_at,identity_issuer,identity_subject,sso_issuer,sso_subject,sso_sid)
	      SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`
	args := []any{tokenHash(s.ID), s.UserID, s.Email, s.Name, s.Role, s.ExpiresAt.UTC(), s.CreatedAt.UTC(), s.IdentityIssuer, s.IdentitySubject, s.SSOIssuer, s.SSOSubject, s.SSOSID}
	if strings.HasPrefix(s.UserID, "installer-") || s.IdentityIssuer != "" {
		if s.UserID != InstallerIdentityID(s.IdentityIssuer, s.IdentitySubject) || s.Role != "admin" {
			return errors.New("invalid managed session binding")
		}
		q += ` WHERE EXISTS (SELECT 1 FROM system_settings WHERE key=? AND json_extract(value,'$.Active')=1 AND json_extract(value,'$.Issuer')=? AND json_extract(value,'$.Subject')=?)`
		args = append(args, "installer.identity."+s.UserID, s.IdentityIssuer, s.IdentitySubject)
	}
	if s.NativeSSO {
		// Same statement as the insert, so activation cannot land between check and write.
		q += ` WHERE NOT EXISTS (SELECT 1 FROM system_settings WHERE key=?)`
		args = append(args, InstallerConfigKey)
	}
	result, err := d.conn.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrSessionRefused
	}
	if s.SSOIssuer == "" {
		return nil
	}
	// Checked only after the insert: EndSSOSession and EndSSOSubject write their fence
	// before deleting, so one of the two always sees the other.
	now := time.Now().UTC()
	var fenced bool
	err = d.conn.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM sso_logouts WHERE issuer = ? AND sid = ? AND sid <> '' AND expires_at > ?) OR
		EXISTS (SELECT 1 FROM sso_subject_logouts WHERE issuer = ? AND subject = ? AND cutoff > ? AND expires_at > ?)`,
		s.SSOIssuer, s.SSOSID, now, s.SSOIssuer, s.SSOSubject, s.CreatedAt.UTC(), now).Scan(&fenced)
	if err != nil || fenced {
		_ = d.DeleteSession(ctx, s.ID)
		if err != nil {
			return err
		}
		return ErrSSOSessionEnded
	}
	return nil
}

// EndSSOSession ends every session of one provider login and fences its sid until
// fenceUntil, so a login callback still in flight cannot revive it.
func (d *DB) EndSSOSession(ctx context.Context, issuer, sid string, fenceUntil time.Time) error {
	if _, err := d.conn.ExecContext(ctx, `DELETE FROM sso_logouts WHERE expires_at <= ?`, time.Now().UTC()); err != nil {
		return err
	}
	if _, err := d.conn.ExecContext(ctx, `INSERT INTO sso_logouts (issuer, sid, expires_at) VALUES (?, ?, ?)
	      ON CONFLICT (issuer, sid) DO UPDATE SET expires_at = excluded.expires_at`, issuer, sid, fenceUntil.UTC()); err != nil {
		return err
	}
	_, err := d.conn.ExecContext(ctx, `DELETE FROM sessions WHERE sso_issuer = ? AND sso_sid = ?`, issuer, sid)
	return err
}

// EndSSOSubject ends a provider subject's sessions dated before cutoff and fences that
// subject's earlier logins until fenceUntil. It is keyed by subject, not by a local
// session, so it also fences a first login whose callback is still in flight. Sessions
// are dated by their ID token's iat, so cutoff and created_at are both provider time.
func (d *DB) EndSSOSubject(ctx context.Context, issuer, subject string, cutoff, fenceUntil time.Time) error {
	if _, err := d.conn.ExecContext(ctx, `DELETE FROM sso_subject_logouts WHERE expires_at <= ?`, time.Now().UTC()); err != nil {
		return err
	}
	// An older logout delivered late never moves the cutoff back.
	if _, err := d.conn.ExecContext(ctx, `INSERT INTO sso_subject_logouts (issuer, subject, cutoff, expires_at) VALUES (?, ?, ?, ?)
	      ON CONFLICT (issuer, subject) DO UPDATE SET cutoff = excluded.cutoff, expires_at = excluded.expires_at
	      WHERE sso_subject_logouts.cutoff < excluded.cutoff`, issuer, subject, cutoff.UTC(), fenceUntil.UTC()); err != nil {
		return err
	}
	_, err := d.conn.ExecContext(ctx, `DELETE FROM sessions WHERE sso_issuer = ? AND sso_subject = ? AND created_at < ?`, issuer, subject, cutoff.UTC())
	return err
}

// GetSession retrieves an active session by its raw session ID, looked up by hash.
func (d *DB) GetSession(ctx context.Context, id string) (*SessionRecord, error) {
	q := `SELECT user_id, email, name, role, expires_at, created_at,identity_issuer,identity_subject FROM sessions WHERE id = ?`
	row := d.conn.QueryRowContext(ctx, q, tokenHash(id))
	s := SessionRecord{ID: id}
	if err := row.Scan(&s.UserID, &s.Email, &s.Name, &s.Role, &s.ExpiresAt, &s.CreatedAt, &s.IdentityIssuer, &s.IdentitySubject); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		_ = d.DeleteSession(ctx, id)
		return nil, nil
	}
	return &s, nil
}

// DeleteUserSessionsExcept ends every session belonging to a user apart from one,
// so a password change cannot leave the old password's sessions alive. keepSessionID is
// the raw ID, as GetSession returns it.
func (d *DB) DeleteUserSessionsExcept(ctx context.Context, userID, keepSessionID string) error {
	q := `DELETE FROM sessions WHERE user_id = ? AND id != ?`
	_, err := d.conn.ExecContext(ctx, q, userID, tokenHash(keepSessionID))
	return err
}

// DeleteSession removes a session, given its raw ID, from SQLite.
func (d *DB) DeleteSession(ctx context.Context, id string) error {
	q := `DELETE FROM sessions WHERE id = ?`
	_, err := d.conn.ExecContext(ctx, q, tokenHash(id))
	return err
}

// InsertReplicationTarget creates or updates an offsite replication target.
func (d *DB) InsertReplicationTarget(ctx context.Context, t ReplicationTargetRecord) error {
	q := `INSERT INTO replication_targets (id, name, type, endpoint, bucket, region, access_key, secret_key, prefix, host_key, auto_sync, status, last_sync_at, created_at)
	      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	      ON CONFLICT(id) DO UPDATE SET
	      name=excluded.name, type=excluded.type, endpoint=excluded.endpoint, bucket=excluded.bucket,
	      region=excluded.region, access_key=excluded.access_key, secret_key=excluded.secret_key,
	      prefix=excluded.prefix, host_key=excluded.host_key, auto_sync=excluded.auto_sync, status=excluded.status`
	autoSyncInt := 0
	if t.AutoSync {
		autoSyncInt = 1
	}
	sealedSecret, err := d.keys.Seal(t.SecretKey)
	if err != nil {
		return fmt.Errorf("failed sealing replication secret key: %w", err)
	}
	_, err = d.conn.ExecContext(ctx, q, t.ID, t.Name, t.Type, t.Endpoint, t.Bucket, t.Region, t.AccessKey, sealedSecret, t.Prefix, t.HostKey, autoSyncInt, t.Status, t.LastSyncAt, t.CreatedAt.UTC())
	return err
}

// GetReplicationTarget returns a target by ID.
func (d *DB) GetReplicationTarget(ctx context.Context, id string) (*ReplicationTargetRecord, error) {
	q := `SELECT id, name, type, endpoint, bucket, region, access_key, secret_key, prefix, host_key, auto_sync, status, last_sync_at, created_at
	      FROM replication_targets WHERE id = ?`
	row := d.conn.QueryRowContext(ctx, q, id)
	var t ReplicationTargetRecord
	var autoSyncInt int
	var err error
	if err = row.Scan(&t.ID, &t.Name, &t.Type, &t.Endpoint, &t.Bucket, &t.Region, &t.AccessKey, &t.SecretKey, &t.Prefix, &t.HostKey, &autoSyncInt, &t.Status, &t.LastSyncAt, &t.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	t.AutoSync = autoSyncInt == 1
	if t.SecretKey, err = d.keys.Open(t.SecretKey); err != nil {
		return nil, err
	}
	return &t, nil
}

// ListReplicationTargets returns all configured replication targets.
func (d *DB) ListReplicationTargets(ctx context.Context) ([]ReplicationTargetRecord, error) {
	q := `SELECT id, name, type, endpoint, bucket, region, access_key, secret_key, prefix, host_key, auto_sync, status, last_sync_at, created_at
	      FROM replication_targets ORDER BY created_at ASC`
	rows, err := d.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReplicationTargetRecord
	for rows.Next() {
		var t ReplicationTargetRecord
		var autoSyncInt int
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Endpoint, &t.Bucket, &t.Region, &t.AccessKey, &t.SecretKey, &t.Prefix, &t.HostKey, &autoSyncInt, &t.Status, &t.LastSyncAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.AutoSync = autoSyncInt == 1
		// One unopenable secret must not hide every target: the operator needs to see the
		// broken row to fix or delete it, so it is listed with no credential.
		if opened, openErr := d.keys.Open(t.SecretKey); openErr != nil {
			log.Printf("db: replication target %s has an unopenable secret key: %v", t.ID, openErr)
			t.SecretKey = ""
		} else {
			t.SecretKey = opened
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

// UpdateReplicationTargetLastSync updates the sync timestamp and status.
func (d *DB) UpdateReplicationTargetLastSync(ctx context.Context, id, status string) error {
	now := time.Now().UTC()
	q := `UPDATE replication_targets SET last_sync_at = ?, status = ? WHERE id = ?`
	_, err := d.conn.ExecContext(ctx, q, now, status, id)
	return err
}

// DeleteReplicationTarget deletes a target by ID.
func (d *DB) DeleteReplicationTarget(ctx context.Context, id string) error {
	q := `DELETE FROM replication_targets WHERE id = ?`
	_, err := d.conn.ExecContext(ctx, q, id)
	return err
}

// InsertReplicationLog writes a log event for a capsule replication transfer.
func (d *DB) InsertReplicationLog(ctx context.Context, l ReplicationLogRecord) error {
	q := `INSERT INTO replication_logs (target_id, capsule_id, bytes_transferred, duration_ms, status, error_message, created_at)
	      VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := d.conn.ExecContext(ctx, q, l.TargetID, l.CapsuleID, l.BytesTransferred, l.DurationMs, l.Status, l.ErrorMessage, l.CreatedAt.UTC())
	return err
}

// ListReplicationLogs returns recent transfer logs.
func (d *DB) ListReplicationLogs(ctx context.Context, limit int) ([]ReplicationLogRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT id, target_id, capsule_id, bytes_transferred, duration_ms, status, error_message, created_at
	      FROM replication_logs ORDER BY created_at DESC LIMIT ?`
	rows, err := d.conn.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ReplicationLogRecord
	for rows.Next() {
		var l ReplicationLogRecord
		if err := rows.Scan(&l.ID, &l.TargetID, &l.CapsuleID, &l.BytesTransferred, &l.DurationMs, &l.Status, &l.ErrorMessage, &l.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

// InsertUser creates a new local user.
func (d *DB) InsertUser(ctx context.Context, u UserRecord) error {
	q := `INSERT INTO users (id, username, password_hash, email, name, role, created_at)
	      VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := d.conn.ExecContext(ctx, q, u.ID, u.Username, u.PasswordHash, u.Email, u.Name, u.Role, u.CreatedAt.UTC())
	return err
}

// GetUserByUsername finds a user by username.
func (d *DB) GetUserByUsername(ctx context.Context, username string) (*UserRecord, error) {
	q := `SELECT id, username, password_hash, email, name, role, created_at FROM users WHERE username = ?`
	row := d.conn.QueryRowContext(ctx, q, username)
	var u UserRecord
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// GetUserByID finds a user by ID.
func (d *DB) GetUserByID(ctx context.Context, id string) (*UserRecord, error) {
	q := `SELECT id, username, password_hash, email, name, role, created_at FROM users WHERE id = ?`
	row := d.conn.QueryRowContext(ctx, q, id)
	var u UserRecord
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// UpdateUserPassword updates the password hash for a user.
func (d *DB) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {
	q := `UPDATE users SET password_hash = ? WHERE id = ?`
	_, err := d.conn.ExecContext(ctx, q, passwordHash, id)
	return err
}

// CountUsers returns the total count of registered local users.
func (d *DB) CountUsers(ctx context.Context) (int, error) {
	q := `SELECT COUNT(*) FROM users`
	var count int
	err := d.conn.QueryRowContext(ctx, q).Scan(&count)
	return count, err
}

// GetSetting retrieves a system setting value by key.
func (d *DB) GetSetting(ctx context.Context, key string) (string, error) {
	q := `SELECT value FROM system_settings WHERE key = ?`
	var val string
	err := d.conn.QueryRowContext(ctx, q, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

// SetSetting saves or updates a system setting value.
func (d *DB) SetSetting(ctx context.Context, key, value string) error {
	if strings.HasPrefix(key, "installer.") {
		return errors.New("installer settings require owned audited mutation")
	}
	q := `INSERT INTO system_settings (key, value, updated_at) VALUES (?, ?, ?)
	      ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`
	args := []any{key, value, time.Now().UTC()}
	if strings.HasPrefix(key, "sso.") {
		q = `INSERT INTO system_settings (key,value,updated_at) SELECT ?,?,? WHERE NOT EXISTS (SELECT 1 FROM system_settings WHERE key=?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`
		args = append(args, InstallerConfigKey)
	}
	result, err := d.conn.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("owned settings conflict")
	}
	return nil
}

// GetAllSettings returns all key-value pairs from system_settings.
func (d *DB) GetAllSettings(ctx context.Context) (map[string]string, error) {
	q := `SELECT key, value FROM system_settings`
	rows, err := d.conn.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
}
