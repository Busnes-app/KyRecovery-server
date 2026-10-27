# KyRecovery Council round 2: Opus 5.5

Reviewer `claude-opus-5-5`, rubric `ky-suite-v2` (weights 0.20/0.15/0.20/0.15/0.15/0.15, half-up per image), stage `02-after`. Captured source `4cec46d`, evidence commit `2e13195`, ky-ui vendor 0.4.3. Playwright 1.63 / Chromium 153, DPR1, 1280x900 and 390x844, OS light/dark, no saved theme. Machine record: `opus-round2.json`.

All 52 originals matched `manifest-02-after.json` and `inventory-v2.json` (sha256, bytes, IHDR). Duplicate audit: 650 pairs, 0 near duplicates. Each image was viewed individually at full resolution. Scores were frozen to `/tmp/opus-r2-kyrec/scores.py` (sha256 `6fd46d37…`) before reading round 1, any peer file, or the live browser.

## Cohort results (not a ranking, no pooled score)

| Cohort | n | Quality sum | Exact mean | Min–max | Files ≥8 | ky_fit mean | ≥8 |
|---|---|---|---|---|---|---|---|
| admin-console | 24 | 183.4 | 7.641666666666666666666666667 | 7.0–8.0 | 4 | 7.9416 | no |
| auth | 4 | 32.0 | 8.0 | 8.0–8.0 | 4 | 8.0 | yes |
| settings | 24 | 187.2 | 7.8 | 7.5–8.0 | 12 | 7.9833 | no |

**Satisfied: false.** Only `auth` reaches 8.0. Coverage is the empty or sparse initial viewport only; it does not show populated-state or whole-product quality.

## Per-file scores (H T D C A Co → total; ky_fit; round-1 total → Δ)

