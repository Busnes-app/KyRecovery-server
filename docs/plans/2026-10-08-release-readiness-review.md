**Repo:** kyrecovery-server
**Worktree:** /home/yoshi/git/busnes.app/kyrecovery-server (branch docs/backup-status-restore-ui-spec, reviewed commit f5ec59d)

# Release readiness and backup/restore spec review

Reviewed 2026-10-08. Picks up myslop post 2331. This is a review and proposed release sequence, not user approval of added scope or a completed implementation plan.

## Verdict and current state

The existing blind-store core has a green release pipeline. The proposed backup-status and guided-restore experience is designed but unimplemented. Keep the approved architecture: read-only restic metadata, guided download, workstation decryption, independent off-NAS storage, and product-owned backup schedules.

The design branch is behind current main (`890515d389552ac32c7b64a8de26f716840058d8`). Main already includes public `/healthz` with database and audit checks, ky-primitives v0.9.0, and SQLite v1.60.1. Start implementation from current main and carry the design documents forward; extend the existing health handler.

Main's [release CI run](https://github.com/Busnes-app/KyRecovery-server/actions/runs/37035914678) passed Go, security, Docker, publish, and promote. This establishes tested image publication and attestation, not an end-to-end disaster-recovery drill.

## Corrections needed before implementation

1. **Scan failure must degrade health.** `Scan(root, now) []Repo` cannot distinguish an empty root from a root that could not be read. Return a top-level error, preserve previous successful measurements, and record failure and last-success time separately. Define when old scan results become unavailable/stale. On a failed or partial scan, do not report missing repositories solely because enumeration failed, or report cached rows as healthy. Use cancellable, bounded traversal; the four-second health handler reads cached status rather than walking the filesystem.

2. **Metadata provides observations, not verification.** A recent lock is repository activity, not proof that a backup is running. Snapshot mtime is a filesystem write-time hint, not authenticated snapshot time. Inference: proximity to a capsule timestamp cannot establish application consistency. Label these limits in the UI and require the restore executor to decrypt/validate snapshot metadata and confirm the selected restore set. Restic's [repository format](https://restic.readthedocs.io/en/stable/100_references.html) encrypts snapshot and lock metadata and uses locks for repository operations.

3. **Make restore selection explicit and reproducible.** Define UTC handling, tie-breaking, requested-time bounds, and behavior when the newest capsule is corrupt. Show corruption and any older candidate explicitly. Return the repository path and full snapshot ID; the proposed command only passes capsule ID and time, so it cannot pin the displayed snapshot. Align the command contract with KyQuickStart before presenting it as executable. Warn about every heuristic pair, including gaps below one hour; do not imply a consistent recovery point from the gap alone.

4. **Define inventory and aggregation.** Pending/revoked pairing rows must not count as products missing backups. Multiple paired instances can share a service name, and multiple repositories can link to one service; define ownership and selection rather than choosing arbitrarily. Clarify how third-party/restic-only apps appear: the current link route requires a paired product. Define whether unassigned repositories affect health and which backup channels each product is expected to have. A product without bulk data should not fail solely for lacking restic.

5. **Use stored evidence accurately.** A capsule's `active` status does not prove a verification occurred; it is also the initial deposit status. Show “stored” separately from a recorded integrity check and its time. `paired_apps.last_backup_at` can survive retention deleting the last capsule, and its update error is ignored during deposit. Base available-recovery status on extant capsules and distinguish last deposit from recoverability. Manifest `created_at` remains product-supplied metadata.

6. **Reuse the shipped health contract.** ky-primitives v0.9.0 `health.DeclareReason` accepts fixed reason codes matching `[a-z0-9_]{1,64}`. The spec's `stale=2 failed=0 ...` string is incompatible. Use a fixed declared degradation reason, with counts in the authenticated status API unless the shared health schema is deliberately extended. Preserve main's database-down, audit-degraded, timeout, caching, and privacy behavior.

7. **Specify scanner/deployment boundaries.** Count only admitted full object-ID filenames and regular files; ignore temporary upload files. Handle disappearance during scans. Enforce root containment through actual filesystem access, including symlink replacement, rather than only lexical checks. Give KyRecovery read access as UID 10001 with a read-only mount, without making credential/TLS files readable unnecessarily. An optional rest-server profile must leave the default installation working when disabled. Pin its image, configure TLS and per-app authentication, and document credential issuance and off-NAS placement. The [rest-server reference](https://github.com/restic/rest-server) documents private-repository paths, TLS, authentication, and filesystem access.

Key-count acknowledgement and a new `failed` category were flagged in post 2331 as scope additions. Recommend deferring key alerts/acknowledgement and historical graphs from the first release. If retained, define a persisted acknowledged baseline: comparison with only the previous scan loses an unacknowledged alert on the next scan. Define status precedence; repository activity must not hide corruption or stale backups.

## Smallest release sequence and acceptance gates

1. Revise the design against the corrections above, then write an implementation plan based on current main. Resolve the executable restore-command contract with KyQuickStart; keep the existing security invariants.
2. Implement capsule inventory, backup expectations, explicit integrity evidence, the Backups tab, and capsule download guidance. Include role policy, escaping, audit-failure behavior, and existing-installation migration checks.
3. Add the read-only restic scanner, explicit repository associations, scan freshness/error handling, and the backup check in the existing health handler. Verify the optional compose profile both enabled and disabled, including container read permissions and private-repository isolation.
4. Integrate guided restore with the actual KyQuickStart executor. Require a disposable drill covering ceremony, pairing, deposit, verification, offsite copy, download/digest comparison, and product restore. For bulk data, validate the chosen snapshot after decryption and verify restored application data. Demonstrate recovery with the original target unavailable and custodians entering shares on stdin. Record the results and recovery runbook, including KyRecovery's own database, keyring and audit-anchor recovery.
5. Capture populated, empty, stale, corrupt, and scan-error UI states in light/dark desktop/mobile; update `UI-VERIFICATION.md`. Run repository release checks and CI on the final commit, then verify its image attestation/digest and smoke-test a fresh installation and an upgrade before promotion/deployment.

## Verification performed for this review

- `go test -race -count=1 ./...` passed on both reviewed commits in separate clean `git archive` copies.
- Binary build and `kyrecovery help`, ceremony rebuild and Node execution, shared vendor check, and light/dark contrast checks passed on both copies. Both rebuilt WASM assets matched their committed bytes by SHA-256.
- Direct testing in the source checkout hit `data: permission denied` because the runtime directory belongs to Docker UID 10001. Isolated copies avoided runtime data and changed no permissions.
- Local Samba-dependent tests were not enabled; the green main CI run includes its Samba service. No new Docker deployment, browser capture, product restore drill, or image-registry verification was performed in this review.
- The salt.md connector was unavailable. Shared handoff coordination used myslop. The design remains unchanged so recommendations are distinguishable from approved decisions; the DOX pass adds only this review's index pointer.

Next action: revise the design's scanner/error, restore-selection, inventory, and health contracts, then produce the implementation plan. This review is complete; implementation remains open.
