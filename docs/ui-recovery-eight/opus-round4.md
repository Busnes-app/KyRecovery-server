# KyRecovery UI Council — Opus 5.5, round 4

Reviewer `claude-opus-5-5`. Rubric `ky-suite-v2` from ky-ui@639e588 (rubric sha256 `ad351662…`, contract `392afb88…`, skill `6010ea39…`; the local skill copies are hash-equal). Evidence commit `ecece91`. Captured source `2997f64`, which is UI implementation `100b682` plus `.gitattributes` and AGENTS lines only. All 8 source pins verified. Vendor ky-ui 0.4.3 passes `check-vendor.mjs`. Capture conditions: Playwright 1.63 / Chromium 153, DPR1, normal service workers, OS light/dark, no saved theme, initial viewport only. The machine-readable record is `opus-round4.json`.

## Inputs and freeze

- The 52 PNGs match manifest-04 and inventory-v4 on sha256, bytes and PNG dimensions, and they equal the `ecece91` blobs. 0 exact duplicates. I recomputed the 650 same-size pairs: 0 near duplicates, and the closest pair differs in 3.6% of pixels.
- I viewed every original on its own, at original resolution. I used no contact sheets or thumbnails.
- I also viewed the four KyNotes `1ab04e6` brand references. Their hashes match the Pulse `b64ad231` index and the KyNotes git objects. I used them for suite cohesion only. They are not scored.
- I froze my vectors at 2026-10-27T14:05:53Z in `/tmp/opus-r4-scripts/frozen_vectors.py` (sha256 `5d372f96bb7298f53f0c240e37b2d51fe9ae1debdcc9718edeb36343b113787f`). Only after that did I open my own round-3 report.
- I did not open any peer report, COUNCIL.md, CSV or XLSX.
- The prior report and the live check changed no vector.

## Cohort results (KyRecovery only, not a ranking)

| Cohort | n | Quality sum | Exact mean | Fraction | Files ≥8 | Meets 8 | Short by | Ky fit mean | Round 3 mean |
|---|---|---|---|---|---|---|---|---|---|
| admin-console | 24 | 191.6 | 7.983333 | 479/60 | 22 | no | 0.4 | 8.200000 | 7.858333 |
| settings | 24 | 192.4 | 8.016667 | 481/60 | 20 | yes | 0 | 8.266667 | 7.866667 |
| auth | 4 | 33.6 | 8.400000 | 42/5 | 4 | yes | 0 | 8.400000 | 8.150000 |

**Satisfied: false.** settings and auth meet 8. admin-console misses by 0.4 summed points.

That whole shortfall comes from one deduction: density 7 on overview-mobile (both themes). That image is pixel-identical to round 3, where I scored it 8.0. So the miss comes from reviewer variance on an unchanged image, not from a regression. I keep the frozen vector because the defect is visible and live-measured.

This report makes no claim about the whole product, populated states, roles or competitive standing.

## Per-file scores

Vector order: hierarchy, typography, density, color, affordances, coherence. Light and dark vectors are identical for every surface. I looked for dark-specific defects and found none.