| Surface | Variant | Scores | Total | ky_fit | R1 | Δ |
|---|---|---|---|---|---|---|
| appearance | desktop-dark | 8 8 8 8 8 8 | 8.0 | 8.2 | 7.6 | 0.4 |
| appearance | desktop-light | 8 8 8 8 8 8 | 8.0 | 8.2 | 7.6 | 0.4 |
| appearance | mobile-dark | 8 8 8 8 8 8 | 8.0 | 8.2 | 7.6 | 0.4 |
| appearance | mobile-light | 8 8 8 8 8 8 | 8.0 | 8.2 | 7.6 | 0.4 |
| audit | desktop-dark | 7 7 6 8 8 7 | 7.1 | 7.8 | 6.2 | 0.9 |
| audit | desktop-light | 7 7 6 8 8 7 | 7.1 | 7.8 | 6.2 | 0.9 |
| audit | mobile-dark | 7 7 6 8 7 7 | 7.0 | 7.8 | 5.5 | 1.5 |
| audit | mobile-light | 7 7 6 8 7 7 | 7.0 | 7.8 | 5.5 | 1.5 |
| backups | desktop-dark | 7 8 7 8 8 8 | 7.6 | 8.0 | 5.8 | 1.8 |
| backups | desktop-light | 7 8 7 8 8 8 | 7.6 | 8.0 | 5.8 | 1.8 |
| backups | mobile-dark | 7 7 7 8 8 8 | 7.5 | 8.0 | 5.8 | 1.7 |
| backups | mobile-light | 7 7 7 8 8 8 | 7.5 | 8.0 | 5.8 | 1.7 |
| capsules | desktop-dark | 8 7 8 7 8 8 | 7.7 | 7.8 | 6.0 | 1.7 |
| capsules | desktop-light | 8 7 8 8 8 8 | 7.9 | 8.0 | 6.0 | 1.9 |
| capsules | mobile-dark | 8 7 7 8 8 8 | 7.7 | 8.0 | 5.8 | 1.9 |
| capsules | mobile-light | 8 7 7 8 8 8 | 7.7 | 8.0 | 5.8 | 1.9 |
| ceremony | desktop-dark | 8 7 7 8 8 7 | 7.5 | 7.8 | 5.7 | 1.8 |
| ceremony | desktop-light | 8 7 7 8 8 7 | 7.5 | 7.8 | 5.7 | 1.8 |
| ceremony | mobile-dark | 8 7 8 8 8 7 | 7.7 | 7.8 | 5.7 | 2.0 |
| ceremony | mobile-light | 8 7 8 8 8 7 | 7.7 | 7.8 | 5.7 | 2.0 |
| custodian-form | desktop-dark | 8 7 8 8 8 8 | 7.9 | 8.0 | 7.7 | 0.2 |
| custodian-form | desktop-light | 8 7 8 8 8 8 | 7.9 | 8.0 | 7.7 | 0.2 |
| custodian-form | mobile-dark | 8 7 7 8 7 8 | 7.5 | 7.8 | 7.7 | -0.2 |
| custodian-form | mobile-light | 8 7 7 8 7 8 | 7.5 | 7.8 | 7.7 | -0.2 |
| custodians | desktop-dark | 8 8 8 8 8 8 | 8.0 | 8.0 | 5.8 | 2.2 |
| custodians | desktop-light | 8 8 8 8 8 8 | 8.0 | 8.0 | 5.8 | 2.2 |
| custodians | mobile-dark | 8 8 8 8 8 8 | 8.0 | 8.0 | 5.5 | 2.5 |
| custodians | mobile-light | 8 8 8 8 8 8 | 8.0 | 8.0 | 5.5 | 2.5 |
| login | desktop-dark | 8 8 8 8 8 8 | 8.0 | 8.0 | 6.3 | 1.7 |
| login | desktop-light | 8 8 8 8 8 8 | 8.0 | 8.0 | 6.3 | 1.7 |
| login | mobile-dark | 8 8 8 8 8 8 | 8.0 | 8.0 | 6.7 | 1.3 |
| login | mobile-light | 8 8 8 8 8 8 | 8.0 | 8.0 | 6.7 | 1.3 |
| overview | desktop-dark | 7 8 8 8 8 8 | 7.8 | 8.0 | 6.9 | 0.9 |
| overview | desktop-light | 7 8 8 8 8 8 | 7.8 | 8.0 | 6.9 | 0.9 |
| overview | mobile-dark | 7 8 8 8 8 8 | 7.8 | 8.0 | 6.5 | 1.3 |
| overview | mobile-light | 7 8 8 8 8 8 | 7.8 | 8.0 | 6.5 | 1.3 |
| pairing | desktop-dark | 8 8 8 8 8 8 | 8.0 | 8.0 | 6.5 | 1.5 |
| pairing | desktop-light | 8 8 8 8 8 8 | 8.0 | 8.0 | 6.5 | 1.5 |
| pairing-form | desktop-dark | 8 7 8 8 8 8 | 7.9 | 8.0 | 7.7 | 0.2 |
| pairing-form | desktop-light | 8 7 8 8 8 8 | 7.9 | 8.0 | 7.7 | 0.2 |
| pairing-form | mobile-dark | 8 7 7 8 7 8 | 7.5 | 7.8 | 7.7 | -0.2 |
| pairing-form | mobile-light | 8 7 7 8 7 8 | 7.5 | 7.8 | 7.7 | -0.2 |
| pairing | mobile-dark | 8 8 8 8 8 8 | 8.0 | 8.0 | 6.9 | 1.1 |
| pairing | mobile-light | 8 8 8 8 8 8 | 8.0 | 8.0 | 6.9 | 1.1 |
| replication | desktop-dark | 8 8 8 8 8 8 | 8.0 | 8.0 | 7.0 | 1.0 |
| replication | desktop-light | 8 8 8 8 8 8 | 8.0 | 8.0 | 7.0 | 1.0 |
| replication-form | desktop-dark | 8 7 7 8 8 7 | 7.5 | 8.0 | 6.0 | 1.5 |
| replication-form | desktop-light | 8 7 7 8 8 7 | 7.5 | 8.0 | 6.0 | 1.5 |
| replication-form | mobile-dark | 8 7 7 8 8 7 | 7.5 | 8.0 | 5.6 | 1.9 |
| replication-form | mobile-light | 8 7 7 8 8 7 | 7.5 | 8.0 | 5.6 | 1.9 |
| replication | mobile-dark | 8 8 8 8 8 8 | 8.0 | 8.0 | 7.0 | 1.0 |
| replication | mobile-light | 8 8 8 8 8 8 | 8.0 | 8.0 | 7.0 | 1.0 |

Every stage-02 image is a new original, so the deltas reflect the implementation, not rubric drift. No unchanged original was rescored. Per-dimension observations are in the JSON.

## Drift notes

- appearance +0.4: picker unchanged but now spans the content column inside the new shell; the ~40px h1-to-lede gap is unchanged, so part of +0.4 is reviewer variance on unchanged picker content.
- pairing-form/custodian-form mobile -0.2: actual regression. Stage 00 kept both footer buttons on one line (viewed 00-before-pairing-form-mobile-light after freeze); stage 02 styles.css:432 '.modal-footer .btn { flex: 1; }' wraps the primary to two lines at 52px.
- custodians +2.2/+2.5, ceremony +1.8/+2.0, backups/capsules +1.7..+1.9, replication-form +1.5/+1.9, login +1.3/+1.7: shell, h1, single primary, dialog footer, grouped fields, branded ceremony and clean signed-out header implemented as described.
- Pre-freeze exposure: while grepping COUNCIL.md for numbers before scoring, round-1 cohort means for both reviewers were briefly seen. Per-file prior numbers were not seen before freeze.

## Round-1 findings

Resolved: R1 sidebar (240px, one nav, account bottom-pinned; live 8 rows 208x40, 8px); R2 ceremony dropdown/brand/back (0 selects, Back to console href '/'); R3 h1 on all eight tabs (live); R4 one custodian primary (live); R5 replication footer visible, Destination/Credentials groups (checkbox not in initial viewport, accent not re-verified); R6 signed-out header clean; R7 purge danger outline; R8 audit phone region tabindex=0 role=region aria-label + hint; R9 40px/8px controls (live, all visible buttons); R10 restic note once, empty state has action (partial: '--' restic storage still shown).

