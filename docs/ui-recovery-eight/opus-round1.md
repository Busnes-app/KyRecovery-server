# KyRecovery Council round 1 — Opus 5.5 baseline (stage 00-before)

Reviewer: Opus 5.5 (`claude-opus-5-5`). Rubric `ky-suite-v2` (sha256 `ad3516620420fbdc30f5273f82b7c72c76e1365d1b6296fcaa56be3cbebf2c41`), contract sha256 `392afb88c51fb3cfad6c94d6b841804b194d107a0431c31f35ff81d3a7a444d3`.
Runtime `7f71f6162a81dabf97bc0bc454810af5f8157258`, evidence `910838054661e3014015bf9fec192da5d890d86e`, vendored ky-ui 0.4.1.
Frozen payload sha256 `c35c42027bcb3ae4edf1db43a3c6956af293e4d75e863ac6400352b3e907381f` at 2026-10-27T12:14:35+00:00, before any prior score, peer report or Computer Use. Machine record: `opus-round1.json`.

## Verdict

**Not satisfied.** No cohort mean reaches 8.0. Required visible defects remain in the shell, the ceremony page and per-view hierarchy. This is a first baseline: it claims no whole-product 8 or 9, competitive result, CI, security or deploy status. All behaviour gates are `untested`.

## Cohort means (exact)

| Cohort | n | Quality sum | Fraction | Mean | Ky-fit sum | Ky-fit fraction | Ky-fit mean |
|---|---|---|---|---|---|---|---|
| admin-console | 24 | 154.6 | 773/120 | 6.441666 (6.4) | 133.6 | 167/30 | 5.566666 (5.6) |
| settings | 24 | 157.8 | 263/40 | 6.575 (6.6) | 134.8 | 337/60 | 5.616666 (5.6) |
| auth | 4 | 26.0 | 13/2 | 6.5 (6.5) | 24.0 | 6/1 | 6 (6.0) |

These cover KyRecovery files only. There are no competitor captures, so no ranking and no Ky gap. The table is a coverage summary, not a ranking.

## Inventory and verification

52 originals: admin-console 24, settings 24, auth 4. All 52 SHA-256 values are distinct, with 0 exact and 0 near duplicates (parent audit: 650 pairs). For every file, the sha256, bytes, PNG IHDR dimensions, inventory/manifest membership and git blob at the evidence commit all match. The 8 manifest source pins match the runtime commit. I viewed each original individually at full resolution. I made no contact sheets. Unscored files: none.

## Per-file scores

