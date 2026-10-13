# Installer setup

KyQuickStart uses container-local `/app/kyrecovery apply-setup` as the `kyrecovery`
runtime user. `--capabilities` reads no configuration or credentials. `--file`
requires a protected absolute common `ky.product-setup/1` bundle; `--check` only
inspects the live server through an owner-only private Unix socket. `--restart`
is unsupported because setup activates live. The CLI uses `KYRECOVERY_DATA_DIR`
(default `./data`, `/app/data` in the image), which must match the server's
`--data-dir`. Checks never initialize SQLite, mint keys, audit or contact providers.

Use service `kyrecovery`, a stable setup UUID, canonical HTTPS issuer, client ID,
protected client-secret and directory-secret files, and an HTTPS callback ending
`/api/auth/callback`. Admins are explicit issuer/subject/username records. Recovery
is an administrative service; everyday identity assignments are refused, as are
independent SCIM tokens and product backup-enrollment requests. Existing native
SSO and local account-ID collisions are refused before committing any grants.
Matching reruns make no changes; revoked or missing retained grants require explicit
operator resolution and are never repaired by setup.

Owned configuration, exact administrator bindings, the native keyed audit event
and its anchor commit in one SQLite transaction. Authentication reads owned state
live; native SSO saves cannot overwrite it. Managed login requires a verified ID
token from the configured issuer, an assigned active subject and correct nonce;
provider-wide roles and email matches cannot grant access. Sessions retain issuer
and subject bindings and are checked against current authority. Local recovery
credentials remain independent.

`POST /api/installer/directory` uses `ky-primitives/syncauth` over a bounded bare
SCIM body and signed event type. Positive canonical `meta.version` (`W/"2"`) and
an assigned subject are required. Created/updated events require explicit active
state and `kyrecovery.admin` for administrator activation. Revisions/digests,
grants, session revocation and keyed audit commit together. Exact retries are
no-ops; stale or conflicting states are refused. Offboarding retains stored
capsules and all existing product pairings.

`KYRECOVERY_TRUSTED_PROXIES` accepts comma-separated peer IPs or narrow CIDRs,
empty by default, `/0` refused. Forwarded client address and TLS scheme are used
only through a trusted peer. `/api/installer/network` returns three G11 fields
with no-store caching. This resolution also controls limits and secure cookies;
`KYRECOVERY_COOKIE_SECURE` retains its explicit deployment override.

Setup never generates a recovery private key, accepts custodian shares, imports a
public key or consumes pairing codes. The administrator must complete the existing
browser ceremony. Pairing and deposits retain their existing key-pin, service-name
and ceremony gates. Per-service pairing-code orchestration, central identity client/
role/directory composition and published image/library provenance remain installer
acceptance gates. Target credential files must be retained alongside the native
server configuration; offsite capsule replication is not a backup of these files.
