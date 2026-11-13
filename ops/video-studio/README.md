# Local Video Studio

## Studio Static Entry Repair (2026-11-10)

US main and standby now run `studio-startup-20261110`, SHA256
`a2ac43020dfa0f8768a449890976ec78fd438bc4d3e666a7c97441f78c23de22`,
layered on the exactly reproduced `body-limit-20261109` binary. This preserves
the existing P1 and body-limit fixes. HTML entries use `no-store` and ignore
old conditional validators because Nginx rewrites their scripts independently
of the source HTML modification time. Missing `/assets/` and `/plugins/`
resources return 404, while valid assets retain 304 validation and SPA routes
remain available. The standby now mounts the main instance's existing read-only
Canvas directory; its old symlink resolved to an empty directory.

Optional Canvas diagnostics are loaded asynchronously. A browser regression
using the deployed bundle proves the editor can start while that request is
held open. The other existing frontend patches were preserved.

After reproducing the same controller failures against production's baseline,
the P1 refund test was corrected to wait for the existing asynchronous refund,
without changing production accounting. Baseline and candidate regressions
passed; the candidate passed 59 focused top-level tests. Live main/standby and
TLS-origin checks confirmed HTML refresh, script MIME types, missing-resource
404s, existing-asset 304s, health, exact running hashes, restored ingress, and
the removal of this rollout's execution pause. All 27 unrelated container IDs
were unchanged. No migration, historical resubmission, or accounting adjustment
was made; 45 uncertain image jobs remain unchanged.

The user confirms their production page opens and looks normal, with possible
mild sluggishness. The agent's authenticated Edge session still shows the
loading overlay without an observed child-frame/session handshake; its cause
is unknown and must not be generalized to a production outage.

A read-only follow-up matched 52 Canvas files and the requirements module to
the live release. The isolated, single-text-node browser fixture passed all 16
pan, wheel, cloud-content, selection, project-switch and saved-view checks at
normal and 8x CPU throttling, with no page errors, unexpected requests or
viewport anomalies. Its first throttled run requested cloud sync before local
view persistence and hit the existing pending-edit guard; the fixture now waits
for persistence, without changing that production safeguard. These checks do
not establish large-project or network performance. Desktop screenshots were
inspected; 390px screenshots show header overlap and clipped toolbars with the
sidebar open, so small-screen layout remains unverified for usability.

Post-release logs contain successful workspace reads/writes, asset uploads and
job API requests, with no selected 4xx/5xx, panics, missing-schema or refunding
errors. Five fresh Canvas sessions had no viewport anomalies; one 88.6ms frame
gap occurred near resize events, and one frontend error has no message/stack
available for attribution. Exclude pre-release tabs from this sample. No
additional production change, restart or paid generation was made.

Operational scripts and verification are under
`/opt/us-whitelist/ops/studio-startup-20261110/`. Backend rollback is
`python3 -B /opt/us-whitelist/ops/studio-startup-20261110/deploy-newapi.py rollback`;
the deployment state and exact Compose backups are in the matching release
directory. The async-only ingress backup is
`/etc/nginx/conf.d/api-yisama.conf.before-async-diagnostics-20261110`.

## MJ Prompt Length Limit (2026-11-03)

Live probes with the production keys: uuyue accepts 850 characters and rejects 900
(HTTP 400 `media_error`, immediately); osodm accepts 750 and rejects 850 (HTTP 400
`invalid_request` after one to two minutes). The failing canvas prompts were 1791
characters. Studio MJ jobs on both channels (Ember and OC) are limited to 750
characters (`emberMJPromptLimit`, shared by `ocMJPromptLimit`), checked before any
quota is reserved, with a job error naming the limit. The canvas and the model plaza
must show this rule whenever an MJ model is used. osodm's
`/v1/images/midjourney/generations` blocks about 220 s and returns four images.

## Uuyue Protocol Recheck (2026-11-03 15:46)

