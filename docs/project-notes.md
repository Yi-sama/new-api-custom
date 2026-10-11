# Project Notes

## Current Status

- US workstation v33 deployed on 2026-10-05:
  `/opt/image-studio/releases/20261005-chat-preferences-v33/`.
  ELEVEN active Compose layers: retain all ten prior files, including v28
  backend override, then append v33 frontend override. Only WebUI recreated,
  healthy/restart 0, ID
  `2fd1ec98a58b0d853c8078a30d5856d34cfea5351db5acfc892e62ac7053deed`.
  Image/backend/data mounts, Nginx, account settings and all 28 other container
  IDs are unchanged. No remote build/install/pull or paid call.
- Chat model persistence diagnosis: production already had nonempty
  `ui.models`. The chat GROUP was only in sessionStorage, so reopening a tab
  chose a different group; the native picker filters saved models against
  that group's list. Saved production model has enabled abilities only in
  three non-default groups. Group now persists in localStorage keyed by
  New API account ID, before the first native model request; old session
  choices migrate. Revoked/image-only groups still fall back to authorized
  chat groups. This survives closing/reopening in the SAME browser; group
  storage is not cross-device synchronization. Model settings remain on the
  server. Previously lost group choices cannot be reconstructed uniquely;
  choose once after updating when no valid old session choice remains.
- Pixel menu remains 21 presets plus Auto. First sizes are `1024:1024`,
  `1536:1024`, `1024:1536`, `1536:864`, `864:1536`. Only ordering changed;
  fixed-model restrictions, exact dimensions and legacy migration remain.
  V33 changes only group persistence and state size order; all other native/
  helper files are identical to v32 after namespace replacement.
- V33 checks: frontend/native 54/54, backend size 8/8. Actual compiled picker,
  Chat recovery effect and group loader passed synthetic Chrome refresh/
  fresh-page recovery, account isolation, wrong-group reproduction and pixel
  order/save at 1440x900 and 390x844. Zero browser errors. Real-account
  acceptance remains pending; no authentic session/token was extracted.
  Live HTTP bodies for entry/seven assets match release bytes; anonymous
  gateway/public HTTPS checks return 401. Final RAM available 4633/7937 MiB,
  swap used 64 MiB, disk free 3660 MiB.
  Rollback: `sh /opt/image-studio/backups/20261005-chat-preferences-v33/rollback.sh`.
  Frozen sources/helper/manifests/report:
  `.local-builds/studio-chat-preferences-20261005`. Browser evidence:
  `output/playwright/studio-preferences-20261005-{fixture,check}.cjs`.
  Overall `partially_verified`, pending user account acceptance.

- Historical US workstation PIXEL-size picker deployed on local 2026-10-05 as v32,
  superseded by v33:
  `/opt/image-studio/releases/20261005-pixel-sizes-v32/`. Append its
  `frontend.override.json` after ALL nine prior active Compose files,
  including v28 backend image fixes and v31. Ten layers are active.
  Only Open WebUI was recreated, healthy/restart 0; its image and backend/data
  mounts are unchanged. All 28 other container IDs and Nginx config stayed
  unchanged. Main, profitability, Video Studio, Tavern and balances untouched.
- User clarified that "ratios" meant resolutions like `512:512` and
  `1024:1024`, NOT `1:1` or `16:9`. v31 was an intent mismatch and is
  superseded by v32. Native menus now display pixel dimensions with a colon,
  plus Auto, and have no custom input/Add action. Production aliases get 21
  presets: squares 512/768/1024/1536/2048; landscape/portrait pairs
  1024x768, 1152x864, 1280x720, 1536x864, 1536x1024, 1536x768,
  1920x1080 and 2016x864. Selections still send/save canonical `WIDTHxHEIGHT`.
  Exact `gpt-image-2` gets 17 choices: existing bridge rejects 512/768 squares
  and non-16-aligned 1080 edges. GPT Image 1 and DALL-E stay fixed-size.
  No backend validation was relaxed to expose a requested resolution.
  Old standard ratios migrate to their preset pixels; retired arbitrary
  ratios such as `5:4` reset to Auto when models load. Historical
  `newApiImageRatios` data remains inert, not deleted. Fixed-size models retain
  their restrictions and explicit unsupported-selection warning.
- All v32 native/custom helpers share `/studio-v32/`. 52 frontend/native
  tests and 8 backend tests pass; all 80 offered preset/model combinations pass
  the CURRENT production-mounted bridge policy. Custom aliases' upstream
  support is unknown: exact pixels bypass the local unknown-ratio rejection,
  not an upstream capability guarantee. No paid generation was performed.
  Chrome synthetic desktop/mobile checks pass for all four production IDs,
  512x512/1024x1024 reload, legacy `5:4` reset, model limits and text bounds.
  Actual WebUI HTTP entry/helpers match release bytes; loopback gateway and
  Windows public HTTPS entry/helper/billing each return anonymous 401.
  Real-account visual/save acceptance and paid generation remain pending.
- v32 rollback, restoring v31 without rolling back data/settings:
  `sh /opt/image-studio/backups/20261005-pixel-sizes-v32/rollback.sh`.
  Frozen sources, deploy helper, prepared/active manifests and final check:
  `.local-builds/studio-pixel-sizes-20261005`. Progress/debug:
  "US Studio Pixel Sizes (v32)", 2026-10-05.
  Final resources: RAM 4672 MiB available of 7937, swap used 60 MiB,
  disk free 4033 MiB. No server build/install/pull or credential extraction.
- Historical v30 release/rollback remain intact; its user acceptance FAILED
  and is superseded by v32. V31 sources/backups also remain intact.
  See 2026-10-04 and 2026-10-05 progress/debug for historical evidence.
  Concurrent blue/green main instances were observed before this task;
  historical main deployment notes below are not a substitute for inspecting
  its current ingress and active Compose layers before any future main update.
  A timed-out scp export completed later with a matching checksum; tool timeout
  is not proof of transfer termination. Reinspect file size/hash and helper
  status before labeling an archive partial or deleting/moving it.

- US production suite activated on 2026-10-04: updated profitability and full
  Video Studio are deployed at `https://api.yisama.asia`; Tavern is deployed
  with its native legal-configuration notice, NOT available for chat yet.
  User deferred terms/privacy arrangements; never set the legal flag TRUE
  without approved documents/notices. During that suite rollout, workstation
  stayed production v29/v28 with seven layers and unchanged WebUI/gateway IDs.
  The later workstation-only v32 update is recorded above.
- Main version `v1.0.0-rc.28-custom-production-suite-20261003-001`, binary
  SHA-256 `2c205bf966a6d300b8b3a4086329baa4af1f2234973974641b76c7e334d4a7db`.
  Preserve THREE main Compose layers: `/opt/us-whitelist/docker-compose.yml`,
  `releases/studio-credit-20261003-2120/backend.override.json`,
  `releases/production-suite-20261003-001/backend.override.json`.
  Port remains loopback 32794. Do not restart from only the original base file.
- Tavern uses project `us-whitelist-tavern-production`, container
  `risuai-production-20261003`, loopback-only 32831. Preserve BOTH release
  `tavern.compose.json` AND `https-fix/tavern-runtime.override.json`.
  Runtime hash `6e1b6175ae1ff411eb5df11a894fd86eb986daeba843480f81ca52fb3a182065`.
  HTTPS public origins supported, backend still HTTP loopback-only; bridge
  cookies Secure/HttpOnly/SameSite=Strict. New independent runtime/key/storage
  under `/opt/us-whitelist/runtime/tavern-production`; no test data copied.
- Active `/etc/nginx/conf.d/api-yisama.conf` is a symlink to release
  `api-yisama.proxy-assets.conf`, including `tavern-proxy-assets.nginx.conf`.
  Assets are proxied through Tavern because `/opt/us-whitelist` is root-only.
  Keep that permission; do not expose the parent to make a static alias work.
  Hashed Tavern scripts/CSS use gzip and immutable caching; API/HTML do not.
- Verified root-only backup/rollback:
  `/opt/us-whitelist/backups/production-suite-20261003-001/rollback.sh`.
  Restores applications/ingress only, retaining live DB, gift ledgers, media,
  and Tavern saves. Custom PostgreSQL dump and all three data mount archives
  were validated before migration. No old release was overwritten.
- Do NOT promote the test binary directly: frozen test source lacks production
  Image Studio gift model/funding and replaces shared billing-session handling
  with Tavern funding. Integrate both against the verified production baseline
  `.local-builds/studio-credit-20261003/release-source`. Current worktree has both
  paths, but is dirty. New frozen candidate has both funding paths tested;
  original Image Studio funding/model/tests match production exactly.
- Production/old-new-api remain slaves. Active `SYSTEM_TASK_TYPES` allows
  scoped slave execution, restricting scheduler/claim/stale cleanup to
  `video_studio_jobs,async_task_poll`; keep `UPDATE_TASK=true`. No master switch.
  Additive real PostgreSQL migration created `studio_jobs`, `tavern_credits`
  owned by application role `newapi` and added `models.parameters`; no full
  AutoMigrate. Final counts: original Studio credit rows 1, Tavern credits 0,
  video jobs 0. Only main New API was recreated and new Tavern started;
  Nginx reloaded, all other containers unchanged.
- RisuAI portal/public hostnames must match for main refresh-cookie sharing.
  Production must not point back at test IP ports or reuse test database/storage.
  Production HTTPS uses the existing TLS server with scoped Tavern routes.
  No remote build/install/pull or paid call occurred. Final health/restart-0,
  version/hash, 9 public asset hashes/cache/compression, anonymous 401 and
  hostile-origin 403 checks pass. Chrome 1440x900/390x844 uses real public
  assets with synthetic API/account responses; NOT real-account acceptance.
  Human login, gift spending and paid-generation settlement remain pending.
  Evidence: progress/debug "Production Suite Activation" and
  `output/playwright/production-suite-deferred-live/`.

- Historical local preparation before activation (2026-10-04):
  `.local-builds/production-suite-20261003-001`, backend SHA-256
  `4814e686d89eb047501b47fe29ae8d5b524a9e81b0bed3e4a091a75aafb5ae21`.
  Dashboard types/lint/65 tests/build, scoped Go checks, Linux root/relaykit
  builds and 2 additive migration tests pass. RisuAI 252 pass/3 skipped,
  19 bridge checks and types 0/0 pass. Compose scope and Nginx/rollback
  syntax passed. At that stage production still ran the original binary.
  Deferred input: operator-approved terms/privacy documents. Public use is confirmed;
  actual production `/api/user-agreement` and `/api/privacy-policy` return
  empty strings and both enabled flags are false. Do not reuse test
  `VITE_RISU_LEGAL_CONFIGURED=TRUE`. The notice-preserving build, public browser
  checks, backup/upload/migration/activation are now complete; real settlement
  remains pending. Runtime HTTPS regression adds a 20th passing bridge test.
  See `docs/progress/2026-10-04.md`, `docs/debug/2026-10-04-debug.md` and
  candidate `release/PREACTIVATION.md`. Overall: partially_verified.
