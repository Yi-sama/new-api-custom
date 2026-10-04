import assert from "node:assert/strict";
import test from "node:test";
import { createServer } from "vite";

function createManagedStudio(initialSession, renew) {
    const listeners = new Set();
    const messages = [];
    const parent = {
        postMessage(message, origin) {
            messages.push([message, origin]);
            if (message.type === "new-api-video-request-session") {
                for (const receive of listeners) receive({ origin, source: parent, data: { type: "new-api-video-session", session: initialSession } });
            }
            if (message.type === "new-api-video-renew-session") renew({ listeners, parent, origin });
        },
    };
    return {
        messages,
        window: {
            parent,
            location: { origin: "http://localhost:3117" },
            addEventListener: (_type, listener) => listeners.add(listener),
            removeEventListener: (_type, listener) => listeners.delete(listener),
        },
    };
}

test("expired managed sessions renew once before one task request and keep the current account", async () => {
    const server = await createServer({
        configFile: false,
        server: { middlewareMode: true },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, fetch: globalThis.fetch, now: Date.now };
    const initial = { user_id: 7, username: "fixture", token: "old-token", expires_at: 2 };
    const renewed = { ...initial, token: "new-token", expires_at: 100 };
    let respond;
    const managed = createManagedStudio(initial, ({ listeners, parent, origin }) => {
        respond = () => {
            for (const receive of listeners) {
                receive({ origin: "https://other.example", source: parent, data: { type: "new-api-video-session-error", message: "untrusted" } });
                receive({ origin, source: {}, data: { type: "new-api-video-session-error", message: "untrusted" } });
                receive({ origin, source: parent, data: { type: "new-api-video-session", session: renewed } });
            }
        };
    });
    const requests = [];
    globalThis.window = managed.window;
    Date.now = () => 1_000;
    globalThis.fetch = async (url, init) => {
        requests.push([url, init]);
        return Response.json({ success: true, data: { groups: [] } });
    };
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        assert.deepEqual(await studio.getValidStudioSession(), initial);
        assert.equal(managed.messages.filter(([message]) => message.type === "new-api-video-renew-session").length, 0);
        Date.now = () => 2_000;
        const pending = Promise.all([studio.getValidStudioSession(), studio.getValidStudioSession()]);
        assert.equal(managed.messages.filter(([message]) => message.type === "new-api-video-renew-session").length, 1);
        respond();
        const [first, second] = await pending;
        assert.deepEqual(first, renewed);
        assert.deepEqual(second, renewed);
        assert.equal(managed.messages.filter(([message]) => message.type === "new-api-video-renew-session").length, 1);
        await studio.studioFetch("/v1/videos", { method: "POST" });
        assert.equal(requests.length, 2);
        assert.equal(requests[1][0], "/api/video-studio/v1/videos");
        assert.equal(requests[1][1].headers.get("Authorization"), "Bearer new-token");
    } finally {
        globalThis.window = original.window;
        globalThis.fetch = original.fetch;
        Date.now = original.now;
        await server.close();
    }
});

test("renewal rejects host authentication errors and clears the pending renewal", async () => {
    const server = await createServer({
        configFile: false,
        server: { middlewareMode: true },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, fetch: globalThis.fetch, now: Date.now };
    const initial = { user_id: 7, username: "fixture", token: "old-token", expires_at: 2 };
    let attempts = 0;
    const managed = createManagedStudio(initial, ({ listeners, parent, origin }) => {
        attempts += 1;
        const data = attempts === 1 ? { type: "new-api-video-session-error", message: "Unable to renew Video Studio. Please sign in again." } : { type: "new-api-video-session", session: { ...initial, token: "recovered-token", expires_at: 100 } };
        for (const receive of listeners) receive({ origin, source: parent, data });
    });
    globalThis.window = managed.window;
    Date.now = () => 1_000;
    globalThis.fetch = async () => Response.json({ success: true, data: { groups: [] } });
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        Date.now = () => 2_000;
        await assert.rejects(studio.getValidStudioSession(), /Unable to renew Video Studio\. Please sign in again\./);
        assert.equal((await studio.getValidStudioSession()).token, "recovered-token");
        assert.equal(attempts, 2);
    } finally {
        globalThis.window = original.window;
        globalThis.fetch = original.fetch;
        Date.now = original.now;
        await server.close();
    }
});