| Surface | Size | Cohort | Vector | Total (L/D) | Ky fit | Round 3 | Image changed? |
|---|---|---|---|---|---|---|---|
| overview | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 8.0 | no |
| overview | mobile | admin-console | 8,8,7,8,8,8 | 7.8 | 8.2 | 8.0 | no |
| backups | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.5 | yes (2.9%) |
| backups | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.6 | yes (0.6%) |
| capsules | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.9 | yes (10%) |
| capsules | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.5 | yes (27%) |
| pairing | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 8.2 | no |
| pairing | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 8.2 | yes (8%) |
| pairing-form | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 8.0 | no |
| pairing-form | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 8.0 | yes (0.4%) |
| audit | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.9 | yes (0.5%) |
| audit | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.5 | yes (4%) |
| appearance | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| appearance | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| ceremony | desktop | settings | 8,8,8,9,8,8 | 8.2 | 8.4 | 7.5 | yes (10%) |
| ceremony | mobile | settings | 8,8,8,9,8,8 | 8.2 | 8.4 | 7.7 | yes (18%) |
| custodians | desktop | settings | 8,8,8,8,8,7 | 7.9 | 8.2 | 8.0 | no |
| custodians | mobile | settings | 8,8,8,8,8,7 | 7.9 | 8.2 | 7.8 | yes (0.3%) |
| custodian-form | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.2 | 8.0 | no |
| custodian-form | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.2 | 8.0 | yes (0.05%, Δ9) |
| replication | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.9 | no |
| replication | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.9 | yes (10%) |
| replication-form | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.9 | yes (2.3%) |
| replication-form | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.2 | 7.7 | yes (5.7%) |
| login | desktop | auth | 9,8,8,9,8,8 | 8.4 | 8.4 | 8.2 | no |
| login | mobile | auth | 9,8,8,9,8,8 | 8.4 | 8.4 | 8.1 | yes (4.7%) |

Brand vectors (palette, type/geometry, shell, accent, cohesion):

| Surfaces | Vector |
|---|---|
| console | 9,8,8,8,8 |
| appearance | 9,8,8,8,9 |
| ceremony and login | 9,8,8,9,8 |

suite_cohesion is scored against the KyNotes references. Per-file observations, caveats and hashes are in the JSON.

## Changes since round 3

- **Unchanged images (20 of 52):** less than 0.1% of pixels changed and max channel delta ≤8. Their mean delta is −0.02 (−1/50). This is reviewer variance:

  | Surface | Delta | Reason |
  |---|---|---|
  | overview mobile | −0.2 | Density: I now deduct the 2+1 facts grid. |
  | pairing desktop | −0.2 | Hierarchy 9→8: the "Pairing." run-in restates the title. |
  | custodians desktop | −0.1 | Coherence: the "+ Register Custodian" button style. |
  | login desktop | +0.2 | Color 9: orange only on Sign In. |
  | replication desktop | +0.1 | The ".kycap" padding finding is withdrawn, per the parent's computed-style audit. |

  Appearance, overview desktop, custodian-form desktop and pairing-form desktop are unchanged.

- **Changed images (32):** mean delta +0.25 (1/4). These deltas come from the implementation:

  | Surface | Delta | What changed |
  |---|---|---|
  | backups | +0.5 desktop / +0.4 phone | Truthful Restore empty state; centred section rows. |
  | capsules | +0.1 desktop / +0.5 phone | Save retention at its h2 and in view; policy moved into a disclosure. |
  | audit | +0.1 desktop / +0.5 phone | Labelled phone entries with the full hash. |
  | ceremony | +0.7 desktop / +0.5 phone | Header surface matches sign-in, trailing rule removed, content-sized inputs. |
  | replication-form | +0.1 desktop / +0.3 phone | Required marker and shared select style. |
  | login phone | +0.3 | Filename token no longer breaks. Color 9 is variance. |

  Pairing mobile −0.2 changed 8% of pixels, but that delta is the same hierarchy re-reading as desktop. Custodians mobile +0.1: the alignment fix (+density) is offset by the coherence deduction.

- **Ky fit:** up from about 8.0 to 8.2–8.4. Palette 8→9 everywhere, and accent 9 on ceremony and login. This is reviewer variance: round 3 named no palette defect. It is not product improvement.

## Round-3 findings

- **Resolved (images plus live check):**
  - Backups Restore now shows "No services to restore yet." and the controls are hidden.
  - Capsules: Save retention sits at its h2 and is visible on phone; the policy is in a native disclosure.
  - The audit phone entry is flat and labelled, with no horizontal scroll.
  - Phone section rows are centred (live dy 0).
  - "S3 Endpoint URL (required)" is marked, and the preset select uses the shared style.
  - Ceremony: header surface, trailing rule and input widths are fixed.
  - The login token no longer breaks.
