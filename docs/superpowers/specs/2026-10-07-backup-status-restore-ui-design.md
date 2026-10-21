# Backup status and guided restore UI

Date: 2026-10-07, revised 2026-10-10 against `docs/plans/2026-10-08-release-readiness-review.md`
and current main (`890515d`). Status: implemented on `feat/backup-status-restore-ui`; release gates below still open.
Answers `docs/plans/2026-10-07-backup-restore-ui-handoff.md` and KyQuickStart roadmap gate G10.

## Intent

One place to see whether every suite backup is current and to start a restore. KyRecovery shows
health for sealed capsules and restic repositories and plans a restore; it never decrypts,
never holds a restic password and never runs restic. Restores execute on the operator's
workstation through each product's `restore` command, and the tool that made a restic backup.

## Decisions

| Question | Decision |
|---|---|
| What "driving restic" means | Read-only status plus guided restore (handoff option 2a). No job queue, no in-browser decryption, no stored passwords |
| Where decryption happens | Operator's workstation only, shares on stdin. Every security invariant in `AGENTS.md` is unchanged |
| Restic data source | rest-server's data directory mounted read-only into KyRecovery; names, sizes and mtimes only |
| Staleness | Admin-set expected interval per service and per repository, suite default 26 h; observed cadence shown beside it as a hint |
| Alerting | kyPulse, through main's existing `GET /healthz`. No notification code in KyRecovery |
| rest-server credentials | Issued by KyQuickStart. KyRecovery never writes `.htpasswd` or any repository file |
| Key-count alerts (2026-10-10) | Kept, against a persisted acknowledged baseline |
| 90-day history (2026-10-10) | Kept: daily samples and sparklines |
| Restore command (2026-10-10) | Pinned here; KyQuickStart's installer spec is updated in the same change set |
| Restic-only apps (2026-10-10) | An admin links a repository to a free-text service name; no pairing required |

Rejected: target-side job runners (reverses "KyRecovery cannot trigger a backup"; a compromised
KyRecovery could request destructive restores), browser-held restic passwords (a compromised
KyRecovery could serve a page that captures them), KyRecovery holding passwords (breaks the
no-resident-reader rule), rest-server HTTP API (needs a credential per repository), rest-server
Prometheus metrics (no snapshot or lock data).

## What the metadata can and cannot prove

Restic encrypts snapshot, lock and key contents. KyRecovery sees file names, sizes and mtimes.

- A snapshot file's mtime is when rest-server wrote it. The UI says "written at", never "taken
  at"; hosts, tags, paths and the true snapshot time are not visible.
- A lock file is repository activity: some client held a lock at its mtime. It is not proof that
  a backup is running and it does not say which operation.
- A capsule and a snapshot written close together are not thereby consistent with each other.
  The restore plan pairs them by write time and says so; the executor confirms the decrypted
  snapshot before restoring.
- Capsule `created_at` is product-supplied. Freshness and selection use the server-recorded
  `deposited_at`; `created_at` is displayed as the product's claim.

## Inventory

A **service** is a `service_name`. The health table has one row per service in the union of:

- `service_name` of every `paired_apps` row with `status = 'paired'`. These **expect a capsule**.
- `service_name` of every repository linked by an admin. These **expect restic**.

A service expects only the channels it has. A paired product with no linked repository is not
degraded for lacking restic; a restic-only service is not degraded for lacking capsules.
`pending` and `revoked` pairings create no expectation.

- **Several instances per service.** `service_name` is not unique. Each paired instance is
  classified on its own capsules (`capsules.paired_app_id`); the service row shows the worst
  instance and expands to one line per instance.
- **Several repositories per service.** Each linked repository is classified on its own; the
  service row shows the worst.
- **Unassigned repositories** (no link) are listed for linking. They raise only `key_added` and
  `scan_error`, never staleness.
- **Auto-link.** On first sight, a repository whose first path segment (the rest-server user,
  the credential KyQuickStart issued to one app) equals a paired service name links to it,
  with every subrepository under that user. Later segments are chosen by the app's client and
  never link: `mallory/kynotes` must not claim kynotes. Otherwise it stays unassigned. Aligned
  with post 2810, 2026-10-10.

## Architecture

### Repository scanner: `internal/resticscan`

`Scan(ctx context.Context, root *os.Root, lim Limits) ([]Repo, error)`.

- **Top-level failure.** If the root or any user directory cannot be listed or inspected, if
  `ctx` ends, or if a bound below is hit, `Scan` returns an error and the caller changes no
  repository row. An unreadable directory is never an empty one, so it never makes a
  repository read as missing.
