# KyRecovery Council — Sol round 2

**Satisfied: true for the covered initial-view visual 8/10 progression.** This is a judgment of the composition, independent of rounded thresholds. The single desktop navigation/account region, shared control rhythm, clear page hierarchy, restrained primaries and complete visible dialog footers now form a cohesive Busnes console. **Full behavior acceptance: false**: dialog focus containment fails and four full behavior gates remain untested. This is no whole-product 8/9, competitive win, merge or deployment claim.

Only this reviewer’s `sol-round2.md` and `sol-round2.json` were written. No implementation edits, commits, PRs, merges, deployments, delegated assessments or Gemini calls occurred. Peer reports and shared spreadsheets/tables were not touched or read.

## Exact cohort results

| Cohort | n | Quality sum | Exact quality mean | Brand sum | Exact brand mean |
| --- | ---: | ---: | --- | ---: | --- |
| admin-console | 24 | 203.0 | 203/24 = 8.4583333333 | 215.6 | 539/60 = 8.9833333333 |
| settings | 24 | 211.0 | 211/24 = 8.7916666667 | 215.6 | 539/60 = 8.9833333333 |
| auth | 4 | 34.8 | 87/10 = 8.7000000000 | 36.0 | 9 = 9.0000000000 |

No pooled cross-cohort score. All 52 unique originals count once. Quality uses integer dimensions weighted .20/.15/.20/.15/.15/.15, Decimal half-up to one decimal per image, then exact cohort sum/n. Brand uses five equal factors, kept separate. No competitor was scored; competitive comparisons are unmatched. Decimal displays are abbreviated; fractions govern the gate.

| Reviewer | Round | admin-console | settings | auth | Status |
| --- | --- | ---: | ---: | ---: | --- |
| GPT Sol | 1 | 7.2 | 7.35 | 7.5 | Baseline unchanged; not satisfied |
| GPT Sol | 2 | 203/24 | 211/24 | 87/10 | Satisfied for covered visual 8 scope |
| Claude Opus 5.5 | 2 | — | — | — | Pending/unconsulted by this reviewer |
| Gemini 4 Argon | 2 | — | — | — | Unused; wait for both independent reviewers |

Appearance is the strongest repeated vocabulary: preview/name/description/radio cards retain geometry at both sizes and in both Busnes themes. Ceremony now identifies the product and provides a return path. Backups spacing and retention prose remain long on phone but readable. Phone Capsules light is the lowest individual image at7.8; that local weakness is retained. The requested numerical gate is per cohort, and genuine visual satisfaction is stated separately.

## Remaining findings

### B1 — required_before_full_behavior_acceptance

**Location:** `internal/server/static/js/app.js:787`; Replication dialog focus management.

Opening leaves focus on background Add target; Shift+Tab from Close dialog reaches that trigger behind the modal.

**Bounded fix:** Use native dialog or equivalent focus move/containment and restoration; preserve all fields and submit behavior.

**Blocks covered visual 8:** false. **Blocks full acceptance:** true.

### C1 — required_visible_correctness_fix

**Location:** `internal/server/static/index.html:61`; Signed-out password helper.

Generated in server startup log on first launch directs first-time operators to the wrong source; pinned cmd/kyrecovery/app/app.go:327 stores generated bootstrap password in data-dir/initial-admin-password and logs its path.

**Bounded fix:** Correct the helper to the existing bootstrap-file/operator configuration behavior, without exposing a password or changing bootstrap security.

**Blocks covered visual 8:** false. **Blocks full acceptance:** false.

### F1 — minor_visible_contract_gap

**Location:** `internal/server/static/index.html:532`; Form required/optional labels.

Forms do not state required/optional status in text, although e.g. Target Name and Endpoint carry required attributes.

**Bounded fix:** Mark existing required/optional distinctions in labels while preserving validation semantics.

**Blocks covered visual 8:** false. **Blocks full acceptance:** false.

### T1 — optional_taste

**Location:** `internal/server/static/css/styles.css:269`; Backdrop and long sparse task views.

Backdrop blur is heavy; Backups spacing and retention prose can feel long on phone.

**Bounded fix:** Optional later refinement of spacing/blur; do not remove factual recovery/retention guidance merely to shorten the screen.

**Blocks covered visual 8:** false. **Blocks full acceptance:** false.

B1 is an existing observed behavior gap, not demonstrated as a new regression. C1 is existing incorrect operator guidance missed in round 1; it does not justify exposing a credential or changing bootstrap security. F1 is a minor existing form affordance. T1 remains optional taste. For an experienced operator, titles and one primary clarify the task; a keyboard operator still encounters the dialog boundary failure and a first-time administrator could waste time looking in logs. These are concrete task concerns, not measured completion-time or screen-reader claims.