- Scope-confirmation follow-up: frozen source inventory rechecked at 2225/0,
  Image Studio model/funding/tests match production exactly, 91 focused
  top-level Go tests pass (156 including subtests), and Tavern runtime bundle
  syntax passed. Subsequent browser checks found mobile brand-link overflow;
  two layout classes fixed it, types/lint/build and red-to-green browser layout
  recheck pass. Latest candidate/backend hashes are those listed above.

- US Studio account balance/gift display (2026-10-03, v29): historical frontend
  `/opt/image-studio/releases/20261003-2325-account-balance-v29/`.
  Its `frontend.override.json` must be appended after ALL six preexisting
  Compose layers, including the v28 `backend.override.json` image fix.
  That deployment retained v28 bridge/images.py bind mounts, pinned image
  and data volume. Main New API container/binary is unchanged.
- Top badge explicitly says "赠送余额" alongside the separate wallet amount.
  Bottom-left named account button shows only its wallet amount on the right.
  Both share one uncached billing response, refresh every five seconds while
  visible and immediately on focus/visibility, and reuse latest data after
  native sidebar/header remount. Missing/error states do not invent credit.
  The user canceled the welcome toast: active code no longer reads or writes
  its acknowledgement marker. Existing markers are retained as inert data.
- At v29, custom scripts/CSS live under `/studio-v29/custom/` with native modules
  under `/studio-v29/`, bypassing the separately served backend `/static` tree.
  HTTP body hashes are verified against mounted files, not just mount labels
  or symbol presence. Nginx `studio-yisama.conf` adds no-store for HTML through
  `$sent_http_content_type`; cached static native assets are unaffected.
  Fresh real-account acceptance URL: `https://studio.yisama.asia/?ui=29`.
- Prior missing gift diagnosis: screenshot confirms the correct public domain.
  Origin access logs show v25 assets at 21:30 and v26 at 21:47 on October 3;
  no successful public v27 helper load was observed in the inspected tail.
  The old browser's exact loaded body is unavailable, so cache location is
  not claimed proven. Versioned custom paths/HTML freshness address delivery
  independently of the ambiguous browser/CDN cache layer.
- v29 verification: 51 frontend/native-runtime tests + 8 image-size tests pass.
  Synthetic loopback Chrome verified desktop/mobile account placement, header,
  zero gift and wallet/gift updates within five seconds; NOT a production
  account screenshot. Main/WebUI healthy, restart 0, six uploaded source hashes
  match, four actual HTTP asset bodies match deployment, anonymous public
  entry/helper/billing remain 401. Human acceptance remains pending.
  Backup/rollback:
  `/opt/image-studio/backups/20261003-2325-account-balance-v29/rollback.sh`.
  Restores frontend/cache config only, preserving v28 fixes, ledger/settings.

- Studio v27 human acceptance FAILED (2026-10-03): the user reports neither
  requested gift display nor first-entry notice is visible after the release.
  Do not describe this issue as fixed. The production mount and gateway still
  pointed to v27/current backend at `studio.yisama.asia`. The later screenshot
  confirmed the browser URL; its loaded body/authenticated response were unknown.
  This session has no discoverable `node_repl` tool for the installed Computer
  Use plugin. Superseded by v29 above; isolated fixture success is not
  real-account UI acceptance.

- US Studio gift display/notice (2026-10-03): historical frontend v27 was at
  `/opt/image-studio/releases/20261003-2220-gift-notice-v27/`.
  Append its `frontend.override.json` AFTER both v24 Compose files, the v25
  override and the v26 override. Only Open WebUI was recreated; its image,
  data volume, New API container/binary, gateway and credit ledger are unchanged.
  Backup and frontend-only rollback:
  `/opt/image-studio/backups/20261003-2220-gift-notice-v27/rollback.sh`.
- The balance badge explicitly includes workstation gift credit, including
  zero and unavailable states. Native success toast follows account readiness:
  "Thanks for using this workstation" with the USD 1 grant and restricted,
  gift-first spending explanation. `ui.newApiStudioGiftNotified` is persisted
  through the existing per-account WebUI settings API and shared preference
  write queue. Already-granted accounts without a notice receive one; no new
  grant is issued. Model/image/size preferences remain intact.
- v27 verification: 52 frontend/compiled-runtime tests and 8 image-size tests
  pass. Existing isolated browser fixture checks at 1440x900 and 390x844 show
  the gift field without overflow/overlap; native toast state and persisted
  settings confirm one notice, zero notices after reload. Live v27 assets,
  healthy/restart-0 containers, unchanged backend ID/hash and anonymous 401
  boundaries pass. Real-account visual acceptance is still pending.
  Original missing-field cause in the user's old tab is not established:
  the v26 server already contained the gift code and ledger record; a stale
  tab/cache is an inference, not a verified root cause.

- US Open WebUI Studio credit (2026-10-03): the user accepted the previous
  balance/model-preference fix. Production now runs
  `v1.0.0-rc.28-custom-studio-credit-20261003.1`, SHA-256
  `bafd36f8629e154b271f4b93e65a4b049657532364482a28c534e5fd28142d85`.
  Backend release `/opt/us-whitelist/releases/studio-credit-20261003-2120`
  needs the original Compose plus `backend.override.json`. Initial gift frontend was v26,
  `/opt/image-studio/releases/20261003-2120-credit-v26/frontend.override.json`,
  appended after both v24 Compose files and the v25 override; v27 above now supersedes it. Keep the explicit
  project `image-studio` and env `/opt/image-studio/.env`.
- Each account claims USD 1 once via verified Studio identity/context/billing.
  `image_studio_credits` is separate from wallet and `tavern_credits`; only
  signed workstation chat/Responses/image/audio requests spend it, before
  the existing paid wallet/subscription preference. Gift remains at zero after
  exhaustion and reentry never reissues it. No artificial production grant or
  paid upstream test was performed. A real grant row was observed on reinspection;
  detailed real-account display/settlement acceptance remains pending.
- The exact previous production binary was reproducibly rebuilt before
  patching: its SHA-256 is `3b37cfbd261c7c8fa157f4d35f6d50cd2d7bfd0f5f2f0f41b305cce129c5453f`.
  Frozen source and recovery helpers are under
  `D:\Open source project\.local-builds\studio-credit-20261003`.
  Only this gift patch was deployed, not concurrent unfinished work.
  Main site's 232 embedded frontend files remain byte-identical.
- Studio gift table is owned by actual application role `newapi`, with
  SELECT/INSERT/UPDATE verified. Slave mode is unchanged; migration was scoped
  manually. PostgreSQL and WebUI backups/rollback are in
  `/opt/us-whitelist/backups/studio-credit-20261003-2120`.
  Revert applications with its `rollback.sh`, retaining gift records and history.
  WebUI cold startup briefly became unhealthy then healthy; use observed
  bounded readiness checks and persistent activation logs. Do not treat a
  transport timeout as proof that deployment did or did not execute.

- US Tavern group/loading follow-up (2026-10-03): active frontend is
  `releases/risuai-entry-20261003-2300` under
  `/opt/us-whitelist/test-new-api-20261002-174631`.
  Onboarding, model settings and account settings show actual group keys,
  never `desc` as the group name. Only these three Svelte files changed
  from the frozen 2210 source; backend/runtime, storage, gift ledger,
  billing and main group configuration/descriptions remain unchanged.
  The prior aligned account/balance/gift layout is retained.
- Tavern restarts now need NINE Compose files from the active RisuAI
  container label: retain all eight prior layers and append this release's
  `compose.entry.json`. The main container's unchanged label still lists
  seven, so do not obtain the complete frontend config from it.
  Backup: `backups/risuai-entry-20261003-2300`. Frontend-only rollback:
  `bash /opt/us-whitelist/test-new-api-20261002-174631/releases/risuai-entry-20261003-2300/rollback.sh`.
- Tavern static delivery is separately released at
  `releases/risuai-entry-20261003-2330/nginx-http.conf`; configuration
  `/etc/nginx/conf.d/tavern-http-20261003-1708.conf` is its symlink.
  Only `/assets/` enables gzip/buffering/one-year immutable caching.
  HTML/API/streaming retain their existing configuration; account is no-store.
  Static backup: `backups/risuai-entry-20261003-2330-assets`. Rollback:
  `bash /opt/us-whitelist/test-new-api-20261002-174631/releases/risuai-entry-20261003-2330/rollback-assets.sh`.
  Preserve prior immutable releases and current storage/credit records.
- Root cause evidence: multi-megabyte startup assets were uncompressed,
  with `Cache-Control: public, max-age=0`; the same script was fast server-local
  but slow over the public connection. Public browser startup transferred
  8,206,773 bytes before versus 1,718,515 after (79.1% less).
  After optimization, cold readiness was 22,943 ms and same-context reentry
  429 ms with all startup transferSize values 0. Actual public assets and
  normal HTTP cache were used, but account/storage fetches were synthetic
  and external HTTPS fonts/styles were CDP-blocked. Not real-user timing;
  old readiness timed out at 90 seconds, not an exact 90-second baseline.
- Nginx reload is asynchronous: initial immediate static-header check hit
  old workers and automatically rolled back only static configuration.
  The separate 2330 activation checked new cache headers for at most
  20 one-second intervals, then gzip/content equality; attempt 2 passed.
  Do not treat a successful reload signal as observed serving readiness.
- Current/frozen Svelte checks (0 errors/warnings), unminified Vite build,
  four isolated group/continue/layout scenarios and 264 deployed file
  checksums passed. Main container/binary unchanged, both applications
  healthy/restart 0 in final saved inspection. Entry remains plain HTTP
  `http://156.238.252.115:32795/tavern`. Evidence:
  `output/playwright/risuai-entry-20261003-2300/`.
  Owned test browsers were confirmed closed without touching the other
  `statekit-improvement` session. No real generation/paid call.
  overall: partially_verified; real-account visual/loading acceptance and
  paid settlement remain pending.

- US Tavern welcome/gift backend release (2026-10-03): retained immutable release is
  `releases/tavern-credit-20261003-2110` under
  `/opt/us-whitelist/test-new-api-20261002-174631`. Main entry is HTTP
  `156.238.252.115:32795/tavern`; native Tavern is HTTP
  `156.238.252.115:32796/?connect=1`. Native welcome uses site name/Airisu,
  connection uses the Tavern nickname, and account keeps main display name.
  Each account receives USD 1 once via independent `tavern_credits`.
  Only server-proved Tavern chat consumes it, before normal main billing;
  ordinary API/playground/images cannot use it. The browser has no service
  proof key and cannot grant credit by setting request flags.
