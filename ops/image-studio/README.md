# Image Studio

The main site at `http://localhost:3000/` always shows the Image Studio tab.
Until Open WebUI is configured, the tab clearly reports that it is unavailable.
Once configured, the same tab starts a New API login-session handoff to the
Open WebUI application on its separate root origin. Open WebUI is not mounted
under a New API path or embedded in an iframe.

## Local setup

1. Keep New API on `http://localhost:3000` with a persistent `SESSION_SECRET`.
   The local `start-local-b-c.ps1` reads its secret from the ignored
   `.local-tests/image-studio-session-secret` file and sets
   `IMAGE_STUDIO_URL=http://localhost:3002` and
   `IMAGE_STUDIO_IDENTITY_DOMAIN=studio.example.com` when that file exists.
   Leave `IMAGE_STUDIO_COOKIE_DOMAIN` unset for localhost.
2. Copy `.env.example` to `.env` here. Set random Open WebUI secrets and an
   admin password of at most 72 ASCII bytes. The admin email must match
   `new-api-<root-id>@studio.example.com`.
3. The deployed bridge enables OpenAI-compatible chat requests without a
   shared New API token. Keep image generation and editing disabled until a
   live image-capable upstream model is available and manually verified.
4. Start Docker Desktop. From the repository root run
   `docker compose -f ops/image-studio/docker-compose.yml --env-file ops/image-studio/.env up -d`.
5. Sign in at `http://localhost:3000/` and click Image Studio.

For the local single-user preview, the Compose file builds a small derived
Open WebUI image. It adds a rounded balance badge beside Open WebUI's native
top-right actions, showing the associated New API user's remaining and used
   quota, refreshing every 30 seconds, plus a return-to-console link directly
   above Open WebUI's native New Chat item in the sidebar. The link is present
   for ordinary users even when the Workspace item is absent. The local `.env` may use a dedicated token
owned by the preview account and enable chat; do not reuse this configuration
for a multi-user deployment.

The gateway alone publishes `127.0.0.1:3002`. It validates every request
against the live New API session and strips untrusted identity headers.
Open WebUI persists accounts and history in the `open_webui_data` volume;
admin access to other users' chats is disabled. Do not publish its container
directly.

The local UI uses Open WebUI's own layout on port 3002, launched from the
New API navigation at port 3000. It is not a re-skinned New API page. The
derived image keeps the native schedule, automation, exploration, status,
online-count and other menus, while synchronizing the visible account name
and hiding the independent Open WebUI sign-out entry. The group picker and
model list are scoped to the current New API account.

## Production

Use a dedicated HTTPS root origin for Open WebUI, preserving WebSocket and
streaming. Set the New API studio URL to that origin and the cookie domain to
the trusted common parent domain. Set the Compose URL to the same origin and
`IMAGE_STUDIO_COOKIE_SECURE=true`. Keep all secrets stable and private.
Before launch, verify that Open WebUI refreshes an existing account's name and
role when New API changes them; its own login session can outlive the bridge.
Preserve Open WebUI's branding and comply with its license.

## Accounting

Open WebUI chat and compatible relay requests go through New API's signed
per-user Studio bridge. The bridge validates the live New API session and
selected group on every request, strips client bearer tokens and unrelated
cookies, and settles quota against the authenticated New API user. Open WebUI
history remains in its own volume and is not a New API usage-log replacement.
Image generation/editing stays disabled locally until an image-capable
provider is available and manually validated through the same bridge.
