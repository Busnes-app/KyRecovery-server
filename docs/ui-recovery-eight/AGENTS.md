# KyRecovery UI Council evidence

## Purpose

Record the focused pass toward the owner's covered visual 8/10 gate and shared Ky product identity.

## Ownership

- `capture.mjs` owns disposable synthetic fixtures, original PNGs and immutable manifests.
- Raw report pairs preserve publisher bytes, including whitespace; root `.gitattributes` limits the end-of-line whitespace exception to `*-round*.md`. Derived artifacts and source retain normal checks.
- Each reviewer owns its own report pair. The parent owns inventories, audits, score tables and spreadsheet exports.

## Local Contracts

- Preserve recovery custody, routes, roles, escaping, CSP, named themes and saved choices.
- Opus and Sol review first; Gemini is advisory after both are satisfied and requires a parent evidence audit.
- Use the fixed ky-suite-v2 rubric, exact per-cohort arithmetic and separate brand scores. Keep raw reports unchanged. Unavailable image access means no score.
- Show every round's scores. Wait for terminal reviews before regenerating shared score artifacts.

## Work Guidance

- Use original images and Computer Use on owned synthetic fixtures. No merge or deployment is authorized.
- Capture 1280×900 and 390×844, OS light/dark, DPR1 with normal service-worker behavior. Baseline scope is an unconfigured administrator instance: eight tabs, three open forms, sign-in and the initial ceremony page. No custodian shares are generated or imported by the capture.
- Stage 01 is the initial implementation capture; stage 02 is its bounded cleanup recapture for Council round 2. Preserve both manifests. Stage 03 is the task/keyboard pass for Council round 3. Stage 04 covers empty/configuration states and phone ledger readability for round 4.
- Keep empty-state and initial-viewport coverage explicit; it does not establish populated-state or whole-product acceptance.

## Verification

- `node docs/ui-recovery-eight/capture.mjs <nn> before|after` atomically reserves the stage manifest before building or writing screenshots, refuses an existing or reserved manifest, and removes an incomplete manifest on a handled capture failure. It builds and runs an owned server. `KEEP_FIXTURE=1` retains it for Computer Use until SIGTERM.
- `python3 docs/ui-recovery-eight/check-capture.py` exercises the real capture CLI: frozen/competing writer refusal, failed-build cleanup, a complete 52-image run, and frozen-file preservation. It requires unused scratch stages 98/99 and removes its own outputs.
- Product checks remain owned by the root and browser AGENTS contracts.

## Child DOX Index

- None.
