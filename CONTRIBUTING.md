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

