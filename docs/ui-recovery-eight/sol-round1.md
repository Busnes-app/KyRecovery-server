# KyRecovery Council — Sol Round 1 baseline

Fresh independent visual baseline of the frozen current implementation. **Not satisfied for progression; no acceptance, whole-product 8/9, competitive win, merge or deployment claim.** No earlier Recovery scores or peer report were read.

Runtime: `7f71f6162a81dabf97bc0bc454810af5f8157258`. Evidence: `910838054661e3014015bf9fec192da5d890d86e`. Recovery vendors **ky-ui 0.4.1**; all twelve committed vendor hashes match. Later shared 0.4.3 / producer 58626b7 is context only. Dirty ky-ui working files were neither modified nor used as authority.

The complete scoring payload was frozen at **2026-10-27T12:11:52.121907+00:00**, before native browser work. Frozen SHA-256: `b2fd3f66153a8a061f272efa1c3c154de8371685b9ad2e6c99c6c0cb2f0b7819`. The digest covers all JSON top-level fields except `freeze` and the later `native_supplement`, using sorted compact UTF-8 JSON with ensure_ascii=False. Native findings did not change scores.

## Independent results

| Cohort | n | Quality sum | Exact quality mean | Brand sum | Exact brand mean | Brand gap to 8 | Brand gap to 9 |
| --- | ---: | ---: | --- | ---: | --- | --- | --- |
| admin-console | 24 | 172.8 | 36/5 = 7.2 | 174.4 | 109/15 = 7.2666… | -11/15 | -26/15 |
| settings | 24 | 176.4 | 147/20 = 7.35 | 172.0 | 43/6 = 7.1666… | -5/6 | -11/6 |
| auth | 4 | 30.0 | 15/2 = 7.5 | 30.4 | 38/5 = 7.6 | -2/5 | -7/5 |

No cross-cohort average. All 52 originals are unique and count once. Exact per-image weighted totals use integer 0–10 dimensions and Decimal half-up to one decimal; cohort means use the sum of those totals divided by the unique-image count, without threshold rounding. Brand uses five equal dimensions and stays separate from quality. All three quality means are below 8; satisfaction is independently false because the shell, control and initial-task composition gaps remain visible.

| Council member | Round 1 status | admin-console | settings | auth |
| --- | --- | ---: | ---: | ---: |
| GPT Sol | Independent baseline complete | 7.2 | 7.35 | 7.5 |
| Claude Opus 5.5 | Pending in this report; peer report not opened | — | — | — |
| Gemini 4 Argon | Unused; no subagents requested or started | — | — | — |

This is the first fresh current Council baseline. Historical fleet evidence has different revisions and coverage; none was read or pooled. Prior neutral pills, retention padding, phone account popover, tab reveal and shared Appearance remain ordinary current implementation, not newly earned design credit. Backups and the current factual overview are present. Metadata health and audit append availability are not a verified restore.

## What the pictures support

The palette and self-hosted type family already connect Recovery to Busnes. The overview on desktop is the strongest composition: one title, flat factual groups, a clear primary task and quiet alternatives. Appearance reproduces the shared preview/name/radio vocabulary and wraps cleanly on phone. The pairing and custodian dialogs fit their initial viewport and show complete actions.

The product still reads as a compact utility console beside the pinned Notes reference. Top tabs and account buttons replace the single desktop left navigation/account region; buttons have several compressed heights; some views start with section-sized titles and others with large metrics or settings. The ceremony retains readable 390px rendering but lacks the product wordmark/stamp and starts with a compact theme select. These are brand and composition gaps, separate from the quality of an empty or sparse view.

For an experienced operator, the small controls and phone audit-table crop weaken scanning. A first-time operator sees five empty Backups metrics or a full retention configuration before the relevant task context. A distracted phone user sees the replication form end mid-line while its completion controls are below the crop. These are visible hypotheses about task clarity, not measured completion-time, accessibility or feature claims.

## Concrete priorities

### P1 — required_brand_contract