- At gift activation, Tavern Compose needed SEVEN files, superseding the six-layer
  entry repair below: append `compose.credit.json` from the current release.
  Both application labels then listed all seven; the layout frontend above
  now adds the eighth layer for RisuAI. Binary/runtime/original assets reuse
  validated `...-2050` artifacts from the frozen `...-2028` source, not later
  concurrent worktree edits. Existing unrelated work remains uncommitted.
  Backups: `backups/tavern-credit-20261003-2110`. Rollback:
  `bash /opt/us-whitelist/test-new-api-20261002-174631/releases/tavern-credit-20261003-2110/rollback.sh`.
  Preserve gift records/database/storage when reverting applications.
- PostgreSQL deployment must give a new table to the application role,
  not the shared database administrator. Initial `...-2050` activation
  rolled back because GORM could not see administrator-owned tavern_credits.
  Corrected schema transaction assigns only that table and verifies access.
  Actual test NODE_TYPE is master. The retained proof key is backed up and
  reused; do not overwrite it or expose its value.
- Tavern validation is partially_verified: focused Go/Node/Vitest,
  Svelte/build, deployed hashes/health/boundaries and three fixture-assisted
  Chinese/English desktop/mobile UI checks passed. UI static assets used
  local files verified identical to deployment; identities/storage were
  intercepted and no real generation occurred. Public asset loading was
  slow and timed out, so real first-load performance, real-account visual
  acceptance, real paid settlement and MySQL runtime remain pending.
  Evidence: `output/playwright/tavern-credit-20261003-2110/verification.json`
  and today's `US Tavern Restricted Welcome Credit` progress/debug.

- US Open WebUI workstation preferences repair (2026-10-03): production
  uses frontend v25 on the unchanged backend image
  `sha256:9e155cff81dc2ec37031f6ff39af53b6b84a0133b52f058fae609e6d0e2f02e1`.
  A single native header action now receives the balance badge. Explicit
  chat model selections automatically save `ui.models` through the existing
  per-user settings API and share the ordered image-preference save queue.
  Production read-only inspection confirmed the owner's image model was
  already persisted as `gpt-image-2.5-web`; chat defaults were absent.
  No production account settings were directly changed.
- Active workstation deployment: retain the original two Compose layers in
  `/opt/image-studio/releases/20260928-173340-image-studio-redirect-v24/`,
  plus
  `/opt/image-studio/releases/20261003-200103-preferences-v25/frontend.override.yml`.
  Always use `--project-name image-studio --env-file /opt/image-studio/.env`.
  The override pins the existing image and mounts its prepared frontend
  `build/` read-only at `/app/build`. Preserve the existing
  `image-studio_open_webui_data` volume. Only Open WebUI was recreated; no
  build, pull, New API/Caddy restart, upstream inference or generation.
- Workstation verification: frontend 47/47 and image size 8/8; actual native
  controls in an isolated browser retained chat model, image model and size
  after reload. Single-action balance and nonoverlapping layout passed at
  1440x900 and 390x844. Production health/static endpoints returned 200;
  anonymous gateway access stayed 401. Real signed-in acceptance is pending:
  hard-refresh, select the desired chat model once, then revisit a new chat.
  Backup is the old release's `backup-20261003-200103/`. Rollback uses the
  original two Compose files with `up -d --no-build --no-deps open-webui`,
  omitting the frontend override. See today's progress/debug entries.

- US Tavern login return repair (2026-10-03): the Tavern sign-in link now
  targets main `/sign-in?redirect=%2Ftavern`; previously an already signed-in
  main session returned to `/dashboard/overview` instead of Tavern. Main
  `/tavern` itself was already correct. Only the native RisuAI frontend was
  changed, using a new immutable `releases/risuai-entry-20261003-1850` directory
  and `compose.entry.json`, with backup in `backups/risuai-entry-20261003-1850`.
  Active Compose now requires SIX files: the five below plus this latest
  entry override. Every other running container retained its ID/start time;
  New API's binary SHA-256 and data configuration are unchanged.
  RisuAI was recreated once for this repair. Rollback:
  `bash /opt/us-whitelist/test-new-api-20261002-174631/releases/risuai-entry-20261003-1850/rollback.sh`.
  Unit regressions reproduced the missing return parameter before repair;
  11 bridge tests passed afterwards and Svelte check reported 0 errors/warnings.
  Final browser-only checks at 1440x900 and 390x844 covered native sign-in
  links, main entry/login return and native account/chat-model onboarding.
  Both sizes had no page errors or horizontal overflow; generation count 0.
  Screenshots and captured results are under
  `output/playwright/risuai-entry-20261003-1850`. The test browser is closed.
  Real account login and paid settlement are still pending. No credentials
  were exported and no real provider generation was performed.

- US RisuAI HTTP Tavern (2026-10-03): user explicitly authorized deployment
  to the existing isolated test site using public IP/plain HTTP, not HTTPS
  or a domain and not overwriting prior releases. Main test port remains
  `32795`; native Tavern is `http://156.238.252.115:32796/?connect=1`.
  Main release is `releases/tavern-http-20261003-1708`, RisuAI frontend
  correction is `releases/risuai-portal-20261003-1820`; both are under
  `/opt/us-whitelist/test-new-api-20261002-174631`.
  Initial Compose needed all FIVE files: base, Video Studio, corrected Canvas
  CSS, `compose.tavern.json`, and `compose.portal.json`. Main container was
  recreated once; RisuAI was started then recreated for frontend-only repair.
  Node binds `127.0.0.1:32830`, with new Nginx/UFW TCP `32796` configuration.
  Formal services, test Redis, fixed Canvas assets and four reconnected
  Sub2API channels are preserved. No real generation or credential export.
  Frontend environment MUST include both `VITE_NEW_API_BRIDGE=1` and
  `VITE_NEW_API_PORTAL_URL=http://156.238.252.115:32795`; otherwise a signed-out
  user gets a broken Tavern-local login link. Initial runtime detection led
  to a new immutable frontend version, not an in-place asset replacement.
  Local 36 focused tests, types, builds, bundled HTTP boundaries and
  synthetic public-origin 1440x900/390x844 onboarding/model/settings checks
  passed. User count/quota/request counters matched the preactivation dump;
  browser generation count 0. Real account login and paid settlement remain
  pending (`partially_verified`). HTTP is deliberately unencrypted: use
  test credentials. Full backups and rollback commands are in today's
  progress under `US RisuAI HTTP Tavern`.

- US test Sub2API reconnection (2026-10-03) supersedes the synthetic-only
  channel status below. Only test database `newapi_test_20261002174631`
  channels 4/32/34/35 now use their existing, verified Sub2API credentials
  and `http://sub2api:8080`; all four are enabled with 31 enabled ability
  rows. Channel 4 previously had status 1 but disabled ability rows.
  The other 19 channels, pricing, models, groups, Compose, binaries and
  production New API configuration were not changed; no service restarted.
  Root-only backup and scoped rollback:
  `/opt/us-whitelist/test-new-api-20261002-174631/backups/sub2api-channels-20261003-171637`.
  All four upstream model-list GETs and test New API group-key GETs returned
  200. Existing test API keys were used only inside the server, not exported.
  Test quota/consume-log counters remained unchanged; studio/native tasks 0.
  Actual generation and billing remain pending user acceptance and consume
  real upstream credit. Channel 33 and all video providers remain isolated.
  `gpt-6`, `gpt-6-luna` and `gpt-reserve` lack billing configuration in both
  the test and production databases, so New API correctly filters them.
  The mixed-pool upstream also does not advertise `gpt-6`.
  These pricing/model issues were not bypassed or silently assigned prices.
  The other Sub2API test instance has no API keys and is not this connection.

- Canvas test-deployment CSS correction (2026-10-03): user found the
  authenticated workbench unstyled after the first deployment. Staging
  omitted the existing `web/postcss.config.mjs`, so Tailwind layout and
  breakpoint classes were never generated. Restoring this config in the
  isolated copy and rebuilding Canvas fixed the root cause without changing
  business code, backend binary, billing or production. Active static
  release is now
  `/opt/us-whitelist/test-new-api-20261002-174631/releases/canvas-css-20261003-1530`;
  active Compose requires base `compose.yml`, the earlier
  `video-studio-20261003-1433/compose.override.json`, AND
  `canvas-css-20261003-1530/compose.canvas-css.json`.
  Only the static mount differs. Old files are retained, config snapshots
  are under `backups/canvas-css-20261003-1530`.
  Compiled-CSS regression, public served hash, healthy test startup and
  synthetic authenticated 1280x720/390x844 browser layout passed.
  The user confirmed the layout recovered; real generation remains pending.
  Never select a frontend build's root files solely by extensions without
  checking its PostCSS/build configuration; validate compiled utilities.
  Today's progress contains active/rollback commands and screenshot locations.

- US Video Studio test deployment (2026-10-03) supersedes the older test
  binary/slave status below. Only `new-api-test-20261002-174631` was updated
  to `v1.0.0-rc.28-custom-video-studio-20261003.1` using a new immutable
  release and Compose override, not overwriting `compose.yml` or the old
  program. Release root:
  `/opt/us-whitelist/test-new-api-20261002-174631/releases/video-studio-20261003-1433`.
  Original test DB/Redis/data/binary/config backup:
  `backups/video-studio-20261003-1433`, plus a second pre-ownership-change DB
  dump under `backups/video-studio-20261003-1433-ownership`.
  The cloned tables/sequences were owned by `sub2api`, which blocked master
  migration; only their ownership inside the isolated test database was
  assigned to its existing dedicated test role. No production role/database
  or source code changed. Master/UPDATE_TASK now work with synthetic-only
  channels and automatic upstream tests/model/status checks disabled.
  `studio_jobs` and model `parameters` migration, healthy startup, assets,
  unauthenticated 401 boundary and browser login gate passed. No remote
  generation or paid call; authenticated workflows remain pending.
  Public test binding was already `0.0.0.0:32795` before this work; older
  loopback-only notes below are historical. Prefer SSH-tunnel login, since
  the public endpoint is HTTP. Automatic local tunnel startup was blocked
  by tool policy and no tunnel exists.
  Production and test Redis container identities/start times remained
  unchanged. Details, rollback and active Compose command:
  `docs/progress/2026-10-03.md`. Do not run only the base Compose file when
  intending to keep this version: base-only returns to the preserved old
  binary and slave mode.

- RisuAI managed-settings adaptation: keep the upstream native UI and the
  existing New API at localhost:3000 / `.local-tests/abc-shared.db`, with RisuAI
  at localhost:3200. Site labels come from `system_name`, not a hardcoded
  gateway name. Chat groups/models are capability-filtered server-side.
  `gpt-image-*` recognition covers the locally configured image variants.
  Until Auto candidates and non-chat services have verified hosted contracts,
  RisuAI omits Auto and disables image generation and remote TTS; do not claim
  those features are already New API billed. Provider URL/key/tokenizer/network
  controls are hidden in bridge mode, including auxiliary and fallback model
  entry points. Imported credentials are not deleted. Native local/creative
  user settings remain. Scoped automated checks and synthetic desktop/mobile
  verification passed; the user's real-account and paid settlement acceptance
  remain pending. Details: `docs/progress/2026-10-02.md`, RisuAI Managed Settings.

