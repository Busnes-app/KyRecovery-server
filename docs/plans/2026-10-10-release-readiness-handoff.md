**Repo:** kyrecovery-server
**Worktree:** /home/yoshi/git/busnes.app/kyrecovery-server (branch docs/backup-status-restore-ui-spec, HEAD f5ec59d)

# Handoff: release readiness and backup/restore UI

Date: 2026-10-10. Status: open for continuation. No implementation plan, feature implementation, PR, or deployment was created in this session.

## Done

- Picked up myslop post 2331 and reviewed the backup/restore design. Full findings: `docs/plans/2026-10-08-release-readiness-review.md`, mirrored as post 2465 in `kyrecovery-backup-restore-ui`.
- Established the distinction: existing blind-store core has green release CI; the proposed Backups tab, restic scanner, and restore planner remain unimplemented.
- Preserved the approved architecture: KyRecovery reads metadata and guides downloads; decryption and restore execution stay on the operator workstation. Separate rest-server on an independent off-NAS host; read-only mount into KyRecovery; no resident restic passwords or custodian shares here.
- On 2026-10-08, reviewed main at `890515d389552ac32c7b64a8de26f716840058d8`. It already contained `/healthz`, ky-primitives v0.9.0, and SQLite v1.60.1. Its CI passed Go, security, Docker, publish, and promote: https://github.com/Busnes-app/KyRecovery-server/actions/runs/37035914678. Refresh remote state before continuing; this is dated evidence.

## Working tree and documents

The checkout still has the following uncommitted work; preserve it before switching branches or rebasing:

- Modified `AGENTS.md`: release-review pointer plus this handoff pointer.
- Untracked `docs/plans/2026-10-08-release-readiness-review.md`.
- Untracked `docs/plans/2026-10-10-release-readiness-handoff.md` (this document).

The original design and architectural handoff are committed in f5ec59d:

- `docs/superpowers/specs/2026-10-07-backup-status-restore-ui-design.md`.
- `docs/plans/2026-10-07-backup-restore-ui-handoff.md`.

The design has not been revised. Review recommendations are not user approval of added scope.

## Next steps

1. Read the full release review, the design, and applicable `AGENTS.md` files. Refresh main and preserve the local documents; base implementation on current main rather than the old design branch.
2. Correct the design before writing the implementation plan:
   - Top-level scan errors and stale cached observations must degrade health; an unreadable root must not look empty or healthy.
   - Label locks as repository activity and snapshot mtimes as write-time observations. Timestamp proximity does not establish application consistency.
   - Specify deterministic restore selection: UTC, bounds, ties, corrupt latest capsule, explicit repository and full snapshot ID. Align the copied command with the actual KyQuickStart executor.
   - Define active product inventory, multiple instances/repositories, restic-only apps, expected channels, and unassigned repositories.
   - Separate deposited/available capsules from verified integrity evidence; retention can remove the last capsule while `last_backup_at` remains set.
   - Extend main's existing health handler with fixed declared reason codes. The proposed count string is incompatible with ky-primitives v0.9.0's reason-code grammar.
   - Define object filename filtering, bounded/cancellable scans, symlink containment, concurrent disappearance, container read permissions, TLS/authentication, and optional compose profile behavior.
3. Write the implementation plan, then implement capsule status/UI, read-only restic status, and guided restore integration in that order. Key-count alerts/acknowledgement and historical graphs were recommended for deferral; that recommendation remains unapproved. If key alerts stay, persist an acknowledged baseline rather than comparing only adjacent scans.
4. Before declaring the feature releasable, record a disposable recovery drill into a fresh target, including bulk data and workstation share entry; fresh-install and upgrade checks; UI evidence for populated/empty/error states; final CI and attested-image verification. Include recovery of KyRecovery's own database, keyring, and audit anchor in the runbook.

## Verification and limits

On 2026-10-08, both f5ec59d and reviewed main passed `go test -race -count=1 ./...`, binary build/CLI help, ceremony rebuild/Node execution, vendor checks, and light/dark contrast checks in separate clean archive copies. Rebuilt WASM assets matched committed bytes. Local Samba-dependent tests were not enabled; main's CI used Samba.

The source checkout's Docker-owned `data/` directory prevents direct `go test ./...` discovery (`permission denied`). Test an isolated checkout/copy; leave runtime ownership and data untouched. No restore drill, new browser captures, Docker deployment, or registry attestation verification was performed locally. Tests were not rerun for this documentation-only handoff; whitespace was checked.

## Careful

- Keep server/browser restore decryption out of KyRecovery and retain the existing security guard tests.
- Product tokens still reach only deposit. Add API routes to authorization policy and its tests; preserve audited downloads and admin mutations, escaping, CSP, and credential privacy.
- Reuse the shipped health handler and shared UI assets; do not hand-edit vendored ky-ui files.
- The salt.md connector was unavailable during the review. Coordinate continuation through the existing myslop folder, and mirror any future local handoff/status/blocked note there.

Immediate next action: preserve the uncommitted review documents, refresh main, and revise the design's contracts before producing the implementation plan.

## Continuation, 2026-10-10 (later session)

- Work moved to worktree `/home/yoshi/git/busnes.app/kyrecovery-server-backup-ui`, branch `feat/backup-status-restore-ui` from main `890515d` (unchanged since the review). The original checkout and its uncommitted files are untouched. Nothing is committed yet.
- Yoshi decided: keep key-count alerts with a persisted acknowledged baseline; keep 90-day history; pin the restore command contract here and update KyQuickStart's spec in the same change set; allow admin-named restic-only services.
- The design is revised against all seven review corrections and those decisions. KyQuickStart has no `restore` implementation yet, only the spec row.
- Unproven: rest-server file modes and whether UID 10001 can read them through a read-only mount. Determine this during implementation.
- Next: Yoshi reviews the revised design, then write the implementation plan.

## Build, 2026-10-10

Implemented in the worktree above; nothing committed or pushed.

- New: `internal/resticscan`, `internal/backupstatus`, `internal/db/backups.go`, `internal/server/backups.go`, `docker-compose.restic.yml`, `scripts/verify-restic-overlay.sh`, and the Backups tab (`index.html`, `app.js`, `styles.css`).
- Changed: `capsules.verified_at` (set by verify), the `backups` check in `/healthz`, five `/api/backups/*` routes in `apiPolicy`, the `--restic-dir` / `KYRECOVERY_RESTIC_DIR` setting, and README, AGENTS.md and the spec updated to match.
- KyQuickStart: the restore contract row is updated on branch `docs/restore-contract` in `/home/yoshi/git/busnes.app/KyQuickStart-restore-contract`, from origin/master, uncommitted. Its main checkout has unrelated uncommitted work and was not touched.
- Verified locally:
  - `go test -race -count=1 ./...` and `go vet`, with gofmt clean
  - vendor and contrast checks; WASM rebuilds byte-identical
  - `scripts/verify-restic-overlay.sh` passes with real rest-server 0.14.0 and restic 0.19.1. It records files `0640` and directories `0750`, owned `10002:10001`. A negative run without `--group-accessible-repos` fails, as it should.
  - The UI agent drove the tab headless in light, dark and mobile with fake repositories.
- Not verified: UI paths that need real capsules (skipped corrupt, Download, instance cadence); the old-lock, key-added, missing and stale-scan UI states; viewer and operator UI roles; clipboard copy. `UI-VERIFICATION.md` is not updated.
- Release gates still open: the recovery drill, a fresh install and an upgrade on the final image, CI, and attestation. KyQuickStart's `restore` executor does not exist yet.
