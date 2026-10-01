# Local AgnAI Integration

This replaces the rejected custom Tavern with the actual AgnAI application.
The standalone app retains its native SolidJS UI, characters, PNG/JSON imports,
chat history, generation presets, and memory books. No iframe is used.

## Source

- Upstream: `https://github.com/agnaistic/agnai`
- Pinned base: `fccee00f5f7628d150760c787c4be87e6b1a652c`
- Local checkout: `../agnai`, beside this New API repository.
- Upstream license: AGPL-3.0, retained in `agnai/LICENSE.md`.
- Integration changes live in that checkout's `srv/new-api.ts`,
  `web/new-api.ts`, native request/auth hooks, and `web/shared/NewApiBar.tsx`.
- The New API `/tavern` entry only redirects. It does not implement a chat app.

Both working trees are needed. No upstream changes have been committed or
published. A clean upstream clone alone does not contain this integration.

## Run

Keep the existing New API backend and frontend running on ports 3000 and 3107.
Do not replace their database or restart another feature's preview.

```powershell
docker compose -p new-api-agnai -f ops/agnai/docker-compose.yml up -d
.\start-local-tavern.ps1 -Build
```

Use `http://localhost:3107/tavern` after signing in to New API. The native
AgnAI page is on `http://localhost:3200/character/list`.
Use the same hostname throughout: cookies are shared between local ports,
not between `localhost` and `127.0.0.1`.

On this machine the initial Docker Hub pull timed out. The successful local
image overrides were:

```powershell
$env:AGNAI_MONGO_IMAGE = 'docker.m.daocloud.io/library/mongo:8'
$env:AGNAI_REDIS_IMAGE = 'dockerproxy.net/library/redis:7-alpine'
docker compose -p new-api-agnai -f ops/agnai/docker-compose.yml up -d
```

MongoDB binds only `127.0.0.1:27028`, Redis only `127.0.0.1:6388`.
The named `new-api-agnai_agnai-mongo` volume stores native user data.
Uploads, the generated signing secret, and runtime logs are under
`.local-tests/agnai-runtime-3200`. Do not delete these as disposable test output.
The launcher refuses an occupied app port and never stops existing processes.

## Account and Billing

The bridge rotates the existing `new_api_refresh` cookie through New API,
maps the verified immutable user ID to a native AgnAI account, and keeps the
short-lived New API bearer encrypted in an HttpOnly cookie. It never creates
a shared provider key or another wallet. Native users are not AgnAI admins.
Every protected request validates the live New API account/session. A changed
account cannot reuse the previous native token to read its chats.

The account bar reads `/api/user/self`, `/api/user/self/groups`,
`/api/user/models`, and `/api/status`. Model generation rechecks group/model
authorization and uses the normal `/pg/chat/completions` billing path.
The browser-direct-provider option is disabled in bridge mode.
Native image/TTS providers are not connected in this version and are blocked,
not silently billed elsewhere. Use the existing separate Image/Video Studio.

The original custom Tavern's browser-local data has not been erased or
automatically imported. Native AgnAI now owns all new character/chat data.

## Checks

```powershell
# In the AgnAI checkout
pnpm exec tsc -p srv.tsconfig.json
pnpm exec tsc -p tsconfig.json --noEmit
pnpm exec mocha --require module-alias/register --exit tests/new-api.spec.js

# In New API
node --test ops/agnai/client.test.cjs
```

`fixture.cjs` is an isolated synthetic New API protocol fixture on
`127.0.0.1:3211`. The separate native AgnAI test instance uses port 3210 and
database `new_api_agnai_fixture`. Its balances/replies are explicitly test data;
they are not proof of real provider charging. Never use its auth endpoints
against a real account.

## Boundary

This implementation deliberately accepts HTTP loopback URLs only.
It is not a production or US-server deployment. Real paid-model charging,
production reverse proxy/cookie-domain design, broader upstream security
review, and user acceptance must be completed before public launch.