The supplied screenshots compare repeated GET queries for the old
`d8621016-40ad-5218-9102-036cc7ac9c5c` task with synchronous image submissions.
The current public Ember documentation still specifies POST
`/v1/images/generations`, `X-Ember-Image-Protocol: mj-proxy`, a UUID
`Idempotency-Key`, then GET `/v1/mj/tasks/{uuid}` using the same key.
The saved task key matches channel 63's current key: the old user task returns
404, while yesterday's completed probe still returns 200 and four images.
The original user task's upstream submission receipt was not retained, so the
historical rejection or loss of its upstream record remains unproven.

One new independent probe at 15:38:49 returned HTTP 202 in 0.65 seconds with
the original UUID `cbd345e8-cb82-41dc-9e3e-67745686bee4`. Polling reached
`completed` with four images at 15:45:53; all four authenticated PNG downloads
passed (1455018, 1341853, 1727727 and 1319421 bytes). Receipt and raw submission
response are under `/opt/us-whitelist/runtime/mj-uuyue-format-20261103-receipt*`.
No old job was resubmitted and channel 63 remains manually disabled.

This success does not validate OC. User 1's separate 14:39:41 OC v6 job
`studio_1811745b0f9c41c7a9fc371db2365b81` returned upstream HTTP 500 after
1800 seconds, with no task ID; its private response says `server_error`.
It remains uncertain with 160000 quota and no observed refund. No code,
billing records, channel settings or running services were changed here.

## Wallet Schema Incident (2026-11-03)

The 14:27 `settled-balance-20261103-r2` release made `/api/user/self`
read `wallet_pre_consume_records` before its migration was applied. Both main
and standby run with `NODE_TYPE=slave`, so `InitDB` skips automatic migrations.
This caused SQLSTATE 42P01 when opening the studio, independently of MJ.

During this investigation the parallel wallet task completed the existing
migration by 14:32:57 Asia/Shanghai. All eight model columns and four indexes
(including the primary key) were checked. Authenticated `/api/user/self` and
`/api/user/self/balance` returned HTTP 200 and `success: true` on both services;
no subsequent wallet missing-table errors were found. This investigation did
not alter billing data, resubmit jobs, replace binaries or restart services.
Future slave-node releases requiring schema changes must apply and verify the
matching migration before switching traffic; `/api/status` alone is not a
sufficient schema-readiness check.

## OC MJ Async Integration (2026-11-03)

OC's current model documentation provides `POST /v1/images/midjourney/generations`
and `GET /v1/images/midjourney/tasks/{task_id}`. The studio now selects this
protocol only for the OC channel (or `X-OC-Image-Protocol: mj-async` override),
separately from Uuyue's Ember protocol. It saves the assigned upstream task ID,
key and route, polls the same task, and stores all four `SUCCESS` images from
`result.data` behind owner-checked studio asset endpoints. `n=1` reserves four
images; failure refunds once, and a missing receipt becomes uncertain without
querying a fabricated ID or replaying POST. The existing task timeout is unchanged.

Focused service/controller/router, billing/refund and Fast regressions passed
on the complete production source snapshot. Browser checks against live frontend
modules with mocked APIs verified four-image return/storage and passing only
the selected third image's original bytes as a video's `input_reference`.
The dirty D: worktree has concurrent wallet work; its unrelated
`service/wallet_balance_test.go` initially failed to compile with
`undefined: dto.UserSetting`, so none of that work was included in this release.

An independent real OC async submission timed out after 75 seconds without a
task receipt. Its private receipt is
`/opt/us-whitelist/runtime/mj-oc-async-20261103-receipt.json`; it was not replayed.
Real upstream image completion remains unverified. Neither of user 12's
previous uncertain/pending tasks was resubmitted.

Deployed at 14:03 Asia/Shanghai as `mj-oc-async-20261103`, binary SHA256
`6d998d706abaf43c79530f08c839a454a8b88b3389f34358ee48e61db96a368f`.
Only the `new-api` container's binary mount changed. Health and local status API
passed; all 237 embedded frontend files, nginx, Fast behavior and channel states
were preserved. Rollback configuration is stored in
`/opt/us-whitelist/releases/mj-oc-async-20261103/rollback-compose.json`.
Reconstruct source from the base/group-name archive, MJ overlay, Fast overlay,
then this release's `oc-source-overlay.tar.gz`; do not build from the incomplete
D: checkout or publish the obsolete `release-v4/new-api`.

