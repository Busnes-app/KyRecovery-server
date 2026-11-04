# Integrity and audit

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Integrity

- `POST /api/capsules/{id}/verify` (operator, cross-origin protected) re-reads the blob, compares its
  SHA-256 with the stored digest, appends `capsule_verified` or
  `capsule_corrupt`, and flags the row on a mismatch. A missing file counts as
  corrupt. GET is refused with `405`; viewers cannot verify. The capsule detail
  row offers Verify capsule to operators and admins.
- A sweep does the same for every capsule every 24 hours, as actor
  `integrity-sweep`.
- `GET /api/capsules/{id}/download` (operator) returns the bytes with
  `X-Capsule-Digest` and `X-Capsule-Status`. A capsule flagged corrupt still
  downloads, for forensics; the header is how you know.
- `GET /api/readiness` (viewer) reports `capsule_count`, `custodian_count` and
  `audit_append_disabled` and `audit_chain_broken`. A failed full audit verify
  shows Chain broken in Overview until a later successful verify. A blind store cannot open a capsule, so it never
  claims a verified restore.
- `GET /healthz` is public and serves the suite `ky.health/1` response for
  `kyrecovery`. It checks whether SQLite answers and whether the audit ledger
  can append. An unavailable database is `down` (HTTP 503); a poisoned ledger
  is `degraded` (HTTP 200, fixed reason `append_disabled`). Healthy is HTTP 200.
  Responses are bounded and shared for five seconds. They contain no database
  or ledger error text, paths, capsule counts, or ceremony state. The endpoint
  remains healthy before the recovery-key ceremony; use the authenticated
  `/api/readiness` for its existing operational fields.

## Audit ledger

Events are appended to `ky-primitives/auditchain`, with the anchor (count and
last hash) kept outside the log. `POST /api/audit/verify` (operator) returns
`{valid, count, last_hash}`, plus `append_disabled` and `error` when the ledger
is poisoned.

Two operational facts:

- A full verify holds the ledger lock, so deposits stall while a large log is
  being verified.
- If the log and the anchor disagree at startup the ledger refuses to append and
  every deposit gets `503` until an operator repairs the log and restarts. The
  latch is per-process; it is not cleared by anything short of a restart.

A failed full verification persists across restarts and new deposits. Overview shows
**Chain broken**, `/api/readiness` reports `audit_chain_broken: true`, and `/healthz`
reports its audit check as degraded with reason `chain_broken`, still HTTP 200.
Deposits and audited writes continue while the append latch remains healthy: the
warning preserves evidence of the past failure without refusing new backups. Only
a successful full verification clears it. The existing startup append latch for an
anchor/tail mismatch remains separate and still refuses appends. A cancelled or
unreadable verification also retains the warning; investigate the returned error
and repeat the full verification after repair. Health results may be cached for
five seconds.
