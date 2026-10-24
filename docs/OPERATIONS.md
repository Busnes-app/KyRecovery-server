# Operations

[Product overview](../README.md) · [Standalone installation](../INSTALL.md)

Commands and unlinked file paths below are relative to the repository root.

## CLI

```
kyrecovery serve      Start the dashboard and REST API
kyrecovery audit      Inspect or verify the audit chain
kyrecovery pair       generate | list | claim — paired products and 6-digit codes
kyrecovery help
```

`serve` flags: `--port` (8095), `--data-dir` (`./data`), `--sso-issuer`,
`--sso-client-id`, `--sso-client-secret`, `--sso-redirect-url`,
`--sso-admin-email`, `--cookie-secure`, `--restic-dir`.

In the container, run the CLI as the runtime user so anything it writes under
`/app/data` stays owned by `kyrecovery` (`docker exec` otherwise runs as root).
`audit` and `pair` default `--data-dir` to `KYRECOVERY_DATA_DIR`, else `./data`;
the image sets `/app/data`:

```bash
docker exec -u kyrecovery kyrecovery-server /app/kyrecovery pair generate --service kynotes --app "KyNotes"
```

`pair generate` prints the new code once; treat its output as secret. The store
keeps only a keyed hash of the code, so `pair list` shows each row's status and
never a code: generate a new one if a code is lost.

## Configuration

| Variable | Effect |
| :--- | :--- |
| `KYRECOVERY_SECRET_KEY` | The keyring master key, hex or base64. Falls back to `<data-dir>/secret.key` (0600), created on first start. |
| `KY_SSO_ISSUER`, `KY_SSO_CLIENT_ID`, `KY_SSO_CLIENT_SECRET`, `KY_SSO_REDIRECT_URL` | KySignOn OIDC defaults for the matching `serve` flags. SSO is enabled when an issuer is set. |
| `KY_ADMIN_EMAIL` | The SSO identity granted `admin`. |
| `KY_ADMIN_INITIAL_PASSWORD` | The local `admin` password on first start, never printed. Without it one is generated into `<data-dir>/initial-admin-password` (0600; `/app/data/initial-admin-password` in the image) and only that path is logged; startup fails if the file cannot be written. Delete the file after the first sign-in. |
| `KYRECOVERY_COOKIE_SECURE` | `true`, `false`, or empty to follow the request transport. |
| `KYRECOVERY_RESTIC_DIR` | rest-server's data directory, mounted read-only, for restic backup status. Empty disables the scanner. |
| `KYRECOVERY_TRUSTED_PROXIES` | Comma-separated peer IPs or CIDRs whose forwarded client address and scheme are trusted. Empty by default; `/0` is refused. |
| `KYRECOVERY_DATA_DIR` | Default `--data-dir` for `serve`, `audit` and `pair`, and the data directory `apply-setup` uses. The image sets `/app/data`, and the compose file pins it there, since it must match `serve --data-dir`. |
| `KYRECOVERY_BIND` | Compose only: the host address port 8095 is published on. Default `127.0.0.1`. |

`docker-compose.yml` passes `.env` to the container; the restic overlay sets
`KYRECOVERY_RESTIC_DIR` with its mount.

The data directory holds `kyrecovery.db`, `secret.key` and `capsules/`. Back all
three up together: without `secret.key` the sealed replication credentials, the
SSO client secret and the audit chain key are unrecoverable. The `.kycap` files
are not readable with it — they need the custodians' shares.

## Verify

Use the Go version in `go.mod`. These are source checks, separate from installed-service
acceptance and real custodian-card recovery:

```bash
go build -o kyrecovery cmd/kyrecovery/main.go
go vet ./...
go test -race -count=1 ./...
./kyrecovery help
scripts/build-wasm.sh
git diff --exit-code -- internal/server/static/wasm
node scripts/test-wasm.mjs
node scripts/test-overview.mjs
```

The WASM check needs the build prerequisites in `scripts/build-wasm.sh`; Node runs the
ceremony module vectors. Run `scripts/verify-restic-overlay.sh` separately with Docker for
the optional restic overlay. Successful vectors and synthetic drills do not prove printed
cards, off-site replication or an operator-observed restore.

## Upgrading to the secure compose defaults

Releases after 0.4.2 change three defaults:

- **Port.** `docker-compose.yml` publishes 8095 on `127.0.0.1` instead of every
  interface. If something on another host reaches KyRecovery directly, set
  `KYRECOVERY_BIND=0.0.0.0` in `.env` to restore the old behaviour, or the one
  address it should listen on.
- **Settings.** The compose file now passes `.env` to the container; it used to
  pass nothing. A value left in `.env` that did nothing before takes effect on the
  next `docker compose up -d`, so review `.env` first. An override file that
  supplied settings (`env_file:` or `environment:`) keeps working and wins over
  `.env`.
- **Output.** A supplied `KY_ADMIN_INITIAL_PASSWORD` is no longer printed, and
  `pair list` no longer prints codes of claimed, revoked or expired pairings.

## Installer setup

KyQuickStart configures exact administrator subjects through container-local
`apply-setup` as the `kyrecovery` runtime user. See [INSTALLER_SETUP.md](INSTALLER_SETUP.md)
for protected bundles, live read-only inspection, signed offboarding and remaining gates.
`KYRECOVERY_DATA_DIR` must match `serve --data-dir` (image default `/app/data`).
`KYRECOVERY_TRUSTED_PROXIES` accepts comma-separated peer IPs or narrow CIDRs, defaults
to empty and refuses `/0`. The browser custodian ceremony remains an administrator step.