test("a 401 renews once and retries only a managed GET with the new bearer", async () => {
    const server = await createServer({
        configFile: false,
        server: { middlewareMode: true },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, fetch: globalThis.fetch, now: Date.now };
    const initial = { user_id: 7, username: "fixture", token: "old-token", expires_at: 100 };
    const renewed = { ...initial, token: "new-token", expires_at: 200 };
    const managed = createManagedStudio(initial, ({ listeners, parent, origin }) => {
        for (const receive of listeners) receive({ origin, source: parent, data: { type: "new-api-video-session", session: renewed } });
    });
    const requests = [];
    globalThis.window = managed.window;
    Date.now = () => 1_000;
    globalThis.fetch = async (url, init) => {
        requests.push([url, init]);
        if (url.endsWith("/catalog")) return Response.json({ success: true, data: { groups: [] } });
        if (init.headers.get("Authorization") === "Bearer old-token") return new Response(null, { status: 401 });
        return Response.json({ ok: true });
    };
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        const response = await studio.studioFetch("/v1/videos/task-owned");
        assert.equal(response.status, 200);
        assert.equal(managed.messages.filter(([message]) => message.type === "new-api-video-renew-session").length, 1);
        assert.deepEqual(
            requests.slice(1).map(([, init]) => init.headers.get("Authorization")),
            ["Bearer old-token", "Bearer new-token"],
        );
    } finally {
        globalThis.window = original.window;
        globalThis.fetch = original.fetch;
        Date.now = original.now;
        await server.close();
    }
});

test("a 401 renews the bearer without replaying a managed POST", async () => {
    const server = await createServer({
        configFile: false,
        server: { middlewareMode: true },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, fetch: globalThis.fetch, now: Date.now };
    const initial = { user_id: 7, username: "fixture", token: "old-token", expires_at: 100 };
    const renewed = { ...initial, token: "new-token", expires_at: 200 };
    const managed = createManagedStudio(initial, ({ listeners, parent, origin }) => {
        for (const receive of listeners) receive({ origin, source: parent, data: { type: "new-api-video-session", session: renewed } });
    });
    const requests = [];
    globalThis.window = managed.window;
    Date.now = () => 1_000;
    globalThis.fetch = async (url, init) => {
        requests.push([url, init]);
        if (url.endsWith("/catalog")) return Response.json({ success: true, data: { groups: [] } });
        return new Response(null, { status: 401 });
    };
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        const response = await studio.studioFetch("/v1/videos", { method: "POST" });
        assert.equal(response.status, 401);
        assert.equal(managed.messages.filter(([message]) => message.type === "new-api-video-renew-session").length, 1);
        assert.equal(requests.length, 2);
        assert.equal((await studio.getValidStudioSession()).token, "new-token");
    } finally {
        globalThis.window = original.window;
        globalThis.fetch = original.fetch;
        Date.now = original.now;
        await server.close();
    }
});

