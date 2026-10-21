# Backup status

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Backup status and restore planning

The **Backups** tab shows, per service, the newest capsule (stored, or verified
and when) and, with the restic overlay enabled, the newest restic snapshot of
each linked repository. It flags corrupt, missing, never-backed-up, stale, scan
problems, new repository keys and old locks; `/healthz` degrades with one fixed
reason code and no names.

Restic metadata is observation, not proof. KyRecovery reads only file names,
sizes and mtimes: a snapshot shows when rest-server **wrote** it, a lock is
repository activity, and a capsule and snapshot written close together are not
thereby consistent. KyRecovery never holds a restic password and never runs
restic.

The restore wizard picks the capsule and snapshot for a point in time and lists
the restore steps that work today, following
[Recovery and security](RECOVERY-AND-SECURITY.md#restore): download the capsule
and check its SHA-256 against the digest shown, run the **product's own**
`restore` command against it on your workstation with k custodian shares on
standard input, and check the capsule ID and creation time it prints. Shares are
never typed into the dashboard. A paired restic snapshot is named with its
repository; KyRecovery holds no restic password, so restore it with the tool
that made the backup. The wizard prints no single command: no suite-wide tool
takes a capsule yet.

To enable restic status, add `docker-compose.restic.yml` to `COMPOSE_FILE` and
follow the setup at its top. `scripts/verify-restic-overlay.sh` proves the
arrangement with real backups (needs Docker).