## Round1 findings and comparison

| Finding | Status | Evidence |
| --- | --- | --- |
| P1 | resolved | Single 240px desktop navigation/account region visible in all desktop console originals and measured live. |
| P2 | resolved_in_covered_scope | Task/nav/dialog buttons measure 40px; phone wrapped footer actions grow to 52px rather than clip. Inputs measure 40–43px including their box model. |
| P3 | resolved | Replication footer visible at both sizes; body scrolls independently, with phone footer fixed at y706–827. |
| P4 | resolved_with_minor_density_remaining | Every current tab has an h1; Capsules table precedes retention. Phone summaries remain long, but readable and scrollable. |
| P5 | resolved | Only ceremony primary filled on Custodians; checkbox accent is rgb(191,63,24) in native light view. |
| P6 | resolved | Ceremony has stamp/wordmark and Back to console; no compact select. |
| P7 | resolved | Phone audit says Scroll to view all ledger columns; live region is tabbable and overflow-x:auto. |
| T1 | optional_retained | Blur/footer taste remains; no acceptance blocker. |
| brief_signed_out_clutter | resolved | Login originals show no Guest/LOADING account residue. |
| brief_neutral_purge | resolved | Desktop purge uses distinct destructive outline; phone purge lies below initial viewport. |
| brief_ungrouped_fields | resolved | Replication Destination/Credentials groups visible; Sync verified lower in native scrolled body. |
| brief_restic_duplicate_and_empty_guidance | resolved | Restic-off note appears once; Paired products next action visible. |
| parent_cleanup | resolved | No repeated Capsules subheading; Add target belongs to replication h1 row. |

All quality and five-factor brand vectors were frozen at **2026-10-27T12:47:29.259Z**, after viewing every unique original individually at original/full resolution and before opening earlier scores or the live browser. No contact sheets, thumbnails or peer scores were used. Prior numeric reports were first read after this freeze.

Frozen visual-row SHA-256: `8a947bd34fecf6b68820614551ff744a11cdc5b297dd0a4b4803f4448c3288d2`. Hash scope: `frozen_visual_rows`, canonical sorted compact UTF-8 JSON, `ensure_ascii=False`. Retained vectors permit independent checks that prior/live evidence did not alter scores.

All 52 source hashes differ from round 1 predecessors. Unchanged-image rubric drift is therefore **N/A**, not zero; weights, anchors and cohorts did not change. JSON records each prior file/hash/total, current total, delta and concrete implementation explanation, including every absolute delta≥0.5. These are changed-image comparisons, not measured unchanged-image rescoring, and cannot isolate implementation effects from reviewer variability.

| Cohort | Exact mean changed-image delta |
| --- | --- |
| admin-console | +151/120 |
| settings | +173/120 |
| auth | +6/5 |

Ky unchanged-original drift: N/A (0). Competitor drift: N/A (no scored competitors). Peer spreads and any ≥2 dimension reinspection belong to the parent; no peer reports were opened and disagreement was not averaged away. Prior raw reports are preserved unchanged.

## Pins, capture and coverage

- Evidence `2e1319520b587068e4da0ce1b7ecdce7d4fcf604`; captured clean runtime `4cec46dbd9a635cfbaec2247d31e310df26def61`. All eight manifest source hashes match current files and committed git objects.
- Capture 2026-10-27T12:37:23.109Z, Linux7.2.10-1-cachyos, Playwright1.63.0 / Chromium153.0.8010.12, normal service-worker behavior,1280×900 or390×844 CSSpx, DPR1, OSlight/dark, saved choice null.
- Reviewer GPT Sol / gpt-6.1-sol / high. Fixed ky-suite-v2 authority `ky-ui@639e5882cada099dd189197beb2a3e3b4c188658`; committed rubric/design/tokens read, matching the baseline authority. Runtime vendor0.4.3 is separately verified against all 12 VERSION hashes. No dirty root checkout was used as authority.
- Unconfigured admin: eight tabs, three unsubmitted open forms, initial ceremony and sign-in;13 surfaces×two sizes×two OS themes=52. Audit is sparse (real login row). Appearance/forms are configuration states, not populated product records.
- All 52 screenshot hashes, bytes, integer dimensions and inventory/manifest memberships verified. Exact duplicates0. Independently recomputed 650 same-size pairs, near duplicates0, minimum changed-pixel fraction 0.03680121527777778. JSON records every measured pair and max channel delta.
- Brand-only companion: four KyNotes originals at1ab04e64de747f409d88b98f02682e85862defe1, recorded by Pulse b64ad231f3608b184bcdd237522d27b981252844. All four local hashes verified and originals individually viewed full resolution; no Notes quality/cohort score was used.
- Missing coverage: populated capsule/product/custodian/target/transfer/backup/restic states, per-task loading/errors/validation, generated/printed/imported or rotated ceremony, other roles/SSO, all non-Busnes themes rendered across surfaces, full behavior matrix and matched competitors. Empty/sparse initial evidence cannot prove whole-product acceptance or restore readiness; absent content does not establish absent features.

