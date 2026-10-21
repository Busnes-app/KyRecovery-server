# Changelog

## Unreleased

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
- After a rotation, deposits sealed to the old key are refused with 409 telling the
  product to re-pair. Products pick up a new key only by re-pairing, with the operator
  confirming the key ID.
- The database keeps keys in a new `recovery_keys` table. The existing key is copied
  into it on first start; the old `recovery_key` table is kept but no longer written.