## MJ Failure Investigation (2026-11-03)

User 12 (`soulmate`) submitted four `Midjourney-v8.2` jobs at 09:08:46-09:09:20
Asia/Shanghai. All failed at submission, with four matching wallet refunds and
zero remaining task quota. Their original upstream HTTP statuses/bodies were
not retained, so the specific original rejection cannot be reconstructed.

Direct checks found a valid key, USD 9.62 remaining, and the model still listed.
Independent minimal requests with UUID v4 and v5, and the original prompt with
UUID v4, all returned HTTP 503. Queries for these probe UUIDs returned 404;
upstream usage reported zero requests/cost for November 3. Neither UUID version
nor prompt flags is an established cause. Do not deploy the speculative UUID
change or automatically resubmit any affected user job.

The local adapter now includes HTTP status in definite rejection reasons and
logs non-success submission status with the job and upstream UUID, without
logging credentials or prompts. It also keeps at most 4096 bytes of the upstream
error body and its HTTP status in task private data, never in task API responses.
Focused MJ/studio tests cover bounded private diagnostics, 503 recovery, no
repeat POST, and one-time refunds. This diagnostic change was not deployed
during this investigation. Current production is
`fast-sub2api-20261103`; preserve that overlay in any later release.

Follow-up at 12:09 Asia/Shanghai: the same key now lists only Claude models;
the provider's public model plaza also contains no MJ model. Channel 63 is
already manually disabled; this investigation did not change its status.
The actual Go request function, using both HTTP/2 and HTTP/1.1, returns the same
results as Python: yesterday's completed UUID is readable, while the user's
11:30 task `d8621016-40ad-5218-9102-036cc7ac9c5c` returns 404 and still reserves
320000 quota. These read-only checks made no generation request. Current model
availability does not establish the exact cause of the four earlier rejections.

The same user's 11:54 OC job `studio_abecf10bf5f948628e47eea6dc8c8eb2`
also became uncertain after about 600 seconds with `unexpected end of JSON
input`; logs confirm its USD 0.08 reservation was refunded. OC's model list is
reachable, but this real generation did not succeed, so do not call OC a
currently verified working fallback. Neither uncertain job was resubmitted.

## MJ Channel Adapter (2026-11-02)

Managed image jobs use the Ember asynchronous protocol only for MJ models on
`api.uuyue.top`, or a selected channel whose administrator sets
`X-Ember-Image-Protocol: mj-proxy` in its header overrides. OC and other channels
keep the synchronous image relay.

The adapter submits once with a persisted UUID and `n=1`, queries the original
`/v1/mj/tasks/{uuid}`, and stores four authenticated image downloads as normal
owner-scoped studio assets. Ambiguous submissions and `uncertain` query states
never trigger another POST. One Data URI reference is converted to multipart
`image[]`; resolution and ratio are separate `size` and `aspect_ratio` fields.
Existing task billing/refund, worker scheduling and timeout policy are reused.

Focused controller/service tests cover four-image recovery, a 502 submit,
temporarily missing/uncertain tasks, mismatched UUIDs, saved channel credentials,
one-time charging/refunds, reference conversion and redirect protection.
Browser tests with mock jobs cover Uuyue/OC selection and four returned assets.
The paid upstream probe submitted once, returned HTTP 202, reached `completed`,
and downloaded all four images using authenticated task-image endpoints.
Real reference editing and real provider failure/refund remain untested.

The deployed release `/opt/us-whitelist/releases/mj-uuyue-20261102` uses production source commit
`e0b8a9c77d7c91e6c2706499016880ae4c0c9d62` plus the scoped MJ overlay, preserving
all 237 embedded web files. Do not deploy a binary built directly from this
older local worktree over the production snapshot.
Only `new-api` was recreated; the accrual service was unchanged. The saved
`rollback-compose.json` and `nginx-before.conf` are inside that release directory.
The public Canvas entry uses `/video-canvas/mj-uuyue-20261102/`. Post-deploy
browser verification used real static resources and mocked job/catalog responses;
the public status endpoint returned HTTP 200 without mocking. Both services
were healthy, the main service had no startup panic, and available RAM was
approximately 4.2 GiB after deployment. The whole older-worktree test suite was
not green; non-MJ model-list/top-up assertions also fail without running MJ tests.