**Source:** [internal/server/static/index.html:80](../../internal/server/static/index.html#L80). **Region:** Dashboard header/nav and account region; all eight tabs.

**Visible effect:** The top tab bar and top-right admin tools read as a separate admin console rather than the approved single-left-region Ky shell.

**Bounded fix:** Compose the existing eight destinations in one desktop left navigation region with the existing stamp/wordmark and account block; retain a responsive phone pattern, labels, order, routes and all role behavior. No second sidebar.

### P2 — required_visible_defect

**Source:** [internal/server/static/css/styles.css:199](../../internal/server/static/css/styles.css#L199). **Region:** Task buttons, modal footer and close controls; .btn and .btn-sm.

**Visible effect:** Compact 28–35px-looking buttons sit beside roughly 40px inputs and undermine the common control rhythm.

**Bounded fix:** Use shared control height/radius/type tokens for the existing buttons, close controls and inputs; adjust wrapping/layout as necessary without adding actions or changing behavior.

### P3 — required_visible_defect

**Source:** [internal/server/static/css/styles.css:269](../../internal/server/static/css/styles.css#L269). **Region:** Add a replication target dialog, both sizes.

**Visible effect:** Completion buttons are outside the initial visible dialog; on phone the last visible auto-sync line is cut by the rounded boundary.

**Bounded fix:** Keep the existing header/footer visible and confine vertical scrolling to the field body, with sufficient bottom padding. Preserve every field, preset, validation and submit behavior.

### P4 — required_visible_defect

**Source:** [internal/server/static/index.html:131](../../internal/server/static/index.html#L131). **Region:** Backups metrics; Capsules retention at line 203; overview phone summary at CSS line 350.

**Visible effect:** Backups lacks a page title and prioritizes five large empty metrics; phone Capsules opens entirely on retention, while overview task actions fall below three summary blocks.

**Bounded fix:** Give each existing tab a shared page heading; compact phone summaries and put existing task/list identity before supporting settings. Reflow existing content, preserving factual copy and retention semantics.

### P5 — required_visible_defect

**Source:** [internal/server/static/index.html:296](../../internal/server/static/index.html#L296). **Region:** Custodians & Quorum two section actions; replication auto-sync checkbox at line 574.

**Visible effect:** Two orange primary actions compete in Custodians; the desktop native checkbox contributes an unthemed blue selection.

**Bounded fix:** Choose the existing state-appropriate primary action and render the other as secondary; apply the shared accent to the whole native checkbox using native accent-color, preserving state and behavior.

### P6 — required_brand_contract

**Source:** [internal/server/static/ceremony.html:15](../../internal/server/static/ceremony.html#L15). **Region:** Standalone initial ceremony header.

**Visible effect:** No stamp/wordmark identifies the product and a compact theme select leads the page, separating this sensitive surface from the suite identity.

**Bounded fix:** Use a restrained KyRecovery stamp/wordmark and shared page/control geometry; keep all ceremony copy, CSP and cryptographic flow unchanged. Resolve appearance presentation within the existing saved-theme adapter; no key/share workflow additions.

### P7 — required_visible_defect

**Source:** [internal/server/static/index.html:383](../../internal/server/static/index.html#L383). **Region:** Phone Audit ledger table and its horizontal scroll wrapper.

**Visible effect:** The actor value is cut off and the remaining evidence columns are outside the initial card without a clear persistent overflow cue.

**Bounded fix:** Retain all columns and add a visible small-screen horizontal-scroll cue or an existing-data stacked row presentation; do not alter audit data or verification.

### T1 — optional_taste

**Source:** [internal/server/static/css/styles.css:267](../../internal/server/static/css/styles.css#L267). **Region:** Dialog backdrop and sparse-page footer.

**Visible effect:** Strong blur and repeated copyright chrome give the utility a heavier frame than the flatter Notes reference.

**Bounded fix:** Optionally soften blur and reduce footer prominence after the concrete shell/control issues; these do not block task completion.

The required priorities concern existing presentation only. Keep cryptography, custody, APIs, routes, permissions, validation, CSP, recovery data and factual copy intact. T1 is optional taste. Do not turn this baseline into a feature project.

## Inputs and limits

- Capture: 2026-10-27T12:01:23.979Z; Playwright 1.63.0, Chromium 153.0.8010.12, DPR1, normal service-worker behavior.
- CSS viewports: 1280×900 and 390×844; OS light/dark, no saved choice. Inventory `theme` is resolved data-theme; filenames identify OS loop. Every renderedViewport equals its requested size, including phone ceremony.
- Scope: 8 current tabs, 3 open unsubmitted forms, initial ceremony and sign-in, four conditions each. No key, shares, capsules, paired products, custodians, replication targets or restic configuration. Audit shows a real login row, so its observed state is sparse.
- Only initial viewports scored. Populated data, lower content, errors/loading coverage, other roles, non-Busnes rendered themes, contrast, keyboard, performance and completeness are unscored. The visible auth LOADING pill is retained in the picture.
- All 52 originals were individually viewed with original-resolution image display. No contact sheets or transformed images. All hashes, byte counts, IHDR dimensions, manifest membership and evidence git blobs passed.
- All eight manifest runtime source hashes match committed git objects and current files. Rubric and design-contract hashes match the exact requested authorities.
- Independent same-size near-diff recomputation completed after freeze: 650 pairs, zero near duplicates; minimum changed fraction 0.11506944444444445. The frozen parent duplicate-audit result was also zero.
- Four pinned KyNotes originals at commit 1ab04e64de747f409d88b98f02682e85862defe1 were individually viewed for brand only. Their hashes match Pulse’s committed brand-reference manifest at b64ad231f3608b184bcdd237522d27b981252844. Notes was not re-rated or pooled.
- Impeccable context ran once. Its no-PRODUCT/no-DESIGN result did not replace the incumbent visual world. No interview, redesign, detector injection, delegated assessment or source edit occurred. The user’s explicit independent-review limits control.

| Input | SHA-256 |
| --- | --- |
| manifest-00-before.json | `bb0afeb933c7f2b331ec2a9424e82f148da8b7b4b558b1234424331ab29d6648` |
| inventory-v1.json | `5284c541e4274f5c94c2a0724cf642ac21d5449b2e1bae5c9a55a71cdaeb0886` |
| duplicate-audit-v1.json | `b69d10f76a9293ea2600ebd1cc49d94fbe53904f056a3183e031b34ef1fc176f` |
| pinned rubric ky-suite-v2 | `ad3516620420fbdc30f5273f82b7c72c76e1365d1b6296fcaa56be3cbebf2c41` |
| pinned original design contract | `392afb88c51fb3cfad6c94d6b841804b194d107a0431c31f35ff81d3a7a444d3` |

## Per-original scores

H = hierarchy, T = typography, D = density/layout, C = color, A = visible affordances, O = coherence. Weights are .20/.15/.20/.15/.15/.15. All rows have high confidence for the visible original; initial/lower-content caveats remain. The JSON carries exact file/hash/bytes/dimensions, all six quality observations and five separate brand observations for every row.

| Original filename | Cohort | H | T | D | C | A | O | Quality | Brand |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| kyrecovery-00-before-appearance-desktop-dark.png | settings | 8 | 8 | 8 | 8 | 8 | 8 | 8.0 | 7.8 |
| kyrecovery-00-before-appearance-desktop-light.png | settings | 8 | 8 | 8 | 8 | 8 | 8 | 8.0 | 7.8 |
| kyrecovery-00-before-appearance-mobile-dark.png | settings | 8 | 8 | 8 | 8 | 8 | 8 | 8.0 | 8.0 |
| kyrecovery-00-before-appearance-mobile-light.png | settings | 8 | 8 | 8 | 8 | 8 | 8 | 8.0 | 8.0 |
| kyrecovery-00-before-audit-desktop-dark.png | admin-console | 7 | 8 | 8 | 8 | 7 | 8 | 7.7 | 7.0 |
| kyrecovery-00-before-audit-desktop-light.png | admin-console | 7 | 8 | 8 | 8 | 7 | 8 | 7.7 | 7.0 |
| kyrecovery-00-before-audit-mobile-dark.png | admin-console | 6 | 7 | 6 | 8 | 6 | 7 | 6.6 | 7.2 |
| kyrecovery-00-before-audit-mobile-light.png | admin-console | 6 | 7 | 6 | 8 | 6 | 7 | 6.6 | 7.2 |
| kyrecovery-00-before-backups-desktop-dark.png | admin-console | 6 | 7 | 6 | 8 | 6 | 7 | 6.6 | 7.0 |
| kyrecovery-00-before-backups-desktop-light.png | admin-console | 6 | 7 | 6 | 8 | 6 | 7 | 6.6 | 7.0 |
| kyrecovery-00-before-backups-mobile-dark.png | admin-console | 6 | 7 | 5 | 8 | 6 | 7 | 6.4 | 7.2 |
| kyrecovery-00-before-backups-mobile-light.png | admin-console | 6 | 7 | 5 | 8 | 6 | 7 | 6.4 | 7.2 |
| kyrecovery-00-before-capsules-desktop-dark.png | admin-console | 6 | 7 | 7 | 8 | 6 | 7 | 6.8 | 7.0 |
| kyrecovery-00-before-capsules-desktop-light.png | admin-console | 6 | 7 | 7 | 8 | 6 | 7 | 6.8 | 7.0 |
| kyrecovery-00-before-capsules-mobile-dark.png | admin-console | 5 | 7 | 5 | 8 | 6 | 7 | 6.2 | 7.2 |
| kyrecovery-00-before-capsules-mobile-light.png | admin-console | 5 | 7 | 5 | 8 | 6 | 7 | 6.2 | 7.2 |
| kyrecovery-00-before-ceremony-desktop-dark.png | settings | 7 | 7 | 7 | 8 | 8 | 7 | 7.3 | 6.4 |
| kyrecovery-00-before-ceremony-desktop-light.png | settings | 7 | 7 | 7 | 8 | 8 | 7 | 7.3 | 6.4 |
| kyrecovery-00-before-ceremony-mobile-dark.png | settings | 7 | 7 | 7 | 8 | 8 | 7 | 7.3 | 6.4 |
| kyrecovery-00-before-ceremony-mobile-light.png | settings | 7 | 7 | 7 | 8 | 8 | 7 | 7.3 | 6.4 |
| kyrecovery-00-before-custodian-form-desktop-dark.png | settings | 8 | 8 | 8 | 8 | 7 | 8 | 7.9 | 7.4 |
| kyrecovery-00-before-custodian-form-desktop-light.png | settings | 8 | 8 | 8 | 8 | 7 | 8 | 7.9 | 7.4 |
| kyrecovery-00-before-custodian-form-mobile-dark.png | settings | 8 | 8 | 8 | 8 | 7 | 8 | 7.9 | 7.6 |
| kyrecovery-00-before-custodian-form-mobile-light.png | settings | 8 | 8 | 8 | 8 | 7 | 8 | 7.9 | 7.6 |
| kyrecovery-00-before-custodians-desktop-dark.png | settings | 6 | 8 | 8 | 7 | 7 | 7 | 7.2 | 6.4 |
| kyrecovery-00-before-custodians-desktop-light.png | settings | 6 | 8 | 8 | 7 | 7 | 7 | 7.2 | 6.4 |
| kyrecovery-00-before-custodians-mobile-dark.png | settings | 6 | 8 | 8 | 7 | 7 | 7 | 7.2 | 6.6 |
| kyrecovery-00-before-custodians-mobile-light.png | settings | 6 | 8 | 8 | 7 | 7 | 7 | 7.2 | 6.6 |
| kyrecovery-00-before-login-desktop-dark.png | auth | 7 | 8 | 8 | 7 | 8 | 7 | 7.5 | 7.6 |
| kyrecovery-00-before-login-desktop-light.png | auth | 7 | 8 | 8 | 7 | 8 | 7 | 7.5 | 7.6 |
| kyrecovery-00-before-login-mobile-dark.png | auth | 7 | 8 | 8 | 7 | 8 | 7 | 7.5 | 7.6 |
| kyrecovery-00-before-login-mobile-light.png | auth | 7 | 8 | 8 | 7 | 8 | 7 | 7.5 | 7.6 |
| kyrecovery-00-before-overview-desktop-dark.png | admin-console | 8 | 8 | 8 | 9 | 8 | 8 | 8.2 | 7.6 |
| kyrecovery-00-before-overview-desktop-light.png | admin-console | 8 | 8 | 8 | 9 | 8 | 8 | 8.2 | 7.6 |
| kyrecovery-00-before-overview-mobile-dark.png | admin-console | 6 | 8 | 6 | 9 | 6 | 8 | 7.1 | 7.8 |
| kyrecovery-00-before-overview-mobile-light.png | admin-console | 6 | 8 | 6 | 9 | 6 | 8 | 7.1 | 7.8 |
| kyrecovery-00-before-pairing-desktop-dark.png | admin-console | 7 | 8 | 8 | 8 | 7 | 8 | 7.7 | 7.0 |
| kyrecovery-00-before-pairing-desktop-light.png | admin-console | 7 | 8 | 8 | 8 | 7 | 8 | 7.7 | 7.0 |
| kyrecovery-00-before-pairing-form-desktop-dark.png | admin-console | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 7.4 |
| kyrecovery-00-before-pairing-form-desktop-light.png | admin-console | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 7.4 |
| kyrecovery-00-before-pairing-form-mobile-dark.png | admin-console | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 7.6 |
| kyrecovery-00-before-pairing-form-mobile-light.png | admin-console | 8 | 7 | 8 | 8 | 7 | 8 | 7.7 | 7.6 |
| kyrecovery-00-before-pairing-mobile-dark.png | admin-console | 7 | 8 | 8 | 8 | 7 | 8 | 7.7 | 7.2 |
| kyrecovery-00-before-pairing-mobile-light.png | admin-console | 7 | 8 | 8 | 8 | 7 | 8 | 7.7 | 7.2 |
| kyrecovery-00-before-replication-desktop-dark.png | settings | 7 | 8 | 7 | 8 | 7 | 8 | 7.5 | 7.4 |
| kyrecovery-00-before-replication-desktop-light.png | settings | 7 | 8 | 7 | 8 | 7 | 8 | 7.5 | 7.4 |
| kyrecovery-00-before-replication-form-desktop-dark.png | settings | 6 | 8 | 5 | 7 | 5 | 7 | 6.3 | 7.0 |
| kyrecovery-00-before-replication-form-desktop-light.png | settings | 6 | 8 | 5 | 7 | 5 | 7 | 6.3 | 7.0 |
| kyrecovery-00-before-replication-form-mobile-dark.png | settings | 6 | 7 | 4 | 8 | 5 | 7 | 6.1 | 7.4 |
| kyrecovery-00-before-replication-form-mobile-light.png | settings | 6 | 7 | 4 | 8 | 5 | 7 | 6.1 | 7.4 |
| kyrecovery-00-before-replication-mobile-dark.png | settings | 7 | 8 | 7 | 8 | 7 | 8 | 7.5 | 7.6 |
| kyrecovery-00-before-replication-mobile-light.png | settings | 7 | 8 | 7 | 8 | 7 | 8 | 7.5 | 7.6 |

## Native Computer Use supplement — after freeze

Opened a new owned Incognito tab with reuseExistingTab:false after preview_status, then signed in once with the private synthetic fixture. Waited for visible #dashboard-view before visiting the eight nav buttons by exact role/name. The parent tab `tab_2w_2e419d46-7b07-427f-bf30-1831131b4cba` was never operated. Own tab `tab_2x_2e419d46-7b07-427f-bf30-1831131b4cba` was closed. No screenshot files or new captures were written.

The native browser reports Chrome154 / DPR2, whereas the scored originals are Chromium153 / DPR1. It used 1280×900 then390×844, light theme, and retained a null saved theme choice. Its geometry confirms or qualifies observations; it is not a replacement scoring set.

- Header and task buttons measured 27.59375px high; modal footer buttons 34.796875px; text inputs 41px and select 40px.
- Desktop replication dialog clientHeight 808 / scrollHeight 865, footer starts y847.09 within card ending y855: action buttons sit below its initial crop. Phone clientHeight 758 / scrollHeight 901, footer y863.80 while card ends y801.80.
- Native auto-sync checkbox computed accent-color:auto and appearance:auto. Screenshot visual assessment of the blue desktop checkbox remains in the frozen score.
- Phone audit table width 855.05 inside a 356px scrolling wrapper (overflow-x:auto); page overflow zero. This confirms hidden columns exist and are scrollable, rather than missing.
- Selecting Audit Ledger revealed it at x151.5–263.5 in the phone tab row; only narrow navigation reachability was exercised.
- Appearance has 23 native radio options, system checked, no header/nav select, and no saved choice. Theme persistence behavior was not exercised.
- Phone ceremony viewport and page width both390; h1 and form occupy x20–370; h1 is30px. No horizontal clipping observed in this initial light state.

Opened and closed the three forms without submission; reopened the replication form on phone; opened/closed Account; inspected the 23-option Appearance picker without selection; visited initial ceremony directly without generation/import. The exploratory #cards text check was not a generated-output test and is not used as evidence. No configuration, theme, key/share, pairing, custodian, replication-test/save or purge action was invoked.

Existing pre-login polling produced eleven console 401 entries before actual local login. The later network history was bounded; no new authenticated error or browser-tool failure was observed during this limited pass. This does not assert error-free application behavior. The one sign-in created a real audit event, as expected. No credentials are included here.

## Verification and hand-back

The keyboard, focus, contrast, states and responsive gates remain **untested as full gates**. Narrow pointer navigation, modal opening/closing and computed geometry do not prove every task path, theme or state. No repository full checks, CI, security, deployment, cryptographic or restore assurance is claimed; parent owns ongoing browser regression results.

The report pair was checked directly against the named inventory: all 52 entries, all integer dimensions, every weighted total, brand total, exact cohort sum/n and the frozen payload digest. No peer-report glob or validator was used. Only `sol-round1.md` and `sol-round1.json` were written in the repository. DOX pass: owning docs intentionally unchanged because this adds evidence and no durable product/workflow contract changes.

MySlop was not posted: the parent reports an unknown/revoked credential (401), and explicitly prohibited retrying secrets. No retry was made. The completed independent review is available locally for the parent; implementation and progression decisions remain the parent’s task.
