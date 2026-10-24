# Recovery and security

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## Restore

KyRecovery is not in the restore path beyond the download. A fresh product
instance is restored by running the **product's** `restore` command against the
`.kycap` and typing k custodians' shares from their cards. `capsule.Open` proves
the bytes are the sealed ones and were sealed to the pinned key; comparing the
capsule ID and creation time against KyRecovery's deposit record is where
freshness comes from.

### Which cards a capsule needs

Every capsule is recorded against the recovery key it was sealed to. After a
[rotation](CEREMONY.md#rotating-the-recovery-key) the store holds several keys, and a
capsule opens only with the cards of **its** key. The restore plan (Backups tab, or
`GET /api/backups/restore-plan`) returns `recovery_key` with that key's ID, threshold,
card count and status. If the key is not the current one, the plan warns that the
capsule needs k of the cards printed with the old key ID, not the current cards.

Rotation does not touch existing backups. Capsules sealed to the old key stay sealed to
it until retention purges them; the server cannot re-seal them to the new key, because
that would mean decrypting them. Keep the old cards until the dashboard lets you retire
the old key, and until no offsite copy of its capsules remains.

## Security model

[Assurance case](ASSURANCE-CASE.md) ties these protections to the threat model and the code;
[Verifying releases](VERIFYING-RELEASES.md) checks an image's provenance and rebuilds its binary.

- **Roles.** Every `/api/` route is authorized against `viewer` < `operator` <
  `admin` by the `apiPolicy` table; anything unlisted defaults to `admin`.
  Viewers read; operators verify the chain, sync replication and download
  capsules; admins import, rotate and retire recovery keys, pair products, and change
  replication and SSO. An unrecognised role claim from an identity provider is
  treated as `viewer`. An API path is rejected unless it equals `path.Clean` of
  itself, so a URL cannot mean one thing to the policy and another to the
  handler.
- **Credentials.** A product API token is shown once, to the product that claims
  the pairing code; a pairing code is shown once, to the administrator who
  generates it. Neither appears in the paired products listing, and no response
  body contains a session token. The database keeps only hashes of session IDs,
  API tokens and pairing codes, so a copy of it or of a backup holds none of
  them, and `pair list` cannot print a code (see [CLI](OPERATIONS.md#cli)).
- **Transport.** Session cookies are `HttpOnly`, and `Secure` when the login
  arrived over TLS (directly or via `X-Forwarded-Proto`). Run behind TLS and set
  `--cookie-secure=true` to make that unconditional. Changing a password signs
  out every other session on the account.
- **Dashboard.** Values from the database are escaped before they are rendered,
  and every response carries a Content-Security-Policy. `/admin/ceremony` gets a
  stricter one: it needs `wasm-unsafe-eval`, and in exchange carries no inline
  script or style at all.
- **Limits.** API bodies are capped at 1 MiB, deposits at 384 MiB, 60 deposits
  per 15 minutes per paired product, and 4 in flight at once. KyRecovery does not
  reserve or enforce free disk space for the capsule volume — size it for your
  retention.
- **SSO.** The issuer must publish `/.well-known/openid-configuration`; endpoints
  and signing keys come from discovery. Outbound issuer requests refuse
  link-local destinations at connect time. A callback carrying the provider's
  `error` (for example `access_denied`) is answered 401, naming the code when it is well formed;
  the provider's `error_description` is never shown.

## Upgrading from the previous KyRecovery

The obsolete prototype accepted plaintext and encrypted it on the server, unlike the
current blind-store design. Its format has no supported migration to the current capsule
format. Preserve an offline copy of any old database, capsules and keys before evaluating
an upgrade; do not delete an existing `data/` directory as an installation step. Use a
separate empty directory for the new blind store and ceremony. If the old instance has
valuable backups, retain its matching software and prove recovery before retiring it.
Only intentionally disposable prototype data may be removed after that decision.