This integration adds `/video-studio` as a standalone, authenticated page.
It fills the viewport without the New API dashboard header/sidebar or an
outer account bar. The original Canvas navigation has a leftmost return to
`/dashboard`, an account menu and a rightmost rounded rectangular balance.
The managed Infinite Canvas build is still
served in a same-origin iframe from `/video-canvas/`, with the existing login
and billing bridge. It does not deploy anything to the US server.

## Sources

- New API source: this existing, dirty worktree. Preserve the unrelated image,
  profitability, Tavern, and other local changes.
- Canvas source: sibling `../infinite-canvas`, from
  `basketikun/infinite-canvas` v0.19.0, commit
  `dab19adc0847e32e39b7fc8ff90cb392561fb826`.
- The original GitHub archive is retained under the workspace
  `.codex-runtime` directory. The extracted Canvas directory has no Git metadata.
- Preserve both projects' license files and attribution. Canvas's MIT license
  does not replace New API's license.

## Request And Storage Contract

1. A logged-in dashboard session calls `POST /api/video-studio/session`.
2. The parent hands a purpose-scoped bearer to the same-origin iframe through
   an origin/source-checked message. It is held in memory, not stored as an
   API key. Its lifetime is capped by the existing 12-hour studio token policy
   and the login session's expiry. The frame requests renewal from its trusted
   parent without reloading the canvas. Paid submissions are never replayed
   just because an authentication response or network query failed.
3. Every bridge request revalidates the login session and reconstructs the
   current user's relay context. Submissions carry the selected, URL-encoded
   group in `X-New-Api-Group`; the server checks current user access and strips
   that header before upstream forwarding. Omitted groups use the user default.
   Users do not supply provider keys or select arbitrary channels.
4. Billable operations reuse existing New API distribution, pricing,
   pre-consumption, settlement/refund, and logs. The synthetic token ID is `0`;
   it does not require a user's persistent API token.
5. Managed image/video submission uses `/api/video-studio/jobs/...` with a
   user-scoped UUID `Idempotency-Key`. The server persists the request before
   returning a receipt, then runs the existing billable relay independently of
   the browser. Video jobs retain the existing New API native task ID.
   Polling and downloading use owner-checked job endpoints. Image references use `input_reference`.
   Video/audio references and custom provider scripts are not enabled in
   managed mode. Compatibility is limited to the existing New API adapters;
   the model catalog is not a promise that every upstream accepts every option.
6. Submitted generation jobs and their returned media are stored by New API:
   `studio_jobs` contains ownership/status, and `VIDEO_STUDIO_DATA_DIR`
   contains private request/result files. The default directory is
   `data/video-studio` under the process working directory. Keep it outside
   the static tree and back it up together with the database.
   Canvas projects, imported assets, prompts and composition remain browser-local
   and per-user; this change does not synchronize entire canvases.
7. The local CLI Agent, provider configuration, proxy configuration, and
   custom plugins are hidden/disabled in managed mode.
8. A nonvisual parent bridge reads `/api/user/self`, applies the site's
   existing currency formatting and updates its shared user cache. Only the
   active same-origin frame can request refresh, dashboard return or logout
   confirmation. Canvas cannot set amounts. Refreshes occur after generation,
   terminal video polling and window focus; pending/error amounts are marked.
9. The native account menu links to the existing wallet, profile, usage/task
   logs and pricing in new tabs. Development must proxy `/video-canvas` to
   the same backend as `/api`; otherwise Rsbuild serves the wrong app.
10. Canvas static responses use `Cache-Control: no-cache` so normal reloads
    revalidate the entry and mutable config. The host uses
    `/video-canvas/video?v=2` to retire the original entry URL previously
    served without a cache policy; this is not the upstream app version.
11. `GET /api/video-studio/catalog` exposes permitted groups and enabled models
    with their capabilities. Each page filters this catalog for its purpose:
    video omits LLM-only groups and non-video models; image/text/audio retain
    their own options. The same model in two groups remains two distinct choices.
    Settings and canvas nodes retain group/model identity, not an upstream key.
