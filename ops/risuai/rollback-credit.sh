#!/usr/bin/env bash
set -euo pipefail
root=/opt/us-whitelist/test-new-api-20261002-174631
release="$(cd -- "$(dirname -- "$0")" && pwd)"
backup="$root/backups/$(basename "$release")"
[[ "$release" == "$root/releases/"tavern-credit-* ]]
[[ "$(hostname)" == ser021360749320 && "$(id -un)" == root ]]
exec 9>"$root/.video-studio-deploy.lock"
flock -n 9
current="$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' risuai-test-20261003)"
[[ "$current" == *"$release/compose.credit.json" ]] || { echo 'A different release is active.' >&2; exit 1; }
previous=(docker compose -p new-api-test-20261002-174631)
while IFS= read -r file; do
  [[ "$file" == "$root/"* && -f "$file" ]]
  previous+=(-f "$file")
done < "$backup/previous-config-files.txt"
"${previous[@]}" up -d --no-deps --pull never --no-build new-api-test risuai-test
echo 'Previous applications selected. Databases, gift records, Redis and user storage preserved.'
