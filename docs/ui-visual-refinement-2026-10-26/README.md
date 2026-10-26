# Visual refinement evidence: KyRecovery (2026-10-26)

Before/after captures and real-browser checks for the fleet visual-refinement batch
(baseline review gaps 3 and 8: phone Replication clipping, no page `h1`, three-row phone
header). Ratings belong to the independent reviewer; this record claims no score.

## Stages

| Stage | Source | What it shows |
| --- | --- | --- |
| `00-before` | clean `cd058d4` (origin/main) | baseline; 5/7 checks (no Replication `h1`, empty-state text clipped up to 271px) |
| `01-after` | clean `cf2e857` | phone header and Replication fixes; 7/7 checks. The phone-only Account button also showed on desktop |
| `02-after` | clean `36fa319` | the one confirmation recapture after that fix; 7/7 checks |

Each stage is light and dark (OS `prefers-color-scheme`, no saved choice) × 1280×900 and
390×844 CSS px, device scale 1, full-page PNGs: `overview` and `replication-via-overview`
(Offsite Replication opened by the Overview link, as in fleet review F6). Every manifest
records the source commit, a SHA-256 of uncommitted source (null for all three: each
stage ran on a clean commit; this directory is excluded), OS, Playwright/Chromium,
font load state, server, fixture, capture time, each PNG's SHA-256 and the checks.
`captures.csv` and `behaviour-checks.csv` are generated from the manifests by `results.mjs`.

Fixture: a disposable `kyrecovery serve` built from the tree on 127.0.0.1:5391, an empty
scratch vault, the bootstrap admin with a random per-run password. No ceremony, key,
products, capsules or replication, so warning and empty states are real.

## Changes

- Phones (≤680px): brand and an **Account** button on the first row, the vault status
  pill on the second, then the existing sideways tab row. Name, role, SSO pairing,
  Password and Sign Out open from Account as a native `popover` (no script); each action
  also closes it. Wider screens show the same controls inline, as before.
- Offsite Replication has its own `h1`.
- On phones an empty table hides its column headings, so the one-sentence empty state
  wraps inside the card. Tables with rows are unchanged and still scroll sideways.
- Overview introduction sits 8px under the title (was 10px).

Phone header plus tab row: 222px → 131px (check `phone: header and tab row height`).

## Checks

Per stage in `behaviour-checks.csv`: one visible `h1` on both captured views, every account
control reachable on a phone, Replication empty states inside their card, the F6 check
(an Overview link reveals its tab; page scroll and focus unchanged: 851 → 851px), and no
console or CSP errors. Required repository checks are listed in the pull request.

## Known limits

- Backups, Capsules, Paired Products, Custodians and Audit Ledger still have no page `h1`;
  only Overview, Offsite Replication and Appearance do. Not captured in this batch.
- The header popover is fixed below the header row; it does not follow the page if a phone
  user scrolls with it open.
- The driver gained a port-busy guard after stage 00: the first stage-01 attempt found
  5391 held by another process, so its server never started and nothing was written. No
  image from that attempt exists.

## Reproduce

From the repository root, with a directory holding `@playwright/test` and Chromium:
`CAPTURE_PLAYWRIGHT=/path/to/web node docs/ui-visual-refinement-2026-10-26/capture.mjs 02 after`,
then `node docs/ui-visual-refinement-2026-10-26/results.mjs`. Synthetic data only, in a
temporary directory the script deletes.
