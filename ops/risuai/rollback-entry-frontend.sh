#!/usr/bin/env bash
set -euo pipefail
root=/opt/us-whitelist/test-new-api-20261002-174631
release="$(cd -- "$(dirname -- "$0")" && pwd)"
id="$(basename "$release")"
[[ "$release" == "$root/releases/$id" && "$id" =~ ^risuai-entry-[0-9-]+$ ]]
[[ "$(hostname)" == ser021360749320 && "$(id -un)" == root ]]
exec 9>"$root/.video-studio-deploy.lock"
flock -n 9
current="$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' risuai-test-20261003)"
[[ "$current" == *"$release/compose.entry.json" ]] || { echo 'A different frontend is active; refusing stale rollback.' >&2; exit 1; }
previous=(docker compose -p new-api-test-20261002-174631)
while IFS= read -r file; do
  [[ "$file" == "$root/"* && -f "$file" ]]
  previous+=(-f "$file")
done < "$root/backups/$id/previous-config-files.txt"
"${previous[@]}" up -d --no-deps --pull never --no-build risuai-test
printf 'Previous frontend selected. New API and persistent data unchanged.\n'
