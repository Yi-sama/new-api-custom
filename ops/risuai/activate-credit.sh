#!/usr/bin/env bash
set -euo pipefail
umask 077
root=/opt/us-whitelist/test-new-api-20261002-174631
release="$(cd -- "$(dirname -- "$0")" && pwd)"
id="$(basename "$release")"
backup="$root/backups/$id"
main=new-api-test-20261002-174631
tavern=risuai-test-20261003
[[ "$release" == "$root/releases/$id" && "$id" =~ ^tavern-credit-[0-9-]+$ ]]
[[ "$(hostname)" == ser021360749320 && "$(id -un)" == root ]]
exec 9>"$root/.video-studio-deploy.lock"
flock -n 9
[[ "$(docker inspect -f '{{.Id}}' "$main")" == "${1:?Expected main container ID}" ]]
[[ "$(docker inspect -f '{{.Id}}' "$tavern")" == "${2:?Expected Tavern container ID}" ]]
[[ "$(awk '/MemAvailable/ {print $2}' /proc/meminfo)" -gt 1500000 ]]
[[ ! -e "$backup" ]]
database_user="$(docker inspect "$main" | python3 -c '
import json,sys,urllib.parse
env=dict(value.split("=",1) for value in json.load(sys.stdin)[0]["Config"]["Env"])
dsn=urllib.parse.urlparse(env["SQL_DSN"])
assert dsn.username=="newapi_test_20261002174631"
assert dsn.path=="/newapi_test_20261002174631"
print(dsn.username)
')"
cd "$release"
sha256sum -c SHA256SUMS >/dev/null
previous=(docker compose -p new-api-test-20261002-174631)
configs="$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$tavern")"
IFS=',' read -ra paths <<< "$configs"
[[ "${#paths[@]}" == 6 ]]
for file in "${paths[@]}"; do
  [[ "$file" == "$root/"* && -f "$file" ]]
  previous+=(-f "$file")
done
main_configs="$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$main")"
IFS=',' read -ra main_paths <<< "$main_configs"
for file in "${main_paths[@]}"; do
  [[ ",$configs," == *",$file,"* ]]
done
next=("${previous[@]}" -f "$release/compose.credit.json")
mkdir -m 700 "$backup"
printf '%s\n' "${paths[@]}" > "$backup/previous-config-files.txt"
"${previous[@]}" config --format json > "$backup/compose.before.json"
"${next[@]}" config --format json > "$backup/compose.after.json"
python3 - "$backup/compose.before.json" "$backup/compose.after.json" "$release" <<'PY'
import json,sys
a,b=(json.load(open(p)) for p in sys.argv[1:3])
release=sys.argv[3]
for name,targets in [
    ("new-api-test",{"/new-api":release+"/new-api"}),
    ("risuai-test",{"/app/dist":release+"/dist",
                   "/app/server.cjs":release+"/runtime/server.cjs",
                   "/app/server.cjs.LEGAL.txt":release+"/runtime/server.cjs.LEGAL.txt"})]:
    before=a["services"][name]
    after=b["services"][name]
    assert after["environment"].pop("TAVERN_SERVICE_KEY_FILE")=="/run/tavern-service-key"
    if name=="new-api-test":
        assert after["environment"]["VERSION"].startswith("v1.0.0-rc.28-custom-tavern-credit-")
        after["environment"]["VERSION"]=before["environment"]["VERSION"]
    old={v["target"]:v for v in before["volumes"]}
    new={v["target"]:v for v in after["volumes"]}
    assert new["/run/tavern-service-key"]["read_only"]
    assert new["/run/tavern-service-key"]["source"]=="/opt/us-whitelist/test-new-api-20261002-174631/tavern-credit-runtime/service.key"
    for target,source in targets.items():
        assert new[target]["source"]==source and new[target]["read_only"]
    allowed=set(targets)|{"/run/tavern-service-key"}
    unchanged=[v for v in after["volumes"] if v["target"] not in allowed]
    unchanged.extend(v for v in before["volumes"] if v["target"] in allowed)
    after["volumes"]=sorted(unchanged,key=lambda v:v["target"])
    before["volumes"]=sorted(before["volumes"],key=lambda v:v["target"])
