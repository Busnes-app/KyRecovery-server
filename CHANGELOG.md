# Changelog

## Unreleased

### Security

- Pairing codes keep the service selected at generation. An omitted claim service
  inherits it; a mismatch gets `409` without consuming the code. Only `auto-declare`
  codes accept the claimer's service (`generic` when omitted).
- Capsule verification now requires an operator `POST`, protected against cross-origin
  requests. GET returns `405`; the dashboard offers Verify capsule only to operators
  and administrators.

- Every dashboard write refuses a request from another origin, sibling subdomains included.
- `KY_ADMIN_EMAIL` matches only an email the issuer marks verified (`email_verified: true`).
  Microsoft Entra does not send that claim by default: map a verified claim or use
  installer-managed subject grants, or the address no longer grants `admin` after upgrading.
- Sign-in: five failed passwords now block that account only from the address that sent
  them, and the block answers `401` like a wrong password (it was `429` from everywhere,
  so anyone could lock the admin out). An IPv6 client's /64 counts as one address.
- Pairing claims: failed claims from all addresses share 100 per 15 minutes, and an IPv6
  /64 counts as one address. A guessed code is held in memory only as a hash.
- Importing the first recovery key, saving SSO settings and adding or deleting a
  replication target are recorded in the audit ledger first, and refused with `503` when
  the ledger cannot record them (as rotation and retention already were).
- A password change ends the user's other sessions in the same transaction, including a
  sign-in with the old password that was still in progress, and fails if it cannot.
- The audit actor of an SSO user without a verified email is their subject, never their
  display name.
- A cleared SSO admin email stays cleared after a restart instead of falling back to
  `KY_ADMIN_EMAIL`.
- The log warns once when forwarded requests arrive from a proxy that is not in
  `KYRECOVERY_TRUSTED_PROXIES`. Behind a proxy that setting is required: without it every
  client shares the proxy's sign-in and pairing limits.
- The image is built with Go 1.27.4 (was 1.27.3) for GO-2026-6859: an HTTP/1 request
  could crash the server through a race in `net/http`'s `MaxBytesReader`. `go.mod` sets
  `toolchain go1.27.4` so `go build` from source uses it too.

### Fixed

- An incomplete deposit that times out keeps its read deadline while HTTP/1.1 finalizes
  the error response, so it closes without waiting for more body bytes.

- A failed full audit verification persists across restarts until a successful verify.
  Overview says Chain broken, readiness reports `audit_chain_broken`, and `/healthz`
  reports `chain_broken` in its body without changing HTTP status. Deposits continue
  while the append latch is healthy.

- Deposit reads and capsule download writes now allow 5 minutes without byte progress,
  renewed per chunk, with a 24-hour maximum, replacing the fixed 15-minute timeout.

- Replication never sends a capsule marked corrupt or whose file no longer matches its
  digest, so a bad local copy cannot overwrite a good replica.
- Retention never keeps a corrupt capsule as a week's or month's survivor in place of an
  intact one.
- A deposit retry of a capsule marked corrupt is refused with `409` instead of `200`.
- Overview shows a failed replication target as Failing, a service with no deposit yet as
  not backed up, and reloads backup evidence while it is open.
- The ceremony form hides after Generate, so printed cards always match the imported key.
- A pasted SFTP private key works from the dashboard.
- Overview and Replication count the capsules each target has no copy of, so a later
  success no longer hides an earlier failure. `GET /api/replication/targets` adds
  `unreplicated`. A sync cancelled by closing the tab is still logged.
- Purge re-checks each week's or month's surviving capsule before deleting the ones it
  replaces, so a capsule damaged since the last daily check cannot cost the intact one.
- The ceremony enables Import only after the print dialog has opened and the admin ticks
  "I have printed n cards".
- The restore picker offers services whose products were revoked but whose capsules remain
  (`GET /api/backups/status` adds `restorable_services`).
- A restic directory that fails to scan before any repository was found degrades health.
- `audit` and `pair` refuse to run without `KYRECOVERY_SECRET_KEY` or an existing
  `secret.key`, instead of creating a new key in the store.
- Browser Back after opening Settings returns to the tab you left.
- `pair claim --service` works; `audit` and `pair` refuse a data directory with no store.

## 0.6.0 — 2026-11-01

### Added

- Backups has a freshness history: one row per service, a bar per recent capsule
  deposit or restic snapshot sized by the gap before it against the service's
  schedule. `GET /api/backups/status` adds `runs` (the newest 30 per service, read
  from rows already stored); restic history reads the newest 30 snapshot times per
  repository instead of 10.

### Changed

- ky-ui 0.10.0: Account and Appearance move from the navigation into a new Settings
  item, as tabs with their own addresses (`#settings/account`, `#settings/appearance`;
  `#account` and `#appearance` still open them). Sign out in the sidebar and the phone
  header is an icon button.
