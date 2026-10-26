# KyRecovery standalone installation

## Before you start

These steps are for a **new, independent installation** from this repository. Use the
README’s upgrade/restore procedure for an existing instance. Keep its volumes, keys,
configuration and Compose overlays; do not replace its environment files with examples.

Use Linux with Docker Engine and Docker Compose 2.24 or newer, Git, and permission to use Docker.
Choose a release/revision and use its matching Compose files and documentation. Image tags
can move: follow the README/Compose attestation and digest-pinning procedure before relying
on a published image. Source builds use the separate build overlay where provided.

Clone this repository, enter its directory, then run the product steps below:

```sh
git clone https://github.com/Busnes-app/KyRecovery-server.git
cd KyRecovery-server
```

Each service needs its own directory and persistent storage. Several products use host
port 8080 by default; install them on separate hosts or configure distinct host ports and
matching public origins. For remote users, provide a TLS reverse proxy and a hostname you
control. Trust only the proxy’s actual peer address, and forward the original scheme/client
address according to the product’s proxy guide.

## Install the blind backup store

Docker Compose **2.24 or newer** is required by the optional `env_file` declaration.
No KyIdentity installation is needed for local operator sign-in.

```sh
[ -e .env ] || (umask 077; touch .env)
chmod 600 .env
docker compose config --quiet
docker compose pull
docker compose up -d
curl -fsS http://127.0.0.1:8095/healthz
```

Read the one-time generated `admin` password with
`docker compose exec kyrecovery cat /app/data/initial-admin-password`; it is never logged.
Sign in, change it, then delete the file. Alternatively set a generated
`KY_ADMIN_INITIAL_PASSWORD` in the protected `.env` before first start (no file is written).
Open `http://localhost:8095` on the host for a trial; use HTTPS and the exact trusted-proxy
setting for remote users. Follow the [README](README.md) for transport and account setup.

Run the [ceremony](docs/CEREMONY.md#ceremony-creating-the-suite-recovery-keypair) in a fresh private
browser window at `/admin/ceremony`: print-preview throwaway cards first, then generate the
real k-of-n set, print/distribute it, import only the public key and close the tab. Import
is one-shot; a later key replaces it only through [rotation](docs/CEREMONY.md#rotating-the-recovery-key), which re-pairs every product. Treat real cards as secrets, not screenshots
for reports. Pair one product, deposit a capsule and verify its receipt/audit entry.

`./data` persists the blind store, pairings and server keys. The entrypoint assigns it to
UID 10001 before dropping privileges. Local capsule integrity and audit checks do not
prove an off-site replica or a restore with real cards. Follow the README’s restore and
replication procedures and record those checks separately.

Source builds: retain `docker-compose.build.yml` in the `.env` `COMPOSE_FILE` chain and
use `docker compose up -d --build`. Old Compose files through release 0.4.2 have different
bind/environment behavior: read the secure-defaults upgrade section before reusing them.

## Keep it recoverable

Preserve the database **and deployment keys**, not just the image. Configure recovery using
the linked restore runbook, retain an independent copy, and complete a scratch restore with
custodian cards before treating recovery as verified. A synthetic drill uses a throwaway
key and does not prove that the real cards are available or correct.

For upgrades, back up first, read the chosen release’s migration notes, and reuse the same
Compose project, volume mounts and overlay selection for every command. `docker compose
pull` followed by `docker compose up -d` updates a published-image installation; a digest pin
must be deliberately changed after verification. A source install must keep its build
overlay and rebuild. `docker compose down -v` deletes managed volumes: it is not an upgrade.

## What verification means

The startup checks above prove only the checks they perform. Use the README’s development
section and the repository CI workflow for source/build tests; a passing health response or
screenshot is not a substitute. Record the revision, commands, environment and skips when
reporting results. Design plans and dated evidence remain records of their stated scope.

## Strong-key configuration

Apply [the supported key/TLS profile](docs/STRONG-CRYPTO.md) before rollout.
Prepare IdP/SSH endpoint migration and retain the original data/key material.
The existing Compose env_file forwards GODEBUG from the protected .env; preserve
its image/build selection and overlay chain. The selected profile refuses
SMB/NTLM and short S3 signing secrets without rewriting their records.
