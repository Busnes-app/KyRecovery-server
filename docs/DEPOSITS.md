# Deposits

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Pairing and deposit

`POST /api/pairing/generate` (admin) mints a six-digit code, valid for 15 minutes
by default and at most 60. The product calls `POST /api/pairing/claim` with the
code; the response carries the API token **and** the key to seal to:

```json
{
  "id": "pair-...", "status": "paired", "api_token": "kyrec_live_...",
  "service_name": "kynotes", "app_name": "KyNotes Primary", "paired_at": "...",
  "server_url": "recovery.internal:8095",
  "recovery_public_key": "<standard base64, 1216 bytes>",
  "threshold": 3, "total_shares": 5
}
```

If no recovery key has been imported the claim is refused with `409` and the
pairing code is **not** consumed: run the ceremony before pairing anything.

`POST /api/backup/deposit` takes the sealed container itself:
`Authorization: Bearer <api_token>`, `Content-Type: application/octet-stream`,
body = a kycap/3 container sealed to the pinned public key.

| Status | Meaning |
| :--- | :--- |
| `201 Created` | Stored. Body: `{capsule_id, digest, size_bytes, deposited_at}`. |
| `200 OK` | The same capsule ID with the same bytes was already stored; the original record is returned. Re-sending is safe. |
| `400 Bad Request` | Not a kycap/3 container, or its `capsule_id`/`service_name` is not a usable name. |
| `401 Unauthorized` | Missing, invalid or revoked bearer token. |
| `403 Forbidden` | The manifest names a service this token is not paired for. |
| `409 Conflict` | Sealed to a key other than the current recovery key (both IDs are named; a key [rotated out](CEREMONY.md#rotating-the-recovery-key) says so and asks for a re-pair), or this capsule ID is stored with different bytes, or no key has been imported. |
| `413 Content Too Large` | Body over the container ceiling `capsule.MaxContainerBytes` (384 MiB). Larger data needs the streaming container, which does not exist yet. |
| `429 Too Many Requests` | More than 60 deposits from this product in 15 minutes. |
| `503 Service Unavailable` | The audit ledger cannot append, or all 4 deposit slots are busy and the client went away. |

`pkg/client` is the Go SDK for both halves: `ClaimPairing` and `Client.Deposit`.

A stored capsule is replicated to every auto-sync offsite target configured
under `/api/replication/targets`. The bytes are as opaque there as they are
here. Target types:

- `s3`: any S3-compatible bucket, signed with SigV4.
- `sftp`: an SSH server. The secret is a password or a PEM private key. The
  server's host key is pinned: Test Connection reports the SHA256 fingerprint
  of an unknown server, the operator confirms it against `ssh-keygen -lf` and
  saves it, and a later mismatch refuses to connect. No pin, no connection.
- `smb`: a Windows or Samba share over SMB 2 or 3, never SMB1. The host may be
  given as `host`, `host:port`, `\\host\share\dir`, `//host/share/dir` or
  `smb://host/share/dir`; a share or directory in the path fills in whichever
  field was left blank. The user may be `DOMAIN\user`. The client requires message signing. Known limitation: the
  SMB library accepts a server that grants a *guest* session, unsigned, so a
  host impersonating the share can swallow uploads while the sync log records
  success and can observe the NTLMv2 exchange. Give the target a strong
  password, keep it on a trusted network, and check the share occasionally.
  An SMB share mounted on the host and used as a `local` target does not have
  this gap.
- `local`: a directory, typically an NFS or SMB mount managed by the host.

Credentials for every type are sealed with `secret.key` before they reach the
database.

Replication uses `ky-primitives/offsite` v0.1.0 for local, ordinary S3,
account-relative SFTP, and new SMB writes. Existing absolute SFTP directories and
S3 endpoints containing the bucket name retain compatibility clients because the
released library cannot express their original locations. Those branches preserve
existing paths; they are not evidence that transport duplication is fully removed.
S3 endpoints must be HTTPS and redirects are refused on both paths. New target
records are validated before persistence; credentials belong in dedicated fields.

New SMB replicas use `kycap-v1-<SHA-256 of exact capsule ID>.kycap` beneath the
configured prefix. The hash uses the ID's original case and UTF-8 bytes, encoded
as lowercase hexadecimal. Historical `<capsule-id>.kycap` files are read through a
restricted, read-only compatibility adapter, after the canonical path is genuinely
absent. They are never renamed. Existing replicas count as successful only when
both size and SHA-256 match the local capsule receipt; a mismatch refuses the sync.
A rollback binary can still find historical names but cannot discover new hashed
SMB names automatically: retain this mapping with recovery instructions.

The opt-in `KYRECOVERY_SMB_TEST` tests verify new names, historical mixed-case
lookup, duplicate success and corruption refusal against a real Samba share. This
fixture does not substitute for an operator-observed sync to an existing target.
