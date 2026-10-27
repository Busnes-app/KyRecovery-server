# KyRecovery UI Council — Opus 5.5, round 3

Reviewer `claude-opus-5-5`, rubric `ky-suite-v2` (ky-ui@639e588, rubric sha256 `ad351662…`, contract `392afb88…`). Evidence commit `9ae739e`, captured source `675c714` (8 source pins verified), vendor ky-ui 0.4.3 (12 vendored hashes verified). Playwright 1.63 / Chromium 153, DPR1, OS light/dark, no saved theme, initial viewport only. Machine record: `opus-round3.json`.

## Inputs and freeze

- 52 PNGs in manifest-03/inventory-v3: sha256, bytes and dimensions all match; HEAD blobs match. 0 exact duplicates. Recomputed 650 same-size pairs, 0 near duplicates (closest pair 3.6% of pixels changed).
- I viewed each of the 52 originals one at a time at original resolution, plus zoomed crops of three details. I also viewed the four KyNotes `1ab04e6` brand references (hashes match the Pulse `b64ad231` index and KyNotes git) for brand cohesion only. They are not scored.
- Score vectors frozen at 2026-10-27T13:21:32Z to `/tmp/opus-r3/frozen-vectors.json` (sha256 `ac4f7bb84ada86547be061dfbec37c6de75f458cf3178eea9382c4f8679ab406`), before I opened my own round-2 report. I did not open any peer report, COUNCIL.md, CSV or XLSX. The prior report and the live check below changed no vector.

## Cohort results (KyRecovery only, not a ranking)

| Cohort | n | Quality sum | Exact mean | Fraction | Files ≥8 | Meets 8 | Short of 8 by | Ky fit mean | Round 2 mean |
|---|---|---|---|---|---|---|---|---|---|
| admin-console | 24 | 188.6 | 7.858333 | 943/120 | 12 | no | 3.4 | 8.000000 | 7.641667 |
| auth | 4 | 32.6 | 8.150000 | 163/20 | 4 | yes | 0 | 8.000000 | 8.000000 |
| settings | 24 | 188.8 | 7.866667 | 118/15 | 10 | no | 3.2 | 7.966667 | 7.800000 |

**Satisfied: false.** Exact means: admin-console 943/120, settings 118/15, auth 163/20. Only auth meets 8; the two main cohorts remain below 8 under the frozen vectors. No whole-product, populated-state, role or competitive claim.

## Per-file scores

Vectors are hierarchy, typography, density, color, affordances, coherence. Light and dark vectors are identical for every surface; dark-specific defects were looked for and none was visible.

| Surface | Size | Cohort | Vector | Total (L/D) | Ky fit | Round 2 (L/D) | Image changed? |
|---|---|---|---|---|---|---|---|
| appearance | desktop | settings | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 8.0/8.0 | no |
| appearance | mobile | settings | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 8.0/8.0 | no |
| audit | desktop | admin-console | 8,7,8,8,8,8 | 7.9/7.9 | 8.0 | 7.1/7.1 | yes |
| audit | mobile | admin-console | 8,7,7,8,7,8 | 7.5/7.5 | 8.0 | 7.0/7.0 | yes |
| backups | desktop | admin-console | 7,8,7,8,7,8 | 7.5/7.5 | 8.0 | 7.6/7.6 | yes |
| backups | mobile | admin-console | 7,8,7,8,8,8 | 7.6/7.6 | 8.0 | 7.5/7.5 | yes |
| capsules | desktop | admin-console | 8,7,8,8,8,8 | 7.9/7.9 | 8.0 | 7.9/7.7 | yes |
| capsules | mobile | admin-console | 7,7,7,8,8,8 | 7.5/7.5 | 8.0 | 7.7/7.7 | yes |
| ceremony | desktop | settings | 8,7,7,8,8,7 | 7.5/7.5 | 7.8 | 7.5/7.5 | no |
| ceremony | mobile | settings | 8,7,8,8,8,7 | 7.7/7.7 | 7.8 | 7.7/7.7 | no |
| custodian-form | desktop | settings | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 7.9/7.9 | yes |
| custodian-form | mobile | settings | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 7.5/7.5 | yes |
| custodians | desktop | settings | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 8.0/8.0 | no |
| custodians | mobile | settings | 8,8,7,8,8,8 | 7.8/7.8 | 8.0 | 8.0/8.0 | no |
| login | desktop | auth | 9,8,8,8,8,8 | 8.2/8.2 | 8.0 | 8.0/8.0 | yes |
| login | mobile | auth | 9,7,8,8,8,8 | 8.1/8.1 | 8.0 | 8.0/8.0 | yes |
| overview | desktop | admin-console | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 7.8/7.8 | yes |
| overview | mobile | admin-console | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 7.8/7.8 | yes |
| pairing | desktop | admin-console | 9,8,8,8,8,8 | 8.2/8.2 | 8.0 | 8.0/8.0 | no |
| pairing-form | desktop | admin-console | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 7.9/7.9 | no |
| pairing-form | mobile | admin-console | 8,8,8,8,8,8 | 8.0/8.0 | 8.0 | 7.5/7.5 | yes |
| pairing | mobile | admin-console | 9,8,8,8,8,8 | 8.2/8.2 | 8.0 | 8.0/8.0 | no |
| replication | desktop | settings | 8,7,8,8,8,8 | 7.9/7.9 | 8.0 | 8.0/8.0 | no |
| replication-form | desktop | settings | 8,8,8,8,8,7 | 7.9/7.9 | 8.0 | 7.5/7.5 | yes |
| replication-form | mobile | settings | 8,8,7,8,8,7 | 7.7/7.7 | 8.0 | 7.5/7.5 | yes |
| replication | mobile | settings | 8,7,8,8,8,8 | 7.9/7.9 | 8.0 | 8.0/8.0 | no |

