# KyRecovery UI Council — Opus 5.5, round 5

Reviewer `claude-opus-5-5`. Rubric `ky-suite-v2` from ky-ui@639e588 (rubric sha256 `ad351662…`, contract `392afb88…`, skill `6010ea39…`; the local skill copy is hash-equal). Evidence commit `247c10e`, captured source `b2cd2fb`. Since round 4, `b2cd2fb` changes only the phone `.overview-facts` CSS and AGENTS lines. All 8 source pins verified. Vendor ky-ui 0.4.3 passes `check-vendor.mjs`. Capture conditions: Playwright 1.63 / Chromium 153, DPR1, normal service workers, OS light/dark, no saved theme, initial viewport only. The machine-readable record is `opus-round5.json`.

## Inputs and freeze

- Before viewing anything, I checked the 52 PNGs against manifest-05, inventory-v5 and the `247c10e` blobs on sha256, bytes and PNG dimensions (26 at 1280×900, 26 at 390×844). 0 exact duplicates. I recomputed the 650 same-size pairs: 0 near duplicates, and the closest pair differs in 3.6% of pixels.
- Against stage 04:
  - 39 files are byte-identical.
  - 3 Appearance files are near-identical: under 0.01% of pixels changed, max delta ≤8.
  - Backups ×4 differ only in the "Updated" timestamp text.
  - Audit ×4 differ only in timestamp and hash text.
  - Overview phone ×2 changed (about 12% of pixels).
- I viewed every original on its own, at original resolution. I used no contact sheets or thumbnails.
- I viewed the four pinned KyNotes `1ab04e6` brand references, extracted with `git show`. Their hashes match the Pulse `b64ad231` index and my round-4 inputs. I used them for suite cohesion only.
- I froze my vectors at 2026-10-27T14:23:48Z in `/tmp/opus-r5/freeze.json` (sha256 `df102bca77508f3b575daf15b551debe8cdd979a92c41fc82de6e6d652fa1de1`). Only after that did I open my own round-4 report (`opus-round4.json` sha256 `4cb904af…`).
- I did not open any peer report, COUNCIL.md, CSV, XLSX or score-bearing verification file.
- Neither the prior report nor the live check changed any vector.

## Cohort results (KyRecovery only, not a ranking)

| Cohort | n | Quality sum | Exact mean | Fraction | Files ≥8 | Meets 8 | Short by | Ky fit mean | Round 4 mean |
|---|---|---|---|---|---|---|---|---|---|
| admin-console | 24 | 190.8 | 7.950000 | 159/20 | 20 | no | 1.2 | 8.400000 (42/5) | 7.983333 |
| settings | 24 | 192.0 | 8.000000 | 8 | 24 | yes | 0 | 8.433333 (253/30) | 8.016667 |
| auth | 4 | 32.0 | 8.000000 | 8 | 4 | yes | 0 | 8.400000 (42/5) | 8.400000 |

**Satisfied: false.** settings and auth meet 8. admin-console misses by 1.2 summed points.

The rubric's half-up display of 7.95 is 8.0. As in round 4, I apply the gate to the exact mean, and I do not switch convention to meet the target.

The whole shortfall is one deduction on Backups, all four files: hierarchy, plus density on phone. Those images differ from round 4 only in the timestamp text, and round 4 scored them 8.0. So this is reviewer variance, not a regression. I keep the frozen vector because the defect is visible and live-measured.

The round-4 required issue, phone Overview, is resolved.

This report makes no claim about the whole product, populated states, roles, accessibility or competitive standing.

## Per-file scores

Vector order: hierarchy, typography, density, color, affordances, coherence. Light and dark vectors are identical for every surface. I looked for dark-specific defects and found none.

| Surface | Size | Cohort | Vector | Total (L/D) | Ky fit | Round 4 | Image changed? |
|---|---|---|---|---|---|---|---|
| overview | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| overview | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 7.8 | yes (12%) |
| backups | desktop | admin-console | 7,8,8,8,8,8 | 7.8 | 8.4 | 8.0 | timestamp text only |
| backups | mobile | admin-console | 7,8,7,8,8,8 | 7.6 | 8.4 | 8.0 | timestamp text only |
| capsules | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| capsules | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| pairing | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| pairing | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| pairing-form | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| pairing-form | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| audit | desktop | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | timestamp/hash text only |
| audit | mobile | admin-console | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | timestamp/hash text only |
| appearance | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.6 | 8.0 | no |
| appearance | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.6 | 8.0 | no |
| ceremony | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.2 | no |
| ceremony | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.2 | no |
| custodians | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 7.9 | no |
| custodians | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 7.9 | no |
| custodian-form | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| custodian-form | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| replication | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| replication | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| replication-form | desktop | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| replication-form | mobile | settings | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.0 | no |
| login | desktop | auth | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.4 | no |
| login | mobile | auth | 8,8,8,8,8,8 | 8.0 | 8.4 | 8.4 | no |

Brand vectors (palette, type/geometry, shell, accent, cohesion):

| Surfaces | Vector |
|---|---|
| console, ceremony and login | 9,8,8,9,8 |
| appearance | 9,8,8,9,9 |

