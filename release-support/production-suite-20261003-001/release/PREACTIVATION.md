# Production Suite Candidate

Status: prepared locally, NOT uploaded or activated.

Confirmed scope: updated profitability, Tavern and full Video Studio.
Keep the current production workstation, including its v29 frontend and
v28 backend image fix. Do not switch, recreate or reconfigure its containers.
Retain existing Image Studio funding in the shared production backend.

Production baseline:
`studio-credit-20261003-2120`, SHA-256
`bafd36f8629e154b271f4b93e65a4b049657532364482a28c534e5fd28142d85`.
Candidate backend:
`2c205bf966a6d300b8b3a4086329baa4af1f2234973974641b76c7e334d4a7db`.
Additive schema tool:
`22926aaff1210c381cc85c5cd5c9428a541c3e23c32e00e40c3f9e8dee50c191`.

## Completed

- Frozen New API source inventory: 2225 files, zero hash mismatches.
- Production Image Studio credit model, funding and funding tests unchanged.
- Dashboard types, scoped lint, 10 test files/65 tests and production build pass.
- Scoped Go model/service/controller/middleware/router tests pass; common had
  no tests matching this filter. Linux backend and standalone relaykit build pass.
- Explicit task scope applies to scheduling, claiming and stale-lock cleanup.
  Unset scope retains defaults; nonempty unrecognized scope runs no handlers.
- Additive migration tests preserve old rows/credit, repeat without duplicates,
  and roll back rather than repairing an unexpected existing table.
- RisuAI: types 0 errors/warnings; 30 test files, 252 pass/3 skipped;
  19 HTTP bridge tests pass. Production frontend now built locally with
  VITE_RISU_LEGAL_CONFIGURED unset and /tavern-app/ base.
- Actual US Compose merge changes only the named New API environment/mounts.
  Tavern Compose, Nginx snippet and rollback shell syntax pass.
- Scope-confirmation recheck: 2225 frozen New API files match; original Image
  Studio credit model/funding/tests match the production baseline exactly.
  Focused model/service/controller/router rerun passes 91 top-level tests
  (156 pass events including subtests). Evidence: logs/confirmed-scope-go.jsonl.
- Tavern Node runtime bundle is built; node --check passes. Runtime SHA-256:
  `dc502ba47a5c3d5129725a7968ae09df0e021aedea2c98adb5d8ce9d7f0c5ccf`.
  Linked license comments and dependency license collection are retained.
  Native unconfigured-legal notice is retained in the frontend.
- Isolated Chrome checks pass at 1440x900 and 390x844: profit figures,
  full Video Studio/catalog and account menu/balance, production Tavern entry
  and its native legal notice. Synthetic accounts only; zero generation calls.
- Browser checks found a mobile header overflow (390px viewport, 412px page).
  Added min-w-0 to the brand link and shrink-0 to its logo. Rechecked types,
  scoped lint, production frontend build and Linux backend build; browser
  layout now has no overflow. Frozen inventory includes this explicit change.

## Deferred Configuration

The user confirmed public hosting. Actual production user agreement and privacy
policy APIs both return empty content with their enabled flags false. User
authorized deployment preparation now and will arrange documents later.
Deploy the native notice, not a bypass: VITE_RISU_LEGAL_CONFIGURED remains
unset. Tavern chat remains unavailable behind this notice until the operator
configures approved documents. Profitability and Video Studio can be activated.

## Activation Requirements

1. Recheck current host identity, service labels, hashes, available resources
   and unused port 32831. Do not assume saved container IDs remain current.
2. Retain the deferred legal notice. Tavern build and isolated same-origin
   desktop/mobile checks pass; real chat/storage/streaming acceptance is pending.
3. Upload to a new immutable release directory. Verify every deployed hash.
   Keep existing base and production override as the first two Compose layers.
4. Before activation, create root-only verified PostgreSQL dump and archives of
   both production data mounts, logs and exact active Compose/Nginx config.
   The draft rollback expects
   `/opt/us-whitelist/backups/production-suite-20261003-001/api-yisama.conf`.
5. Create new independent Tavern storage and service key on the host. Never
   copy test accounts, secrets, saves or database. Preserve existing data mounts.
6. Run `suite-schema` with the existing application's `SQL_DSN`; default mode
   checks only, explicit `-apply` adds missing `studio_jobs`, `tavern_credits`
   and `models.parameters`. No full AutoMigrate or old database restore.
   PostgreSQL execution and ownership/permissions still need verification.
7. Validate actual release assets, resolved Compose and complete Nginx config.
   Activate only production `new-api` and new Tavern; reload Nginx only.
   Do not restart old-new-api, test instances, Studio, Redis or PostgreSQL.
8. Verify health, version/hash, task whitelist, authentication boundary,
   public asset bytes/cache headers and isolated UI. Do not infer acceptance
   from an anonymous 401 or SPA 200 alone.

Rollback is application/config only. It intentionally keeps all current
databases, new gift ledgers, video media and Tavern saves. No deployed backup
or live rollback execution is claimed by this candidate.

Human validation and separately authorized real paid settlement remain pending.