12. Video POSTs require the selected group's model to have video capability,
    before billing. Task status/content GETs use the task owner and stored
    routing/billing context, not the current catalog or current group selection.
13. The existing master system-task runner executes `video_studio_jobs` under
    its existing DB lease. A process interrupted during an ambiguous upstream
    submission becomes `uncertain`, never automatically resubmitted. Already
    returned image responses and completed native video tasks can resume media
    storage without a new charge. Normal failures/refunds still use New API.
    Multi-node deployments must share this private data directory. Native video
    completion/settlement also requires the existing `UPDATE_TASK` polling.
    No new job retention or cleanup policy is enabled.

## Administrator Configuration

Use existing New API usable-group, channel and model-metadata settings.
The catalog combines supported channel endpoints with model metadata, matching
New API pricing semantics and exact/prefix/suffix/contains name rules.
Video requires the `openai-video` endpoint capability, supplied by a supported
video channel/custom-channel configuration or model metadata. A name containing
`video`, or a model that can only understand video, is not enough.
Only enabled abilities/channels with available keys are offered; disabled models,
disabled channels and groups outside the user's permission are omitted.

The managed frontend loads this catalog when the page opens; reload after an
administrator changes configuration. An empty video catalog disables its selectors
and generation button rather than falling back to an LLM.

The existing model editor stores an optional `parameters` JSON profile with
`image`, `video`, and `groups` overrides. Only explicitly configured options
are offered in managed generation controls; unknown options are left to the
upstream default, not guessed from the model name. These profiles are operator
configuration, not automatic channel-capability discovery. Where multiple
channels route the same group/model, configure their genuinely shared options.
Requests are validated again on the server before execution.

## Build

Dependencies and build caches are on G: because D: has little free space.
Canvas `web/node_modules` is a junction to
`G:\infinite-canvas-video-runtime\node_modules`. Its npm lockfile is retained.
The unused upstream Pro Components wrapper was removed because it requires
Ant Design 5 while this checkout uses Ant Design 6.

From the sibling Canvas `web` directory:

```powershell
$env:VITE_BASE='/video-canvas/'
$env:VITE_NEW_API_STUDIO='1'
.\node_modules\.bin\tsc.cmd -b
.\node_modules\.bin\vite.cmd build --outDir G:\infinite-canvas-video-runtime\dist --emptyOutDir
node --experimental-strip-types --test src/lib/__tests__/new-api-studio.test.mjs
node --experimental-strip-types --test src/lib/__tests__/studio-account-refresh.test.mjs src/lib/__tests__/studio-catalog.test.mjs src/lib/__tests__/studio-selection.test.mjs
```

From New API `web`, build the dashboard before compiling the Go binary,
because the dashboard assets are embedded:

```powershell
$env:GOMAXPROCS='1'
.\node_modules\.bin\tsgo.cmd -b
.\node_modules\.bin\rsbuild.cmd build
.\node_modules\.bin\vitest.cmd run src/features/video-studio/__tests__/session.test.tsx src/features/video-studio/__tests__/navigation.test.tsx src/components/layout/components/__tests__/standalone-video.test.tsx --pool=threads --maxWorkers=1
```

From the New API root:

```powershell
$env:GOCACHE='G:\new-api-video-go-cache'
$env:GOTMPDIR='G:\new-api-video-go-tmp'
$env:GOMAXPROCS='2'
go test -p 1 ./service ./controller ./router -run 'Test(VideoStudio|CreateVideoStudio|GetVideoStudio)' -count=1
go build -p 1 -o G:\new-api-video-runtime\new-api-video.exe .
.\start-local-video-studio.ps1
```

Stop this preview process before replacing its executable. Canvas-only
rebuilds need a browser reload, not a Go rebuild.

## Run And Stop

- Entry: `http://localhost:3000/video-studio`, using existing local account
  credentials. Pass `-Port` if occupied; the script does not stop other services.
