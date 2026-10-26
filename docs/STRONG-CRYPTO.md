# Supported strong-key configuration

[Overview](../README.md) · [Installation](../INSTALL.md) · [Replication](DEPOSITS.md) · [Security model](RECOVERY-AND-SECURITY.md)

This configuration covers the server, CLI, OIDC and offsite replication. It
preserves the blind-store boundary: KyRecovery never opens, decrypts or re-seals
product capsules. It is not FIPS certification or evidence of a live rollout.

## Prepare existing installations

Retain the database, capsule directory, `secret.key` (or the existing
`KYRECOVERY_SECRET_KEY`), audit ledger/anchor, recovery public keys and custodian
cards, paired product records and replication destinations. The master key is
exactly 32 bytes and derives purpose-separated credential, audit and pairing
keys. Losing or replacing it breaks sealed credentials and audit bindings; it
is not a remedy for an incompatible remote certificate or SSH key.

The root `ky-primitives` dependency and recovery format are unchanged. The
independently versioned `offsite` module is pinned to merged source `bae5c14dba4c`
(`v0.2.3-0.20261026050236-bae5c14dba4c`) containing the SSH key/algorithm guards.
`offsite/v0.2.2` predates them. Absolute-directory SFTP retains a compatibility
client with the same minimums; its destination and staging-cleanup rules remain.

Before upgrading:

- Replace SSH host/client/CA RSA keys below 2048 bits and peers requiring SHA-1
  key exchange/signatures, CBC or truncated insecure MACs. Strong Ed25519 and
  supported ECDSA keys remain available. A pin never exempts an undersized key.
  Confirm any replacement host fingerprint out of band and update its stored
  pin deliberately; do not auto-accept a different server.
- Retire weak RSA JWKS members at the IdP, including unused weak keys beside a
  strong key. Newly fetched RSA keys below 2048 bits are refused in every mode.
- Configure the IdP for OIDC authorization-code flow with PKCE and an ID token.
  Missing ID tokens now fail in every mode; there is no userinfo-only fallback.
  Preserve independent local administrator access while repairing an IdP.
- For S3/R2/MinIO, use a provider-issued strong signing secret. The process
  profile refuses fewer than 16 literal bytes; 32 random bytes or the provider's
  normal high-entropy credential is recommended. Length alone does not prove
  entropy. OAuth client secrets, SSH passwords and bearer tokens have separate
  authentication rules; do not reuse suite administrator passwords.

These SSH/OIDC minimums can refuse an old peer even without selecting the
optional TLS profile. A refused connection leaves local capsules, keys,
credentials, target paths and names intact. Migrate the endpoint instead of
weakening verification. Preserve existing offsite objects for recovery.

## Enable the process profile

Use the Go toolchain selected by `go.mod`/the pinned Docker builder and set
`GODEBUG=fips140=on` before starting every server and remote CLI process. Keep
other needed GODEBUG settings and set its `fips140` entry to `on`. Use `on`, not
Go's diagnostic `only` mode; Argon2id and browser ceremony/recovery algorithms
retain their existing formats.

For a process installation, keep its existing arguments/environment and add:

```sh
export GODEBUG=fips140=on
./kyrecovery serve --data-dir /path/to/existing/data --cookie-secure=true
```

For Docker Compose, edit the existing protected `.env` to add or update:

```dotenv
GODEBUG=fips140=on
```

The existing Compose file already forwards `.env` through `env_file`. Preserve
all other settings, the `COMPOSE_FILE` chain, image/digest or source-build
selection, data mounts and optional LAN-DNS/restic overlays. Keep `.env` mode
0600; do not replace it with an example. Review resolved configuration privately
(it can contain secrets), then recreate the service with the same chain. A
source install retains its build overlay and rebuilds; a published image must
contain these fixes before enabling the profile. No new overlay is required.

For systemd, retain the installed invocation and add
`Environment=GODEBUG=fips140=on` in a drop-in, preserving other needed settings;
reload the unit and restart. A host environment change does not reconfigure an
already running container or process.

## Boundaries and enforcement

