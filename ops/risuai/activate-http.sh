#!/usr/bin/env bash
set -euo pipefail
umask 077
root=/opt/us-whitelist/test-new-api-20261002-174631
release="$(cd -- "$(dirname -- "$0")" && pwd)"
id="$(basename "$release")"
target=new-api-test-20261002-174631
backup_id="${2:-$id}"
[[ "$backup_id" == "$id" || "$backup_id" == "$id-configured-channels" ]]
backup="$root/backups/$backup_id"
runtime="$root/risuai-runtime"
nginx_file="/etc/nginx/conf.d/$id.conf"
[[ "$release" == "$root/releases/"tavern-http-* ]]
[[ "$(hostname)" == ser021360749320 && "$(id -un)" == root ]]
[[ "$(docker inspect -f '{{.Id}}' "$target")" == "${1:?Expected current test container ID}" ]]
[[ "$(awk '/MemAvailable/ {print $2}' /proc/meminfo)" -gt 1500000 ]]
[[ ! -e "$backup" && ! -e "$nginx_file" && ! -e "$runtime" ]]
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
next=("${previous[@]}" -f "$release/compose.tavern.json")
[[ "$(docker inspect -f '{{.Id}}' node:24-alpine)" == sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 ]]
mkdir -m 700 "$backup"
printf '%s\n' "${paths[@]}" > "$backup/previous-config-files.txt"
"${previous[@]}" config --format json > "$backup/compose.before.json"
"${next[@]}" config --format json > "$backup/compose.after.json"
python3 - "$backup/compose.before.json" "$backup/compose.after.json" "$release" <<'PY'
import json,sys
a,b=(json.load(open(p)) for p in sys.argv[1:3])
assert set(b["services"])-set(a["services"])=={"risuai-test"}
assert set(a["services"])-set(b["services"])==set()
s=b["services"].pop("risuai-test")
assert s["network_mode"]=="host" and s["read_only"]
assert s["environment"]["NEW_API_URL"]=="http://127.0.0.1:32795"
assert s["environment"]["NEW_API_PUBLIC_URL"]=="http://156.238.252.115:32796"
v={x["target"]:x for x in b["services"]["new-api-test"]["volumes"]}
assert v["/new-api"]["source"]==sys.argv[3]+"/new-api" and v["/new-api"]["read_only"]
b["services"]["new-api-test"]["volumes"]=a["services"]["new-api-test"]["volumes"]
b["services"]["new-api-test"]["environment"]["VERSION"]=a["services"]["new-api-test"]["environment"]["VERSION"]
assert a==b
print("COMPOSE_SCOPE=PASS")
PY
protected=(new-api old-new-api sub2api-postgres redis sub2api codex-proxy-rs-postgres-1 new-api-test-redis-20261002-174631)
docker inspect "${protected[@]}" > "$backup/protected.before.json"
docker inspect "$target" > "$backup/test.before.json"
for file in "${paths[@]}"; do cp -a --parents "$file" "$backup"; done
cp -a /etc/nginx/nginx.conf "$backup/nginx.conf"
ufw status numbered > "$backup/ufw.before.txt"
nginx -T > "$backup/nginx.before.txt" 2>&1
docker exec sub2api-postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" -d newapi_test_20261002174631 -Fc' > "$backup/test-db.dump"
docker exec -i sub2api-postgres pg_restore --list < "$backup/test-db.dump" > "$backup/test-db.list"
docker exec -e SNAPSHOT_NAME="$id.rdb" new-api-test-redis-20261002-174631 sh -c 'export REDISCLI_AUTH="$REDIS_PASSWORD"; redis-cli --rdb "/tmp/$SNAPSHOT_NAME"; redis-check-rdb "/tmp/$SNAPSHOT_NAME"' > "$backup/test-redis.check.txt" 2>&1
docker cp "new-api-test-redis-20261002-174631:/tmp/$id.rdb" "$backup/test-redis.rdb"
docker exec sub2api-postgres sh -c 'exec psql -X -At -U "$POSTGRES_USER" -d newapi_test_20261002174631 -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM tasks; SELECT count(*) FROM studio_jobs WHERE status IN ('\''queued'\'','\''running'\'');"' > "$backup/safety-counts.txt"
[[ "$(cat "$backup/safety-counts.txt")" == $'0\n0' ]]
docker exec "$target" sh -c 'test "$CHANNEL_TEST_ENABLED" = false && test "$CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED" = false'
tar -czf "$backup/test-app-data.tgz" -C "$root" app-data logs
cp -a "$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(next(v["source"] for v in d["services"]["new-api-test"]["volumes"] if v["target"]=="/new-api"))' "$backup/compose.before.json")" "$backup/new-api.before"
sha256sum "$backup/test-db.dump" "$backup/test-redis.rdb" "$backup/test-app-data.tgz" "$backup/new-api.before" > "$backup/SHA256SUMS"
mkdir -m 700 "$runtime" "$runtime/data" "$runtime/save"
chown -R 1000:1000 "$runtime"
printf '%s\n' "$backup_id" > "$release/active-backup.txt"
switched=0
proxy_added=0
firewall_added=0
recover() {
  code=$?
  if [[ "$code" != 0 ]]; then
    if [[ "$proxy_added" == 1 && -e "$nginx_file" ]]; then
      mv "$nginx_file" "$backup/nginx.failed.conf"
      nginx -t && systemctl reload nginx || true
    fi
    if [[ "$firewall_added" == 1 ]]; then ufw delete allow 32796/tcp || true; fi
    if [[ "$switched" == 1 ]]; then
      docker logs --tail 100 "$target" > "$backup/failed-startup.log" 2>&1 || true
      "${next[@]}" stop risuai-test || true
      "${previous[@]}" up -d --no-deps --pull never --no-build new-api-test || true
    fi
    printf 'Activation failed; previous release selected. Backups retained: %s\n' "$backup" >&2
  fi
  exit "$code"
}
trap recover EXIT
switched=1
"${next[@]}" up -d --no-deps --pull never --no-build new-api-test
"${next[@]}" up -d --no-deps --pull never --no-build risuai-test
for i in $(seq 1 60); do
  a="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$target")"
  b="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' risuai-test-20261003)"
  [[ "$a" != healthy || "$b" != healthy ]] || break
  [[ "$(docker inspect -f '{{.RestartCount}}' "$target")" == 0 ]]
  [[ "$(docker inspect -f '{{.RestartCount}}' risuai-test-20261003)" == 0 ]]
  sleep 2
done
[[ "$a" == healthy && "$b" == healthy ]]
curl -fsS http://127.0.0.1:32795/api/status >/dev/null
curl -fsS http://127.0.0.1:32830/ >/dev/null
install -m 644 "$release/nginx.tavern.conf" "$nginx_file"
proxy_added=1
nginx -t
systemctl reload nginx
ufw allow 32796/tcp comment 'risuai-private-test-http'
firewall_added=1
curl -fsS -H 'Host: 156.238.252.115:32796' http://127.0.0.1:32796/ >/dev/null
docker inspect "${protected[@]}" > "$backup/protected.after.json"
python3 - "$backup/protected.before.json" "$backup/protected.after.json" <<'PY'
import json,sys
def identity(p):
    return {c["Name"]:(c["Id"],c["State"]["StartedAt"]) for c in json.load(open(p))}
assert identity(sys.argv[1])==identity(sys.argv[2])
print("PROTECTED_CONTAINERS=UNCHANGED")
PY
switched=0
proxy_added=0
firewall_added=0
printf 'RELEASE=%s\nBACKUP=%s\n' "$release" "$backup"