- US isolated New API test instance (2026-10-02): built from the current
  local dirty worktree, not the older GitHub snapshot. Local frontend
  profitability tests passed 29/29, frontend typecheck/lint/format passed,
  backend `Profitability|Sub2API` tests passed, and the Linux amd64 CGO-free
  binary was built with SHA-256
  `d503e5953159a2eed4e4e4ac28234d6377c3d971010109265926d49435b5e750`.
  The test service runs as `new-api-test-20261002-174631` on
  `127.0.0.1:32795`, version
  `v1.0.0-rc.28-custom-local-test-20261002`, with an independent Redis
  container, a cloned PostgreSQL database, a distinct database user,
  distinct session/crypto secrets, and `NODE_TYPE=slave`.
  The cloned test database has 23 channels rewritten to loopback address
  `127.0.0.1:9` with fake keys; one channel remains synthetically enabled
  for selector testing. Production `new-api`, `old-new-api`, Sub2API,
  production Redis and PostgreSQL container identities/start times remained
  unchanged. Test endpoint health and unauthenticated profitability boundary
  checks passed. Test-only options disable Turnstile and registration and set
  the server address to `http://127.0.0.1:32795`, so SSH-tunnel login does
  not depend on production origin validation. Human browser/login acceptance
  is pending.

- Video Studio background integration (2026-10-02): the managed image/video
  workbench now uses durable New API-owned jobs and private result storage.
  Accepted submissions survive browser close, polling resumes by the same job
  identity, and a lost submission response is resolved by its idempotency
  request ID without a second paid POST. Ambiguous upstream submission is
  marked uncertain and is never blindly resubmitted. A local synthetic
  end-to-end check submitted one image, closed the page while the mock held
  the response, released it, reopened the workbench, recovered the result,
  and observed exactly one synthetic charge.
- Model-specific creation parameters are configured in the existing admin
  Models -> Metadata -> Edit drawer under "Studio parameter profiles".
  Supported allowlists cover image/video sizes, ratios, quality/count,
  duration/resolution/mode, audio/watermark, and FPS; group JSON overrides
  replace only the configured fields for that group. The local fixture was
  checked by overriding the default group's video duration to 8 seconds and
  observing only 8 seconds in the managed video selector.
- Session renewal is limited to the trusted managed bridge. GET/polling may
  renew once; a billable POST is never replayed after a 401 or uncertain
  response. Canvas state stays mounted during renewal, so renewal does not
  replay paid work or lose the current page state.
- Current synthetic verification preview: `http://localhost:3121/video-studio`
  with fixture account `videotest` and a local mock provider on port 3108.
  The account and provider are synthetic only. Port 3000 is currently stopped;
  the normal account-snapshot launcher remains available but must not be used
  for paid acceptance because it runs as a slave with task updates disabled.
- Real upstream video completion/download, production database migration,
  MySQL/PostgreSQL runtime, and US-server deployment remain unverified. The
  original `.local-tests/abc-shared.db` was not modified.

- Video Studio background-job clarification (2026-10-01): "background" means
  server-owned generation that continues after browser disconnect/close, NOT
  a new administrator management page. User asks for the design of per-model
  parameters and explanation of session expiry before further implementation.
  Current studio image routes still use synchronous relay and browser-side
  result persistence; video already uses New API task records/polling primitives.
  Proposed direction: durable server task/result ownership, reuse existing
  routing and billing, and resume by task identity without blind resubmission.
  Capability constraints should come from verified server configuration, with
  group differences respected; the current catalog exposes only broad types.
  Browser token renewal is separate from execution of an accepted server job.
  No implementation or production change has been made for this clarification.

- Video Studio follow-up review (2026-10-01): user defers real image-model and
  billing acceptance until server rollout. Before broad release, prioritize
  distinguishing workbench polling/download interruption from terminal task
  failure: the existing Retry starts a new potentially billable task.
  Parameter-aware controls and expired-session recovery are recommendations,
  not authorized new implementation. See today's review record; no production
  operation or feature edit was made during this assessment.

- Video Studio group/model filtering (2026-10-01): `/api/video-studio/catalog`
  derives permitted groups and model capabilities from existing New API settings.
  Video selectors show only usable video models/groups; image/text/audio use
  independent capability filters. Submissions carry an encoded selected group,
  validated before routing; video capability is checked before charging.
  Existing tasks remain owner-readable after a group/model is removed.
  Same-name models retain distinct group identity in persisted settings/nodes.
  No separate administrator settings or wallet were added.
  Focused Go tests, 11 Canvas checks, typecheck and builds passed; desktop
  filtering/persistence/image separation/empty-state checks passed. Mobile
  selector verification is pending because the browser viewport override had
  no observable effect. Paid provider settlement remains untested.
  Local 3000 PID 10728 and synthetic 3117 PID 38400 run the same tested binary.
  Backup: `G:\new-api-video-runtime\before-catalog-20261001-153103`.
  Live Canvas entry: `index-DPGUdI7K.js`; full details are in today's progress
  and debug records. The 3000 account snapshot currently has no video models,
  so its empty state is expected. Original database and US services were not
  modified. User accepted the prior navigation placement; this new filtering
  behavior still needs user acceptance.

- US profitability deployment preflight (2026-10-01): production remains
  healthy on `v1.0.0-rc.28-custom-profitability-20260930.2`; no deployment or
  restart occurred. The exact old binary is retained, but its source baseline
  is unverified (`c5a2846e858922d5620dde44e9032e4bd8c99613`, modified tree).
  Known release metadata does not contain the source snapshot. Do not assume
  the current dirty checkout or the September 28 Git snapshot reproduces it.
  Resume after recovering a verifiable baseline or separately approving
  review/release of the broader local version.
  Superseding authenticated GitHub check on October 1 confirmed New API
  `main=a2cc5d6a9c5bd68e866fbfc5d37d0223c55ccb1e`, committed at
  15:31:02 +08:00. All 14 checked profitability/integration/locale files
  match the local source, including group ratios and enabled-only purchases.
  The earlier September 28 backup gap is resolved. This snapshot also
  contains Video Studio routes and the Tavern entry; its 73-file diff is
  against the September 28 backup, not the exact running production source.
  Await explicit approval for that broader New API release scope before
  building/deploying the complete snapshot. No US changes were made.

- Video Studio refresh follow-up (2026-10-01): the static route now sends
  `Cache-Control: no-cache`; the host uses `/video-canvas/video?v=2` once to
  retire the old unversioned entry cached without that policy. Port 3000
  was rebuilt/restarted with the same isolated preview.db; HTTP checks
  confirm the new host bundle, current Canvas entry and conditional 304
  cache policy. The identical executable on 3117 passed normal reload and
  dashboard-return UI checks. The user's authenticated Edge view could not
  be inspected because the connector rejects the current API-key auth mode;
  its exact old-cache cause and final user acceptance remain unconfirmed.
  Backup: `G:\new-api-video-runtime\before-entry-refresh-20261001-143929`.
  Original database, production and the 3107 dev process remain untouched.

- Video Studio navigation follow-up (2026-10-01): the managed Canvas header
  now starts with a dashboard-return control. Its wallet balance is a
  rightmost 8px-rounded rectangle after the account menu and remains visible
  on phones. Mobile docs entry moves into the account menu. Canvas assets
  were initially rebuilt in the shared G: dist only; no host rebuild/server restart,
  database write or billing change. Desktop light/dark and 320/390px layouts
  plus return navigation passed; final visual acceptance remains pending.

- 2026-10-01 (Asia/Shanghai) local profitability now shows an independent
  table row for each active group: current configured rate, charges,
  attributed Sub usage, charge/Sub ratio, and estimated group break-even.
  Break-even reconstructs a rate-one revenue basis from historical applied
  call rates, not today's setting. Channel-local Sub shares allocate purchase
  cost once; switching administrator scope does not remove cost. All-call
  break-even is an all-calls-paid comparison, while operating profit always
  excludes administrator/gift income. Missing history, pricing overrides,
  incomplete attribution/procurement and cash mode keep break-even unknown.
  26 UI/API tests, targeted Go tests, type/lint/format and sequential builds
  pass. Desktop/mobile synthetic UI checks pass; user and real-data acceptance
  remain pending. New fixture preview: `http://127.0.0.1:3116/dashboard/profitability`.
  The old 3106 fixture process remains unchanged and has no group data.
  Not deployed; see `docs/progress/2026-10-01.md` and today's debug record.
  Follow-up: configured rate now sits immediately before Status, after
  estimated group break-even; 27 UI/API tests and local layout checks pass.
  Purchase-entry follow-up: only enabled channels are offered for new account
  costs; disabled statistical history stays available. 29 UI/API tests and
  type/lint/format checks pass; not built or deployed in this follow-up.

- The user approved the upstream Infinite Canvas UI and resumed LOCAL
  integration. `/video-studio` now has no outer title/account bar. The
  original Canvas navigation includes a compact account menu and desktop
  balance link. Wallet/profile/usage/task/pricing open the same New API;
  return and confirmed logout use the host application.
- Balance is read from `/api/user/self`, formatted with New API's existing
  currency helper, and synchronized to its shared account cache. Trusted
  Canvas events trigger refresh after generation and terminal video status;
  focus refreshes after recharge. No separate wallet or price calculation.
- Fixed `/video-canvas` in the Rsbuild proxy. Live GET on port 3107 now
  returns Canvas assets, not the New API dev entry. This was the loading
  screen root cause; its regression test failed before the fix.
- Dedicated synthetic preview: `http://localhost:3117/video-studio`,
  runtime/database under `G:\new-api-video-runtime\native-preview\verification`.
  It contains no copied provider keys. Its displayed $200 is fixture credit,
  not the user's real balance. Desktop 1280x720 and mobile 390x844 UI and
  native return-to-dashboard were observed. Paid generation remains untested.
- The owned account-snapshot preview at `http://localhost:3000/video-studio`
  has been rebuilt/restarted with the native UI. Use the existing local
  account credentials; it still runs against isolated `preview.db` as slave.
  Both preview JS/CSS fingerprint paths match the final build. The original
  `.local-tests/abc-shared.db` remains untouched. Keep paid acceptance out of
  this slave preview; real channels may still be called by clicking generate.
- Final validation: 18 New API UI tests, 6 Canvas tests, scoped Go billing
  tests, both typechecks/builds and scoped New API lint/format passed.
  Native menu return, confirmed logout, and new-tab same-account wallet
  were browser-tested. User visual acceptance and real async generation,
  content download, live provider pricing/refunds remain pending.
- Retained upstream source:
  `G:\infinite-canvas-original-preview\source\basketikun-infinite-canvas-dab19ad`.
  Rejected account-header backup:
  `G:\infinite-canvas-original-preview\paused-account-integration`.
  No US server operations are authorized by this integration request.