- Listener: `127.0.0.1` only, controlled by `BIND_ADDRESS`.
- Database: `G:\new-api-video-runtime\preview.db`, created once with SQLite's
  backup API from `.local-tests\abc-shared.db`. The original is never migrated
  or written by this launcher. A restart reuses the snapshot.
- Default preview runs as `NODE_TYPE=slave` with task updates disabled, so
  copied credentials are not automatically refreshed and scheduled upstream
  work is not started. Do not use this preview for paid generation acceptance:
  asynchronous settlement/polling needs a controlled master test environment.
  Clicking generation can still call a configured real channel.
- Logs and the exact process ID are under `G:\new-api-video-runtime`.
- For an isolated, synthetic account/model catalog, run
  `.\ops\video-studio\start-verification.ps1`. It uses port 3117 by default and a separate
  database under `verification`, and does not submit generation.
  The native-account preview uses
  `-RuntimeRoot G:\new-api-video-runtime\native-preview`, with a separately compiled
  executable. Its synthetic balance is not the user's balance.

To stop only the preview created by this launcher:

```powershell
$runtime='G:\new-api-video-runtime'
$serverId=[int](Get-Content -LiteralPath "$runtime\server-3000.pid")
$server=Get-Process -Id $serverId -ErrorAction Stop
if ($server.Path -ne "$runtime\new-api-video.exe") { throw 'Process identity mismatch' }
Stop-Process -Id $serverId
```

Rollback needs no database restore: stop this preview and use the pre-existing
local launcher and original database. Do not reset the dirty worktree.

## Verification

- Go tests cover scoped credentials, real relay submission through a local
  mock upstream, wallet debit, failed-submission refund, and task/content
  ownership. No paid provider is called.
- Twenty-six New API frontend tests cover the dev proxy, native account
  bridge, trusted events, loading/errors, stale-account responses, navigation,
  session handoff, account switching and shell bypass. Twenty-five Canvas
  checks cover trusted account/session messages, per-user storage, refresh,
  model profiles, renewal and durable job recovery.
- Browser checks passed for login, model selection, creating a canvas/video
  node, and preservation after reload.
- Standalone-page browser checks passed at 1280x720 and 390x844, including
  entering from the dashboard, returning to it, mobile parameters and mobile
  navigation. The fixed mobile form no longer overflows horizontally.
  Desktop/mobile screenshots are under workspace `.codex-runtime` with the
  `video-studio-standalone-` prefix.
- Real video completion/download, actual prices/refunds, cross-device behavior,
  and production deployment remain unverified. No production service or
  real balance was changed.
- Native account integration was browser-checked at 1280x720 and 390x844:
  no outer header, matching video/wallet balance, same-account wallet in a
  new tab, return to dashboard, and confirmed logout to the sign-in page.
  Screenshots use `.codex-runtime/video-studio-native-` at workspace root.
- Group/model filtering passed focused Go tests and 11 Canvas checks, including
  selected-group charges, rejected non-video submissions, encoded group names,
  selection persistence and owner-scoped task access after catalog removal.
  Desktop browser checks confirmed video-only choices, separate image options,
  cross-group persistence, and disabled generation with an empty catalog.
  A real 390x844 browser check now confirms the managed parameter drawer fits
  without horizontal overflow and applies a saved group override.
- The current isolated background-job verification preview is
  `http://localhost:3121/video-studio`, using the synthetic `videotest`
  account and a local mock provider on port 3108. It completed one image job
  after the browser was closed and charged the synthetic account once.
  Port 3000 is currently stopped. No real provider or US-server generation
  was submitted.

## MJ Limit Rollout (2026-11-03 19:40)

Canvas: patched live chunks `image-`, `app-`, `index-C2Z7Mhae-pricing-2cf602609685.js` into
`/var/www/mj-limit-20261103` (copy of `mj-uuyue-20261102`) and pointed both nginx
`sub_filter` entry lines at it; previous conf is
`/opt/us-whitelist/ops/mj-limit-20261103/api-yisama.conf.before`. Plaza: the 11 MJ
`models.description` rows got a 750-character note; originals are in
`models-descriptions.before.csv` in the same directory. The Go limit is source-only until
a binary is rebuilt from the current production overlay chain (not done).