| Boundary | Supported configuration |
| --- | --- |
| Server keyring | Existing exact 32-byte master key, AES-256-GCM credential envelopes, HKDF-SHA256 purpose separation and 32-byte audit/pairing keys. Do not replace the retained key. |
| Managed signed directory | Use an independent strong HMAC secret with at least 16 literal bytes. The event boundary already enforces that floor. Retain protected credential paths and coordinate rotation with the sender. |
| OIDC | Direct HTTPS issuer, authorization, token, JWKS and optional userinfo metadata URLs without embedded credentials. The profile refuses discovery failures, plaintext endpoints and redirects. Every login requires a verified ID token; issuer/audience/expiry/signature/nonce and managed subject grants remain. RSA JWKS members need 2048 bits in all modes. The verifier retains its cache/rotation behavior. |
| S3/R2/MinIO | Verified HTTPS only, no redirects and a strong signing secret of at least 16 bytes under the profile. Both ordinary and retained virtual-host destination shapes remain. Preserve exact bucket/prefix concatenation and object names. |
| SFTP | Both account-relative shared-library and absolute-directory compatibility paths use `ssh.SupportedAlgorithms()` and validate peer/client/certificate-CA keys. RSA needs 2048 bits; ECDSA needs at least 224 bits (the supported SSH curves are P-256/P-384/P-521); Ed25519 has its fixed size. RSA user authentication offers SHA-2 only. Pin the host fingerprint after checking it independently. |
| SMB/NTLM | Refused by target tests and transfers under the profile, before credentials are sent. Stored records and existing replicas remain; failed syncs are reported and audited. Normal mode retains the explicit compatibility option with the risks below. |
| Local destinations | Existing local file/mount paths remain. Host-mounted storage must have its own strong transport/authentication policy; a local path does not prove that the host uses secure SMB/NFS/SSH. |
| Browser/product/CLI HTTPS | KyRecovery serves internal HTTP. Use a trusted TLS proxy and restrict backend access to that proxy. Configure its certificates and TLS policy independently; run the remote Go CLI with the process profile. Ordinary loopback HTTP is local IPC, not authenticated TLS. |
| Ceremony and deposited containers | The browser generates/splits the fixed suite recovery key and posts only public material. Products seal capsules before deposit. Server transport policy does not grant it a private key/share or change historical capsule decryptability. |

Go's selected TLS policy filters verified certificate chains, including trusted
roots/intermediates: RSA-2048+ and approved P-256/P-384/P-521/fixed Ed25519 keys.
Private trust roots do not waive the floor. Keep normal certificate and hostname
verification enabled; do not use plaintext S3/OIDC endpoints or insecure custom
transports to work around a refusal. The issuer client's connect-time refusal
of unspecified, link-local and multicast destinations remains.

For the HTTPS edge, use TLS 1.2/1.3 and a verified RSA-2048+ or P-256+ chain. An
Nginx/OpenSSL proxy's relevant settings are:

```nginx
ssl_protocols TLSv1.2 TLSv1.3;
ssl_ciphers 'DEFAULT@SECLEVEL=2';
```

Configure paths/routing for the actual proxy and verify it before rollout.
Go's setting does not configure browsers, an external proxy or a separate
rest-server. Keep secure cookies and trusted-proxy restrictions as documented.

## Explicit SMB compatibility outside the profile

SMB accepts `host`, `host:port`, `\\host\share\dir`, `//host/share/dir` or
`smb://host/share/dir`; path fields fill an otherwise blank share/directory.
The username may be `DOMAIN\user`.

SMB targets are never enabled by default. Selecting the legacy SMB type in
normal mode enables NTLMv2, including MD4/HMAC-MD5 and RC4-based session-key
handling in the SMB dependency. These mechanisms have known limitations;
transport signing does not repair the password-derived authentication or provide
forward secrecy. A captured exchange can enable offline password guessing, and
relay/impersonation risks depend on the server/network authentication policy.

The clients require message signing and refuse unsigned/no-key guest sessions.
Use a dedicated account with a unique strong password, restrict it to the backup
share, restrict the network and validate the actual server. Never reuse a suite
administrator credential. Capsules remain sealed, but that does not protect the
SMB credential or establish remote availability/integrity by itself.

Prefer pinned SFTP or verified HTTPS S3. A host-mounted share is an alternative
only when the host independently enforces appropriate authentication/transport
(for example Kerberos-based SMB); mounting alone does not remove NTLM. Enabling
the strong process profile intentionally refuses SMB operations, retaining the
records for deliberate migration or an informed compatibility configuration.

## Verification and rollout

Run ordinary and configured full race suites, build/vet, the existing fuzz
counts, ceremony/WASM reproduction and vectors, UI-asset checks and Docker
checks. Real Samba fixtures verify ordinary compatibility; configured tests
verify refusal without rewriting records or local capsules. SSH fixtures check
both paths, pins, strong acceptance, weak-key/algorithm refusal and SHA-2 user
authentication. OIDC fixtures verify signed-token floors, retained JWKS caching,
unsafe metadata/redirect refusal and weak trusted-root TLS rejection.

Before deployment, verify the actual IdP and proxy, normal local/SSO sign-in,
product pairing/deposit/receipt, audit-chain verification, each selected offsite
replica and an offline restore with the appropriate custodian cards. Local
fixtures do not prove real card custody, external endpoint policy or an actual
restore. Keep the pre-upgrade database, keys and sealed objects until those
checks pass. Never restore/decrypt a product capsule inside this server.

Primary reference: [Go FIPS runtime mode and platform limits](https://go.dev/doc/security/fips140).
This runtime policy does not certify the whole application as FIPS compliant.