- 2026-10-01 Tavern direction changed at the user's request to upstream
  RisuAI. Source: `D:\Open source project\V Visa-free\risuai`, commit
  `f9728b14c733d073168d8d0c6a8f34f8febf0304`. Preserve the native Svelte UI
  and upstream attribution. Do not add a header, iframe, or custom chat UI.
  Native settings hold New API account/balance and group/model selection.
- `/tavern` now redirects to `http://localhost:3200/?connect=1`.
  `start-local-tavern.ps1` launches the native Node server. Backend 3000 and
  the same local New API at 3000 is the login portal; this integration does
  not start another New API instance.
  Configuration and verification commands are in `ops/risuai/README.md`.
- RisuAI uses New API's existing session refresh and fixed
  `/pg/chat/completions` billing path. Bearers remain in encrypted HttpOnly
  cookies. There is no shared provider key or second wallet. Storage goes
  to `.local-tests/risuai-runtime-3200/data/users/<New API user ID>`;
  preserve that directory and `data/.session-secret`.
- The integration is HTTP-loopback-only and needs no Mongo/Redis/Docker.
  External plugins, image generation, and TTS are not claimed as integrated
  billing features. No production/US operations or paid inference authorized.
- The retired Tavern containers, Mongo/Redis volumes, and project network were
  removed after a data export. The offline backup is
  `.local-tests/retired-tavern-backup-20261001`: Mongo archive (one user, one
  profile, one character, no chats/messages), original `.token_secret`, and
  Redis data. Mongo dump and restore dry-run passed; archive SHA256:
  `1A7B7A255A9B0CBA1A1550867C4799F8E8533D697D3B6BD5C55553806236ECA5`.
- The active portal helper/configuration files for the retired Tavern were
  removed or replaced. Historical audit records and any offline backup copies
  are not runtime inputs; do not load them into the active deployment.
- RisuAI dependencies use C: due D: having only about 630 MiB free:
  `C:\tmp\risuai-node-modules` (virtual store) and
  `C:\tmp\risuai-pnpm-store`. The lockfile was not changed.
  Build/runtime status is recorded in `docs/progress/2026-10-02.md`.
- October 2 verification: RisuAI build, Svelte check (0 errors/warnings),
  13 HTTP bridge tests, 5 frontend bridge tests, and 3 New API entry tests
  passed. New API typecheck and scoped Tavern lint passed. October 1 browser
  verified native onboarding, character creation, chat, model settings,
  and synthetic debit (901: 5,000,000 -> 4,987,500; 902 unchanged).
  The observed browser request was non-streaming; SSE is tested at the
  bridge HTTP boundary, not yet verified in the final browser build.
- Earlier on October 2 the resumed environment had no listeners and background
  launch was rejected by tool policy. The continuation later started only the
  synthetic fixture on `127.0.0.1:3211` and RisuAI on `127.0.0.1:3210`.
  The launcher rebuild with `VITE_RISU_LEGAL_CONFIGURED=TRUE` was required
  after a standalone build overwrote the private self-hosting flag.
- Final synthetic browser verification loaded the native RisuAI chat page,
  restored Alice's chat, showed the selected `Fixture-chat` model badge, and
  returned 200 from `/api/new-api/account`. Desktop and 390x844 screenshots
  are in `output/playwright/`. The only console error is the expected
  RisuRealm 404 because external Realm/plugin routes are unavailable in
  bridge mode. No paid provider request or US-server operation was performed.
- Current local account flow uses the existing New API process on
  `http://localhost:3000` with `.local-tests/abc-shared.db`; RisuAI is a
  separate native frontend on `http://localhost:3200/?connect=1`. Both
  bridge and portal URLs use `localhost:3000` so the login cookie is shared
  across ports. A 401 before the first login is expected.
- The local3000 executable must be rebuilt after frontend changes because
  `main.go` embeds `web/dist`. The September 27 binary lacked the Tavern tab;
  the pre-rebuild binary is preserved at
  `.local-tests/new-api-local-b-c.before-tavern-20261002.exe`.
- The default RisuAI `pnpm test` now excludes the Node-native bridge test file
  from Vitest and runs it afterward with Node: 28 Vitest files / 246 tests
  passed with 3 skipped, followed by 13 Node-native tests passed.

- 2026-09-30 expanded local retirement audit: exact full paths and restore
  evidence are in `docs/local-cleanup-paths-2026-09-30.md`. After formal
  approval, removed three GitHub-backed publishing copies, the empty backup
  directory, and two old Go caches: 16,598 files, 2.21 GiB logical size.
  Retained Git state, 834 other local-test files, databases/secrets, and nine
  duplicate-preservation hashes were verified unchanged before doc updates.
  Active main, distinct rc28/quality changes, data, and linked owners remain.
- US read-only inspection found New API running the September 30
  profitability executable, whereas the custom GitHub main snapshot
  remains September 28 (`2ec1006`). Deployment and full source backup are
  separate facts; do not infer all current local work is already pushed.
- 2026-09-28 local cleanup: `new-api`, `new-api-rc28`, and
  `new-api-turnstile-deploy` share this repository's `.git`; do not delete
  the owner while linked worktrees are needed. Four redundant branches and
  three absent-worktree registrations were removed.
- `.local-tests` is NOT disposable as a whole. Local startup scripts use
  `abc-shared.db` and `image-studio-session-secret` there; it also holds
  backups and Studio fixtures. These live files remain present and the local
  health check passed after cleanup.
- About 4.48 GiB of logical file content was removed: old binaries, main
  `web/dist`, and dependencies in the two older New API worktrees. Three
  tracked `bin` scripts accidentally included in the list were restored
  from the index and verified. Main dependencies and the shared junction remain.
  Rebuild frontend assets before a future source build.
- Verified GitHub snapshots are not complete current-state backups:
  `new-api-custom:2ec1006` differs from seven current files before this journal
  update; `sub2api-custom:b990204` omits three source/documentation files
  retained in the original checkout. See today's local cleanup audit.

- 2026-09-28 v23 is deployed with per-user image model/size preferences via
  the existing Open WebUI user settings API (`ui.newApiImageModel`,
  `ui.newApiImageSize`). Fresh chats restore preferences; explicit draft
  values win. Saves are ordered and failures are visible. Existing data
  volume is retained; New API/Caddy were not restarted. Image:
  `sha256:a2f65548591f6e1bf263e9f4b3987f33d08e74d869ae186620ffb34100d29627`.
  Frontend 43/43, bridge/size 28/28 and native delivery 13/13 pass.
  Signed-in refresh/new-chat acceptance is pending, without needing generation.
  Latest image failure is `ConnectionTimeoutError` to `image.cjyyswq.com`;
  host and container both reproduce it, while container external HTTPS works.
  Image delivery remains unresolved and must not be described as fixed.
  No paid generation, system proxy or billing changes were made this round.
- 2026-09-28 actual image request display and delivery diagnostics are
  deployed in WebUI image
  `sha256:6b35b4bf7e0315133a6228b850ccec6b2d4654ddcfadb379713ff0d2fb69bf5c`.
  `generate_image` keeps the model's original arguments separate from the
  persisted actual request summary; the latter shows the effective model and
  size from the same tool result, including failed delivery results. The
  bridge records model/size/count/stage, prefers inline image data, bounds URL
  downloads, returns a delivery-specific error, and blocks an automatic
  repeat after a possibly charged delivery failure.
  Focused and full isolated checks are green: delivery/native 13/13,
  image-size 8/8, bridge 20/20, and frontend/link/billing/tool-result 41/41.
  Container health and internal `/health` are good on the original volume.
  The observed prior request was charged and then failed at URL download with
  `ServerDisconnectedError`; the upstream URL/body cause is still unknown.
  Real signed-in generation and visual acceptance remain pending.
- Per-request image size control is now deployed in local Studio image
  `sha256:520ea9dba272cd428cfc665a7ef1b776e7555a982aef01ea42cc5fea5923e3e`.
  The checked-by-default image toggle has a native size picker with `Auto`,
  model-supported ratios, and custom pixel input. Conversation dimensions or
  ratios take precedence over the picker, and the picker takes precedence
  over the administrator default. Unsupported values are rejected before
  relay without automatic resizing or retry.
  Focused checks are green: image-size 8/8, bridge 20/20, native tool/payload
  7/7, and frontend/link/billing 34/34. The container is healthy on the
  existing data volume; real signed-in generation, billing, and visual
  acceptance remain user-owned because the connected browser was blocked.
- 2026-09-28 local Studio has a backend-only fix on the v20 frontend.
  `get_builtin_tools` had omitted `__features__` while binding the image
  tool, causing a 400 despite a selected image model. `patch_tools` now binds
  the request features; executors, model validation and group policy are
  unchanged. Old-image reproduction fails; new-image native tool checks pass
  3/3, bridge 20/20, model HTTP 3/3 and frontend tests 33/33. Only WebUI was
  recreated and is healthy with the original volume; no paid inference was
  sent. User generation/actual billing validation remains pending.
  Config retains group `group_1`, default `gpt-image-1`, size `512x512` and
  editing off. `CreateImageForm.size` supports an override, but chat has no
  user size control yet and uses the global default. See September 28 notes.
- 2026-09-27 v20 is deployed locally and healthy. Admins choose a dedicated
  image group in the native 图片 settings; users choose any model returned by
  that group beside the checked-by-default image-generation toggle. The
  configured image group is excluded from the chat LLM group picker, while
  image listing and generation use the same configured group with the signed
  current user. No group or unavailable image list leaves ordinary chat
  usable. Real signed-in image output and billing remain user-owned pending.
- The duplicate picker regression is fixed by placing the custom picker and
  native image toggle in one conditional DOM range. v20 frontend/native/link/
  billing tests pass 33/33, bridge tests pass 20/20, and a desktop/mobile
  fixture confirmed the settings field and toggle lifecycle. The deployed
  container uses the existing `image-studio_open_webui_data` volume; no
  provider request was sent.
- 2026-09-27 local Studio now has a per-request image-generation model
  selector beside the native Image toggle. The toggle starts enabled for a
  new or legacy draft; the picker prefers `gpt-image-1`, lists only
  `supported_endpoint_types: image-generation` models from the current
  account/group, and stores the explicit choice in the draft. The bridge
  validates that choice against the request-local authorized model registry
  before image generation. The deployed Open WebUI image is v19, healthy, and
  uses the existing data volume. Real signed-in visual acceptance and real
  image output remain pending; no upstream inference request was sent.
- 2026-09-27 local Studio image generation is now enabled at the user's
  explicit request; real generation testing is user-owned. Local `.env`
  and WebUI persisted config both enable generation and use `gpt-image-1`
  as the default. Image editing stays disabled. The existing per-user
  Studio bridge, group authorization and accounting are unchanged.
  Enabled New API channel 4/group `1` currently lists `gpt-image-1`,
  `gpt-image-1.5` and `gpt-image-2`; actual upstream support is unverified.
  WebUI was recreated with the same v18 image and data volume and is healthy.
  Refresh the page and enable the chat's image-generation feature for testing.
  This supersedes historical notes that local image generation is disabled.
  The user confirmed the preceding return-to-console UI is satisfactory.