| File | Cohort | H | T | D | C | A | Co | Total | Ky fit (pal/type/shell/acc/suite) |
|---|---|---|---|---|---|---|---|---|---|
| appearance-desktop-dark.png | settings | 7 | 8 | 7 | 8 | 8 | 8 | 7.6 | 6.6 (8/7/3/8/7) |
| appearance-desktop-light.png | settings | 7 | 8 | 7 | 8 | 8 | 8 | 7.6 | 6.6 (8/7/3/8/7) |
| appearance-mobile-dark.png | settings | 7 | 8 | 7 | 8 | 8 | 8 | 7.6 | 6.8 (8/7/4/8/7) |
| appearance-mobile-light.png | settings | 7 | 8 | 7 | 8 | 8 | 8 | 7.6 | 6.8 (8/7/4/8/7) |
| audit-desktop-dark.png | admin-console | 6 | 6 | 6 | 7 | 6 | 6 | 6.2 | 5.4 (7/5/3/8/4) |
| audit-desktop-light.png | admin-console | 6 | 6 | 6 | 7 | 6 | 6 | 6.2 | 5.4 (7/5/3/8/4) |
| audit-mobile-dark.png | admin-console | 6 | 6 | 4 | 7 | 5 | 5 | 5.5 | 5.6 (7/5/4/8/4) |
| audit-mobile-light.png | admin-console | 6 | 6 | 4 | 7 | 5 | 5 | 5.5 | 5.6 (7/5/4/8/4) |
| backups-desktop-dark.png | admin-console | 5 | 6 | 6 | 7 | 5 | 6 | 5.8 | 5.2 (7/5/3/7/4) |
| backups-desktop-light.png | admin-console | 5 | 6 | 6 | 7 | 5 | 6 | 5.8 | 5.2 (7/5/3/7/4) |
| backups-mobile-dark.png | admin-console | 5 | 6 | 6 | 7 | 5 | 6 | 5.8 | 5.4 (7/5/4/7/4) |
| backups-mobile-light.png | admin-console | 5 | 6 | 6 | 7 | 5 | 6 | 5.8 | 5.4 (7/5/4/7/4) |
| capsules-desktop-dark.png | admin-console | 5 | 6 | 6 | 7 | 6 | 6 | 6.0 | 5.2 (7/5/3/7/4) |
| capsules-desktop-light.png | admin-console | 5 | 6 | 6 | 7 | 6 | 6 | 6.0 | 5.2 (7/5/3/7/4) |
| capsules-mobile-dark.png | admin-console | 5 | 6 | 5 | 7 | 6 | 6 | 5.8 | 5.4 (7/5/4/7/4) |
| capsules-mobile-light.png | admin-console | 5 | 6 | 5 | 7 | 6 | 6 | 5.8 | 5.4 (7/5/4/7/4) |
| ceremony-desktop-dark.png | settings | 6 | 5 | 5 | 7 | 6 | 5 | 5.7 | 4.8 (7/4/2/8/3) |
| ceremony-desktop-light.png | settings | 6 | 5 | 5 | 7 | 6 | 5 | 5.7 | 4.8 (7/4/2/8/3) |
| ceremony-mobile-dark.png | settings | 6 | 5 | 5 | 7 | 6 | 5 | 5.7 | 4.8 (7/4/2/8/3) |
| ceremony-mobile-light.png | settings | 6 | 5 | 5 | 7 | 6 | 5 | 5.7 | 4.8 (7/4/2/8/3) |
| custodian-form-desktop-dark.png | settings | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 6.2 (8/7/3/8/5) |
| custodian-form-desktop-light.png | settings | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 6.2 (8/7/3/8/5) |
| custodian-form-mobile-dark.png | settings | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 6.4 (8/7/4/8/5) |
| custodian-form-mobile-light.png | settings | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 6.4 (8/7/4/8/5) |
| custodians-desktop-dark.png | settings | 5 | 6 | 6 | 6 | 6 | 6 | 5.8 | 4.8 (7/5/3/5/4) |
| custodians-desktop-light.png | settings | 5 | 6 | 6 | 6 | 6 | 6 | 5.8 | 4.8 (7/5/3/5/4) |
| custodians-mobile-dark.png | settings | 5 | 6 | 5 | 6 | 6 | 5 | 5.5 | 5.0 (7/5/4/5/4) |
| custodians-mobile-light.png | settings | 5 | 6 | 5 | 6 | 6 | 5 | 5.5 | 5.0 (7/5/4/5/4) |
| login-desktop-dark.png | auth | 6 | 7 | 6 | 6 | 7 | 6 | 6.3 | 5.8 (7/7/5/5/5) |
| login-desktop-light.png | auth | 6 | 7 | 6 | 6 | 7 | 6 | 6.3 | 5.8 (7/7/5/5/5) |
| login-mobile-dark.png | auth | 7 | 7 | 7 | 7 | 6 | 6 | 6.7 | 6.2 (7/7/5/7/5) |
| login-mobile-light.png | auth | 7 | 7 | 7 | 7 | 6 | 6 | 6.7 | 6.2 (7/7/5/7/5) |
| overview-desktop-dark.png | admin-console | 7 | 7 | 7 | 7 | 7 | 6 | 6.9 | 5.4 (7/6/3/7/4) |
| overview-desktop-light.png | admin-console | 7 | 7 | 7 | 7 | 7 | 6 | 6.9 | 5.4 (7/6/3/7/4) |
| overview-mobile-dark.png | admin-console | 6 | 7 | 6 | 7 | 6 | 7 | 6.5 | 5.6 (7/6/4/7/4) |
| overview-mobile-light.png | admin-console | 6 | 7 | 6 | 7 | 6 | 7 | 6.5 | 5.6 (7/6/4/7/4) |
| pairing-desktop-dark.png | admin-console | 6 | 6 | 6 | 7 | 7 | 7 | 6.5 | 5.4 (7/5/3/8/4) |
| pairing-desktop-light.png | admin-console | 6 | 6 | 6 | 7 | 7 | 7 | 6.5 | 5.4 (7/5/3/8/4) |
| pairing-form-desktop-dark.png | admin-console | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 6.2 (8/7/3/8/5) |
| pairing-form-desktop-light.png | admin-console | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 6.2 (8/7/3/8/5) |
| pairing-form-mobile-dark.png | admin-console | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 6.4 (8/7/4/8/5) |
| pairing-form-mobile-light.png | admin-console | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 6.4 (8/7/4/8/5) |
| pairing-mobile-dark.png | admin-console | 7 | 6 | 7 | 7 | 7 | 7 | 6.9 | 5.6 (7/5/4/8/4) |
| pairing-mobile-light.png | admin-console | 7 | 6 | 7 | 7 | 7 | 7 | 6.9 | 5.6 (7/5/4/8/4) |
| replication-desktop-dark.png | settings | 7 | 7 | 7 | 7 | 7 | 7 | 7.0 | 5.4 (7/5/3/8/4) |
| replication-desktop-light.png | settings | 7 | 7 | 7 | 7 | 7 | 7 | 7.0 | 5.4 (7/5/3/8/4) |
| replication-form-desktop-dark.png | settings | 5 | 7 | 6 | 6 | 6 | 6 | 6.0 | 5.4 (6/7/3/6/5) |
| replication-form-desktop-light.png | settings | 5 | 7 | 6 | 6 | 6 | 6 | 6.0 | 5.4 (6/7/3/6/5) |
| replication-form-mobile-dark.png | settings | 5 | 7 | 5 | 6 | 5 | 6 | 5.6 | 5.6 (6/7/4/6/5) |
| replication-form-mobile-light.png | settings | 5 | 7 | 5 | 6 | 5 | 6 | 5.6 | 5.6 (6/7/4/6/5) |
| replication-mobile-dark.png | settings | 7 | 7 | 7 | 7 | 7 | 7 | 7.0 | 5.6 (7/5/4/8/4) |
| replication-mobile-light.png | settings | 7 | 7 | 7 | 7 | 7 | 7 | 7.0 | 5.6 (7/5/4/8/4) |

