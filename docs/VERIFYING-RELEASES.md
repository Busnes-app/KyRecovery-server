# Verifying releases

[Product overview](../README.md) · [Recovery and security](RECOVERY-AND-SECURITY.md) · [Operations](OPERATIONS.md)

A KyRecovery release is the container image `ghcr.io/busnes-app/kyrecovery-server:X.Y.Z` (or
`X.Y.Z.N` for a dependency-only release). This page shows how to check that an image was built by
this repository's release workflow from the tagged commit, and how to rebuild its binary yourself
and get the same bytes. Commands run from an empty directory unless they say otherwise.

## What is signed, and by whom

- `release-image.yml` builds each release from its tag and publishes a SLSA build-provenance
  attestation for the image digest through GitHub's Sigstore integration. It refuses to build a
  commit on which the `go`, `security` and `docker` CI jobs did not pass, and it verifies the
  attestation before it moves `:stable`, so `:stable` never points at an unattested image.
- Every push to `main` publishes `:<commit sha>`, attested by `ci.yml` on `main`, and moves
  `:latest`. The pin block at the top of [`docker-compose.yml`](../docker-compose.yml) verifies
  those.
- There is no long-lived signing key to download. The signature is tied to a workflow identity,
  and that identity is the thing you check:
  `https://github.com/Busnes-app/KyRecovery-server/.github/workflows/release-image.yml@refs/tags/v<version>`.
  `gh` fetches Sigstore's public trust root itself.
- Each image also carries an SPDX software bill of materials made by BuildKit. The SBOM sits in
  the image index and is **not** separately signed; trust it as far as you trust the digest you
  verified.

## Verify a release image

You need `gh` (logged in) and `docker` with buildx. Name the version yourself, and verify the
version tag, not `:stable`: tags can move, so the check resolves the digest first and then requires
the attestation to name the commit the release tag points at.

```bash
v=<release version, e.g. 0.4.3>
sha=$(gh api repos/Busnes-app/KyRecovery-server/commits/refs/tags/v$v --jq .sha) \
  && d=$(docker buildx imagetools inspect ghcr.io/busnes-app/kyrecovery-server:$v --format '{{.Manifest.Digest}}') \
  && gh attestation verify "oci://ghcr.io/busnes-app/kyrecovery-server@$d" --repo Busnes-app/KyRecovery-server \
       --cert-identity https://github.com/Busnes-app/KyRecovery-server/.github/workflows/release-image.yml@refs/tags/v$v \
  && [ "$(gh attestation verify "oci://ghcr.io/busnes-app/kyrecovery-server@$d" --repo Busnes-app/KyRecovery-server \
       --cert-identity https://github.com/Busnes-app/KyRecovery-server/.github/workflows/release-image.yml@refs/tags/v$v \
       --format json --jq '.[0].verificationResult.statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit')" = "$sha" ] \
  && echo "OK: ghcr.io/busnes-app/kyrecovery-server@$d is release v$v, built from $sha"
```

Then run that digest, not the tag. The release block at the top of
[`docker-compose.yml`](../docker-compose.yml) runs the same check and writes
`KYRECOVERY_IMAGE=ghcr.io/busnes-app/kyrecovery-server@<digest>` into `.env` safely.

Any failure means: do not run that image. A different workflow, a branch instead of the tag,
another tag, or another repository all fail the `--cert-identity` or `--repo` check.

To read the SBOM:

```bash
docker buildx imagetools inspect ghcr.io/busnes-app/kyrecovery-server@$d --format '{{ json .SBOM }}'
```

## Rebuild the binary and compare

Releases after 0.4.3 are built with `-trimpath`, so the same commit, Go version and version
string give the same binary on any machine and in any directory. CI checks this on every push.
Images up to 0.4.3 recorded their build directory (`/app`) and Go's install path in the binary,
so a build elsewhere does not compare with them.

```bash
v=<release version>
git clone --branch v$v https://github.com/Busnes-app/KyRecovery-server.git kyrecovery-v$v && cd kyrecovery-v$v
go_version=$(sed -n 's/^FROM golang:\([0-9.]*\)-.*/\1/p' Dockerfile)
scripts/build-wasm.sh
GOTOOLCHAIN=go$go_version CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath \
  -ldflags="-s -w -X github.com/Busnes-app/kyrecovery-server/internal/appversion.release=$v" \
  -o kyrecovery-from-source cmd/kyrecovery/main.go
cid=$(docker create ghcr.io/busnes-app/kyrecovery-server@$d) \
  && docker cp "$cid:/app/kyrecovery" kyrecovery-from-image && docker rm "$cid" >/dev/null
sha256sum kyrecovery-from-source kyrecovery-from-image
```

The two hashes must match. Build `cmd/kyrecovery/main.go` exactly as written: the Dockerfile
names the file, not the package, and `./cmd/kyrecovery` gives a different (equally repeatable)
binary. `GOARCH=amd64` matches the published image; `podman` works in place of `docker` for the
last two commands. The image layers around the binary (Alpine, CA certificates, time zones) are
not rebuilt bit for bit; the binary is what you are checking.

### The ceremony module inside the binary

The server embeds `internal/server/static/wasm/ceremony.wasm`, the module `/admin/ceremony`
serves. `scripts/build-wasm.sh` above builds it from this commit's `cmd/ceremony-wasm` (it pins
the Go version in `go.mod`'s `go` line), so matching hashes prove the image serves the module this
commit's source produces. Older releases committed the module instead; at their tags the same
script rebuilds those bytes, which CI checked on every push.