- 2026-09-27 v18 is deployed locally. The return-to-console link is inserted
  immediately above Open WebUI's native New Chat item and is available to
  ordinary users even without Workspace. It copies the native row/link
  classes and icon treatment; hidden duplicate mobile buttons are ignored.
  The existing local New API instance was restored after a normal termination;
  `/api/status` is 200, anonymous Studio metadata remains 401, and Open WebUI
  is healthy. Signed-in visual confirmation is still pending.
- 2026-09-27 Arena placeholder removed from Studio. `Arena Model`
  (`arena-model`) was Open WebUI's built-in evaluation fallback, enabled by
  default, not a New API model. The bridge patch now excludes Arena models
  only in bridge mode; native non-bridge Open WebUI behavior is unchanged.
  Deployed container AST confirms the guard and the model HTTP regression is
  green. Refresh Studio to confirm the row disappears; signed-in visual
  confirmation remains pending.
- 2026-09-27 v17 is deployed locally for the group-picker visual pass.
  The picker has clearer hover/open/focus feedback, more label/value spacing,
  right-aligned selected values, and per-option account-scoped ratios
  (`N×`, `min-max×`, or server text such as `自动`). The existing WebUI
  Select and `/studio-v17/` runtime/store namespace are reused. No backend,
  permissions or billing logic changed. 24 JS tests and deployed content
  checks pass; WebUI is healthy with the original volume. Signed-in visual
  confirmation remains pending.
- 2026-09-27 v16.1 is the current local Studio deployment. Model HTTP 401
  was a bridge implementation defect: matching the base URL missed the
  `/models` request and omitted the signed cookie/identity/group. Fixed
  bounded matching/forwarding and error propagation; no shared key or
  relaxed New API permission/overload guard. External-connection admin
  navigation and the model empty-state setup action/prompt are hidden.
  Native modules and custom imports share `/studio-v16/`; index loads
  `studio-link.js?v=16.1` for the model-request sequencing fix, other custom
  assets remain v16. This replaces unchanged native asset URLs without
  claiming that browser cache was proven on the user's page.
  Verification: 23 JS, 12 Python bridge and 3 isolated real-HTTP tests pass.
  WebUI is healthy with original volume and matching source asset hashes;
  New API/Caddy were not restarted. Signed-in visual/model/provider/billing
  acceptance remains pending. See today's progress/debug sections.
  HTTP regression command (run from repo root with the built image):
  `docker run --rm --network none --entrypoint python --mount
  "type=bind,source=D:\Open source project\V Visa-free\new-api\ops\image-studio,target=/studio,readonly"
  image-studio-open-webui:local -B /studio/test_open_webui_model_http.py -v`.
- 2026-09-27 v15 supersedes the earlier v14 group-picker implementation:
  group selection is now part of the native model popup's Svelte lifecycle,
  directly reusing WebUI Select/dropdown/icons, not an HTML select or observer
  insertion. Source: `ops/image-studio/studio-group-picker.js` and the
  `patch_model_selector` hook in `patch_frontend.py`. The pinned runtime W
  export is the required branch effect; N is not interchangeable.
  Bridge/unit and real compiled-component DOM checks pass 21/21. Only local
  WebUI was rebuilt, healthy with v15 assets and the original data volume;
  New API/Caddy were not restarted. Actual signed-in visual acceptance is
  still pending because the browser blocks localhost navigation.
  Native integration command: `node --experimental-vm-modules --test
  --test-timeout=15000 ops/image-studio/studio-native.integration.cjs`.
  It reads v14 base container chunks copied to the ignored
  `.local-tests/studio-native-v15/chunks` (or `STUDIO_WEBUI_CHUNKS`), applying
  the current selector patch in memory. It does not verify real browser layout.
- 2026-09-27 metadata availability fix: account context/groups no longer pass
  through relay overload admission. They still require the current signed
  Studio session; `/v1` models and generation keep the existing guard.
  Reproduced failure was host memory 92% against the unchanged 90% threshold.
  Focused router/controller tests passed and the local backend was rebuilt
  and restarted with the original database/secret. Backup:
  `.local-tests/studio-metadata-before-20260927/`. Signed-in acceptance remains
  pending; high host memory can still legitimately prevent model requests.
- 2026-09-27 v14: group selection now mounts inside the native model popup
  above `model-search-input`, not alongside the trigger in the composer.
  Styling follows the native compact menu and dark/light themes. Popup
  recreation retains the selected group; 18 JS regressions pass and deployed
  resource hashes match source. No account, model-access or billing policy
  changed. Signed-in desktop/mobile visual acceptance is pending because
  browser navigation was blocked by the client.
- 2026-09-27 v13 startup follow-up: corrected the live Caddy routing after
  `studio-link.js` received HTML for its context request. The mounted file
  already included the new context/groups routes, but the active gateway
  still forwarded only billing to New API. Validated and hot-reloaded the
  existing file; live/adapted configs now match and anonymous account routes
  still return 401. No application rebuild or database change was needed.
  Signed-in browser acceptance remains pending after a page refresh.
- 2026-09-27 account-scoped integration: Open WebUI now uses the New API
  account name, hides its independent sign-out entry, and checks the live New
  API session before continuing. Groups and models are fetched for the current
  account; group changes reset native selection and are sent with every relay
  request. Chat/Responses/audio/image relay requests use the signed Studio
  cookie and settle against the current New API user's quota, with no shared
  Studio key. The local Open WebUI image is deployed at asset version v13 and
  is healthy. Native schedule, automation, exploration, status, and online
  count features remain. Image generation/editing are still disabled pending
  live image-provider validation. Signed-in browser acceptance remains
  pending. See `docs/progress/2026-09-27.md`.
- 2026-09-27 follow-up: Open WebUI now runs injected assets `v=12`. User confirmed v11 sidebar alignment is correct. v12 adds the native online count immediately after used quota, fetching `/api/usage` with existing same-origin cookies every 30 seconds. Count means users active in Open WebUI during the last three minutes, not New API site-wide users. Failed counts show unavailable, independently of billing. Eight jsdom tests pass; the container is healthy and JS/CSS match source. The actual page was blocked by the browser client, so v12 visual placement and signed-in count still need user confirmation. Existing menus and permissions are preserved; username/session unification and group/model/per-user billing integration remain pending. See `docs/progress/2026-09-27.md`.
- 2026-09-26 follow-up: The Open WebUI billing overlay was replaced with a sidebar return link below “Workspace” and a rounded balance badge placed immediately before the native top-right action buttons. The first implementation inserted the sidebar link into a horizontal row and selected a login toast button by coordinates; this was corrected by locating the outer navigation item and restricting balance placement to `header`/`banner` controls. The derived image now injects `billing-bar.css?v=5` and `billing-link.js?v=5`; focused script tests and container hash checks pass. A signed-in desktop/mobile visual check is still pending because a fresh Playwright session receives the expected anonymous `401`.
- 2026-09-25 follow-up: The active local `http://localhost:3000/` has a
  distinct authenticated "Image Workbench" top entry at `/image-workbench`,
  now redirecting to the full-page `/studio/` workbench without an iframe.
  The page refreshes the New API session via its same-origin HttpOnly cookie
  and keeps the short-lived access token in memory. Its loopback
  history service is on `127.0.0.1:8787`, with user-scoped assets in
  `ai-image-workbench/.image-sub2api-studio-data/newapi`. The previous
  `/image-studio` and Open WebUI on 3002 remain installed but are not the
  new entry. Mobile reference tray and prompt were checked at 390px and
  320px. Mock user isolation passed; real signed-in browser integration,
  generation/edit and per-user billing still need an image-capable provider
  and live validation. See `docs/progress/2026-09-25.md` and the workbench's
  `docs/NEW-API.md` for startup and verification scope.
- Later on 2026-09-25: Open WebUI `v0.11.4-slim` and Caddy are running locally
  on `127.0.0.1:3002`, launched via the `3000` Image Studio tab. New API
  SSO for administrator and regular user, account isolation and volume-backed
  chat persistence were verified. Config: `ops/image-studio/.env` (ignored);
  persistent New API secret: `.local-tests/image-studio-session-secret`
  (ignored). The local DB backup is
  `.local-tests/image-studio-runtime-20260925/`; the Open WebUI DB backup is
  in that directory too. A user must sign in again after the secret rotation.
  The Open WebUI page keeps its native layout, not New API's UI. Generation,
  editing and chat inference remain intentionally disabled because the local
  site has no image model and Open WebUI's global image key would bill one
  account rather than the initiating user. Real image generation is not
  delivered or verified; an image provider and signed per-user relay are next.
- 2026-09-25: Local `http://localhost:3000/image-studio` no longer presents
  New API Playground as Open WebUI. It shows an unavailable state while
  `image_studio_enabled=false`. The active binary was rebuilt and restarted
  with the original `.local-tests/abc-shared.db`; backup is at
  `.local-tests/image-studio-honest-state-20260925/`. At that checkpoint Open
  WebUI was unstarted: its `.env` was absent, the local three channels had no
  image model, official image pulls had timed out, and the Compose key would
  bill one shared owner rather than each visitor. A real Open WebUI product
  experience at the 3000 entry is not yet delivered.
- 2026-09-24: The local `http://localhost:3000/` process now serves the
  Image Studio top tab from this active `new-api` repository (PID 19336 at
  verification). At that date, with Open WebUI unconfigured, the tab opened the
  Playground image conversation (changed on 2026-09-25). When configured, the same route uses the
  session bridge to a separate Open WebUI origin. The live SQLite database is
  still `.local-tests/abc-shared.db`; its online backup and the prior binary
  are in `.local-tests/image-studio-before-20260924/`. Open WebUI runtime,
  SSO, per-user accounting and real image generation remain unverified.
