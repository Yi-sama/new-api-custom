#!/usr/bin/env bash
set -euo pipefail
umask 077
root=/opt/us-whitelist/test-new-api-20261002-174631
release="$(cd -- "$(dirname -- "$0")" && pwd)"
id="$(basename "$release")"
backup="$root/backups/$id"
target=risuai-test-20261003
[[ "$release" == "$root/releases/$id" && "$id" =~ ^risuai-entry-[0-9-]+$ ]]
[[ "$(hostname)" == ser021360749320 && "$(id -un)" == root ]]
[[ "$(docker inspect -f '{{.Id}}' "$target")" == "${1:?Expected current RisuAI container ID}" ]]
[[ ! -e "$backup" ]]
exec 9>"$root/.video-studio-deploy.lock"
flock -n 9
cd "$release"
sha256sum -c SHA256SUMS >/dev/null
previous=(docker compose -p new-api-test-20261002-174631)
configs="$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$target")"
IFS=',' read -ra paths <<< "$configs"
for file in "${paths[@]}"; do
  [[ "$file" == "$root/"* && -f "$file" ]]
  previous+=(-f "$file")
done
next=("${previous[@]}" -f "$release/compose.entry.json")
mkdir -m 700 "$backup"
printf '%s\n' "${paths[@]}" > "$backup/previous-config-files.txt"
"${previous[@]}" config --format json > "$backup/compose.before.json"
"${next[@]}" config --format json > "$backup/compose.after.json"
python3 - "$backup/compose.before.json" "$backup/compose.after.json" "$release" <<'PY'
import json,sys
a,b=(json.load(open(p)) for p in sys.argv[1:3])
old=[v for v in a["services"]["risuai-test"]["volumes"] if v["target"]=="/app/dist"]
new=[v for v in b["services"]["risuai-test"]["volumes"] if v["target"]=="/app/dist"]
assert len(old)==len(new)==1
assert new[0]["source"]==sys.argv[3]+"/dist" and new[0]["read_only"]
new[0]["source"]=old[0]["source"]
assert a==b
print("RISUAI_FRONTEND_SOURCE_ONLY=PASS")
PY
mapfile -t protected < <(docker ps -q | grep -v "^${1:0:12}$")
docker inspect "${protected[@]}" > "$backup/protected.before.json"
docker inspect "$target" > "$backup/risuai.before.json"
tar -czf "$backup/risuai-data.tgz" -C "$root" risuai-runtime
switched=0
recover() {
  code=$?
  if [[ "$code" != 0 && "$switched" == 1 ]]; then
    docker logs --tail 60 "$target" > "$backup/failed-startup.log" 2>&1 || true
    if ! "${previous[@]}" up -d --no-deps --pull never --no-build risuai-test; then
      echo "Frontend rollback failed; inspect $backup" >&2
    fi
  fi
  exit "$code"
}
trap recover EXIT
switched=1
"${next[@]}" up -d --no-deps --pull never --no-build risuai-test
for i in $(seq 1 30); do
  health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$target")"
  [[ "$health" != healthy ]] || break
  [[ "$(docker inspect -f '{{.RestartCount}}' "$target")" == 0 ]]
  sleep 2
done
[[ "$health" == healthy ]]
curl -fsS -H 'Host: 156.238.252.115:32796' http://127.0.0.1:32796/ >/dev/null
docker inspect "${protected[@]}" > "$backup/protected.after.json"
python3 - "$backup/protected.before.json" "$backup/protected.after.json" <<'PY'
import json,sys
def identity(p):
    return {c["Name"]:(c["Id"],c["State"]["StartedAt"]) for c in json.load(open(p))}
assert identity(sys.argv[1])==identity(sys.argv[2])
print("ALL_OTHER_RUNNING_CONTAINERS=UNCHANGED")
PY
switched=0
printf 'FRONTEND=%s\nBACKUP=%s\n' "$release" "$backup"