- **Withdrawn:** the ".kycap" inline-code padding mechanism. Computed padding and margin are 0. What remains is the monospace glyph advance, which I do not deduct.

## Required blockers

1. **overview mobile (x2, admin-console).**
   - **Effect:** At 390px the facts become two 179px cells with four-line wraps, plus a full-width third cell. The third cell's value is 17px/600 text beside 28px/400 numerals (live: grid `179px 179px`).
   - **Fix:** Stack the three facts as equal single-column rows, or give all three one cell width and one value style.
   - **Why it counts:** This one item is admin-console's entire 0.4 deficit. The image is unchanged since round 3. A new capture and a new round would be needed for any change to count.

Nothing else blocks a cohort under these vectors.

**Minor, not blocking (settings passes by 0.4 summed points):**

- Custodians: "+ Register Custodian" is the only "+"-prefixed Title Case button, and it sits beside a sentence-case primary. The h1 says "quorum" while the nav says "Quorum".
- Replication form: the Credentials heading sits 16px under the Region field, which is the same as a field gap.
- Phone page headers: the h1 sits 5px above its button's centre on four tabs (live).

**Optional taste (not behavior requirements):**

- Sentence case for the Title Case buttons and labels.
- A pairing action in the Capsules empty state; Compare versions is shown on an empty list.
- Double parentheticals in labels, e.g. "(days) (required)".
- Fewer ceremony bold run-ins, and the rule at the paragraph's measure.
- The Appearance h1-to-lede gap.
- The warm status pill sitting beside the accent selection.
- Upstream ky-ui: field focus uses an accent border plus a 3px accent-soft shadow instead of the 2px outline, and controls render at 41px.

## Behavior gates

| Gate | Result | Evidence |
|---|---|---|
| keyboard | untested | Live, own incognito tab, fixture 127.0.0.1:35627, source 2997f64. Sign-in submitted with Enter. The replication dialog is `:modal`. Tab moved to the preset select inside it with `:focus-visible`. The background nav was not hittable. Escape closed the dialog and returned focus to Add target. The parent reports 24 browser tests. Not every path was walked; no failure observed. |
| focus | untested | Add target: 2px solid rgb(191,63,24), 2px offset. Preset select: accent border plus a 3px rgb(251,240,236) shadow, outline none. No sweep, no dark check, ring contrast not measured. |
| contrast | untested | No computed sweep. |
| states | untested | Only empty states were captured. |
| responsive | untested | Partial pass. No overflow on all eight tabs at 390 and 1280. One 240px Primary nav. One h1 per tab. |
| required_labels_contract | pass | Every HTML-required dashboard input has "(required)" in its label (live DOM). Ceremony k/n are marked in the images. |

## Live supplement (after freeze)

- **Setup:** T3 preview in my own incognito tab `tab_35_…`, opened with `reuseExistingTab:false` and closed afterwards. No other tab was driven. Browser: Chrome 154, DPR2, system light theme. These conditions differ from the scored Chromium 153 / DPR1 captures, so the live check changes no score.
- **Actions:** sign-in, clicking the eight nav items, opening the replication dialog and closing it with Escape. No submits, theme changes, key or share actions, data edits or sign-out. The password was read privately and is not recorded.
- **Unexplained artifact:** the DPR2 preview screenshot of the open dialog showed a mirrored ghost of the sidebar inside the blurred backdrop (`::backdrop` blur 3px). It does not appear in the frozen captures. The cause is unproven, and it is not scored.

## Limitations

- MySlop returned 401 (per the parent), so nothing was posted or retried.
- Evidence is empty or sparse states in the initial viewport only. There are no competitor, populated, role, error or loading captures.
- Cohorts follow manifest-04, so ceremony is scored in settings.
- I did not re-run the parent-reported tests or checks.
