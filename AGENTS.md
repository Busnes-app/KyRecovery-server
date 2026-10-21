# KyRecovery Server

KyRecovery Server is the self-hosted **blind store** for the Ky Suite (KySignOn, KyPassword, KyBookmarks, KyNotes, KyPost) and the busnes.app ecosystem. Products seal their own `.kycap` containers with `ky-primitives/capsule` and deposit the ciphertext here. The server holds no recovery private key, no Shamir share and no plaintext, and no code path in it decrypts a capsule.

## Core Capabilities & Responsibilities

1. **Sealed-container deposit**: `POST /api/backup/deposit` takes an `application/octet-stream` body under a paired product's bearer token, reads `capsule.ReadUnverifiedManifest`, checks the manifest's recovery key ID against the pinned key and its service name against the token, hashes the bytes and stores them. `201` for a new capsule, `200` for a byte-identical re-send. The wire contract is `zero_code_pairing_handoff_spec.md`.
2. **Browser keypair ceremony**: `/admin/ceremony` (admin session) serves `ceremony.wasm`, a `GOOS=js GOARCH=wasm` build of `cmd/ceremony-wasm` compiled from this commit. It calls `recoverykey.Generate` and `recoverykey.Split` in the tab, prints custodian cards, and posts only `{public_key, threshold, total_shares}` to `POST /api/recovery-key`. The first import is single-shot; a second one is `409`. Rotation is a new ceremony posting the same fields plus `replaces_key_id` to `POST /api/recovery-key/rotate`: the new key becomes `current`, the old one `previous`. Only the current key is handed out at pairing and accepted on deposit, so products learn a new key only by re-pairing. `POST /api/recovery-key/retire` refuses while any capsule row names the key. `recovery_keys` holds every key; the legacy `recovery_key` singleton is copied once and never written.
3. **Zero-code product pairing**: ephemeral six-digit codes (`/api/pairing/*`). `POST /api/pairing/claim` returns the API token plus `recovery_public_key`, `threshold` and `total_shares`, and is refused `409` before the ceremony has run, without consuming the code.
4. **Local admin bootstrap & KySignOn OIDC SSO**: local `admin` credentials on first start, PHC-format passwords via `ky-primitives/password`, SQLite session tokens, and an OIDC pairing portal with PKCE (`S256`). Every API route is authorized by `apiPolicy` in `internal/server/server.go` against `admin` > `operator` > `viewer`; SSO identities are verified against the issuer's JWKS before a session exists.
5. **Integrity attestation**: `GET /api/capsules/{id}/verify` re-hashes the stored blob and appends `capsule_verified` or `capsule_corrupt`, flagging the row; a sweep does the same for every capsule every 24 hours. `GET /api/capsules/{id}/download` returns the bytes with `X-Capsule-Digest` and `X-Capsule-Status`.
6. **Offsite replication (S3 / Cloudflare R2 / MinIO / local mounts)**: replicates stored containers through `ky-primitives/offsite` v0.1.0. Absolute SFTP directories and S3 endpoints containing the bucket name retain compatibility clients to preserve existing destinations. SMB writes use `kycap-v1-<SHA-256 of exact capsule ID>.kycap`; a restricted read-only adapter finds historical mixed-case names only after canonical absence. Existing SMB replicas require matching size/digest before success. No historical objects are renamed. S3 requires HTTPS with no redirects.
7. **Capsule diff & timeline inspector**: `internal/diff` computes drift across deposits from `capsules` rows — the recorded manifest fields — never by opening a container.
8. **Hash-chained audit ledger**: `ky-primitives/auditchain`, keyed from the keyring, with the anchor (count, last hash) kept outside the log. `POST /api/audit/verify` returns `{valid, count, last_hash}` and `append_disabled` + `error` when the ledger is poisoned. A poisoned ledger refuses deposits with `503` until an operator repairs the log and restarts.
9. **Privacy-safe structured logging**: `LOGGING.md` — structured JSON/logfmt to stdout/stderr, never secrets, keys or capsule contents.
10. **Public service health**: `GET /healthz` serves `ky.health/1` for `kyrecovery`. It checks database reachability, the audit ledger's append latch and stored backup status, caches one bounded result for five seconds, and exposes fixed status codes only. A poisoned ledger is degraded; an unavailable database is down; a backup finding degrades with one reason from `backupstatus.Reasons`. The authenticated `/api/readiness` retains its capsule, custodian and audit fields.
11. **Backup status and restore planning**: `/api/backups/*` and the Backups tab. With `--restic-dir`/`KYRECOVERY_RESTIC_DIR` set, a scanner reads rest-server's data directory (names, sizes, mtimes only) on start and every 15 minutes. `restore-plan` returns `steps`: the documented offline restore (download, check the digest, the product's own `restore` with k shares). It prints no command until a suite-wide restore tool that takes a capsule exists. Design: `docs/superpowers/specs/2026-10-07-backup-status-restore-ui-design.md`.

