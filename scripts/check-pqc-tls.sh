#!/usr/bin/env bash
# Refuse classical TLS overrides that downgrade Go's X25519MLKEM768 default, and
# prove KyRecovery's outbound TLS client negotiates hybrid PQ key exchange.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# 1. Refuse CurvePreferences in non-test Go code (overrides Go's hybrid default).
bad_curves=$(
  git ls-files -z --cached --others --exclude-standard -- '*.go' ':!*_test.go' \
    | sed -z 's|^|./|' \
    | xargs -0 -r grep -nH -F -- 'CurvePreferences' || true
)
if [ -n "$bad_curves" ]; then
  echo "Classical TLS CurvePreferences override found in non-test Go code:" >&2
  echo "$bad_curves" >&2
  exit 1
fi

# 2. Refuse tlsmlkem overrides in non-test code, go.mod, workflows, Dockerfiles, and Compose files.
bad_mlkem=$(
  git ls-files -z --cached --others --exclude-standard -- \
      '*.go' ':!*_test.go' \
      '*.sh' ':!scripts/check-pqc-tls.sh' \
      'go.mod' \
      '.github/workflows/*.yml' '.github/workflows/*.yaml' \
      '*Dockerfile*' \
      'docker-compose*.yml' 'docker-compose*.yaml' \
      'compose*.yml' 'compose*.yaml' \
    | sed -z 's|^|./|' \
    | xargs -0 -r grep -nH -F -- 'tlsmlkem' || true
)
if [ -n "$bad_mlkem" ]; then
  echo "tlsmlkem override found in non-test code, go.mod, workflow, Dockerfile, or Compose file:" >&2
  echo "$bad_mlkem" >&2
  exit 1
fi

# 3. Prove KyRecovery's outbound TLS client negotiates X25519MLKEM768 in both modes.
go test -race -count=1 -run '^TestOutboundPQTLSHandshake$' ./internal/auth
GODEBUG=fips140=on go test -race -count=1 -run '^TestOutboundPQTLSHandshake$' ./internal/auth