test("a second GET 401 stops replay and invalidates the renewed bearer for the next poll", async () => {
    const server = await createServer({
        configFile: false,
        server: { middlewareMode: true, hmr: false, ws: false },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, fetch: globalThis.fetch, now: Date.now };
    const initial = { user_id: 7, username: "fixture", token: "old-token", expires_at: 100 };
    let renewals = 0;
    const managed = createManagedStudio(initial, ({ listeners, parent, origin }) => {
        renewals += 1;
        const session = { ...initial, token: `new-token-${renewals}`, expires_at: 200 };
        for (const receive of listeners) receive({ origin, source: parent, data: { type: "new-api-video-session", session } });
    });
    const requests = [];
    globalThis.window = managed.window;
    Date.now = () => 1_000;
    globalThis.fetch = async (url, init) => {
        if (url.endsWith("/catalog")) return Response.json({ success: true, data: { groups: [] } });
        requests.push(init.headers.get("Authorization"));
        return new Response(null, { status: 401 });
    };
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        assert.equal((await studio.studioFetch("/jobs/task-owned")).status, 401);
        assert.deepEqual(requests, ["Bearer old-token", "Bearer new-token-1"]);
        assert.equal(renewals, 1);
        assert.equal((await studio.getValidStudioSession()).token, "new-token-2");
        assert.equal(renewals, 2);
        assert.equal(requests.length, 2);
    } finally {
        globalThis.window = original.window;
        globalThis.fetch = original.fetch;
        Date.now = original.now;
        await server.close();
    }
});

test("an early 401 with revoked host auth rejects without replay and leaves renewal available for a later explicit attempt", async () => {
    const server = await createServer({
        configFile: false,
        server: { middlewareMode: true, hmr: false, ws: false },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, fetch: globalThis.fetch, now: Date.now };
    const initial = { user_id: 7, username: "fixture", token: "old-token", expires_at: 100 };
    let renewals = 0;
    const managed = createManagedStudio(initial, ({ listeners, parent, origin }) => {
        renewals += 1;
        for (const receive of listeners)
            receive({
                origin,
                source: parent,
                data: { type: "new-api-video-session-error", message: "Unable to renew Video Studio. Please sign in again." },
            });
    });
    let requests = 0;
    globalThis.window = managed.window;
    Date.now = () => 1_000;
    globalThis.fetch = async (url) => {
        if (url.endsWith("/catalog")) return Response.json({ success: true, data: { groups: [] } });
        requests += 1;
        return new Response(null, { status: 401 });
    };
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        await assert.rejects(studio.studioFetch("/jobs/task-owned"), /Please sign in again/);
        assert.equal(requests, 1);
        assert.equal(renewals, 1);
        assert.equal(studio.getStudioSession().expires_at, 0);
        await assert.rejects(studio.getValidStudioSession(), /Please sign in again/);
        assert.equal(renewals, 2);
        assert.equal(requests, 1);
    } finally {
        globalThis.window = original.window;
        globalThis.fetch = original.fetch;
        Date.now = original.now;
        await server.close();
    }
});

test("the account-refresh wrapper renews before resolving the request bearer and skips the request on auth failure", async () => {
    const server = await createServer({
        configFile: false,
        server: { middlewareMode: true, hmr: false, ws: false },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, fetch: globalThis.fetch, now: Date.now };
    const initial = { user_id: 7, username: "fixture", token: "old-token", expires_at: 2 };
    let revoked = false;
    const managed = createManagedStudio(initial, ({ listeners, parent, origin }) => {
        const data = revoked ? { type: "new-api-video-session-error", message: "Unable to renew Video Studio. Please sign in again." } : { type: "new-api-video-session", session: { ...initial, token: "new-token", expires_at: 3 } };
        for (const receive of listeners) receive({ origin, source: parent, data });
    });
    globalThis.window = managed.window;
    Date.now = () => 1_000;
    globalThis.fetch = async () => Response.json({ success: true, data: { groups: [] } });
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        const requests = [];
        const request = studio.withStudioAccountRefresh(async (prompt) => {
            requests.push([prompt, studio.getStudioSession().token]);
            return "answer";
        });
        Date.now = () => 2_000;
        assert.equal(await request("text or audio input"), "answer");
        assert.deepEqual(requests, [["text or audio input", "new-token"]]);
        assert.equal(managed.messages.at(-1)[0].type, "new-api-video-account-changed");
        Date.now = () => 3_000;
        revoked = true;
        await assert.rejects(request("not submitted"), /Please sign in again/);
        assert.equal(requests.length, 1);
    } finally {
        globalThis.window = original.window;
        globalThis.fetch = original.fetch;
        Date.now = original.now;
        await server.close();
    }
});