- **Containment.** The scanner only accesses the filesystem through the `os.Root` opened on
  `--restic-dir`, so no path, symlink or `..` can escape it. Entries are read with `Lstat`;
  symlinks are skipped and never followed.
- **Layout.** A repository is a directory holding a regular file `config` and a directory
  `keys/`, at most two levels below the root (`<user>/` or `<user>/<repo>/`, the
  `--private-repos` layout).
- **Admitted files.** In `snapshots/`, `keys/`, `locks/` and `data/<2 hex>/`, only regular files
  whose name is exactly 64 lowercase hex characters count. Anything else, including rest-server's
  temporary upload files, is ignored.
- **Per repository:**
  - `snapshots/`: ID (the file name) and mtime per file.
  - `data/`: total bytes.
  - `locks/`: newest mtime.
  - `keys/`: count.
- **Concurrent change.** An entry that vanishes between listing and `Lstat` is skipped. A
  repository that fails mid-read gets a per-repository `scan_error`, and its previous
  measurements are kept, marked as from the last good scan.
- **Bounds.** 2-minute deadline, at most 1,000 repositories and 2,000,000 directory entries per
  scan. Exceeding a bound fails the whole scan rather than reporting a partial inventory.

The server scans on start and every 15 minutes in its own goroutine, cancelled on shutdown. It
never runs two scans at once. A failed scan logs, records itself, keeps every previous row and
never stops the server.

### Storage (`internal/db`)

New tables use `CREATE TABLE IF NOT EXISTS`; the new column uses `addColumn`. Existing
installations migrate in place.

- `restic_scan_state` (single row): `last_attempt_at`, `last_success_at`, `last_error`.
- `restic_repos`:

  | Column | Meaning |
  |---|---|
  | `path` | primary key, relative to the root |
  | `service_name` | `''` when unassigned |
  | `expected_interval_seconds` | `0` inherits the service interval |
  | `snapshot_count`, `last_snapshot_at`, `size_bytes` | latest measurements |
  | `lock_seen_at` | newest lock mtime, nullable |
  | `key_count` | latest measurement |
  | `acked_key_count` | the acknowledged baseline, below |
  | `missing_since` | nullable |
  | `scan_error` | `''` after a good read |
  | `measured_at` | time of the last good read of this repository |
- `restic_snapshots`: `path`, `id` (64 hex), `written_at`. Replaced per repository on each good
  read. Feeds the restore plan and the cadence hint.
- `restic_samples`: `path`, `day` (UTC date), `size_bytes`, `snapshot_count`. One row per
  repository per day, upserted by a successful scan. The same scan deletes rows older than
  90 days, the span the graphs show.
- `backup_expectations`: `service_name` (primary key), `expected_interval_seconds`.
- `capsules.verified_at` (new, nullable): set by `verifyCapsule` when the digest matches.
  `status` stays `active`/`corrupt`. Existing capsules read "stored, not yet verified" until the
  next sweep.

**Key baseline.** `acked_key_count` is set to `key_count` when a repository is first seen.
`key_added` holds while `key_count > acked_key_count`, across any number of scans, until an
admin acknowledges, which sets the baseline to the current count. On each scan the baseline
drops to `min(acked_key_count, key_count)`, so a removed key that comes back alerts again.

**Missing repositories.** Only a successful scan that does not find a known repository sets
`missing_since`. A failed scan never marks anything missing.

### Status: pure functions (`internal/backupstatus`)

Inputs are rows plus `now`; no filesystem access.

- **Capsule side, per paired instance.**
  - Candidates are the instance's extant capsules, ordered by `deposited_at`, then `id`.
  - `pending`: no extant capsule, paired within one expected interval. Not a finding; health
    stays ok (adopted from the parallel spec, post 2654, 2026-10-10).
  - `never`: no extant capsule, paired longer ago.
  - `corrupt`: the newest capsule is `corrupt`.
  - `stale`: the newest capsule's `deposited_at` is older than the expected interval plus 10 %.
  - `ok`: otherwise.
  - `paired_apps.last_backup_at` is shown as "last deposit" only. Retention can remove a
    service's last capsule while `last_backup_at` stays set, so it never decides
    recoverability.
- **Restic side, per linked repository.**
  - `missing`, `never` (no snapshots), `stale` (on `last_snapshot_at`), `ok`.
  - Separate flags: `key_added`; `old_lock` (newest lock older than 30 min, with the
    `restic unlock` hint); `activity` (lock younger than 30 min, informational).
- **Scan freshness.** If the last successful scan is older than 1 h, or the latest attempt
  failed, every restic-derived result is shown as "as of <last success>", and the service is
  `scan_error`. Cached rows never read as healthy on their own authority.