Per-file observations, caveats, hashes and brand vectors are in the JSON.

## Changes since round 2

- 22 of 52 originals are effectively unchanged (<0.1% of pixels changed, max channel delta ≤8). Their mean delta is 0.0; any per-file movement there is reviewer variance, not a product change. The other 30 have a mean delta of +0.2.
- Reviewer variance on unchanged originals: pairing +0.2 (x4, hierarchy 9 for the clean header-primary/empty-state pairing); custodians mobile -0.2 (x2, now penalising the 7px h2/button misalignment, live-measured); replication -0.1 (x4, now penalising the .kycap gap that round 2 noted but did not deduct); pairing-form desktop +0.1 (x2, typography 8 on a <0.02%-pixel change); appearance ky_fit 8.2 -> 8.0 (x4, suite_cohesion 9 -> 8). Ceremony, custodians desktop and appearance totals unchanged.
- Implementation: audit +0.8 desktop / +0.5 mobile (rows no longer wrap; phone still horizontal-scroll); overview +0.2 (key/custodians primary first when no key); custodian-form/pairing-form mobile +0.5 (footer labels one line, 40px); login +0.2 desktop / +0.1 mobile (helper copy; phone deducts the wrapped mono token).
- Minimal image change, mostly re-read: replication-form desktop +0.4 and custodian-form desktop +0.1 changed <0.08% of pixels (replication-form mobile +0.2 at 0.7%); I credit grouping and footer more than round 2 did, cause correction rather than product change.
- Backups desktop -0.1 (x2): the notes rhythm fix landed, but I now deduct affordances for the blank Service select on an instance with no services; mostly a re-read, cause correction. Capsules mobile -0.2 (x2): hierarchy deduction for no visible primary; image change is 0.25% pixels; cause correction. Capsules desktop dark +0.2: round 2 scored light/dark differently for a theme-neutral defect; now equal.

## Round-2 findings

- **Resolved:** R2-1 phone dialog footer labels: one line, 40px (image; live 40px at 390); R2-2 desktop audit ID/date wrap: nowrap, one-line row (image); R2-3 Overview unconfigured primary: View custodians & quorum is the only filled primary, first row (image; live); R2-4 Backups notes: 8px rhythm, mono tokens unbroken (image); Bootstrap helper copy: now names the configured password / initial-admin-password file; Dialog keyboard (round-2 gate fail): native modal dialogs, focus inside on open, Escape closes, focus returns to trigger (live).
- **Partly resolved:** Required labels: 8 of 9 HTML-required dialog fields marked; repl-endpoint is required but its label is not (index.html:534; app.js:843-857 rewrites the label per preset without the marker).
- **Still open, optional:** Title Case labels/buttons; ceremony input widths, bold run-ins, trailing divider, header surface; capsules empty state without action; audit phone right border; appearance h1-to-lede gap; .kycap gap; upstream accent/danger hue and mono font.

## Required blockers (concrete, visible or measured)