- Playground image generation was converted on 2026-09-23 to a conversation-style UI. It exposes only count, size, quality, and style; supports JSON conversation import/export/clear; requests image-only models through `/api/user/models?modality=image`; and clears stale chat models while switching modalities.
- The final local build is running at `http://localhost:3000` from `bin/new-api-local-b-c.exe` with `.local-tests/abc-shared.db`. Pre-deployment executable backups are `bin/new-api-local-b-c.exe.pre-image-chat-20260923` and `bin/new-api-local-b-c.exe.pre-image-chat-model-clear-20260923`.
- The local image-generation relay route is `/pg/images/generations`. Real generation on the current test database can still be blocked by the existing disk-overload guard when disk usage exceeds its configured threshold; that is separate from route or UI availability.
- Online GeoIP verification completed 2026-09-22: `api.yisama.asia` is Cloudflare-proxied, the deployed `new-api` container is healthy, and `/cdn-cgi/trace` returned `loc=CN` from the current Chinese network and `loc=US` from the US server exit IP. The deployed container had no `LOCAL_GEOIP_COUNTRY` fixture variable. No remote mutation was needed after inspection.
- Homepage-only GeoIP notice verified 2026-09-22: only `web/src/routes/index.tsx` mounts `GeoIPNotice`; `/sign-in`, `/pricing`, `/about`, and authenticated settings pages do not perform the lookup. The active local build is served at `http://localhost:3000/`. `LOCAL_GEOIP_COUNTRY=CN` is a local simulation, not real IP geolocation.
- GeoIP daily-dismissal update verified 2026-09-22: unified `http://localhost:3000/` uses `bin/new-api-local-b-c.exe`, PID 17784 at verification, the existing `.local-tests/abc-shared.db`, and unchanged standard startup/PID files. Backup: `.local-tests/geoip-today-backup-20260922-093259`. The runtime still sets `LOCAL_GEOIP_COUNTRY=CN` for local preview; this is not real geolocation. The startup script itself does not set that fixture variable.
- GeoIP preference is `notification-storage.state.geoIPClosedUntilDate` in same-origin browser localStorage, compared with the visitor's local `Date.toDateString()`. Checkbox + dismissal suppresses same-day revisits, not API calls. Next-day navigation/reload restores eligibility; it does not force-open at midnight in an idle tab. Real hosting must be proxied by Cloudflare for same-origin `/cdn-cgi/trace`; failure/missing trace is unknown, not overseas.

- Local unified entry confirmed 2026-09-22: `http://localhost:3000/`, PID 18348 at verification, `bin/new-api-local-b-c.exe`, existing `.local-tests/abc-shared.db`. Status images are under Extra Settings at `/system-settings/operations/status-images`; port 3001 is stopped. Local source of truth for this integration is this `new-api` repository, not sibling `new-api-rc28`. Backup: `.local-tests/status-images-unify-20260921`. Original accounts/settings remain; Sub is still sample-only.

- Inactive API account cleanup now runs against the user's existing local database (2026-09-21 follow-up): `http://localhost:3000/system-settings/operations/inactive-users`, executable `bin/new-api-local-b-c.exe`, database `.local-tests/abc-shared.db`. The user explicitly requested existing data rather than the synthetic preview. Port 3101 is stopped; its test database is preserved but not used. Neither US instance was contacted or modified.
- Existing-data switch backup: `.local-tests/inactive-existing-backup-20260921-212935` contains the SQLite backup, old executable, startup script, prior stdout, and a checksum-only business-data manifest. All original columns in users/channels/tokens/options were verified unchanged after migration. Original login credentials remain in use; a restart may require signing in again.
- A retained local sample account `inactive_sample_90d` was added to `.local-tests/abc-shared.db` on 2026-09-22 after the user reported that the earlier deletion-test fixture was no longer visible. It is an enabled ordinary user with activity observation starting about 120 days ago and its last API call about 90 days ago, so it appears with the default 30-day threshold. Backup: `.local-tests/inactive-sample-backup-20260922-093138.db` (SHA-256 `3AEA2DCBD522FC8081BF8E8F9E0F41C93E2026F06952DF0FF12D2EA8B9CA4056`).
- Local `codexadmin` login recovery on 2026-09-22: the account had exactly 50 active `user_sessions`, matching `USER_SESSION_ACTIVE_LIMIT=50`, so new login was rejected. After a consistent backup to `.local-tests/codexadmin-session-backup-20260922-094436.db`, all 50 active sessions for that user were marked revoked with reason `local_admin_recovery`. Password, role, status, and auth version were unchanged; other users were untouched. Redis is disabled for this local instance.
- Inactive cleanup apply-button behavior was tightened on 2026-09-22: the button is disabled when the displayed threshold equals the saved threshold, enabled only for a valid unsaved change, and disabled again after a successful save or when the user restores the saved value. The updated local executable is running on port 3000 with the existing database. Previous executable backup: `.local-tests/apply-button-backup-20260922-0958/new-api-local-b-c.exe`.
- Profitability planning calculator is integrated into the existing admin Dashboard at `/dashboard/profitability`. It is frontend-only, uses monthly scenario inputs, does not persist data, and is served by the same port-3000 executable. Backup before the restart: `.local-tests/profitability-before-20260922-1445/new-api-local-b-c.exe`.
- 2026-09-25 profitability dashboard implementation now records RMB account purchase costs by New API channel, reads Sub2API admin usage automatically, and correlates usage through `client:<request_id>` from `X-Client-Request-ID`. It has call-charge and cash-received views; unmatched New API/Sub2API usage keeps profit unknown. The current local startup script still does not set `SUB2API_BASE_URL` or `SUB2API_ADMIN_API_KEY`, so live Sub synchronization is pending configuration.
- 2026-09-25 follow-up: profitability build is now deployed at the original `http://localhost:3000/dashboard/profitability` with the original SQLite database; backup is `.local-tests/profitability-deploy-backup-20260925-140442/`. A prior isolated preview on port 3102 was intentionally stopped. The deployment process briefly hit a `Stop-Process` object error and a PowerShell startup `OutOfMemoryException`; both occurred before binary replacement. The verified old PID was then stopped using .NET `Process.Kill`, and the new PID 45052 passed HTTP and database checks.
- 2026-09-30 US profitability: channel multi-selection with per-user browser persistence is deployed, together with the two missing PostgreSQL columns and a root-only Sub2API environment file. `NODE_TYPE=slave` remains unchanged; only `new-api` restarted. Original backup: `/opt/us-whitelist/backups/profitability-20260930-161100/`; second pre-update backup: `/opt/us-whitelist/backups/profitability-20260930-162748/`. Current executable SHA-256: `3b37cfbd261c7c8fa157f4d35f6d50cd2d7bfd0f5f2f0f41b305cce129c5453f`, version `v1.0.0-rc.28-custom-profitability-20260930.2`.
- Live Sub2API usage must be paged using `sort_by=id&sort_order=asc`: descending offset pages overlap while new calls arrive. This fix is deployed and real authenticated reconciliation now reports connected. A temporary build failure in concurrent video work resolved when that work supplied its missing function; no video files were changed by the profitability task. At final verification, channels 4/32/34 matched 4,087 calls, but 26,840 old calls remained unmatched and procurement was absent, so profit correctly remained unknown. Signed-in production UI and real procurement acceptance remain pending. See `docs/progress/2026-09-30.md` and `docs/debug/2026-09-30-debug.md`.

- 2026-09-30 local profitability follow-up: the user chose whole-pool totals and administrator-inclusive/customer-only comparisons, not model purchase-cost allocation. The new `pool` API separates gross call charges from paid customer income and procurement profit. Unknown role attribution leaves customer Sub usage null; raw total ratios use `unverified_scope`, known excluded-channel usage uses `scope_mismatch`, and cash-global mode uses `cash_scope` with no pool ratio. This follow-up is local and is not the deployed `20260930.2` release. Frontend integration, focused tests, builds, and synthetic desktop/mobile browser checks passed; user and real-data acceptance remain pending. Preview: `http://127.0.0.1:3106/dashboard/profitability` (synthetic, procurement writes disabled). See today's profitability progress section. Historical admin classification uses currently stored roles, including soft-deleted users, not a role-at-call snapshot.
- Historical, superseded by the October 1 per-group table: cost-basis group
  break-even was added to this local ledger on 2026-09-30:
  full recorded purchase cost / selected call-scope Sub usage, under the 1:1
  numeric convention. Customer-only mode keeps all procurement costs and
  excludes administrator upstream usage. The metric is not a configured
  per-model/group pricing recommendation or a paid-income profit assertion;
  gift/admin charges remain excluded from operating income. Incomplete scope
  or procurement, zero usage, and cash mode produce an unknown metric.
  Local tests/build and desktop/mobile fixture checks passed; not deployed.

- Auto 已实现并于 2026-09-21 部署到美国服务器；自动检查、本地运行和线上健康检查通过，尚未使用真实上游完成端到端计费验收。
- Redis 重启后限流持续返回 `NOSCRIPT` 的根因已修复并于 2026-09-21 部署；限流脚本现在通过 go-redis `Script.Run` 在 `EVALSHA` 缓存丢失时自动回退到 `EVAL`，无需重启 New API。
- 当前本地实例运行于 `http://localhost:3000`，绑定原有 `.local-tests/abc-shared.db`；`3100` 隔离假数据实例已停止。
- 管理员在“额外设置 → Auto 默认设置”维护站点默认模型；模型广场只保留每个用户自己的 Auto 配置，不新增全模型分组或显示开关。
- 当前旧库没有有效的 `AutoGroups` 配置时，Auto 会回退到用户可用的全部真实分组，并按分组倍率从低到高排序。

## Decisions

### Local Video Studio (2026-09-30)

- Video Studio is implemented in this existing local checkout and the sibling
  `infinite-canvas` source pinned to v0.19.0 / `dab19adc0847e32e39b7fc8ff90cb392561fb826`.
  No SSH or production change was part of this work.
- Entry: `http://localhost:3000/video-studio`; launcher:
  `start-local-video-studio.ps1`. It uses `G:\new-api-video-runtime\preview.db`,
  a one-time SQLite snapshot of `.local-tests\abc-shared.db`, and binds only
  `127.0.0.1`. Users/channels/tokens/options matched the source after startup.
- The user requested a separate page: `/video-studio` bypasses the dashboard
  shell while retaining the authenticated route. The original Canvas
  navigation contains return/account actions; no host header is rendered.
  Canvas remains a same-origin iframe.
- The preview is a slave with background task updates disabled to prevent
  automatic use of copied provider credentials. Paid generation acceptance
  requires a separately controlled master environment; do not interpret
  the running preview as an end-to-end provider/billing sign-off.
- Dashboard identity is handed to Canvas as an in-memory purpose-scoped
  token. The backend revalidates its login session and reuses the user's
  default group, existing relay billing, task ownership, and downloads.
- Canvas projects/media/history are browser-local and per-user namespaced,
  not cloud-synced. Custom provider scripts/proxies/plugins and the local
  CLI Agent are disabled in the managed build.
- Build/runtime/cache paths, exact commands, source provenance, limitations,
  and rollback are recorded in `ops/video-studio/README.md`.
- Focused Go tests, 9 New API UI tests, 3 Canvas tests, both typechecks,
  frontend builds, and the Go executable build passed. Browser connection
  recovered; desktop login/model/canvas/reload checks and standalone-page
  navigation passed. The 390x844 post-fix mobile check passed, including
  parameter/navigation drawers and absence of horizontal form overflow.
- Standalone preview executable backup:
  `G:\new-api-video-runtime\new-api-video-before-standalone-20260930-172127.exe`.
  The preview was restarted using the same isolated database; real-provider
  generation, settlement, and user acceptance remain pending.