The JSON holds six quality observations and five brand observations for every file, plus confidence and a caveat.

## Brand gap (separate from quality)

- **Shell (3 desktop, 4 phone, 2 ceremony).** The console uses a top tab bar plus a header button cluster, not the single left navigation region with a bottom account region. The ceremony page has no shell at all and puts a compact native theme dropdown first.
- **Type geometry (mostly 5).** Header, card-header and form buttons are 28px next to 40px inputs, and radii are 6px. The ceremony h1 is 30px/700.
- **Palette (7–8).** Cream/charcoal tokens read correctly. The replication checkbox falls back to native browser blue.
- **Accent (5–8).** Accent use is disciplined except on Custodians (two orange primaries) and the signed-out header (a second orange Sign In plus a danger-tinted Guest chip).
- **Suite cohesion (3–7).** The Appearance picker matches the KyNotes references almost exactly. Everything wrapped in the top-tab shell reads as a different product family.

## Required visible defects (priority order)

1. **all 46 console captures**: Top tab bar + header button cluster instead of the one left navigation region; account actions (SSO pairing, Password, Sign Out) float top-right as 28px buttons; header/nav start at x32 while content starts at x122. Source: internal/server/static/index.html <header> (lines ~20-43) and <nav class="nav-tabs"> (~80); css/styles.css header/.nav-tabs rules. Bounded fix: Move the existing eight nav buttons into a 240px left sidebar with wordmark/stamp at top and a bottom-pinned account region (name, role, SSO pairing, Password, Sign Out) using ky-nav-item rows (40px, 8px radius, accent rail). Keep the phone drawer/scroll pattern with the same order and labels. No route, role or nav-ID change.
2. **ceremony x4**: A compact native theme dropdown is the first element (owner rejects compact theme dropdowns); no wordmark/stamp or way back to the console; h1 ~30px/700 crammed 4-5px between the select and the first card. Source: internal/server/static/ceremony.html line 16 (<select class="theme-selector" data-theme-select>) and line 17 h1; css/ceremony.css. Bounded fix: Drop the visible select while keeping the saved/OS theme applied through the existing theme.js adapter and storage key and add the same wordmark header with a quiet "Back to console" link; set h1 to the 24px/600 title token with 16-24px spacing; keep CSP (no inline script/style) and ceremony behaviour unchanged.
3. **backups, capsules, pairing, custodians, audit (desktop+mobile)**: Five views have no h1; a card title or KPI row stands in, so title hierarchy changes from tab to tab (Overview/Replication/Appearance have h1). Source: index.html #tab-backups, #tab-capsules, #tab-pairing, #tab-custodians, #tab-audit. Bounded fix: Give each tab the same page header used by #tab-replication: one h1 on the left and the view primary on the right; demote card titles to section headings.
4. **custodians x4**: Two filled orange primaries compete in one view. Source: index.html #tab-custodians card headers (Run recovery key ceremony, + Register Custodian), btn-primary on both. Bounded fix: Keep one primary per state (ceremony while no key exists) and render Register Custodian as btn-secondary; no behaviour change.
5. **replication-form x4**: Footer actions scroll out of the initial viewport (no visible primary); nine fields are one ungrouped column; native checkbox renders browser blue; phone clips the checkbox row. Source: styles.css .modal-card (max-height:90vh; overflow-y:auto) and .modal-footer; index.html #replication-modal fields and #repl-auto-sync checkbox. Bounded fix: Make .modal-body the scroll region with a fixed footer; group fields under plain headings (Destination, Credentials, Sync); set accent-color to the ky accent token for checkboxes. Do not change fields, validation or test/save behaviour.
6. **login x4**: Signed-out header shows an uppercase LOADING pill, a danger-tinted Guest chip and a second orange Sign In beside the form primary (desktop) or an Account button (phone). Source: index.html header #system-status-pill (initial text LOADING), #user-role-badge (status-pill danger "Guest"), #btn-auth-action (btn-primary Sign In). Bounded fix: Hide the status pill, role badge and header auth action while #login-gateway-view is shown, leaving wordmark only; the form keeps the single primary.
7. **capsules x4**: Destructive "Purge expired" looks identical to neutral "Refresh preview" next to Save; retention settings precede the capsule inventory. Source: index.html #tab-capsules line ~227 (Purge expired as btn-secondary btn-sm) and card order. Bounded fix: Style Purge expired with the danger outline/text treatment set apart from Save (keep its existing behaviour) and place the inventory card first or retention under a section heading below it.
8. **audit mobile x2**: Ledger table clipped mid-word at the card edge with no scroll cue; Verify button indented ~12px from the card title. Source: styles.css table wrapper/card header rules at phone width; index.html #tab-audit. Bounded fix: Align card-header actions to the title edge and either stack ledger rows as label/value pairs below the breakpoint or show an overflow-scroll wrapper with a visible edge cue.
9. **all console captures**: Header, card-header and form buttons render ~28px while inputs are ~40px; the contract asks 40px controls. Source: styles.css .btn-sm (padding 5px 10px; font-size 13px). Bounded fix: Raise .btn and .btn-sm to the shared 40px control height (or reserve btn-sm for dense table rows only).
10. **backups x4**: Same "Restic scanning is off" sentence appears twice; empty state has no action; "--" metric unexplained. Source: index.html #tab-backups (KPI row, Backup health and Unassigned repositories cards). Bounded fix: Show the scanner-off note once at the top of the view with the existing docs reference; keep metadata health distinct from verified restore in wording.

