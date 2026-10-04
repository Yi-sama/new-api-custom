#!/usr/bin/env bash
set -euo pipefail
umask 077
root=/opt/us-whitelist/test-new-api-20261002-174631
release="$(cd -- "$(dirname -- "$0")" && pwd)"
id="$(basename "$release")"
backup_id="$id"
if [[ -f "$release/active-backup.txt" ]]; then read -r backup_id < "$release/active-backup.txt"; fi
[[ "$backup_id" == "$id" || "$backup_id" == "$id-configured-channels" ]]
backup="$root/backups/$backup_id"
nginx_file="/etc/nginx/conf.d/$id.conf"
[[ "$release" == "$root/releases/"tavern-http-* ]]
[[ "$(hostname)" == ser021360749320 && "$(id -un)" == root ]]
exec 9>"$root/.video-studio-deploy.lock"
flock -n 9
previous=(docker compose -p new-api-test-20261002-174631)
while IFS= read -r file; do
  [[ "$file" == "$root/"* && -f "$file" ]]
  previous+=(-f "$file")
done < "$backup/previous-config-files.txt"
current="$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' new-api-test-20261002-174631)"
[[ "$current" == *"$release/compose.tavern.json" ]] || { echo 'A newer deployment is active; refusing stale rollback.' >&2; exit 1; }
if [[ -e "$nginx_file" ]]; then
  mv "$nginx_file" "$backup/nginx.rolled-back.conf"
  nginx -t
  systemctl reload nginx
fi
ufw delete allow 32796/tcp
"${previous[@]}" -f "$release/compose.tavern.json" stop risuai-test
"${previous[@]}" up -d --no-deps --pull never --no-build new-api-test
printf 'Selected previous release. Database, Redis and RisuAI data were not restored or deleted.\n'