- **Precedence** for the single status pill:
  `corrupt` > `scan_error` > `missing` > `never` > `stale` > `key_added` > `old_lock` > `pending` > `ok`.
  Every flag that applies is listed beside the pill. Activity never hides another state.
- **Observed cadence.** The median gap across the last 10 `deposited_at` values or snapshot
  write times. Flagged when it differs from the expected interval by more than 2×. Fewer than
  3 points: no hint.

### Restore plan

`RestorePlan(rows, service, instance, repo, at)`:

- `at` is RFC 3339 with an offset and is converted to UTC. Default is now. It is rejected if it
  is later than now + 5 min. Earlier than every backup, the plan reports no capsule and no
  snapshot.
- `instance` is required when the service has more than one paired instance. `repo` is required
  when more than one repository is linked. An ambiguous request returns 400 listing the choices,
  never an arbitrary pick.
- **Capsule:** the newest extant capsule of the instance with `deposited_at <= at`; ties break
  on `id`. If that capsule is `corrupt`, the plan returns it flagged and selects the next older
  `active` capsule. The UI shows both; it never substitutes silently.
- **Snapshot:** in the chosen repository, the snapshot whose `written_at` is nearest the
  selected capsule's `deposited_at`. Ties go to the earlier time, then the lower ID. With no
  capsule (a restic-only service), the newest snapshot written at or before `at`. The plan
  returns the full snapshot ID and the signed gap.
- **Warnings:** every capsule/snapshot pairing carries the "paired by write time; consistency is
  not established" warning. A gap over 1 h adds a mismatch warning.
- A missing capsule, snapshot or repository is reported in the plan, not as an error.

**Restore steps** (superseded the pinned command contract, 2026-10-21):

The plan once printed `kyquickstart restore <service> --capsule <capsule-id> --snapshot
<repo-path>:<snapshot-id>`. KyQuickStart never shipped that contract: its `restore APP
[--snapshot ID]` restores restic snapshots only, takes no capsule and reads no custodian
shares. The plan now returns `steps`, plain-text instructions for the restore that works
today (`docs/RECOVERY-AND-SECURITY.md`): download the capsule and check its digest, run the
product's own `restore` command with k shares on stdin, compare the printed capsule ID and
creation time, and, for a paired snapshot, restore it with the tool that made the backup.
Values are shown as text and escaped, never as a command to paste. When a suite-wide
executor exists, print its exact command and test it against that CLI.

### Configuration and deployment

- `--restic-dir` flag and `KYRECOVERY_RESTIC_DIR` env.
  - Unset: no scanner, and the restic panels explain how to enable it.
  - Set but unopenable: logged, and the scan state shows the error. Health degrades only once a
    service expects restic.
- `docker-compose.yml` is unchanged; the default installation runs without restic.
- `docker-compose.restic.yml`, an opt-in override, adds:
  - `rest-server`: image pinned by digest; `--private-repos --append-only`; TLS; `.htpasswd`
    issued by KyQuickStart. Its data path is a required variable that the docs say must not be
    on the NAS (catalog spec, Independence).
  - A read-only mount of that data path into `kyrecovery` at `/app/restic`.

  TLS and `.htpasswd` files are mounted into `rest-server` only.
- rest-server runs as `10002:10001` with `--group-accessible-repos`; the data directory is
  created `10002:10001 0750`. `scripts/verify-restic-overlay.sh` proves, with real restic
  backups, that it then writes files `0640` and directories `0750`, that UID 10001 reads and
  cannot write them, that the mount is read-only even to root in the KyRecovery container,
  that the htpasswd and TLS directory is invisible to KyRecovery, and that the base file alone
  stays healthy. Without the flag rest-server writes `0700` and the script fails.

### Health: extend main's handler

Add one `health.Check{Name: "backups"}` to `healthHandler` (`internal/server/health.go`). It
reads stored status under the handler's context and never touches the filesystem. It returns
`health.Degrade` with the highest-precedence declared reason:

`backup_corrupt`, `backup_scan_failed`, `backup_missing`, `backup_never`, `backup_stale`,
`restic_key_added`, `restic_old_lock`.

The response never contains service names, repository paths or counts; the counts are in the
authenticated status API. The existing database, audit, timeout, caching and privacy behaviour
is unchanged. kyPulse's "app degraded" alert does the notifying.

## API

Every route is added to `apiPolicy` and `TestRequiredRolePolicy`. Admin writes are audited
before they are made, as purge does: if `ledger.Record` fails, nothing changes and the
response is 503. Admin writes also carry `http.CrossOriginProtection`.

