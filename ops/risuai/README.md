# Local RisuAI Tavern

This integration uses the upstream RisuAI Svelte application, not an iframe
or a replacement chat UI. Source lives in the sibling `../risuai` checkout,
based on commit `f9728b14c733d073168d8d0c6a8f34f8febf0304`.
Keep its upstream license and attribution.

## Run

Requirements: Node 22.12+ and the dependencies from RisuAI's pnpm lockfile.
The existing local New API on port 3000 must already be running. This launcher
does not start another New API instance.

```powershell
cd '..\risuai'
pnpm install --frozen-lockfile
cd '..\new-api'
.\start-local-tavern.ps1 -Build
```

- New API login / portal: `http://localhost:3000/`
- Native app: `http://localhost:3200/?connect=1`
- Use `localhost` for both New API and RisuAI so the refresh cookie is shared.
  The RisuAI server listens only on IPv4 loopback.
- No MongoDB, Redis, or Docker service is required for this Tavern.
- This private loopback launcher sets `VITE_RISU_LEGAL_CONFIGURED=TRUE`
  using upstream's private self-hosting allowance. The upstream notice is
  retained. Do not reuse this configuration for public service: configure
  your own terms/privacy pages and required upstream-service notices first.
- Stop only the PID printed by the launcher when restarting this instance.
- Persistent data: `.local-tests/risuai-runtime-3200/data`. Keep both
  `.session-secret` and the `users` directory. Never commit either.

On this machine, limited D: space required installing with
`--store-dir C:\tmp\risuai-pnpm-store --virtual-store-dir C:\tmp\risuai-node-modules`.
The RisuAI `node_modules` links depend on that virtual store. It is not a
required project-wide installation layout.

## Account And Billing

New API is the sole account and wallet authority. A same-origin backend
bridge exchanges its HttpOnly refresh cookie for a short-lived session
bearer. The bearer stays in an encrypted HttpOnly cookie, never frontend
configuration. Balance, available groups, and models come from New API.

The original RisuAI text generation pipeline is routed to the fixed
`/pg/chat/completions` endpoint. New API performs authorization, pricing,
deduction, and usage recording. There is no second wallet or shared
provider key. RisuAI storage is partitioned by the validated New API user ID;
requests from stale tabs are rejected after switching accounts.

This is a local integration, not a public multi-tenant deployment.
Independent RisuAI login, native generic proxy routes, and WebSocket proxy
jobs are unavailable in bridge mode. Image generation, TTS, and third-party
plugins are not claimed as New API-billed features.

## Verification

```powershell
cd '..\risuai'
node --test server/node/new-api.test.cjs
pnpm check
pnpm test
```

`fixture.cjs` is an isolated fake New API on `127.0.0.1:3211`. It has
synthetic users 901/902, balances, model lists, and streaming replies.
For browser tests, run a second RisuAI instance on 3210 using
`-BackendUrl http://127.0.0.1:3211 -PortalUrl http://127.0.0.1:3211`.
Start from the fixture's visible account links. Never route the fixture
to a real provider, account, or production database.

Real provider settlement and visual acceptance require separate user
validation; automated tests do not incur charges.

## US HTTP Test Deployment

The existing isolated US New API test instance now hosts the native Tavern
integration using IP-based plain HTTP, as explicitly requested by the user.
No domain, certificate or HTTPS listener was added.

- Sign in to `http://156.238.252.115:32795`, then enter `/tavern`.
- Native Tavern: `http://156.238.252.115:32796/?connect=1`.
- Node listens only on `127.0.0.1:32830`; the dedicated Nginx port proxies it.
- The bridge requires `NEW_API_ALLOW_PUBLIC_HTTP=1`, with matching portal
  and Tavern hostnames. Backend URLs remain loopback-only.
- Frontend build variables are `VITE_RISUAI_URL` for New API and
  `VITE_NEW_API_BRIDGE=1` plus `VITE_NEW_API_PORTAL_URL` for RisuAI.
  The latter is necessary for the signed-out login link.
- Tavern sign-in links preserve `redirect=/tavern`, so main login returns
  to the native application instead of the dashboard.
- Only the authenticated private test deployment uses upstream's private
  self-hosting allowance. The original legal component and terms/privacy
  prompt remain. This is not acceptance of a public customer deployment.
- Existing releases, base Compose, test data and all Canvas overrides are
  retained. No remote dependency install, compilation or image pull.
- HTTP transmits credentials without transport encryption. Use test
  credentials, not a production password.

Build/upload/rollback evidence and the full active Compose command are in
`docs/progress/2026-10-03.md`, section `US RisuAI HTTP Tavern`. Do not use
base-only Compose or omit the latest frontend override.

`verify-public-http.cjs` checks the real signed-out HTTP boundary, then uses
browser-intercepted synthetic identities and storage to inspect deployed
desktop/mobile assets. It blocks generation and sends no authenticated
requests to the real backend. Its passing result is not real account or
paid-settlement acceptance.
