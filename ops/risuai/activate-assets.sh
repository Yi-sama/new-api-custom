#!/usr/bin/env bash
set -euo pipefail
umask 077
root=/opt/us-whitelist/test-new-api-20261002-174631
release="$(cd -- "$(dirname -- "$0")" && pwd)"
id="$(basename "$release")"
backup="$root/backups/$id-assets"
config=/etc/nginx/conf.d/tavern-http-20261003-1708.conf
[[ "$release" == "$root/releases/$id" && "$id" =~ ^risuai-entry-[0-9-]+$ ]]
[[ "$(hostname)" == ser021360749320 && "$(id -un)" == root ]]
[[ -f "$config" && ! -L "$config" && ! -e "$backup" ]]
[[ "$(sha256sum "$config" | cut -d ' ' -f 1)" == "${1:?Expected original Nginx config hash}" ]]
exec 9>"$root/.video-studio-deploy.lock"
flock -n 9
cd "$release"
sha256sum -c SHA256SUMS >/dev/null
mkdir -m 700 "$backup"
cp -a "$config" "$backup/nginx.before.conf"
docker inspect $(docker ps -q) > "$backup/containers.before.json"
nginx -T > "$backup/nginx.before.txt" 2>&1
frontend="$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/app/dist"}}{{.Source}}{{end}}{{end}}' risuai-test-20261003)"
[[ "$frontend" == "$root/releases/"*"/dist" && -f "$frontend/assets/stores.svelte-Cf4Uo7s1.js" ]]
switched=0
recover() {
    code=$?
    if [[ "$code" != 0 && "$switched" == 1 ]]; then
        rm -- "$config"
        mv -T "$backup/nginx.original.conf" "$config"
        nginx -t && systemctl reload nginx
        echo 'Original Tavern Nginx config restored.' >&2
    fi
    exit "$code"
}
trap recover EXIT
ln -s "$release/nginx-http.conf" "$backup/nginx.next.link"
mv -T "$config" "$backup/nginx.original.conf"
switched=1
mv -T "$backup/nginx.next.link" "$config"
nginx -t
systemctl reload nginx
# A reload signals the master; responses can still use the previous workers.
active=0
for attempt in $(seq 1 20); do
    curl -fsSI --max-time 5 -H 'Host: 156.238.252.115:32796' \
        http://127.0.0.1:32796/assets/stores.svelte-Cf4Uo7s1.js > "$backup/readiness.headers"
    if grep -qi '^Cache-Control: public, max-age=31536000, immutable' "$backup/readiness.headers"; then
        active=1
        break
    fi
    printf 'STATIC_CONFIG_PENDING_ATTEMPT=%s\n' "$attempt"
    sleep 1
done
[[ "$active" == 1 ]]
printf 'STATIC_CONFIG_ACTIVE_ATTEMPT=%s\n' "$attempt"
curl -fsS --max-time 15 -H 'Host: 156.238.252.115:32796' -H 'Accept-Encoding: gzip' \
    -D "$backup/assets.headers" -o "$backup/stores.js.gz" \
    http://127.0.0.1:32796/assets/stores.svelte-Cf4Uo7s1.js
grep -qi '^Content-Encoding: gzip' "$backup/assets.headers"
grep -qi '^Cache-Control: public, max-age=31536000, immutable' "$backup/assets.headers"
python3 - "$backup/stores.js.gz" "$frontend/assets/stores.svelte-Cf4Uo7s1.js" <<'PY'
import gzip,pathlib,sys
assert gzip.decompress(pathlib.Path(sys.argv[1]).read_bytes()) == pathlib.Path(sys.argv[2]).read_bytes()
print("STATIC_GZIP_DECODED_BYTES_MATCH=PASS")
PY
[[ "$(curl -sS --max-time 15 -o /dev/null -w '%{http_code}' -H 'Host: 156.238.252.115:32796' http://127.0.0.1:32796/api/new-api/account)" == 401 ]]
docker inspect $(docker ps -q) > "$backup/containers.after.json"
python3 - "$backup/containers.before.json" "$backup/containers.after.json" <<'PY'
import json,sys
def identity(path):
    return {c["Name"]:(c["Id"],c["State"]["StartedAt"]) for c in json.load(open(path))}
assert identity(sys.argv[1])==identity(sys.argv[2])
print("ALL_CONTAINERS_UNCHANGED_DURING_NGINX_RELOAD=PASS")
PY
switched=0
printf 'ASSETS_CONFIG=%s\nBACKUP=%s\n' "$release/nginx-http.conf" "$backup"