| Input | SHA-256 |
| --- | --- |
| manifest-02-after.json | `3db2627f74e532ae3ffb676a2588403580340431d470111082ec01f489aa6ac4` |
| inventory-v2.json | `95932c6dca24e468fa414a0cd00ccf917296f903c0cef2882c33a77613ac1d31` |
| duplicate-audit-v2.json | `b69d10f76a9293ea2600ebd1cc49d94fbe53904f056a3183e031b34ef1fc176f` |
| sol-round1.md | `c9697b80491b2c78195f4dc705cbadb66feb476001a294a1f4219b5681b07dae` |
| sol-round1.json | `c98d50ea2c086d42692d0868c680eef7f9ba9db1015bd7b5ef4833df782c4e4b` |
| Pinned rubric | `ad3516620420fbdc30f5273f82b7c72c76e1365d1b6296fcaa56be3cbebf2c41` |
| Pinned design contract | `392afb88c51fb3cfad6c94d6b841804b194d107a0431c31f35ff81d3a7a444d3` |

## Native Computer Use — after freeze

Called preview_status then opened a new owned incognito tab, reuseExistingTab=false, always passing tabId `tab_30_2e419d46-7b07-427f-bf30-1831131b4cba`. Only that tab was driven and closed. Other/parent tabs and the retained fixture server were left alone. No screenshot files or new scoring captures were written.

Native Chrome154 / Linux / DPR2 differs from the scored Chromium153 / DPR1. Set1280×900 then390×844. One fixture login created a normal session/audit event; no password appears in the report pair.

- Pointer-visited all eight tabs at both sizes: one current h1, page overflow0, active phone destination revealed. Desktop navigation/header width240px; account region at bottom. Shared task/nav/dialog buttons measured40px. Text fields40–43px by box model; wrapped phone footer buttons52px rather than clipped labels.
- Replication desktop body client720/scroll883px, footer y810–883. Phone body client616/scroll887, footer y706–827. Scrolling body to270.5px exposed the lower Sync checkbox while footer stayed fixed. Checkbox computed accent-color rgb(191,63,24).
- Three phone forms opened/closed without submission; Account native popover opened/closed. No pairing generation, custodian registration, connection test, Save, purge, key/share generation or import occurred.
- Phone audit region role=region, label=Audit ledger columns, tabindex0, client357/scroll866px, overflow-x:auto, page overflow0. Hint visible in both phone originals. Actual full keyboard horizontal scrolling was not established.
- Appearance23 native radios; saved choice stayed null, no option selected. Initial ceremony brand/back links and no select; page overflow0 at390px. Immediately after OS dark emulation, prefers-dark=true but data-theme remained light; no settled wait/reload occurred, so no live theme-following pass or regression is claimed.
- Prelogin background polling produced401s; observed authenticated navigation requests succeeded. This limited trace is no global error-free claim. Resize-mode, missing-type-locator and unsupported-role-attribute caller mistakes were corrected; these were not product failures.

## Behavior gates

| Gate | Status | Evidence / limit |
| --- | --- | --- |
| keyboard | **fail** | Native owned incognito tab at 390×844: Add target -> Tab focuses Close dialog -> Shift+Tab focuses background Add target, insideDialog=false while modal display=flex. Opening also leaves focus on trigger. Modal has no dialog role. See native_supplement.keyboard_focus_escape. One concrete failure is enough to fail; not an exhaustive keyboard audit. Pre-existing modal behavior, not demonstrated as a round2 regression. |
| focus | **untested** | Close dialog received a visible/computed rgb(191,63,24) solid 2px outline in the light native browser. Every interactive control in both themes was not exercised.  |
| contrast | **untested** | No complete computed text/control/focus/state contrast matrix across the 23 shipped theme choices was run; apparent screenshot contrast is not a pass.  |
| states | **untested** | Manifest02 captures empty/unconfigured screens, sparse login audit and open forms. Per-task loading/error and announced inline validation paths are missing; no form submission was authorized.  |
| responsive | **untested** | All eight tabs were pointer-navigated at 1280×900 and 390×844 with page overflow=0; active phone nav entries were revealed. Three phone forms fit; replication body scroll/footer and ceremony width checked. Limited light native supplement and frozen Busnes images do not exercise every shipped theme, task state, target and responsive interaction. |

