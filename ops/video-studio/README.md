# Local Video Studio

This integration adds `/video-studio` as a standalone, authenticated page.
It fills the viewport without the New API dashboard header/sidebar or an
outer account bar. The original Canvas navigation offers an account menu
with return to `/dashboard`. The managed Infinite Canvas build is still
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
   and the login session's expiry. Reload to establish a fresh studio session.
3. Every bridge request revalidates the login session and reconstructs the
   current user's relay context. Requests use the user's default group.
   Users do not supply provider keys or select arbitrary channels.
4. Billable operations reuse existing New API distribution, pricing,
   pre-consumption, settlement/refund, and logs. The synthetic token ID is `0`;
   it does not require a user's persistent API token.
5. Video submission uses `/api/video-studio/v1/videos`. Polling and downloading
   use owner-checked task endpoints. Image references use `input_reference`.
   Video/audio references and custom provider scripts are not enabled in
   managed mode. Compatibility is limited to the existing New API adapters;
   the model catalog is not a promise that every upstream accepts every option.
6. Canvas projects, assets, prompts, and local generation history are stored
   in per-user browser namespaces. This is local persistence, not cloud sync
   or encryption against someone with access to the browser profile.
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
go test -p 1 ./service ./controller ./router -run '^(TestVideoStudio|TestCreateVideoStudioSession)' -count=1
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
  `.\ops\video-studio\start-verification.ps1`. It uses port 3117 and a separate
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
- Eighteen New API frontend tests cover the dev proxy, native account
  bridge, trusted events, loading/errors, stale-account responses, navigation,
  session handoff, account switching and shell bypass. Six Canvas tests
  cover trusted account/session messages, per-user storage and refresh events.
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