| Route | Role | Notes |
|---|---|---|
| `GET /api/backups/status` | viewer | services, instances, repositories, scan state, counts per state, 90-day samples |
| `GET /api/backups/restore-plan?service=&instance=&repo=&at=` | viewer | metadata only, as above |
| `POST /api/backups/expectations` | admin | `{service_name \| repo_path, expected_interval_seconds}`; 900 s to 400 days; audited `backup_expectation_set` |
| `POST /api/backups/repos/link` | admin | `{repo_path, service_name}`; `service_name` uses the existing service-name grammar and need not be paired; `""` unlinks; audited `restic_repo_linked` |
| `POST /api/backups/repos/ack-key` | admin | `{repo_path}`; baseline := current count; audited `restic_key_acknowledged` |

Capsule download stays `GET /api/capsules/{id}/download`: operator, audited `capsule_downloaded`.

## Dashboard: "Backups" tab

A new tab after Overview:

- A `ky-nav-item tab-btn` button with `data-tab="tab-backups"`.
- A `loadBackups()` loader in `app.js`.
- Styles in `styles.css`.

It uses the shared tokens. Status pills are neutral except for failure states and warnings.

- **Summary cards:** healthy, stale or never, corrupt, scan problems, total restic storage. A
  banner shows when the last successful scan is older than 1 h or the latest scan failed.
- **Health table:** one row per service. Each row shows:
  - channels expected
  - last capsule deposit age, "stored" or "verified <time>"
  - last snapshot write age
  - expected interval, with the observed cadence and ⚠ on mismatch
  - the status pill and its flags

  Admins edit the interval inline. A row expands into instances and repositories.
- **Repository detail:**
  - 90-day size and snapshot-count sparkline (inline SVG)
  - lock activity or the old-lock `restic unlock` hint
  - key count, with acknowledge for admins
  - measured time
- **Unassigned repositories:** a link control for admins that accepts a paired service or a new
  name.
- **Restore wizard:**
  1. Service, then instance and repository if ambiguous, then a date-time picker (local time
     shown, UTC sent).
  2. The capsule: ID, digest, deposited and created times, stored or verified, any corrupt
     newer capsule, and Download for operators.
  3. The snapshot: ID, write time and gap, with the pairing warnings.
  4. The copyable command, with the stdin note.
- A fixed line states that restic contents are encrypted and only write times are visible.

All interpolated values go through `esc` / `escJs`; no new CSP relaxation.

## Testing

- **`internal/resticscan`**, repositories in `t.TempDir`:
  - config, keys, snapshots, packs, fresh and old locks
  - temp-upload and non-hex names
  - symlink replacement of a repository and of a subdirectory
  - unreadable root (error, not empty)
  - unreadable repository
  - an entry deleted mid-scan
  - bound and deadline exceeded
  - nested `--private-repos` layout
- **`internal/backupstatus`**, table tests:
  - every state and precedence order
  - restic-only and capsule-only services
  - several instances and several repositories
  - retention emptying a service while `last_backup_at` stays set
  - key baseline across three scans, plus removal and re-add
  - stale scan state
  - cadence hint
- **`RestorePlan`**:
  - UTC conversion, bounds and ambiguity errors
  - ties
  - corrupt newest capsule
  - no capsule, no snapshot
  - gap at and over 1 h
- **Server:**
  - role policy for every new route
  - audit fail-closed for each admin write
  - `/healthz` reasons, and a body that never contains a service name, path or count
  - a failed scan leaves rows unchanged
  - migration on a pre-existing database
- **Existing guards unchanged:** `TestNothingInTheServerDecrypts`,
  `TestEmbeddedDashboardEscapesEveryInnerHTMLSink`, `TestSessionTokenNeverAppearsInAResponseBody`,
  `TestCapsuleListDoesNotLeakServerPaths`.
- **Rendered check:** populated, empty, stale, corrupt and scan-error states, in light and dark,
  desktop and mobile, recorded in `UI-VERIFICATION.md`.

## Release gates

From the review; the feature is not releasable until all hold:

1. A disposable drill into a fresh target covering:
   - ceremony, pairing, deposit and verification
   - offsite copy, download and digest comparison
   - product restore with the chosen snapshot validated after decryption
   - the original target unavailable and shares entered on stdin

   The results go into a runbook that also covers recovering KyRecovery's own database, keyring
   and audit anchor.
2. A fresh install and an upgrade, with the restic override both absent and enabled.
3. CI green on the final commit, and the image attestation and digest verified before promotion.

## Out of scope / follow-ups elsewhere

- KyQuickStart:
  - implement the pinned `restore` contract and update the `restore <app>` row of the installer
    architecture spec in the same change set
  - issue rest-server credentials
  - decide where `forget`/`prune` runs (it needs the repository password, so not here)
- Backup and `check` results from targets reach kyPulse directly, not through KyRecovery.
