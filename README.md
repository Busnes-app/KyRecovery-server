# KyRecovery

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/15934/badge)](https://www.bestpractices.dev/projects/15934)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Busnes-app/KyRecovery-server/badge)](https://scorecard.dev/viewer/?uri=github.com/Busnes-app/KyRecovery-server)

**Start here:** [Standalone installation](INSTALL.md). Local operator sign-in works without KyIdentity. The browser ceremony creates the recovery key and custodian cards; the server stores sealed capsules, public-key metadata and audit evidence. Stored capsules alone do not prove that a restore or off-site custody has been tested.

KyRecovery is a self-hosted **blind store** for the Ky Suite's encrypted
backups. Products seal their own `.kycap` containers and deposit the ciphertext
here. KyRecovery holds no recovery private key, no Shamir share and no plaintext,
and there is no code path in the server that decrypts a capsule — a test,
`TestNothingInTheServerDecrypts`, fails the build if one appears.

For supported key/TLS configuration and migration, read
[Strong-key configuration](docs/STRONG-CRYPTO.md) before upgrading an existing instance.

## Quick start

```bash
v=<release version you intend to run>
mkdir kyrecovery && cd kyrecovery
curl -fsSLO "https://raw.githubusercontent.com/Busnes-app/KyRecovery-server/v$v/docker-compose.yml"
```

Pin the image: run the release verify-then-pin block at the top of that
`docker-compose.yml` in this directory. It writes `KYRECOVERY_IMAGE` into `.env`.
Releases up to 0.4.2 ship a compose file that forwards no settings and publishes
on every interface; see [Upgrading to the secure compose defaults](docs/OPERATIONS.md#upgrading-to-the-secure-compose-defaults).

Settings go in the same `.env`, which can hold secrets, so keep it mode 0600. The
compose file passes `.env` to the container whole (Docker Compose 2.24 or later);
an unset variable keeps its default. Single-quote a value that contains `$`.

```bash
chmod 600 .env
cat >> .env <<'EOF'
# KYRECOVERY_TRUSTED_PROXIES=<reverse proxy IP>
# KYRECOVERY_COOKIE_SECURE=true
# KY_ADMIN_INITIAL_PASSWORD='<password>'
EOF
docker compose up -d
curl -fsS --retry 10 --retry-all-errors --retry-delay 1 http://127.0.0.1:8095/healthz
```

The port is published on `127.0.0.1` only. Put a TLS reverse proxy on the same
host in front of it, or set `KYRECOVERY_BIND` in `.env` to the host address your
proxy reaches.

On first start without `KY_ADMIN_INITIAL_PASSWORD`, the generated `admin` password
is written to `/app/data/initial-admin-password` (0600, owned by uid 10001); the
log names only the file. Read it with
`docker compose exec kyrecovery cat /app/data/initial-admin-password`, sign in,
change the password, then delete the file. Neither password is ever logged. Run the
[ceremony](docs/CEREMONY.md#ceremony-creating-the-suite-recovery-keypair), then pair products.

The entrypoint runs as root only to `chown` `./data` to uid 10001 on each start,
then drops to that user, so `./data` ends up owned by uid 10001, not your host
account.

## What it holds

- kycap/3 containers exactly as deposited, plus the SHA-256 KyRecovery computed
  over each one at deposit.
- The manifest fields the container declares about itself: capsule ID, service
  name, app version, payload hash, threshold, total shares, recovery key ID and
  encapsulated key. They are recorded and displayed, never acted on — the one
  decision taken from a manifest is whether its recovery key ID is the current
  one.
- A hash-chained audit ledger over deposits, verifications, pairings and the key
  import, rotations and retirements, keyed outside the database.
- The suite recovery **public** keys and their IDs: the current key, and any
  previous key still needed to open retained capsules. Custodian names, paired-app
  API tokens, and the server keyring for its own secrets.

## License

KyRecovery is released under the [MIT License](LICENSE.txt).

[![OctoCounts](https://api.octocounts.com/badge/Busnes-app/KyRecovery-server/branch/main)](https://octocounts.com/github/Busnes-app/KyRecovery-server/tree/main)

## Documentation

- [Ceremony](docs/CEREMONY.md) — Ceremony: creating the suite recovery keypair, and rotating it. <a id="ceremony-creating-the-suite-recovery-keypair"></a>
- [Deposits](docs/DEPOSITS.md) — Pairing and deposit. <a id="pairing-and-deposit"></a>
- [Integrity and audit](docs/INTEGRITY-AND-AUDIT.md) — Integrity, Audit ledger. <a id="integrity"></a><a id="audit-ledger"></a>
- [Retention](docs/RETENTION.md) — Backup retention. <a id="backup-retention"></a>
- [Recovery and security](docs/RECOVERY-AND-SECURITY.md) — Restore, Security model, Upgrading from the previous KyRecovery. <a id="restore"></a><a id="security-model"></a><a id="upgrading-from-the-previous-kyrecovery"></a>
- [Backup status](docs/BACKUP-STATUS.md) — Backup status and restore planning. <a id="backup-status-and-restore-planning"></a>
- [Operations](docs/OPERATIONS.md) — CLI, Configuration, Verify, Upgrading to the secure compose defaults, and related procedures. <a id="cli"></a><a id="configuration"></a><a id="verify"></a><a id="upgrading-to-the-secure-compose-defaults"></a><a id="installer-setup"></a>
- [Verifying releases](docs/VERIFYING-RELEASES.md) — Check a release image's signature and source commit, and rebuild its binary to the same bytes.
- [Assurance case](docs/ASSURANCE-CASE.md) — Threat model, trust boundaries, and why the security requirements are met.
- [Contributing](CONTRIBUTING.md) — How to report bugs, propose changes, the test policy and the coding standard.
