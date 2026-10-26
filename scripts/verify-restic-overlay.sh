#!/usr/bin/env bash
# Proves docker-compose.restic.yml end to end in a disposable project:
#   - rest-server writes repositories KyRecovery's uid can read but not write;
#   - real restic backups are scanned and reported through /api/backups/status;
#   - the htpasswd/TLS directory is invisible to KyRecovery and its mount is read-only;
#   - the base compose file still runs, and is healthy, without the overlay.
# Uses its own project names, random loopback ports and a temp directory; touches nothing else.
set -euo pipefail

repo=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
project=kyr-restic-verify-$$
image=kyrecovery-server:restic-verify-$$
restic_image=restic/restic:0.19.1@sha256:136600b6ff6843d61d355f7f71f460a166429f35de6fd11b568fece3c9a4d510
rs_image=$(sed -n 's/^ *image: \(restic\/rest-server.*\)$/\1/p' "$repo/docker-compose.restic.yml")

files=(-f "$repo/docker-compose.yml" -f "$repo/docker-compose.restic.yml" -f "$work/test.yml")
base_files=(-f "$repo/docker-compose.yml" -f "$work/base/test.yml")
# Each project resolves ./data against its own directory, so the two never share a database.
compose() { docker compose -p "$project" --project-directory "$work" --env-file "$work/.env" "${files[@]}" "$@"; }
compose_base() { docker compose -p "$project-base" --project-directory "$work/base" --env-file "$work/base/.env" "${base_files[@]}" "$@"; }
cleanup() {
	compose down -v >/dev/null 2>&1 || true
	compose_base down -v >/dev/null 2>&1 || true
	docker image rm "$image" >/dev/null 2>&1 || true
	# rest-server and KyRecovery own files in here; remove them as root inside a container.
	docker run --rm -v "$work:/w" alpine sh -c 'rm -rf /w/* /w/.env' >/dev/null 2>&1 || true
	rmdir "$work" 2>/dev/null || true
}
trap cleanup EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }

docker build -q -t "$image" "$repo" >/dev/null
mkdir -p "$work/base" "$work/restic" "$work/config/tls"
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes -days 1 -subj /CN=rest-server \
	-addext subjectAltName=DNS:rest-server -keyout "$work/config/tls/key.pem" -out "$work/config/tls/cert.pem" 2>/dev/null
docker run --rm --entrypoint htpasswd "$rs_image" -B -bn alice alice-pass >"$work/config/htpasswd"
chmod -R a+rX "$work/config"
docker run --rm -v "$work/restic:/d" alpine sh -c 'chown 10002:10001 /d && chmod 0750 /d'

cat >"$work/.env" <<EOF
KYRECOVERY_IMAGE=$image
RESTIC_DATA_DIR=$work/restic
REST_SERVER_CONFIG_DIR=$work/config
EOF
# Test-only: no fixed names or host ports, so nothing running on this machine is disturbed.
cat >"$work/test.yml" <<'EOF'
services:
  kyrecovery:
    container_name: !reset null
    ports: !override ["127.0.0.1::8095"]
    environment:
      - KY_ADMIN_INITIAL_PASSWORD=verify-admin-pass-1
EOF
cat >>"$work/test.yml" <<'EOF'
  rest-server:
    container_name: !reset null
    ports: !override ["127.0.0.1::8000"]
EOF
sed '/^  rest-server:/,$d' "$work/test.yml" >"$work/base/test.yml"
echo "KYRECOVERY_IMAGE=$image" >"$work/base/.env"

compose up -d --quiet-pull >/dev/null

restic() {
	docker run --rm --network "${project}_default" -e RESTIC_PASSWORD=repo-pass \
		-v "$work/config/tls/cert.pem:/cert.pem:ro" -v "$repo/go.mod:/src/go.mod:ro" "$restic_image" \
		-r rest:https://alice:alice-pass@rest-server:8000/alice/kynotes --cacert /cert.pem "$@" >/dev/null
}
for _ in $(seq 1 30); do restic init 2>/dev/null && break; sleep 1; done
restic backup /src
restic backup /src

# Every file rest-server wrote: owned 10002:10001, group-readable, never group-writable or world-readable.
modes=$(docker run --rm -v "$work/restic:/d:ro" alpine find /d -mindepth 1 -exec stat -c '%u:%g %a %n' {} +)
echo "$modes" | awk '$1 != "10002:10001" { exit 1 }' || fail "unexpected owner: $modes"
echo "$modes" | awk '{ g = substr($2, length($2) - 1, 1); o = substr($2, length($2), 1); if ((g != "4" && g != "5") || o != "0") exit 1 }' ||
	fail "unexpected modes: $modes"
echo "rest-server file modes (count, owner, mode):"
echo "$modes" | awk '{ print $1, $2 }' | sort | uniq -c
docker run --rm --user 10001:10001 -v "$work/restic:/d" alpine sh -c 'cat /d/alice/kynotes/config >/dev/null' ||
	fail "uid 10001 cannot read the repository"
if docker run --rm --user 10001:10001 -v "$work/restic:/d" alpine touch /d/alice/kynotes/written-by-kyrecovery 2>/dev/null; then
	fail "uid 10001 can write the repository"
fi

# The scanner runs at start; restart so it sees the backups made above.
compose restart kyrecovery >/dev/null
port=$(compose port kyrecovery 8095 | sed 's/.*://')
base="http://127.0.0.1:$port"
for _ in $(seq 1 60); do curl -fs "$base/healthz" >/dev/null && break; sleep 1; done

if compose exec -T kyrecovery ls /etc/rest-server >/dev/null 2>&1; then
	fail "KyRecovery can see the rest-server config directory"
fi
if compose exec -T kyrecovery sh -c 'ls -a /app/restic' | grep -q htpasswd; then
	fail "htpasswd is inside the scanned directory"
fi
if compose exec -T -u 0 kyrecovery touch /app/restic/x 2>/dev/null; then
	fail "the restic mount is writable inside KyRecovery"
fi

jar="$work/cookies"
curl -fs -c "$jar" -H 'Content-Type: application/json' -d '{"username":"admin","password":"verify-admin-pass-1"}' \
	"$base/api/auth/login/local" >/dev/null || fail "login"
status=$(curl -fs -b "$jar" "$base/api/backups/status")
python3 -I -c '
import json, sys
s = json.loads(sys.argv[1])
assert s["scan"]["enabled"] and not s["scan"].get("last_error"), s["scan"]
repos = {r["path"]: r for r in s["report"]["unassigned"]}
r = repos.get("alice/kynotes")
assert r and r["snapshot_count"] == 2 and r["key_count"] == 1 and r["size_bytes"] > 0 and not r.get("scan_error"), repos
' "$status" || fail "status: $status"

# Without the overlay the base file still runs and is healthy.
compose_base up -d >/dev/null
port=$(compose_base port kyrecovery 8095 | sed 's/.*://')
for _ in $(seq 1 60); do curl -fs "http://127.0.0.1:$port/healthz" >/dev/null && break; sleep 1; done
health=$(curl -fs "http://127.0.0.1:$port/healthz") || fail "base install not healthy"
python3 -I -c '
import json, sys
h = json.loads(sys.argv[1])
assert h["status"] == "ok" and {c["name"]: c["status"] for c in h["checks"]}.get("backups") == "ok", h
' "$health" || fail "base health: $health"

echo "restic overlay verified: modes, read-only access, scan, credential isolation, base install"
