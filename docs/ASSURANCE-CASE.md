# Assurance case

[Product overview](../README.md) · [Recovery and security](RECOVERY-AND-SECURITY.md) · [Verifying releases](VERIFYING-RELEASES.md)

This is the argument that KyRecovery meets its security requirements: what it protects, from whom,
where the trust boundaries are, and why the design and the code hold at each one. The detailed
behaviour lives in [Recovery and security](RECOVERY-AND-SECURITY.md#security-model) and the
security invariants in [AGENTS.md](../AGENTS.md#security-invariants); this page ties them together
and points at the evidence. Where a claim names code, the path is relative to the repository root.

## What KyRecovery must protect

KyRecovery is a blind store: products seal `.kycap` containers themselves and deposit the
ciphertext here. Its security requirements:

1. **The server can never open a capsule.** It holds no recovery private key, no Shamir share and
   no plaintext, and no server code path decrypts. Losing the whole server must not expose a
   capsule's contents.
2. **A stored capsule is the one that was deposited.** Its bytes are kept exactly, a digest taken
   at deposit proves later that they have not changed, and one product cannot overwrite another's
   capsule.
3. **Products seal to the right public key.** The recovery public key is created in a ceremony
   the administrator runs, is never replaced silently, and is handed only to products that paired.
4. **Only the right people and products act.** Viewers, operators and admins get only their own
   rights; only a paired product's token can deposit.
5. **What happened is recorded and tamper-evident.** Deposits, verifications, pairings and key
   changes are in a keyed hash chain; a deposit that cannot be recorded is refused.
6. **The server's own secrets stay secret** (replication credentials, the SSO client secret, the
   audit key) from anyone who copies the database without the key file, and never reach a log.

## Threat model

| Attacker | Can | Wants |
|---|---|---|
| Anonymous network client | Send any request to the published port or through the TLS proxy | Guess a pairing code or password, deposit junk, exhaust disk or memory |
| Signed-in viewer or operator | Use their own session | Reach admin functions (key import, pairing, replication, SSO) |
| Malicious or compromised paired product | Hold a valid API token, send any container | Overwrite another product's capsule, make the server act on a forged manifest |
| Hostile web page in the operator's browser | Make the browser send requests | Change the recovery key or settings through the operator's session |
| Identity provider or network path to it | Answer OIDC requests | Forge a sign-in, make the server reach internal addresses |
| Someone with a copy of the data volume | Read `kyrecovery.db` and `capsules/` | Read capsules, recover credentials |

Out of scope: an attacker who runs code on the host or holds the data volume **and**
`secret.key` (they are the server, though they still cannot open a capsule without k custodian
cards), a malicious administrator, and the custodians colluding. Those are covered by the
ceremony and deployment guidance, not by the software.

## Trust boundaries

1. **Products → deposit API.** A product authenticates with the bearer token it got by claiming a
   six-digit code. The container is untrusted input: its manifest is read with
   `capsule.ReadUnverifiedManifest`, displayed, and used for exactly one decision, whether its
   recovery key ID is the current key's (`internal/server/deposit.go`).
2. **Browser → dashboard and admin API.** Cookie sessions, the `apiPolicy` role table
   (`internal/server/server.go`), and a Content-Security-Policy on every response.
3. **Browser tab → server, during the ceremony.** `cmd/ceremony-wasm` generates and splits the
   private key inside the tab and posts only the public key and the k-of-n numbers. The private key
   never crosses this boundary.
4. **KyRecovery → identity provider.** OIDC with PKCE `S256`. Every login requires an
   ID token whose signature, issuer, audience, expiry and nonce pass verification.
   A missing or failed ID token never falls back to `userinfo`; newly fetched RSA
   JWKS members below 2048 bits are refused. The supported strong process profile
   also requires direct HTTPS discovery/endpoints and refuses redirects. Issuer
   requests retain connect-time refusal of unspecified, link-local and multicast
   addresses (`internal/auth/auth.go`, `strong_crypto.go`, `jwks_strength.go`).
   The provider's back-channel logout token passes the same verifier plus the logout-token
   claim rules before it ends the sessions of that issuer's `sid` or subject; it can only end
   sessions, never create or widen one (`internal/auth/backchannel.go`). The `sid`, or the
   subject's cutoff, is fenced for ten minutes so a login callback in flight cannot revive
   it; SSO sessions are dated by their ID token's `iat` (`internal/db/db.go`). The route is
   anonymous, so discovery retries and JWKS fetches are spaced at least 30 seconds apart.
   SSO sessions from before back-channel logout carry no `sid`, so the migration that adds
   `sessions.sso_sid` deletes them in the same transaction, once; local sessions stay.
5. **KyRecovery → offsite targets** (S3, R2, MinIO, SFTP, SMB, local mounts). The targets are
   configured by an admin and receive only ciphertext; S3 endpoints must be `https`
   (`internal/replication/offsite.go`).
6. **KyRecovery → its data volume.** Credentials it holds for others are sealed with the keyring
   before they reach SQLite; the master key lives in `KYRECOVERY_SECRET_KEY` or a separate
   `secret.key` (`internal/secrets`).

## Secure design principles applied

- **Least privilege.** Three ranked roles; an unrecognised IdP role claim becomes `viewer`. The
  container runs as uid 10001 after the entrypoint's `chown`, and the binary stays root-owned
  (`Dockerfile`, `docker-entrypoint.sh`).
- **Fail-safe defaults.** A route missing from `apiPolicy` requires `admin`. The port is published
  on `127.0.0.1` only. Forwarded headers are trusted only from `KYRECOVERY_TRUSTED_PROXIES`, and
  `/0` is refused. A poisoned audit ledger refuses deposits with `503` until repaired.
- **Complete mediation.** Policy and handler read a capsule URL through the same
  `parseCapsulePath`, and an API path is refused unless it equals `path.Clean` of itself, so the
  check and the action cannot see different paths.
- **Economy of mechanism.** The server's only crypto of its own is one AES-256-GCM envelope
  (`internal/crypto/envelope.go`); capsules, passwords, key files and the audit chain use the
  shared, separately tested `ky-primitives` packages.
- **Separation of privilege.** Opening a capsule needs k of n custodian cards that the server never
  sees, so no single operator, and not the server, can read a backup.
- **Open design.** Only published algorithms: HPKE and Shamir splitting in the capsule format,
  Argon2id passwords, AES-256-GCM, SHA-256 digests and hash chains, OIDC with PKCE. The code and
  this argument are public.
- **Psychological acceptability.** The generated admin password goes to a 0600 file, never the
  log, and the README says how to read and delete it.

## Common weaknesses and how they are countered

| Weakness | Counter | Where |
|---|---|---|
| Server-side decryption creeping in | `TestNothingInTheServerDecrypts` walks every non-test `.go` file outside `cmd/ceremony-wasm` and fails on `recoverykey.Generate/Split/Combine/FromSeed`, `capsule.Open`, `capsule.Seal` or `hpke.NewRecipient` | `internal/server/nodecrypt_test.go` |
| Trusting attacker-supplied manifests | Manifest read unverified, stored and displayed; only its recovery key ID is compared with the current key | `internal/server/deposit.go` |
| Path traversal and overwrite through a capsule ID | ID held to `capsuleIDPattern` with no `..`; published by `os.Link`, which refuses an existing target, under a per-ID lock | `internal/server/limits.go` |
| Broken access control | `apiPolicy` table, default `admin`; `TestRequiredRolePolicy` covers each route | `internal/server/server.go`, `internal/server/authz_test.go` |
| Credential leaks in responses | Tokens, pairing codes, session IDs and file paths are `json:"-"`; `TestSessionTokenNeverAppearsInAResponseBody` checks real bodies | `internal/db/db.go`, `internal/server/regression_test.go` |
| Password guessing | Argon2id PHC hashes via `ky-primitives/password`; 20 login attempts per address and 5 per account per 15 minutes | `internal/auth/auth.go`, `internal/server/limits.go` |
| Pairing-code guessing and double claims | Six-digit code from `crypto/rand`, at most 60 minutes, rate limited; the single-use and expiry guards are in the SQL `UPDATE` | `internal/pairing/pairing.go`, `internal/db/db.go` |
| Session theft | 24-byte random session ID in an `HttpOnly`, `SameSite=Lax` cookie, `Secure` over TLS; a password change ends the user's other sessions | `internal/auth/auth.go`, `internal/db/db.go` (`DeleteUserSessionsExcept`) |
| Credentials read from a copy of the database or a backup | Only hashes are stored: SHA-256 of session IDs and API tokens, HMAC-SHA256 of pairing codes under a keyring key; lookups hash the presented value. The API token is minted at claim and returned once. Stores from before hashing are rewritten in place on open. `TestStolenDatabaseYieldsNoUsableCredentials` presents the stored values as credentials; `TestOpenHashesCredentialsStoredAsIssued` covers the rewrite | `internal/db/db.go` (`tokenHash`, `codeHash`, `hashStoredTokens`), `internal/db/tokens_test.go` |
| Cross-site request forgery | `SameSite=Lax` session cookie on every route, plus `http.NewCrossOriginProtection` on key rotation and retirement, retention, pairing clear and the backup writes | `internal/server/server.go` |
| Cross-site scripting in the dashboard | Every interpolated value goes through `esc` or `escJs`; `TestEmbeddedDashboardEscapesEveryInnerHTMLSink` fails on a new unescaped sink; CSP with `frame-ancestors 'none'`, and a stricter one with no inline script on `/admin/ceremony` | `internal/server/static/`, `internal/server/regression_test.go`, `internal/server/server.go` |
| SQL injection | `database/sql` with `?` parameters; the one formatted statement, `ALTER TABLE` in `addColumn`, takes table and column names from constants in the migration code | `internal/db/db.go` |
| Forged sign-in | ID token verified against the issuer's JWKS with nonce check; a failed ID token is never retried against `userinfo`; a callback carrying an IdP `error` is refused before any exchange and its `error_description` is never echoed | `internal/auth/auth.go`, `internal/server/server.go` (`oauthErrorCode`) |
| Resource exhaustion | 1 MiB API bodies, `capsule.MaxContainerBytes` for deposits, 60 deposits per token per 15 minutes, 4 in flight | `internal/server/limits.go` |
| Secrets at rest | Replication secret keys and the SSO client secret sealed with AES-256-GCM under the keyring | `internal/secrets/secrets.go`, `internal/db/db.go`, `internal/auth/auth.go` |
| Tampering with the history | `ky-primitives/auditchain` keyed from the keyring, anchor kept outside the log; `POST /api/audit/verify` checks it; failed appends refuse the action | `internal/audit/chain.go` |
| Vulnerable dependencies | Dependabot, `govulncheck` in the `security` job, OSV ignore list with reasons; dependency-only releases ship daily | `.github/`, `osv-scanner.toml` |
| Supply-chain tampering | Actions and base images pinned by digest; images carry build provenance verified before `:stable` or `:latest` moves; the binary and the ceremony module rebuild to the same bytes | [VERIFYING-RELEASES.md](VERIFYING-RELEASES.md) |

## Supported key-strength configuration

[STRONG-CRYPTO.md](STRONG-CRYPTO.md) covers the complete supported server/CLI,
OIDC, TLS and replication configuration. Both SFTP paths refuse weak keys and
algorithms; the process profile also refuses SMB/NTLM operations and short S3
signing secrets. Normal-mode SMB remains explicit legacy compatibility with
documented risks. This preserves the blind store and existing destinations,
not a FIPS certification or proof of deployed endpoint policy.

## Residual risk

- **Pairing codes are hashed under a key from `secret.key`.** Someone with the database **and**
  the key file can recover a pending code by trying all 900,000; it is valid for at most 60
  minutes and single use. Session IDs and API tokens are unkeyed SHA-256 of 192- and 256-bit
  random values and are not recoverable from either.
- **A binary older than the hashing release cannot use a hashed store.** Rolling back after an
  upgrade signs everyone out and makes paired products' deposits fail with `401` until they
  re-pair. Keep the pre-upgrade backup if a rollback may be needed.
- **Cross-origin protection is uneven.** Older state-changing routes (pairing generate and revoke,
  recovery-key import, replication targets, SSO configuration, password change) rely on the
  `SameSite=Lax` cookie alone, which does not stop a request from another site on the same
  registrable domain.
- **The dashboard CSP allows `'unsafe-inline'` scripts.** Escaping and its test are the XSS counter
  for the dashboard; the CSP is not. `/admin/ceremony` does not have this weakness.
- **Rate limits are process-local** and reset on restart.
- **Free disk space is not reserved or enforced** for the capsule volume
  ([Recovery and security](RECOVERY-AND-SECURITY.md#security-model)).
- **No independent security audit** has been done.

## Evidence

- **Tests:** the security tests named above, plus the whole suite under the race detector on
  every push and pull request (`go` job). The SMB replication tests run against Samba in CI.
  That job also runs `node scripts/test-overview.mjs` against the shipped dashboard script
  and element IDs, covering audit labels, capsule counts, failed responses and recovery.
  Its minimal DOM stub does not establish real-browser behavior or complete UI coverage.
- **Browser workflows:** `tests/browser` runs committed assets against disposable real
  servers on desktop/phone Chromium, including local authentication/session changes,
  persisted form values, pairing, actual WASM ceremony/public-key import and rotation,
  owned local replication and theme behavior. The `browser` job gates image publication
  and tag releases. It does not prove offsite disaster recovery or a completed restore.
- **Static analysis:** `go vet` and `govulncheck` on every push and pull request, and GitHub CodeQL
  (default setup, configured in the repository settings rather than a workflow file) for Go,
  JavaScript/TypeScript and Actions.
- **Dynamic analysis:** the race detector on the full suite, and `node scripts/test-wasm.mjs`,
  which runs a 3-of-5 ceremony in the built module and checks the key ID, public key and shares.
- **Releases:** attested images and a repeatable binary ([VERIFYING-RELEASES.md](VERIFYING-RELEASES.md)).

This case is updated when a trust boundary or a security requirement changes. A pull request that
moves one must update this page in the same change.
