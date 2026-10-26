# Security Policy

KyRecovery is a blind store. It holds sealed capsules it cannot open, pins the suite recovery public key established by the custodian ceremony, and issues scoped deposit tokens to paired products. Only k custodians together, offline, can open a capsule.

## Reporting vulnerabilities

Report security vulnerabilities privately through GitHub Security Advisories:
<https://github.com/Busnes-app/KyRecovery-server/security/advisories>. Do not open a public issue.
Include the affected area, reproduction steps, impact, and affected versions.
We aim to acknowledge reports within two business days and will coordinate a fix
and disclosure.

## Trust boundaries and limitations

- KyRecovery cannot open what it stores. It never holds the recovery private key, a seed, or a share; capsules are opened only offline by k custodians during a product's restore.
- A product's deposit token is scoped to its own pinned service and reaches only `/api/backup/deposit`. It cannot list, download, or verify.
- The custodian ceremony posts only the public key. Until it runs, every pairing claim is refused with 409 and the code is not consumed.
- Pairing is pinned write-once: a token is bound to one service name and key, and a mismatched later deposit is refused with 403.
- TLS protects the public key at pairing (trust on first use), the token, and the receipts — not the capsule. Operators pin the key by hand or compare fingerprints.

## Deployment requirements

- Serve over HTTPS; the client refuses redirects and non-HTTPS recovery URLs.
- Keep the sealed-capsule store and the pinned keys private and backed up; losing the ceremony's custodian cards loses recovery.
- Enforce the documented deposit budgets and container size caps; audit every pairing and deposit without logging the token.

## Strong-key configuration and compatibility

[docs/STRONG-CRYPTO.md](docs/STRONG-CRYPTO.md) owns supported key/TLS boundaries,
endpoint migration and data preservation. OIDC requires a verified ID token
and RSA-2048+ JWKS keys; both SFTP paths require supported algorithms and strong
peer/client/CA keys. The optional process profile enforces TLS key policy, direct
HTTPS OIDC, a 16-byte S3 signing-secret floor and refusal of SMB/NTLM operations.
Normal-mode SMB is an explicit legacy compatibility choice, with MD4/HMAC-MD5,
RC4-based session-key handling, credential-capture/relay risks and no PFS. Use
dedicated credentials and restricted networks; prefer pinned SFTP or verified
HTTPS S3. Required SMB message signing does not remove these authentication
limitations, and a host mount needs its own transport policy.

## Security-sensitive changes

Changes to the ceremony, pairing/claim, token scoping, the deposit route, capsule storage, or operator and custodian flows require tests for both the expected path and the relevant
failure or attack path, plus a description of the trust boundary in the pull
request.

## Related documents

- [CONTRIBUTING.md](CONTRIBUTING.md) — contribution and review requirements
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — community expectations
- [README.md](README.md) — what this project is and how to run it