suite_cohesion is scored against the KyNotes references. The tokens, type, selection treatment and outline secondaries match. The header composition differs: Recovery puts its wordmark in the sidebar. Per-file observations, caveats and hashes are in the JSON.

## Changes since round 4

No file moved by 0.5 or more.

- **Unchanged images (42):** mean delta −1/21 (−0.048). This is reviewer variance, not regression or progress:

  | Surface | Delta | Reason |
  |---|---|---|
  | login | −0.4 | Hierarchy and color are 8, not 9. |
  | ceremony | −0.2 | Color 8, not 9. |
  | custodians | +0.1 | I no longer deduct coherence for the "+ Register Custodian" style; it remains a non-blocking note. |

- **Text-only changed images (8, backups and audit):** mean delta −3/20 (−0.15).
  - Backups: −0.2 desktop, −0.4 phone. This is the new hierarchy and phone density deduction, described under Required blockers. The only pixel change is the timestamp, so this is reviewer variance.
  - Audit: unchanged at 8.0.
- **Changed images (2, overview phone):** +0.2 (7.8→8.0). This is the product change in `b2cd2fb`.
- **Ky fit:** accent_discipline 8→9 on all surfaces. I now read the labelled warning-tint status pill as the warning token, not as an accent breach. This is reviewer variance on unchanged images.

## Round-4 findings

- **Resolved: phone Overview facts (required in round 4).** The images show three equal 358px single-column rows. Each has its label on the left, its value on the right and a full-width explanation below. All three values share one style.
  - Live, at 390×844: grid `1fr`, every `dd` 24px/600, notes 358px wide, View custodians & quorum at y 777–817, scrollWidth 390.
  - The desktop Overview images are byte-identical to stage 04.
- **Still withdrawn:** the ".kycap" padding mechanism. I do not raise it.
- **Still non-blocking, not new feature requirements:** the plus-prefix and capitalization, the Credentials spacing, the phone h1 centre offset, and the other optional taste items below.

## Required blockers

1. **Backups (×4, admin-console): the metric strip outranks the page title.**
   - **Effect on desktop:** the five metric values are 28px/400 (live). They read as large as the 24px/600 h1, so the strip carries the page's first emphasis. The page has no filled primary.
   - **Effect on phone:** the values are 24px/400, the same size as the h1. The 2-column grid leaves "Off" alone on a third row, and the empty-state metrics fill the top third of the viewport.
   - **Fix:** set the metric values below the h1, at section scale (17px, or at most 20px). On phone, remove the orphan row, for example with single-column label/value rows like the new Overview facts. No new primary, feature or copy is needed.
   - **Why it counts:** this deduction is admin-console's entire 1.2-point deficit. Round 4 scored the same pixels (timestamp aside) 8.0, so it is reviewer variance on an unchanged image, reported honestly rather than dropped. A recapture and a fresh round would be needed for any change to count.

Nothing else blocks a cohort under these vectors.

**Minor, not blocking:**

- Custodians: the "+ Register Custodian" plus-prefix in Title Case; the h1 says "quorum" while the nav says "Quorum".
- Replication form: the Credentials heading sits close under the Region field.
- Phone page headers: the h1 sits slightly above its button's centre.

**Optional taste:**

- Sentence case for the Title Case buttons and labels.
- Double parentheticals in labels, e.g. "(days) (required)".
- Fewer ceremony bold run-ins.
- The Appearance h1-to-lede gap.
- A quieter value weight for phone Overview's "Accepting entries".
- A pairing action in the Capsules empty state.

## Behavior gates

| Gate | Result | Evidence |
|---|---|---|
| keyboard | untested | Not walked this round; sign-in only, submitted with Enter. The parent reports 24 browser tests passed at `b2cd2fb`. I did not re-run them. |
| focus | untested | No sweep. |
| contrast | untested | No computed sweep. |
| states | untested | Only empty and sparse states were captured. |
| responsive | untested | Partial: live scrollWidth 390 on phone Overview and Backups, 1280 on desktop Backups. One 240px Primary nav. Not a full sweep. |

## Live supplement (after freeze)

- **Setup:** T3 preview in my own incognito tab `tab_37_…`, opened with `reuseExistingTab:false` and closed afterwards. No other tab was driven. Browser: Chrome 154, DPR2. These conditions differ from the scored Chromium 153 / DPR1 captures, so the live check changes no score.
- **Actions:** sign-in, viewing Overview, clicking Backups, and resizing between 390×844 and 1280×900. No submit, save, connection test, theme choice (`kyrecovery-theme` stayed null), key or share action, download or sign-out. The password was read privately and is not recorded.
- **Measurements:**
  - Phone Overview: as described under Round-4 findings.
  - Backups: h1 24px/600; metric values 24px/400 on phone and 28px/400 on desktop; no filled primary.

## Limitations

- MySlop was not attempted (known 401).
- Evidence is initial-viewport, empty and sparse states only. There are no competitor, populated, other-role, loading, error, full focus or contrast captures.
- Cohorts follow manifest-05, so ceremony is scored in settings, as in rounds 2–4.
- I did not re-run or read the parent's tests or `r5-results.json`.