All gate evidence is tied to captured source 4cec46d. Parent reports 18 browser tests and ordinary race suite passing; reviewer did not rerun them or treat them as full gate evidence. Full/configured/Samba/crypto/restore repository checks were not run for this review-only report pair.

## Per-original frozen results

H/T/D/C/A/O: hierarchy, typography, density/layout, color, visible affordances, coherence. JSON carries all six quality observations, five separate brand observations, file/hash/bytes/dimensions/cohort, caveats, prior comparisons and raw frozen vectors. Each original has high confidence for its visible area and explicit initial-viewport limits.

| Original | Cohort | H | T | D | C | A | O | Quality | Brand |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| kyrecovery-02-after-appearance-desktop-dark.png | settings | 9 | 9 | 9 | 9 | 9 | 9 | 9.0 | 9.0 |
| kyrecovery-02-after-appearance-desktop-light.png | settings | 9 | 9 | 9 | 9 | 9 | 9 | 9.0 | 9.0 |
| kyrecovery-02-after-appearance-mobile-dark.png | settings | 9 | 9 | 9 | 9 | 9 | 9 | 9.0 | 9.0 |
| kyrecovery-02-after-appearance-mobile-light.png | settings | 9 | 9 | 9 | 9 | 9 | 9 | 9.0 | 9.0 |
| kyrecovery-02-after-audit-desktop-dark.png | admin-console | 9 | 8 | 8 | 9 | 8 | 9 | 8.5 | 9.0 |
| kyrecovery-02-after-audit-desktop-light.png | admin-console | 9 | 8 | 8 | 9 | 8 | 9 | 8.5 | 9.0 |
| kyrecovery-02-after-audit-mobile-dark.png | admin-console | 8 | 8 | 8 | 9 | 8 | 9 | 8.3 | 9.0 |
| kyrecovery-02-after-audit-mobile-light.png | admin-console | 8 | 8 | 8 | 9 | 8 | 9 | 8.3 | 9.0 |
| kyrecovery-02-after-backups-desktop-dark.png | admin-console | 8 | 8 | 7 | 9 | 8 | 8 | 8.0 | 9.0 |
| kyrecovery-02-after-backups-desktop-light.png | admin-console | 8 | 8 | 7 | 9 | 8 | 8 | 8.0 | 9.0 |
| kyrecovery-02-after-backups-mobile-dark.png | admin-console | 8 | 8 | 7 | 9 | 8 | 8 | 8.0 | 9.0 |
| kyrecovery-02-after-backups-mobile-light.png | admin-console | 8 | 8 | 7 | 9 | 8 | 8 | 8.0 | 9.0 |
| kyrecovery-02-after-capsules-desktop-dark.png | admin-console | 8 | 8 | 8 | 9 | 8 | 8 | 8.2 | 9.0 |
| kyrecovery-02-after-capsules-desktop-light.png | admin-console | 8 | 8 | 8 | 8 | 8 | 8 | 8.0 | 8.8 |
| kyrecovery-02-after-capsules-mobile-dark.png | admin-console | 8 | 8 | 7 | 9 | 8 | 8 | 8.0 | 9.0 |
| kyrecovery-02-after-capsules-mobile-light.png | admin-console | 8 | 8 | 7 | 8 | 8 | 8 | 7.8 | 8.8 |
| kyrecovery-02-after-ceremony-desktop-dark.png | settings | 9 | 9 | 9 | 9 | 9 | 9 | 9.0 | 9.0 |
| kyrecovery-02-after-ceremony-desktop-light.png | settings | 9 | 9 | 9 | 9 | 9 | 9 | 9.0 | 9.0 |
| kyrecovery-02-after-ceremony-mobile-dark.png | settings | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-ceremony-mobile-light.png | settings | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-custodian-form-desktop-dark.png | settings | 9 | 9 | 9 | 9 | 8 | 9 | 8.9 | 9.0 |
| kyrecovery-02-after-custodian-form-desktop-light.png | settings | 9 | 9 | 9 | 9 | 8 | 9 | 8.9 | 9.0 |
| kyrecovery-02-after-custodian-form-mobile-dark.png | settings | 9 | 9 | 9 | 9 | 8 | 9 | 8.9 | 9.0 |
| kyrecovery-02-after-custodian-form-mobile-light.png | settings | 9 | 9 | 9 | 9 | 8 | 9 | 8.9 | 9.0 |
| kyrecovery-02-after-custodians-desktop-dark.png | settings | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-custodians-desktop-light.png | settings | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-custodians-mobile-dark.png | settings | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-custodians-mobile-light.png | settings | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-login-desktop-dark.png | auth | 8 | 9 | 9 | 9 | 8 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-login-desktop-light.png | auth | 8 | 9 | 9 | 9 | 8 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-login-mobile-dark.png | auth | 8 | 9 | 9 | 9 | 8 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-login-mobile-light.png | auth | 8 | 9 | 9 | 9 | 8 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-overview-desktop-dark.png | admin-console | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-overview-desktop-light.png | admin-console | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-overview-mobile-dark.png | admin-console | 9 | 8 | 8 | 9 | 9 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-overview-mobile-light.png | admin-console | 9 | 8 | 8 | 9 | 9 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-pairing-desktop-dark.png | admin-console | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-pairing-desktop-light.png | admin-console | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-pairing-form-desktop-dark.png | admin-console | 9 | 9 | 9 | 9 | 8 | 9 | 8.9 | 9.0 |
| kyrecovery-02-after-pairing-form-desktop-light.png | admin-console | 9 | 9 | 9 | 9 | 8 | 9 | 8.9 | 9.0 |
| kyrecovery-02-after-pairing-form-mobile-dark.png | admin-console | 9 | 9 | 8 | 9 | 8 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-pairing-form-mobile-light.png | admin-console | 9 | 9 | 8 | 9 | 8 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-pairing-mobile-dark.png | admin-console | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-pairing-mobile-light.png | admin-console | 9 | 9 | 8 | 9 | 9 | 9 | 8.8 | 9.0 |
| kyrecovery-02-after-replication-desktop-dark.png | settings | 8 | 9 | 8 | 9 | 9 | 9 | 8.6 | 9.0 |
| kyrecovery-02-after-replication-desktop-light.png | settings | 8 | 9 | 8 | 9 | 9 | 9 | 8.6 | 9.0 |
| kyrecovery-02-after-replication-form-desktop-dark.png | settings | 9 | 9 | 8 | 9 | 8 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-replication-form-desktop-light.png | settings | 9 | 9 | 8 | 9 | 8 | 9 | 8.7 | 9.0 |
| kyrecovery-02-after-replication-form-mobile-dark.png | settings | 9 | 8 | 8 | 9 | 8 | 9 | 8.5 | 8.8 |
| kyrecovery-02-after-replication-form-mobile-light.png | settings | 9 | 8 | 8 | 9 | 8 | 9 | 8.5 | 8.8 |
| kyrecovery-02-after-replication-mobile-dark.png | settings | 8 | 8 | 8 | 9 | 9 | 9 | 8.5 | 9.0 |
| kyrecovery-02-after-replication-mobile-light.png | settings | 8 | 8 | 8 | 9 | 9 | 9 | 8.5 | 9.0 |