Still open (optional): overview mixed metric value styles; appearance h1-to-lede gap; audit row typography; stray space before '.kycap'; custodian dialog title/verb mismatch.

## Required fixes (visible defects or regressions)

1. **pairing-form, custodian-form mobile (x4)** (regression). Equal-width footer buttons wrap 'Generate Pairing Code'/'Register Custodian' to two lines; live 52px tall at 390px. Fix: Keep footer labels on one line (content-sized or full-width stacked buttons) at 40px. Source: `internal/server/static/css/styles.css:432`.
2. **audit desktop (x2) and row scaling** (visible defect). With 976px of table, 'usr-admin-001' and the timestamp wrap; live row height 71px. Fix: nowrap identifier/timestamp cells and size columns to content. Source: `audit table column sizing`.
3. **overview (x4)** (visible defect). In the unconfigured state the only filled primary is 'Review backups' while the visible next step (no ceremony) is not emphasised. Fix: When no recovery key exists, make the ceremony/custodians action the primary; keep Review backups secondary. Source: `overview recovery-path rows`.
4. **backups (x4)** (visible defect). Uneven paragraph gaps around the restic notes; '--restic-dir' breaks mid-token on phone. Fix: One 8px-scale gap between notes; nowrap inline code tokens. Source: `backups health notes`.

## Optional taste

- Sentence case for Title Case labels/buttons ('Generate Pairing Code', 'Register Custodian', 'Save Target', 'Verify Cryptographic Chain').
- Ceremony: size k/n inputs to content and drop the trailing empty divider; reduce bold run-ins.
- Header surface: login (page) vs ceremony (panel) differ.
- Replication form: more space or a rule before 'Credentials'.
- Capsules empty state: add the pairing action as Backups does.
- Audit phone region: drop the 1px right border that reads as a stray line.
- Upstream ky-ui (not KyRecovery): dark --ky-danger #ff8176 vs --ky-accent #f5865f are close in hue; --ky-font-mono lacks IBM Plex Mono per contract.

## Behavior gates

| Gate | Result | Evidence |
|---|---|---|
| keyboard | fail | Live 2026-10-27 own incognito tab, source 4cec46d via fixture server 127.0.0.1:42565: real click on 'New pairing code' left focus on the trigger outside #pairing-gen-modal; Escape did not close it; Shift+Tab from the dialog returned to the background trigger (no trap); Cancel left document.activeElement=BODY (no focus return). No role=dialog/aria-modal on the overlay. |
| focus | untested | One sample: modal close button showed 2px solid rgb(191,63,24) outline, 2px offset (light). Not measured on every element or in dark. |
| contrast | untested | No computed contrast sweep. |
| states | untested | Loading/error states not captured; empty states only. |
| responsive | untested | Partial: scrollWidth == viewport at 1280 on all eight tabs and at 390 on overview/pairing/audit/ceremony; active phone tab revealed; Account is a 40px popovertarget button; 18/18 Playwright tests in checks/r2-browser.txt. Not a full 390/1280 sweep of every path. |

The 18 browser tests and the race suites passed (`checks/r2-*.txt`). They do not establish these gates.

## Live supplement (after freeze; changes no score)

Chrome 154, DPR2, freeform 1280x900 then 390x844, OS appearance emulated dark then reset to system; differs from Chromium 153/DPR1 captures, so it changes no score. Signed in with fixture admin; clicked nav items; opened/cancelled the pairing dialog unsubmitted; visited /admin/ceremony without generating; no saved theme (kyrecovery-theme stayed null), no fixture data change.

- Desktop: one <nav>, 240px wide, 8 rows 208x40 radius 8px, 14px Space Grotesk, aria-current=page on active; every visible button 40px/8px.
- Each tab has exactly one h1; filled primaries: Overview Review backups, Backups Find restore point, Capsules Save retention, Pairing New pairing code, Custodians Run recovery key ceremony, Replication Add target; Audit/Appearance none.
- Login username 'admin' is a placeholder, not a value.
- Audit mono stack is ui-monospace/SF Mono/Menlo/Consolas/Liberation Mono; IBM Plex Mono not loaded.
- Dark: Save retention bg #f5865f; Purge expired outline/text #ff8176.
- Ceremony: 0 selects, Back to console link 40px href '/', inputs 41px tall, h1 24px.
- Dialog keyboard failures as in gates.keyboard.

## Limitations

- MySlop: credential returned 401 per parent; no post attempted or retried.
- Initial-viewport, empty/sparse evidence only.
- Pre-freeze exposure: while grepping COUNCIL.md for numbers before scoring, round-1 cohort means for both reviewers were briefly seen. Per-file prior numbers were not seen before freeze.
