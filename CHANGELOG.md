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

- After a rotation, deposits sealed to the old key are refused with 409 telling the
  product to re-pair. Products pick up a new key only by re-pairing, with the operator
  confirming the key ID.
- The database keeps keys in a new `recovery_keys` table. The existing key is copied
  into it on first start; the old `recovery_key` table is kept but no longer written.