- Keep existing Open WebUI status, active-user display, calendar, automation, exploration and other features unless explicitly requested otherwise. Integration priorities are New API username consistency, unified logout/session handling, and user-scoped groups/models/billing. Do not treat the online-count UI addition as completion of those integrations.
- User approved durable API tracking but requested local implementation before any server changes. New `last_api_call_at` and `api_activity_started_at` user columns record authenticated API requests independently of consumption logs. Migration starts a fresh observation window for historical users; unknown history is not proof of inactivity.
- Cleanup endpoints are root-only and exclude every administrator account. Deletion rechecks eligibility under the same user lock as activity recording, then reuses existing authentication revocation and hard-deletion cleanup. Profile edits cannot overwrite either tracking timestamp.
- Before future production rollout, both shared-database new-api instances must record API activity and a complete observation period must be established. Local implementation does not authorize deployment or production account deletion.
- Full controller tests currently include a separate failing payment assertion: `TestRequestAmountRejectsTopUpThatCannotBeSettled` expects the quota-ceiling message but receives the existing 1-to-1000 amount-limit message. It reproduces alone; this cleanup task did not change payment code or that test.
- Existing-database feature test completed on 2026-09-21: temporary ordinary user `codex_inactive_test_0921` was listed and deleted through the real local UI, then removed from the database; temporary root `codex_cleanup_admin` used for the test was hard-deleted through the existing cleanup path. On 2026-09-22, retained sample `inactive_sample_90d` was added alongside the original `codexadmin` for visible local testing. No production or remote account was touched.

- 不新增 `all` 或“全模型”API 分组；模型广场已有的“全部分组”只是筛选项。
- `Auto` 是特殊模型，复用既有跨分组 `group=auto` 机制；实际计费使用最终命中的真实分组、渠道倍率和模型价格。
- Auto 策略由用户选择：“低倍率优先”或“模型顺序优先”，策略解释在选择器下方常显，悬浮说明仍保留。
- 用户自定义优先；未自定义时继承管理员站点默认。
- 不新增虚拟“全模型”API 分组；模型广场的“全部分组”仍是原有筛选项，Auto 复用已有 `auto` token group。
- 目前 Auto 仅支持 OpenAI Chat Completions 与 Responses JSON 请求。

## Commands

- After changing `ops/image-studio/Caddyfile`, validate and load it with
  `docker compose --env-file ops/image-studio/.env -f ops/image-studio/docker-compose.yml exec -T gateway caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile`
  and then the same command with `reload` instead of `validate`.
  Compare the live admin `/config/` with `caddy adapt` output afterward.
  An Open WebUI-only rebuild and anonymous 401 checks do not verify that
  new authenticated account routes have become active.
- Frontend typecheck on this Windows machine: `$env:GOMAXPROCS='1'; .\node_modules\.bin\tsgo.cmd -b`. Higher tsgo parallelism can exhaust available memory in this large worktree.
- Image payload test: `.\node_modules\.bin\vitest.cmd run src/features/playground/lib/__tests__/image-generation-payload.test.ts --pool=threads --maxWorkers=1 --reporter=verbose`
- 后端定向测试：`go test ./service ./controller ./middleware -run 'AutoModel|RequestAutoGroups|RewriteJSONRequestModel|GetChannelSkipsUnavailableAutoModelCandidate|ApplyAutoModelCandidate' -count=1`
- 前端测试：`$env:CI='true'; .\node_modules\.bin\vitest.cmd run src/features/pricing/components/__tests__/auto-model-configurator.test.tsx src/features/system-settings/extra/__tests__/auto-model.test.tsx --pool=threads --maxWorkers=1 --reporter=verbose`
- 前端构建：`npm run build`
- relaykit：`$env:GOWORK='off'; go build ./...`
- 根构建：`go build ./...`

## Known Validation Gaps

- 真实上游失败切换、流式响应已经开始后的不可重试边界、真实账单日志仍需人工灰度验证。
- 当前旧库的 Auto 详情已在独立登录态浏览器标签页中确认显示 `2 / 13`；用户现有标签页若仍显示旧结果，需要刷新并重新打开详情以清除旧 React Query 响应。
- 窄屏视觉与真实上游请求仍需人工灰度验证；自动交互、typecheck、受影响文件 lint、前端生产构建、定向 Go 测试和根 Go 构建均已通过。

## US Deployment

- Windows 自动备份任务为 `US server application backup pull`，每 6 小时运行 `%LOCALAPPDATA%\NewApiBackup\new-api-backup-pull.ps1`，使用 `C:\Users\liurunsen\.ssh\us-server-ed25519` 连接 `156.238.252.115`；本地归档目录为 `D:\Server Backups\us-whitelist`，保留 30 天。运行目录与归档目录 ACL 仅允许当前用户、SYSTEM 和管理员。
- 备份归档包含 `newapi`/`sub2api` PostgreSQL custom dump、两套 Redis RDB、`/opt/us-whitelist/docker-compose.yml`、env 和 Sub2API 配置/模型定价文件；PostgreSQL 经 `pg_restore --list`、Redis 经 `redis-check-rdb`、下载经 SHA-256 校验。传输使用 `.partial` 原子落盘并保留 `.sha256`，成功或失败均清理远端临时文件。
- 2026-09-21 已修复 `newapi.users_id_seq` 与 `newapi.channels_id_seq` 的恢复后漂移，并轮换曾进入 PostgreSQL 错误日志的 New API 渠道 32 / Sub2API API Key 1 共用密钥。不得在日志或项目文档记录密钥明文。
- `old-new-api` 是保留给内部人员的备用实例，继续监听 `127.0.0.1:3000`；它与新 New API 共用 PostgreSQL、Redis 和 Session Secret。
- `old-new-api` 与 New API Redis 已配置 Docker healthcheck。Clewdr 当前镜像没有 shell、curl、wget 或原生健康子命令；不要添加必然失败的容器内探针，需等镜像原生支持或使用定制镜像。

- `api.yisama.asia` 的 `new-api` 容器通过宿主机挂载的单文件运行，当前 `/opt/proxy/newapi/releases/20260921-1318-auto-e468b739/new-api` 映射为容器入口 `/new-api`；部署单文件修复时无需重建 Docker 镜像。
- 2026-09-21 已部署包含 Auto、Turnstile 修复和当前本地其他功能的完整工作树版本；当前产物 SHA-256 为 `7e706da04f204d82bf2415a165bbbdd418fad793ffbbf4b529d42a1138045a0a`，版本为 `v1.0.0-rc.28-custom-e468b739-auto-20260921`。
- 当前 Compose 项目为 `us-whitelist`，配置位于 `/opt/us-whitelist/docker-compose.yml`；更新时只重建 `new-api`，不要触碰 `old-new-api`、PostgreSQL 或 Redis。`old-new-api` 仍挂载 `/opt/proxy/newapi/releases/20260915-1434-e7c508e3/new-api`。
- 本次回滚与数据库备份位于 `/opt/us-whitelist/backups/new-api-auto-20260921-1318-e468b739`；恢复该目录中的 `docker-compose.yml` 后执行 `docker compose -f /opt/us-whitelist/docker-compose.yml -p us-whitelist up -d --no-deps --force-recreate new-api` 可回到部署前二进制。
- Turnstile 故障的 durable finding：React 回调变化会触发旧组件 effect 重跑；显式渲染模式必须保存 widget ID，并在卸载或更换站点 key 时调用 `turnstile.remove()`，不能对已有 widget 的容器重复 `render()`。
- 2026-09-21 线上 Turnstile siteverify 的最终配置根因：数据库中的 `TurnstileSecretKey` 被误填成与 `TurnstileSiteKey` 相同的值，Cloudflare 返回 `invalid-input-secret`。正确 Secret 必须从对应 Turnstile widget 的 Cloudflare 控制台获取，不能从 Site Key 推导。
- 2026-09-21 新注册账号登录 500 的根因是恢复后的 `users_id_seq` 复用了仍有 Redis 永久鉴权版本键的历史用户 ID。生产用户 `13`、`14`、`15` 已将数据库 `auth_version` 对齐到 Redis floor `2`，序列已前推到历史占用上限 `16`，下一注册 ID 为 `17`；全用户版本扫描无不一致。
- PostgreSQL 恢复或迁移时，`users_id_seq` 不能只按当前 `users.id` 最大值修复；必须同时越过保留历史身份的引用（至少 `logs.user_id`）和 Redis `auth:user:version:*` 的最大用户 ID，否则新账号会因安全版本下限冲突而 fail-closed。不得通过降低或删除 Redis floor 来规避该检查。
- 本次账号修复备份位于 `/opt/us-whitelist/backups/new-api-auth-repair-20260921-180331`，包含完整 custom-format PostgreSQL dump、SHA-256、`pg_restore -l` 清单及变更前后状态；修复未重启任何服务。
- Redis `NOSCRIPT` 修复版本为 `v1.0.0-rc.28-custom-e468b739-auto-noscript-20260921`，发布文件为 `/opt/proxy/newapi/releases/20260921-1838-noscript-e468b739/new-api`，SHA-256 为 `7f73d7b516cde46e94a40c715a27dde4519626a21c3016efe9f507b2e1206179`。
- 本次只重建 `new-api`，未触碰 `old-new-api`、PostgreSQL 或 Redis。部署备份位于 `/opt/us-whitelist/backups/new-api-noscript-20260921-1838-e468b739`。
- 回滚时恢复该备份目录中的 `docker-compose.yml`，再执行 `docker compose -f /opt/us-whitelist/docker-compose.yml -p us-whitelist up -d --no-deps --force-recreate new-api`。

## Durable Debug Findings

- Local cleanup audit on 2026-09-30: `.local-tests/channel-status-go-cache`
  and `.local-tests/channel-status-go-tmp` were removed after formal approval,
  clearing approximately 2.06 GiB of old Go build artifacts. Earlier policy
  blocks are historical. Do not delete all of `.local-tests`: its databases,
  session secrets, backups, and fixtures remain and were verified unchanged.
- The parent workspace contains dirty test-named checkouts and linked Git
  worktrees. Names such as `test`, `reference`, or `staging` are not evidence
  that a whole directory is disposable. See the September 28 and 30 progress
  entries for the cleanup audit and preservation decisions.

- 2026-09-23 US first-token investigation: host resources and the Nginx/Cloudflare ingress path were fast and healthy. The deployed `new-api` executable is a read-only host bind mount under `/opt/proxy/newapi/releases/20260922-1655-local3000-73BC11F5/new-api`, not the image's internal executable.
- `other.frt` is recorded when the upstream SSE scanner sees its first non-empty `data:` event, before downstream response conversion. In the checked 24-hour window, valid FRT samples had a 10.8 s median, 39.5 s P90, and 56.7 s P95; large prompts and `xhigh` reasoning correlated with higher latency.
- Channel 32 had a separate burst of 502 `Upstream service temporarily unavailable` failures on 2026-09-23, typically after about 5.2-5.6 s. Do not treat those failures as an Nginx or host-memory issue without checking the channel's upstream/proxy path.