## Checked and hand-back

**Changed:** this independent review pair records 52 originals, exact cohort arithmetic, separate brand fit, prior comparisons and native supplement. **Review:** B1 dialog focus and C1 bootstrap guidance are the meaningful remaining task concerns. Per-image rows may be skipped when checking only the verdict.

**Checked:** inventory membership; image/source/vendor/reference hashes and dimensions; 650 pair metrics; integer scores, weighted totals, exact cohort sums/means; frozen-vector digest; prior reports unchanged; own-tab closure; authorized writes only. **Not checked:** full behavior/accessibility matrix and repository full checks, for the limitations above. DOX owning docs intentionally unchanged: review evidence introduces no durable structure/behavior/workflow contract.

**Next owner action:** combine independent Opus findings and audit spreads before any Gemini round. Sol is satisfied only for this covered visual 8 scope. No whole-product/behavior acceptance or merge/deployment is authorized.

MySlop mirroring was unavailable: parent confirms credential401 again and explicitly forbids retry. No retry or secret request/inspection occurred. The completed review is local for the parent.

Impeccable context ran once after freeze and found no PRODUCT.md/DESIGN.md. The fixed rubric/pinned references supplied review context. Explicit independent-review/two-file scope overrides generic delegated-assessment, detector, extra-persistence and question steps. Questions skipped: fixed brief requires no decision.
