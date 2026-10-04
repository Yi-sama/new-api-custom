#!/usr/bin/env bash
set -euo pipefail
root=/opt/us-whitelist/test-new-api-20261002-174631
release="$(cd -- "$(dirname -- "$0")" && pwd)"
id="$(basename "$release")"
backup="$root/backups/$id-assets"
config=/etc/nginx/conf.d/tavern-http-20261003-1708.conf
[[ "$release" == "$root/releases/$id" && "$id" =~ ^risuai-entry-[0-9-]+$ ]]
[[ "$(hostname)" == ser021360749320 && "$(id -un)" == root ]]
[[ -L "$config" && "$(readlink "$config")" == "$release/nginx-http.conf" ]]
[[ -f "$backup/nginx.original.conf" ]]
exec 9>"$root/.video-studio-deploy.lock"
flock -n 9
rm -- "$config"
mv -T "$backup/nginx.original.conf" "$config"
nginx -t
systemctl reload nginx
echo 'Original Tavern asset delivery selected. No application or data changes.'