## Optional taste

- overview: Use one metric value style (numeral vs bold phrase) across the three overview metrics.
- appearance: Tighten the ~40px h1-to-lede gap to match other views and let the grid span the content column.
- audit: Settle row typography to two treatments (mono identifiers, proportional text).
- replication: Remove the stray space before ".kycap" in the explainer.
- custodian-form: Match dialog title and button verb ("Add custodian").

## Not requested as cosmetic work

No cryptographic, API, permission or new-feature changes. Backups metadata health is not a verified restore, and the wording should keep the two apart.

## Computer Use supplement (after freeze; changes no score)

Native Chrome 154, DPR2, 1280x800 then 390x844 freeform; differs from scored Chromium 153/DPR1 captures, so it changes no score.

Synthetic fixture sign-in via #login-username/#login-password/#btn-local-login-submit; waited for visible #dashboard-view; nav.nav-tabs role=button by exact name; opened and cancelled the replication dialog unsubmitted; navigated to /admin/ceremony without generating anything. No theme, config or data change.

- Header SSO pairing / Sign Out buttons 28px tall, 6px radius; overview primary 40px, 6px radius; inputs 6px radius (contract asks 8px).
- Nav rows are 40px with 14px text and 8 .ky-nav-item elements, laid out horizontally; selected row x32 while overview h1 x122 (two left edges).
- Overview h1 24px/600 (contract-sized).
- #tab-custodians: 0 h1/h2/h3; two visible .btn-primary (Run recovery key ceremony, + Register Custodian), both 28px, background rgb(191,63,24).
- #replication-modal at 800px height: .modal-card overflow-y auto, scrollHeight 865 > 720; footer (Test Connection, Cancel, Save Target) top at 842, outside the viewport; checkbox accent-color auto; no fieldset/legend/group headings.
- Phone audit: 855px table inside a 356px overflow-x:auto wrapper (scrollable, no visible cue); document scrollWidth 390 (no page overflow); Verify button x49 vs card title x37.
- /admin/ceremony at 390: no header, nav or links; native select (accessible name "Color theme", 23 options, 36px); h1 30px/700; h1 box starts at the select bottom and the next card starts at the h1 bottom.
- Console: 11 x 401 at 12:14:58Z and 12:15:08Z, before own sign-in at 12:15:12Z (pre-login status poll). GET /api/recovery-key 404 on the ceremony page in a no-key fixture (expected product response); ERR_ABORTED on in-flight polls when navigating away. No own JavaScript errors observed.

