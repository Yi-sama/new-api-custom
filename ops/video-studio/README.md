# Local Video Studio

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
