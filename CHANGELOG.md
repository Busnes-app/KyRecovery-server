# Changelog

## Unreleased

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