## Drift against historical fleet reports (read after freeze)

Historical fleet rated 12 KyRecovery full-page stitched images from b60e571 / refinement stages (overview, replication, appearance; service workers blocked); replication was counted in admin-console. This round scores 52 initial-viewport DPR1 images from 7f71f61 with replication in settings. Different images, so deltas are informational, not product change.

- overview desktop: 7.9 → 6.9 (correction). Prior scored coherence/density 8 and 7; this round counts the x32/x122 two-edge layout and the lone boxed note against density, colour restraint at 7 and coherence 6.
- appearance desktop: 8.0 → 7.6 (correction). This round counts the ~40px title-lede gap and short grid against hierarchy and density.
- replication desktop: 6.8 → 7.0 (cohort). Now in settings; current capture has an h1 that the prior capture lacked.
- replication mobile: 6.1 → 7.0 (cohort). Prior capture clipped empty-table sentences; current phone hides empty table headers.
- Brand shell: Prior shell 5 everywhere; this round 3 (desktop) / 4 (phone) / 2 (ceremony), because the owner now requires one left navigation region and rejects the compact ceremony theme dropdown.
- sol-round1.json exists in the directory and was not opened.

## Caveats

Coverage is the initial viewport of an empty, unconfigured administrator instance, plus three open forms, sign-in and the initial ceremony page. Populated, low-content, error, loading and other-role states are unscored. Live dates earn no design credit. One admin login wrote real audit entries. Pictures cannot show contrast, keyboard, focus, performance or feature completeness. The MySlop board was not posted to (credential status unknown/401).
