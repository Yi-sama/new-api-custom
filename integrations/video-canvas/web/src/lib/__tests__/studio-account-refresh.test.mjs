import assert from "node:assert/strict";
import test from "node:test";
import { createServer } from "vite";

test("managed requests refresh the authoritative wallet after success and failure without changing the result", async () => {
    const server = await createServer({
        configFile: false,
        server: { middlewareMode: true, hmr: false, ws: false },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const previousWindow = globalThis.window;
    const previousFetch = globalThis.fetch;
    const messages = [];
    const listeners = new Set();
    const session = { user_id: 7, username: "fixture", token: "fixture-token", expires_at: 4102444800 };
    const parent = {
        postMessage(message, origin) {
            if (message.type === "new-api-video-request-session") {
                for (const receive of listeners) receive({ origin, source: parent, data: { type: "new-api-video-session", session } });
            } else messages.push([message, origin]);
        },
    };
    globalThis.window = {
        parent,
        location: { origin: "http://localhost:3117" },
        addEventListener: (_type, listener) => listeners.add(listener),
        removeEventListener: (_type, listener) => listeners.delete(listener),
    };
    globalThis.fetch = async () => Response.json({ success: true, data: { groups: [] } });
    try {
        const { initializeStudio, withStudioAccountRefresh, requestStudioAccountAction } = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await initializeStudio();
        let resolveRequest;
        const started = Promise.withResolvers();
        const pending = withStudioAccountRefresh(
            () =>
                new Promise((resolve) => {
                    resolveRequest = resolve;
                    started.resolve();
                }),
        )();
        await started.promise;
        assert.equal(messages.length, 0);
        resolveRequest({ id: "video-task" });
        assert.deepEqual(await pending, { id: "video-task" });
        assert.deepEqual(messages, [[{ type: "new-api-video-account-changed", message: undefined }, "http://localhost:3117"]]);
        const failure = new Error("upstream rejected");
        await assert.rejects(
            withStudioAccountRefresh(async () => {
                throw failure;
            })(),
            (error) => error === failure,
        );
        assert.equal(messages.length, 2);
        requestStudioAccountAction("sign-out");
        assert.deepEqual(messages[2], [{ type: "new-api-video-account-action", action: "sign-out" }, "http://localhost:3117"]);
        requestStudioAccountAction("dashboard");
        assert.deepEqual(messages[3], [{ type: "new-api-video-account-action", action: "dashboard" }, "http://localhost:3117"]);
    } finally {
        globalThis.window = previousWindow;
        globalThis.fetch = previousFetch;
        await server.close();
    }
});

test("standalone requests keep working without any parent window", async () => {
    const { withStudioAccountRefresh } = await import("../new-api-studio.ts");
    assert.equal(await withStudioAccountRefresh(async (value) => value + 1)(4), 5);
});