## Package index

| Path | Holds |
| :--- | :--- |
| `cmd/kyrecovery` | The binary. `app.Run` dispatches `serve`, `audit`, `pair`, `help`. |
| `cmd/ceremony-wasm` | The `js/wasm` ceremony module. The only code in the repo that generates or splits a private key. It never links into the server binary. |
| `internal/server` | Routes, `apiPolicy`, deposit, recovery-key import, verify sweep, limits, embedded dashboard and ceremony page under `static/`. |
| `internal/auth` | Sessions, local admin, OIDC/PKCE, role ranking. |
| `internal/db` | SQLite schema and every query, including `capsules`, `recovery_key` and `paired_apps`. |
| `internal/audit` | The ledger over `auditchain`, its anchor and health latch. |
| `internal/pairing` | Pairing code generation and claim. |
| `internal/replication` | Shared offsite adapter, product sync/audit ownership, and explicit legacy location compatibility clients. |
| `internal/secrets` | The keyring: master key from `KYRECOVERY_SECRET_KEY` or `<data-dir>/secret.key` via `ky-primitives/keyfile`. |
| `internal/crypto` | AES-256-GCM envelope used by the keyring, and nothing else. |
| `internal/diff` | The capsule diff and timeline inspector. |
| `internal/resticscan` | Reads restic repository metadata through an `os.Root`; never file contents. |
| `internal/backupstatus` | Pure classification of capsule and restic evidence, health reasons, and the restore plan. |
| `pkg/client` | The product-side SDK: `ClaimPairing`, `Client.Deposit`. |
| `scripts` | `build-wasm.sh` rebuilds the committed module byte-for-byte; `test-wasm.mjs` runs it under Node; `verify-restic-overlay.sh` proves `docker-compose.restic.yml` with Docker. |

Absolute-SFTP compatibility uploads reconcile regular `.ky-offsite-compat-*` staging files before Put and connection tests, after the greater of the default/current transfer budget plus one minute. Recent files and unrelated names are preserved. Cleanup listing/removal errors are logged and do not block uploads or connection tests; transfer errors still fail the sync. SMB legacy names outside the admitted grammar count as absent, allowing canonical replication of any valid capsule ID.

## Security Invariants