assert a==b
print("COMPOSE_SCOPE=PASS (binary, Tavern assets/runtime, backend proof only)")
PY
mapfile -t protected < <(docker ps -q | grep -v -e "^${1:0:12}$" -e "^${2:0:12}$")
docker inspect "${protected[@]}" > "$backup/protected.before.json"
docker inspect "$main" "$tavern" > "$backup/apps.before.json"
for file in "${paths[@]}"; do cp -a --parents "$file" "$backup"; done
docker exec sub2api-postgres sh -c 'exec pg_dump -U "$POSTGRES_USER" -d newapi_test_20261002174631 -Fc' > "$backup/test-db.dump"
docker exec -i sub2api-postgres pg_restore --list < "$backup/test-db.dump" > "$backup/test-db.list"
docker exec sub2api-postgres sh -c 'exec psql -X -At -U "$POSTGRES_USER" -d newapi_test_20261002174631 -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM tasks; SELECT count(*) FROM studio_jobs WHERE status IN ('\''queued'\'','\''running'\''); SELECT count(*),coalesce(sum(quota),0),coalesce(sum(used_quota),0),coalesce(sum(request_count),0) FROM users;"' > "$backup/counts.before.txt"
[[ "$(head -n 2 "$backup/counts.before.txt")" == $'0\n0' ]]
docker exec "$main" sh -c 'test "$CHANNEL_TEST_ENABLED" = false && test "$CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED" = false'
tar -czf "$backup/app-data.tgz" -C "$root" app-data logs risuai-runtime
docker exec -e SNAPSHOT_NAME="$id.rdb" new-api-test-redis-20261002-174631 sh -c 'export REDISCLI_AUTH="$REDIS_PASSWORD"; redis-cli --rdb "/tmp/$SNAPSHOT_NAME"; redis-check-rdb "/tmp/$SNAPSHOT_NAME"' > "$backup/redis-check.txt" 2>&1
docker cp "new-api-test-redis-20261002-174631:/tmp/$id.rdb" "$backup/test-redis.rdb" >/dev/null
sha256sum "$backup/test-db.dump" "$backup/app-data.tgz" "$backup/test-redis.rdb" > "$backup/SHA256SUMS"
if [[ -e "$root/tavern-credit-runtime" ]]; then
  [[ -d "$root/tavern-credit-runtime" && ! -L "$root/tavern-credit-runtime" ]]
  [[ "$(stat -c '%u:%g:%a' "$root/tavern-credit-runtime/service.key")" == 0:1000:440 ]]
  grep -Eq '^[[:xdigit:]]{64}$' "$root/tavern-credit-runtime/service.key"
  cp -a "$root/tavern-credit-runtime" "$backup/"
else
  mkdir -m 700 "$root/tavern-credit-runtime"
  openssl rand -hex 32 > "$root/tavern-credit-runtime/service.key"
  chown root:1000 "$root/tavern-credit-runtime/service.key"
  chmod 440 "$root/tavern-credit-runtime/service.key"
fi
docker exec -i -e TAVERN_DB_OWNER="$database_user" sub2api-postgres sh -c 'exec psql -X -U "$POSTGRES_USER" -d newapi_test_20261002174631 -v ON_ERROR_STOP=1 -v tavern_owner="$TAVERN_DB_OWNER"' < "$release/credit-schema.pg.sql" > "$backup/schema.log"
switched=0
recover() {
  code=$?
  if [[ "$code" != 0 && "$switched" == 1 ]]; then
    docker logs --tail 60 "$main" > "$backup/main-failed.log" 2>&1 || true
    docker logs --tail 60 "$tavern" > "$backup/tavern-failed.log" 2>&1 || true
    "${previous[@]}" up -d --no-deps --pull never --no-build new-api-test risuai-test || echo "Rollback failed; inspect $backup" >&2
  fi
  exit "$code"
}
trap recover EXIT
switched=1
"${next[@]}" up -d --no-deps --pull never --no-build new-api-test
"${next[@]}" up -d --no-deps --pull never --no-build risuai-test
for i in $(seq 1 45); do
  a="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$main")"
  b="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$tavern")"
  [[ "$a" != healthy || "$b" != healthy ]] || break
  [[ "$(docker inspect -f '{{.RestartCount}}' "$main")" == 0 ]]
  [[ "$(docker inspect -f '{{.RestartCount}}' "$tavern")" == 0 ]]
  sleep 2
done
[[ "$a" == healthy && "$b" == healthy ]]
curl -fsS http://127.0.0.1:32795/api/status >/dev/null
curl -fsS -H 'Host: 156.238.252.115:32796' http://127.0.0.1:32796/ >/dev/null
docker inspect "${protected[@]}" > "$backup/protected.after.json"
python3 - "$backup/protected.before.json" "$backup/protected.after.json" <<'PY'
import json,sys
def identities(p):
    return {c["Name"]:(c["Id"],c["State"]["StartedAt"]) for c in json.load(open(p))}
assert identities(sys.argv[1])==identities(sys.argv[2])
print("ALL_OTHER_RUNNING_CONTAINERS=UNCHANGED")
PY
docker exec sub2api-postgres sh -c 'exec psql -X -At -U "$POSTGRES_USER" -d newapi_test_20261002174631 -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM tasks; SELECT count(*) FROM studio_jobs WHERE status IN ('\''queued'\'','\''running'\''); SELECT count(*),coalesce(sum(quota),0),coalesce(sum(used_quota),0),coalesce(sum(request_count),0) FROM users; SELECT count(*),coalesce(sum(quota),0) FROM tavern_credits;"' > "$backup/counts.after.txt"
if cmp -s "$backup/counts.before.txt" <(head -n 3 "$backup/counts.after.txt"); then
  echo "MAIN_ACCOUNT_COUNTERS=UNCHANGED"
else
  echo "Account counters changed during rollout; inspect backups (no generation performed by this script)."
fi
switched=0
printf 'RELEASE=%s\nBACKUP=%s\n' "$release" "$backup"
