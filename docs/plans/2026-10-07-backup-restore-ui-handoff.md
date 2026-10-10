# Hand-off: backup and restore UI in KyRecovery

Date: 2026-10-07. Status: decisions below made 2026-10-07; designed in
`docs/superpowers/specs/2026-10-07-backup-status-restore-ui-design.md`.

## Decisions (2026-10-07)

1. **Decryption stays on the operator's workstation (option 2a).** KyRecovery guides and
   downloads; it never combines shares or opens a capsule, in the server or in pages it serves.
   Reason: combining k shares rebuilds the suite recovery private key, so a browser restore would
   let a compromised KyRecovery capture it. The decryption invariant is unchanged.
2. **rest-server runs on the KyRecovery host** as a separate container (not embedded, so restic
   upgrades do not ride KyRecovery releases). KyRecovery mounts the repository directory
   read-only for status. Host and storage must be off the NAS.
3. **Restores start from either KyRecovery's UI or the CLI; `kyquickstart restore <app>` runs
   them.** KyRecovery's UI picks product and point in time, shows the matching capsule and
   nearest restic snapshot, and gives the exact command. With the target alive, restic restores
   on the target with its own password; with the target lost, the password comes from the
   capsule opened on the workstation with custodians present.

These supersede the open questions and the 2b option below, which stay for context.

## Goal

Give the suite one place to see backup health and to run restores. KyRecovery is that place:
it is the off-NAS, independent host that already holds every product's sealed capsules, and it
already has SSO and `viewer` < `operator` < `admin` roles.

## Background

- **Small, secret data:** each product seals `.kycap` capsules (`ky-primitives/capsule`) and
  deposits them here. KyRecovery cannot open them. Today restore is the product's own `restore`
  command plus k custodian shares typed on stdin (root `AGENTS.md`, KyRecovery integration,
  step 8; this repo's README, Restore).
- **Bulk data** (photo libraries, repositories, uploads, database dumps) cannot fit a capsule
  (384 MiB cap). The KyQuickStart catalog spec puts it in restic, one repository and password per
  app, on an append-only rest-server (`--append-only --private-repos`, TLS) on a host off the
  NAS. K8up runs each app's dump command on Kubernetes; a timer runs restic on Docker hosts.
  - The restic password lives on the app's target (restic encryption is symmetric and the backup
    client needs it to write) and a copy is sealed in KyRecovery.
  - The rest-server host never holds a password.
  - Spec: `KyQuickStart/docs/superpowers/specs/2026-10-06-third-party-catalog-design.md`,
    section Backups.
- Restic has no UI. Backrest (GPL-3.0) was rejected: no OIDC, wants to run restic itself
  (conflicts with K8up schedules), and must hold every repository password.

## What the user asked for

A UI for backup and restore in KyRecovery.

## Constraints that must hold

- **Nothing in the server decrypts.** `TestNothingInTheServerDecrypts` forbids
  `recoverykey.Combine`, `capsule.Open` and friends outside `cmd/ceremony-wasm`. Do not weaken it
  to land this feature.
- **No product token gains reach.** A product token reaches only `/api/backup/deposit`.
- **KyRecovery cannot trigger a product's backup.** It holds no product credential. Backup
  schedules stay in each product's own UI (KyRecovery integration, step 7).
- **The rest-server host never holds a restic password**, and neither does any resident service
  that could read every app's history.
- **New `/api/` routes go in `apiPolicy` and `TestRequiredRolePolicy`.** Restores are `operator`
  or `admin`, audited in the hash-chained ledger.
- **Shares are never accepted in argv, logs or server requests.**

## Proposed scope (to confirm in brainstorming)

1. **Backup status, no secrets needed.**
   - Per paired product: last deposit, capsule count and age, verify result, staleness against
     its expected interval.
   - Per restic repository, if rest-server runs on this host (see open question 2): last
     snapshot time, snapshot count and repository size. All of these are readable from the
     repository's file layout (`snapshots/` entries, sizes, mtimes) without the password.
   - Stale or failed backups surface in the dashboard and alert through kyPulse.
2. **Guided restore.** Pick a product and point in time. Show the matching capsule and the
   restic snapshot nearest it (same time window, so database and files agree). Then either:
   - **2a. Guide and download only:** download the capsule, and show the exact product `restore`
     and `kyquickstart restore <app>` commands. Decryption stays on the operator's machine. No
     invariant change.
   - **2b. In-browser restore:** a second `js/wasm` module, like the ceremony, combines the typed
     shares and opens the capsule in the tab, then hands the plaintext to the product's restore
     endpoint or a local download. This needs an explicit, user-approved change to the
     decryption invariant and to "Restore is the product's", and a design for where the
     plaintext goes.

## Open questions for the user

1. **Where does decryption happen?** 2a keeps every invariant; 2b is friendlier but moves share
   handling into KyRecovery's pages.
2. **Does this host run the rest-server?** KyRecovery is already the independent off-NAS host, so
   it is the natural home. Either a separate rest-server container beside KyRecovery, or
   rest-server's Go handler embedded in the binary, with per-app repository credentials issued at
   pairing.
3. **Who runs a bulk restore?** `kyquickstart restore <app>` on the operator's machine, or a
   KyRecovery-driven flow, which would need a path to the target.

## Related work

- KyQuickStart roadmap Phase 8 (backups) and Phase 10 (upgrades) assume restic status reaches
  kyPulse; align with whatever this UI reports.
- `kyquickstart restore <app>` is not yet in the installer spec. Add it there if the user picks 2a.