- **Nothing in the server decrypts.** `recoverykey.Generate/Split/Combine/FromSeed`, `capsule.Open`, `capsule.Seal` and `hpke.NewRecipient` may not appear in any non-test `.go` file outside `cmd/ceremony-wasm`. `TestNothingInTheServerDecrypts` walks the repo and fails if one does. Do not weaken it to land a feature.
- A deposit's manifest is displayed, never obeyed. The only decision taken from it is comparing `RecoveryKeyID` with the current key ID, a hash of a public key the server already holds.
- A recovery key is never replaced silently. Rotation names the key it replaces, is admin-only, cross-origin protected and recorded in the ledger before it is made; the store never re-admits a key it has held; retirement's guards (not current, no capsule names it) live in the SQL `UPDATE`.
- The server must never store plaintext master keys, passwords or third-party credentials. Replication secret keys and the SSO client secret are sealed with the keyring in `internal/secrets`.
- Every `/api/` route has an entry in `apiPolicy`; anything unlisted defaults to `admin`. New routes must be added to the table and to `TestRequiredRolePolicy`.
- An `/api/` path is rejected unless it equals `path.Clean` of itself, and both the capsule policy and the capsule handler read the URL through `parseCapsulePath`. Authorization and dispatch must never parse a URL twice.
- Credentials are never serialised by default. `PairedAppRecord.APIToken`, `PairedAppRecord.PairingCode`, `SessionRecord.ID` and `CapsuleRecord.FilePath` are `json:"-"`; the pairing code is returned only by `POST /api/pairing/generate` and the token only by `POST /api/pairing/claim`, each building its response explicitly.
- An OIDC identity is only trusted after `verifier.Verify` accepts the ID token's signature, issuer, audience and expiry, and the login nonce matches. A failed ID token is never retried against `userinfo`; unknown role claims become `viewer`. A callback carrying an IdP `error` is refused before any exchange; only a code matching `oauthErrorCode` is echoed, never `error_description`.
- Console output is a log. `serve` prints only a generated admin password, never a supplied `KY_ADMIN_INITIAL_PASSWORD`; `pair list` prints a code only for a pending, unexpired row.
- Outbound issuer requests use `auth.httpClient`, whose dialer refuses link-local destinations at connect time (covering redirects and DNS rebinding).
- A capsule ID becomes a filename, so it is held to `capsuleIDPattern` and published temp-file → fsync → row insert → `capsule_deposited` appended → hard link (never rename: a link refuses an existing target, so a case-variant ID on a case-folding volume cannot overwrite another product's bytes). The database row is the mutual-exclusion primitive and a per-ID lock covers the whole publish, so a retry of an in-flight deposit waits for a whole one; a row whose file is missing outside that window is marked corrupt rather than treated as a successful deposit.
- Every API request body is bounded (`bodyLimit`): 1 MiB, or `capsule.MaxContainerBytes` for the deposit. Deposits are rate limited per token and capped at 4 in flight.
- KyRecovery never holds a restic password, never runs restic and never writes the restic directory. The scanner touches it only through `os.Root`, skips symlinks, and fails the whole scan rather than report an unreadable directory as empty; only a successful scan marks a repository missing.
- Backup admin writes (`/api/backups/expectations`, `repos/link`, `repos/ack-key`) are recorded in the ledger before they are made, and refused with 503 if the record fails.
- A deposit that cannot be recorded in the ledger is refused. The audit trail is most of a blind store's evidence that a deposit happened.
- Session cookies are `HttpOnly`, and `Secure` whenever the login arrived over TLS or `--cookie-secure=true` is set. Session tokens are never returned in a response body — `TestSessionTokenNeverAppearsInAResponseBody` checks the bodies, not just the handler that used to leak one. Changing a password ends every other session belonging to that user.
- SQLite transactions must be atomic and use WAL mode with foreign key constraints.
- Sensitive HTTP responses must enforce `Cache-Control: no-store` and standard security headers.
- Unauthenticated pairing claims are rate limited per source address and per code, and the claim's single-use and expiry guards live in the SQL `UPDATE`, not only in Go checks.
- The audit ledger records a resolved identity. `Server.actor` derives it from the session; a caller-supplied name goes in `details` under a `claimed_` key, never in `actor`.
- The dashboard escapes every value it interpolates into HTML — `esc` for markup, `escJs` for a value inside an inline handler's JS string — and every response carries a Content-Security-Policy. `/admin/ceremony` carries a stricter one with `wasm-unsafe-eval` and no inline script or style. `TestEmbeddedDashboardEscapesEveryInnerHTMLSink` reads the shipped asset and fails on a new unescaped sink.

# Ponytail, lazy senior dev mode

Use the smallest correct change.

1. Reuse what already exists.
2. Prefer stdlib and native platform APIs.
3. Add dependencies only when they remove meaningful code.
4. Fix shared root causes, not one caller.
5. If a shortcut has a limit, mark it with `ponytail:` and name the upgrade path.

Non-trivial logic must include one runnable check (unit test or minimal self-check).

# DOX framework

## Core Contract

- AGENTS.md files are binding contracts for their subtree.
- Read from root to nearest AGENTS.md before editing.
- The nearest AGENTS.md controls local details; parent docs keep global rules.

## Update After Editing

- Run a DOX pass for every meaningful change.
- Update nearest owning AGENTS.md when behavior, responsibilities, or verification changes.
## Verification

- `node scripts/test-overview.mjs` checks the overview audit-evidence labels. The overview must distinguish stored capsule counts, custodian directory size and audit append availability from a verified restore.

Run all unit and integration tests across the repository:
```bash
go test -race -count=1 ./...
```

- On a push to `main` that passes every job, `publish` pushes the exact image the Docker check ran against (handed over as an artifact, no rebuild) to `ghcr.io/busnes-app/kyrecovery-server:<commit sha>`, attests it and verifies the attestation pinned to this workflow on `main`; `promote` then moves `:latest` to that digest, only at the tip of `main`, and asserts the tag resolves to the attested digest. `docker-compose.yml` names the published image and never builds; source installs add `docker-compose.build.yml` to the `COMPOSE_FILE` chain in `.env` (overlay tags `kyrecovery-server:local`) so every compose command, recovery docs included, uses the local build.
- Publishing a GitHub release `vX.Y.Z` runs `release-image.yml`. It refuses a malformed tag and any commit on which a ci.yml job in its `REQUIRED` list (`go`, `security` and `docker`) did not pass. It also refuses a tag that disagrees with `const version` in `cmd/kyrecovery/app/app.go` (what `kyrecovery version` prints). It then builds `:X.Y.Z` with provenance and SBOM, attests and verifies it (identity `release-image.yml@refs/tags/vX.Y.Z`), and moves `:stable` to that digest only for the highest non-prerelease. It never moves `:latest`; `main` CI owns that. A job added to ci.yml must be added to `REQUIRED` too.

Prove the restic overlay (Docker; uses its own project, loopback ports and a temp dir):
```bash
scripts/verify-restic-overlay.sh
```

Verify the binary builds and the CLI answers:
```bash
go build -o kyrecovery cmd/kyrecovery/main.go
./kyrecovery help
```

Rebuild the ceremony module and prove the committed artefact matches, then run it:
```bash
scripts/build-wasm.sh && git diff --exit-code -- internal/server/static/wasm
node scripts/test-wasm.mjs
```

## Installer setup

- `docs/INSTALLER_SETUP.md` owns common bundles, private live IPC, read-only checks,
  runtime inputs and remaining ceremony/composition gates.
- Exact managed administrator subjects, configuration, authority and native keyed
  audit/anchor commit in one SQLite transaction. Matching reruns do not write;
  revoked or missing retained authority is never repaired automatically.
- Managed OIDC requires verified issuer/subject bindings; no userinfo-only fallback,
  global-role grant or email promotion. Owned session bindings are checked live.
- Signed directory revisions/digests fence grant updates and session revocation in
  the same keyed audit transaction; capsules and paired products are retained.
- `KYRECOVERY_DATA_DIR` must match the serve data directory; setup runs as `kyrecovery`. `defaultDataDir` gives every subcommand the same default, and `docker-compose.yml` pins it while passing `.env` through as an `env_file` (never `${VAR:-}` entries, which would blank an override's `env_file`). `TestComposeForwardsSettingsFromDotEnv` and `TestComposePublishesOnLoopbackByDefault` hold the compose contract; the port defaults to `${KYRECOVERY_BIND:-127.0.0.1}`.
- `KYRECOVERY_TRUSTED_PROXIES` owns forwarding trust for limits, G11 and cookie TLS;
  default empty, `/0` refused. Explicit secure-cookie configuration still wins.
- Installer setup never conducts the custodian ceremony or imports a recovery key.

## Shared browser UI

- `internal/server/static/ky-ui/` is generated from Busnes-app/ky-ui, pinned by `VERSION` file hashes. Change shared colors, navigation states and storage helpers upstream, then run its consumer sync with an explicit worktree map; do not hand-edit vendored files.
- Products own layout, routes, saved choice keys and named palettes. Busnes aliases consume shared tokens; mark primary navigation with `ky-nav-item` while preserving current-page semantics.
- Verify vendored files with `node internal/server/static/ky-ui/check-vendor.mjs` from this document's directory. Builds/CI run that check. Rendered evidence and capture limitations are recorded in the repository-root `UI-VERIFICATION.md`.

## Child DOX Index
- `docs/INSTALLER_SETUP.md`: installer setup and runtime security contracts; read before changing setup, identity grants, signed directory or forwarded-header handling.
- `docs/plans/2026-10-10-release-readiness-handoff.md`: read when resuming the release-readiness and backup/restore work; lists local changes, next steps, and verification limits.
- `docs/plans/2026-10-08-release-readiness-review.md`: read before planning the backup/restore implementation or assessing its release gates; records spec corrections and verification against current main.
- `zero_code_pairing_handoff_spec.md`: the wire contract for pairing and deposit that this server exposes (`/api/pairing/generate`, `/api/pairing/claim`, `/api/backup/deposit`). A change here is breaking for every paired product; update their copies in the same change set.

- `docs/plans/2026-09-05-offsite-migration.md`: migration plan and compatibility gates; read before removing the remaining absolute-SFTP, virtual-host-S3 or read-only SMB adapters.

- `docs/superpowers/specs/2026-10-07-backup-status-restore-ui-design.md`: backup status (capsules, read-only restic repositories, `/healthz` for kyPulse) and guided restore; read before touching the Backups tab or `/api/backups/*`. Its input was `docs/plans/2026-10-07-backup-restore-ui-handoff.md`.

## Theme contract

- Web themes default to the Busnes.app cream/light and charcoal/dark palettes with orange accents, following the OS until a browser-local choice is saved. Preserve existing named themes and saved choices.

- Routine status pills use neutral theme tokens; reserve green for explicit verification/success feedback. Retention controls live in a padded panel body, with three columns on desktop and stacked fields on mobile.

## Operator documentation

`INSTALL.md` is the root-owned standalone entry point. When installation prerequisites, bootstrap, persistence or verification change, update it together with `README.md` and the linked operator guides.


## README and topic guides

Keep `README.md` as the concise product overview and navigation. Detailed procedures live in `docs/CEREMONY.md`, `docs/DEPOSITS.md`, `docs/INTEGRITY-AND-AUDIT.md`, `docs/RETENTION.md`, `docs/RECOVERY-AND-SECURITY.md`, `docs/BACKUP-STATUS.md`, `docs/OPERATIONS.md`. Update the owning topic guide and preserve README links when a procedure changes. Commands in these guides run from the repository root.
