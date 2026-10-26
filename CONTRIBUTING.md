# Contributing to KyRecovery

KyRecovery is a self-hosted blind store for the suite's encrypted recovery capsules. It keeps sealed kycap/3 containers it cannot open, pins the suite recovery public key, and hands that key to each product at pairing. Contributions must preserve the blind-store property: KyRecovery never holds the means to open a capsule.

## Before contributing

Read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) and [SECURITY.md](SECURITY.md),
the [README](README.md), and any AGENTS.md in the subtree you are about to
touch. Open an issue before proposing a feature larger than a bug fix or
documentation change.

Keep changes small, reuse existing code, and fix the shared root cause rather
than the reported symptom. New non-trivial logic ships with a runnable test or
self-check.

## Security requirements

- KyRecovery stores sealed containers and never the recovery private key, a seed, or a custodian share; no route may change that.
- The product deposit token reaches exactly one route (`/api/backup/deposit`); it cannot list, download, or verify capsules.
- Pairing pins a service's public key and topology write-once; a second pairing to a different key must fail, not overwrite.
- Follow the wire contract in `zero_code_pairing_handoff_spec.md`; the retired plaintext push is not reintroduced.
- Do not add telemetry, phone-home, analytics, or unconfigured third-party
  services.
- Do not add silent security fallbacks; fail closed with a readable error and a
  remediation step.
- Name and document any security trade-off in [SECURITY.md](SECURITY.md) before
  it merges.

## Verification

Run the checks that apply to the files you changed, and the repository's
full required checks before hand-off. Record the exact commands and their
results in the pull request.

For overview changes, run `node scripts/test-overview.mjs` (also required by CI's
`go` job). It checks the shipped script against the dashboard IDs and synthetic
readiness responses; it does not replace a real-browser check.

Run `cd tests/browser && npm ci && npx playwright install chromium && npm test`
for dashboard/ceremony changes. The locked private package builds the current
binary and tests desktop/phone Chromium with disposable credentials, data and
local replication directories. Existing browser caches are reused. The `browser`
CI job gates main publication and release images; its failure artifacts contain
only disposable test data. This is not a completed product restore drill.

The [supported strong-key profile](docs/STRONG-CRYPTO.md) requires the full race
suite with `GODEBUG=fips140=on` as well as ordinary mode. Keep configured
SMB refusal and ordinary Samba compatibility coverage distinct; both preserve
stored destinations and capsules. Existing WASM, fuzz, Docker and restic checks
remain required.

## AI attribution

AI-assisted contributions are welcome; undisclosed ones are not. Disclose
material AI assistance in the pull request — the tool, what it changed, and what
you personally ran, read, and verified — and add a commit trailer naming any
tool that materially contributed. You are the author of the code you submit for
every purpose, regardless of how it was produced.

## Pull requests

Branch off `main` with a focused name, and use a Conventional Commit subject
(`fix:`, `feat:`, `docs:`, `refactor:`, `test:`, or `chore:`). The pull request
explains why the change exists, the verification performed, the security impact,
and any known limitations.

Every pull request gets an adversarial review pass; see
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md#hostile-reviews-are-about-code-never-people)
for what that is and the boundary it keeps.

## Licence

KyRecovery is licensed under the MIT License. See [LICENSE.txt](LICENSE.txt).

