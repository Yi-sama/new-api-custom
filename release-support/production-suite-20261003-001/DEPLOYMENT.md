# Production State - 2026-10-04

Main and Tavern activated on the US host. Profitability and full Video Studio
are deployed. Tavern retains the native unconfigured-legal page until approved
operator documents/notices are configured. Real account/paid acceptance pending.

## Active Layers

Main project `us-whitelist`, service `new-api`, loopback 32794:
1. `/opt/us-whitelist/docker-compose.yml`
2. `/opt/us-whitelist/releases/studio-credit-20261003-2120/backend.override.json`
3. `/opt/us-whitelist/releases/production-suite-20261003-001/backend.override.json`

Tavern project `us-whitelist-tavern-production`, loopback 32831:
1. Release `tavern.compose.json`
2. Release `https-fix/tavern-runtime.override.json`

Nginx `/etc/nginx/conf.d/api-yisama.conf` links to release
`api-yisama.proxy-assets.conf`, including `tavern-proxy-assets.nginx.conf`.
Do not revert to the static alias: its root-only parent is unreadable by workers.

Workstation remains its original seven Compose layers including v28 image fix
and v29 balance UI. Neither WebUI nor gateway container was restarted.

## Backups And Rollback

Backup: `/opt/us-whitelist/backups/production-suite-20261003-001`.
Root-only validated custom PostgreSQL dump, three data mount archives, exact
original Compose/Nginx and container snapshots.

```sh
sh /opt/us-whitelist/backups/production-suite-20261003-001/rollback.sh --allow-interruption
```

Run only during an explicitly approved maintenance window. The default
rollback invocation now refuses to recreate the live single-instance backend.

Rollback keeps all current database rows, gift ledgers, media and Tavern saves.
It restores applications/ingress only. Rollback was actually exercised after
the original HTTP-only bridge rejected HTTPS, before the corrected activation.

## Evidence

- Main backend SHA-256:
  `2c205bf966a6d300b8b3a4086329baa4af1f2234973974641b76c7e334d4a7db`.
- Active Tavern runtime SHA-256:
  `6e1b6175ae1ff411eb5df11a894fd86eb986daeba843480f81ca52fb3a182065`.
- Original release binaries/assets are unchanged. Deployment helpers received
  the ingress hotfix below; the release SHA256SUMS now has 444 entries.
  HTTPS fix remains a separate mount; Python 3.10-compatible helpers and
  asset-proxy config are separate files.
  Original candidate manifest records packaging-time state, not live status.
- Main/Tavern health and version, scoped slave workers, table ownership,
  anonymous 401/hostile 403 and nine actual public asset hashes pass.
- Desktop 1440x900/mobile 390x844 public-asset browser fixtures pass.
  Only API/account responses are synthetic in those deployment checks.
  Those checks used zero generation/paid calls; the ingress acceptance below
  subsequently used one small live generation.
- New studio_jobs and tavern_credits tables belong to application newapi.
  models.parameters exists, original Studio credit row remains.
- Remote result artifacts: deployment-result.json, ingress-static-fix.json,
  final-public-check.json inside the backup directory.
- Local evidence under logs; public screenshots in New API
  output/playwright/production-suite-deferred-live.

spec_check: pass. runtime_check: pass for automated checks. human_check:
pending for real login, balance/gift spending, actual report data and paid jobs.
overall: partially_verified.

## API Ingress Hotfix - 2026-10-04

Root cause: root standalone Nginx tests omitted `user www-data;` and changed
production scratch-directory ownership to nobody. Do not restore these old
test generators or rely on recurring chown.

All five current candidate-check callers use `nginx_validation.py`:
`deploy.py`, `deploy-python310.py`, `https-fix/deploy-v2.py`,
`repair-ingress.py`, and `validate-config.py`.
The helper declares www-data and uses a private mount namespace to isolate
`/var/lib/nginx`, `/var/log/nginx`, and `/var/cache/nginx` when present.
PID/config/prefix are under a bounded temporary /run directory.
An isolation or syntax error fails the check; there is no unisolated fallback.

Three activation scripts and the release/backup rollback entrypoints now
require explicit `--allow-interruption` for container replacement.
Preparation/verification remain usable without this flag.
This is a maintenance gate, NOT zero-downtime deployment. A blue/green design
and old-request draining still require architecture approval and verification.
Manual Docker commands and older release helpers are not covered by this gate.

Exact hotfix backups and evidence:
`/opt/us-whitelist/backups/new-api-ingress-rootfix-20261004-1631`.
The active ingress, backend identity/start time, databases and other services
were not replaced or restored. No service restart/reload or disk cleanup.

Automated regression: all five actual candidate-call expressions pass, three
repeated live-ingress checks preserve production path ownership/modes/inodes,
invalid syntax is rejected, and unapproved activation/rollback is blocked.
Both release checksum manifests pass.
The complete standalone Compose/snippet/rollback validator also passes.

Public 2,097,165-byte anonymous POST reached New API with expected HTTP 401:
`202610040849254726326508268d9d6NUl9zG6w`.
One real gpt-6.1-sol SSE call returned ROOTFIX_OK and response.completed:
`202610040849226761387578268d9d6AooMnI4J`.
PostgreSQL consumption row 91166 confirms channel 42, stream status ok/done,
23 input tokens and 7 output tokens. This verifies complete short streaming,
not long-session acceptance or uninterrupted upgrades.

Verification-tool exceptions were resolved or bounded: the first local probe
accidentally sent GET (404); corrected POST passed. urllib requests receive
Cloudflare 1010/403 without an application request ID, while the established
curl path passes. No Cloudflare security policy was changed.

spec_check: pass for root-cause repair and default maintenance safety.
runtime_check: pass for ingress/regression/short-SSE checks.
human_check: pending for the user's original long conversation.
overall: partially_verified; zero-downtime architecture is not implemented.