1. **backups (x4)**: Restore form on an unconfigured instance shows a blank Service select with no placeholder or no-services option; its filled primary sits below the 900px fold. Fix: With no services, show 'No services to restore yet' (disabled select option or replace the form with one line); keep the primary in view or omit it.
2. **capsules (x4, mostly mobile)**: Six-line (desktop) / nine-line (phone) 13px retention explainer; on phone it pushes fields and the primary below the fold. Fix: One-sentence summary plus the example in a disclosure or helper text under the fields.
3. **audit mobile (x2)**: Horizontal-scroll table hides Actor, Target, Hash and Date at 390px. Fix: Stack each ledger row (seq/action line, actor/time line) below the breakpoint; drop the stray right border.
4. **backups, custodians mobile (x4)**: Section h2 sits 7px above the centre of its right-hand button (live-measured). Fix: align-items: center on the section header row.
5. **replication-form (x4) + contract**: Required 'S3 Endpoint URL' unmarked; native select chevron differs from the custom select used on Backups. Fix: Append '(required)' in the static label and every preset label rewrite; give the preset select the shared select style.
6. **replication (x4)**: Visible gap in 'encrypted .kycap' from the inline code box. Fix: Remove inline code padding/margin in prose, or drop the code element for this token.
7. **ceremony (x4)**: Trailing divider closes on empty space; header surface differs from login. Fix: Drop the trailing rule (or render it only above generated content); use one header surface for signed-out and ceremony.

admin-console needs +3.4 summed points and settings +3.2 to reach an exact 8.0 mean under these vectors; items 1-3 address admin-console, items 4-7 plus ceremony taste address settings. Fixing items does not raise frozen scores; a new capture and round are required.

**Optional taste:**

- Ceremony: content-width k/n inputs, fewer bold run-ins (part of settings deficit, not a blocker by itself).
- Sentence case for Title Case labels/buttons.
- Login phone: nowrap for the initial-admin-password token (styles.css:443 scopes nowrap to Backups notes only).
- Appearance h1-to-lede gap.
- Capsules empty state: add the pairing action as Backups does.
- Overview: matching metric style for the audit status.
- Upstream ky-ui: dark accent/danger hue proximity; mono stack lacks IBM Plex Mono.

## Behavior gates

| Gate | Result | Evidence |
|---|---|---|
| keyboard | untested | Live (own incognito tab, fixture 127.0.0.1:45235, source 675c714): pairing dialog opened as :modal with focus on Close; Shift+Tab from the first control left the document (document.hasFocus()==false, native browser-chrome boundary, not a background control); Tab returned to Close; elementFromPoint on three background controls hit the dialog/backdrop (inert); Escape closed and focus returned to New pairing code with :focus-visible. Replication Cancel returned focus to Add target. Password account dialog Escape returned focus to Password. Parent reports 20 real-server browser tests. Not every task path (nav, forms, picker) was walked by keyboard, so the gate is not passed. No failure observed. |
| focus | untested | Sample: dialog Close button 2px solid rgb(191,63,24) outline, 2px offset, :focus-visible (light). Not swept per element or in dark. |
| contrast | untested | No computed contrast sweep. |
| states | untested | Empty states only; loading/error states not captured. |
| responsive | untested | Partial pass: at 390x844 all eight tabs scrollWidth==390 with one h1 each; at 1280x900 Overview scrollWidth==1280, one 240px nav. Not a full path sweep. |
| required_labels_contract | fail | Live DOM: 9 HTML-required dialog inputs, 8 labels marked; repl-endpoint label 'S3 Endpoint URL' unmarked. Source 675c714 internal/server/static/index.html:534, js/app.js:843-857. |

## Live supplement (after freeze)

T3 preview, own Incognito tab tab_32_2e419d46-... opened with reuseExistingTab:false, closed afterwards; no other tab driven. Chrome 154, DPR2, freeform 1280x900 then 390x844, system appearance (light); differs from Chromium 153/DPR1 captures, so it changes no score. Signed in as fixture admin (password read privately); clicked nav items; opened and closed the pairing, replication and change-password dialogs without submitting; no theme saved (kyrecovery-theme null), no data change, no key/share generation or import.

- One nav, 240px; Overview h1 and single filled primary View custodians & quorum.
- Seven native dialogs each labelled by its title.
- Pairing/password dialogs: Escape and focus return pass; pairing footer buttons 40px.
- repl-endpoint required but unmarked.
- Phone: no horizontal overflow on eight tabs; h2/button centre offset -7px on Backups and Custodians.

## Limitations

- MySlop: credential returned 401 per parent; nothing posted or retried.
- Initial-viewport, empty/sparse evidence only; no competitor captures.
- Cohort assignment follows manifest-03 (as in round 2) for matched comparison.