- ky-ui 0.8.0: status pills use the shared `.ky-status` with danger, warning and
  success colours that no longer borrow the accent. Replication shows Automatic or
  Manual instead of ENABLED or MANUAL, and a revoked pairing reads as neutral.

## 0.5.0 — 2026-10-28

### Added

- KyIdentity back-channel logout. Signing out of KyIdentity, or an administrator ending a
  login there, now ends the matching KyRecovery sessions. KyIdentity POSTs a signed
  `logout_token` to `POST /api/auth/backchannel-logout`; register
  `<KyRecovery URL>/api/auth/backchannel-logout` as the client's back-channel logout URI.
  A token naming a `sid` ends that login; one naming only the subject ends every login
  of the subject whose ID token predates the logout. A logout that arrives while the
  browser is still finishing that login, including a first login, stops the login from
  completing. SSO sessions are dated by the ID token's `iat`. Repeated logout requests
  never make KyRecovery re-run discovery or refetch the provider's keys more than once
  every 30 seconds. The event is audited as `user_logged_out_sso`.
- Upgrading signs out every SSO session created before this release, once, as the
  `sessions.sso_sid` column is added. Those sessions carry no `sid` and no ID-token date,
  so no KyIdentity logout could end them. Local password sessions are kept; SSO users
  sign in once more.

### Changed

- The dashboard has an Appearance tab with the shared ky-ui 0.4.1 theme cards: Match
  system plus every suite theme. It replaces the header select; the standalone ceremony
  page keeps its own.
  Saved `system`, `light` and `dark` choices keep their values. Text now uses
  self-hosted Space Grotesk (OFL) instead of system and serif fonts. No server or
  stored-data change.
- On phones, the dashboard tab row brings the current tab into view when an Overview link opens it; the page does not scroll.
- The saved theme applies before first paint again: `theme.js` is a classic head script
  over ky-ui 0.4.1's generated `themes-browser.js` instead of a deferred module.

### Security

- Session IDs, product API tokens and pairing codes are no longer stored as issued.
  `kyrecovery.db` keeps the SHA-256 of each session ID and API token and a keyed
  HMAC-SHA256 of each pairing code, so a copy of the database or of a backup no longer
  holds a usable credential. Existing stores are rewritten in place the first time the
  new binary opens them; signed-in users and paired products keep working. An older
  binary cannot use the rewritten store (users are signed out and products must
  re-pair), so keep the pre-upgrade backup if you may roll back.
- The API token is now minted when a product claims its code, not when the code is
  generated, and is returned only in the claim response.
- `pair list` no longer prints any pairing code; `pair generate` shows it once.

## 0.4.3 — 2026-10-23

### Added

- Recovery-key rotation. A new browser ceremony on `/admin/ceremony` replaces the
  current key through `POST /api/recovery-key/rotate`; the old key stays as
  `previous` so its capsules remain restorable with its cards. `GET /api/recovery-keys`
  lists every key with its k-of-n, status, retained capsule count and the products
  still pinned to it. `POST /api/recovery-key/retire` retires a previous key, refused
  while any retained capsule is sealed to it. All admin-only and audited. See
  [docs/CEREMONY.md](docs/CEREMONY.md#rotating-the-recovery-key).
- The restore plan names the recovery key and custodian cards a capsule needs, and
  warns when that is not the current key.

### Changed

- Security: the image is built with Go 1.27.3 (was 1.27.2) for GO-2026-6809:
  `crypto/x509` now distrusts compromised intermediate certificates. `go.mod` sets
  `toolchain go1.27.3` so `go build` from source uses it too.
- The restore plan returns `steps`, the documented offline restore (download the
  capsule, check its digest, run the product's own `restore` with k shares), instead of
  a `kyquickstart restore --capsule` command that KyQuickStart does not implement. The
  `command` field is gone.
- A deposit refused because its key was rotated out now carries
  `"code": "recovery_key_rotated"`, `sealed_key_id` and `current_key_id`, so products can
  report "re-pair" without parsing the message.
- Dashboard copy names the Ky Suite and busnes.app (was "KySecurity" and "Business.app").
  The custodians table's "Public Key Fingerprint" column is now "Record ID": it is a hash
  of the name, email and registration time, not a key. The ceremony link reads "Rotate
  recovery key" once a key exists, the login form no longer prefills `admin`, and "Sync
  All" reports its result in the page, counting failures, instead of in an alert.
- A generated local `admin` password is no longer printed to the log. It is written
  to `<data-dir>/initial-admin-password` (0600; `/app/data/initial-admin-password` in
  the image) and the log names only that file; `serve` refuses to start if the file
  cannot be written. Delete it after the first sign-in. (#48)
- After a rotation, deposits sealed to the old key are refused with 409 telling the
  product to re-pair. Products pick up a new key only by re-pairing, with the operator
  confirming the key ID.
- The database keeps keys in a new `recovery_keys` table. The existing key is copied
  into it on first start; the old `recovery_key` table is kept but no longer written.
