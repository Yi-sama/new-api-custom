import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

test("managed selections retain capability, group and request identity across settings reload", async (t) => {
    const server = await createServer({
        configFile: false,
        resolve: { alias: { "@": fileURLToPath(new URL("../../", import.meta.url)) } },
        server: { middlewareMode: true, hmr: false, ws: false },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, localStorage: globalThis.localStorage, fetch: globalThis.fetch };
    const storage = new Map();
    const listeners = new Set();
    const session = { user_id: 7, username: "fixture", token: "fixture-token", expires_at: 4102444800 };
    const groups = [
        { id: "mixed", description: "", ratio: 1, models: [
            { id: "render-alias", capabilities: ["video"] },
            { id: "gpt-chat", capabilities: ["text"] },
            { id: "image-alias", capabilities: ["image"], parameters: { image: { counts: [1], aspect_ratios: ["3:2"], qualities: ["standard"] } } },
        ] },
        { id: "premium", description: "", ratio: 2, models: [{ id: "render-alias", capabilities: ["video"] }] },
    ];
    const parent = {
        postMessage(message) {
            if (message.type === "new-api-video-request-session") {
                for (const receive of listeners) receive({ origin: "http://localhost:3117", source: parent, data: { type: "new-api-video-session", session } });
            }
        },
    };
    globalThis.window = {
        parent, location: { origin: "http://localhost:3117" },
        addEventListener: (_type, listener) => listeners.add(listener),
        removeEventListener: (_type, listener) => listeners.delete(listener),
    };
    globalThis.localStorage = {
        getItem: (key) => storage.get(key) ?? null,
        setItem: (key, value) => storage.set(key, value),
        removeItem: (key) => storage.delete(key),
    };
    globalThis.window.localStorage = globalThis.localStorage;
    globalThis.fetch = async (url, options) => {
        assert.equal(url, "/api/video-studio/catalog");
        assert.equal(new Headers(options.headers).get("Authorization"), "Bearer fixture-token");
        return Response.json({ success: true, data: { groups } });
    };
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        const { defaultConfig, useConfigStore, selectableModelsByCapability, resolveModelRequestConfig, resolveModelForCapability } = await server.ssrLoadModule("/src/stores/use-config-store.ts");
        assert.deepEqual(selectableModelsByCapability(defaultConfig, "video"), ["site:mixed::render-alias", "site:premium::render-alias"]);
        const snapshot = { ...defaultConfig, model: "site:premium::render-alias" };
        const request = resolveModelRequestConfig(snapshot, snapshot.model);
        assert.equal(request.model, "render-alias");
        assert.equal(request.studioGroup, "premium");
        assert.equal(request.apiKey, "fixture-token");
        assert.equal(snapshot.model, "site:premium::render-alias");
        assert.throws(() => resolveModelRequestConfig(snapshot, "site:private::render-alias"));
        assert.throws(() => resolveModelRequestConfig(snapshot, "site:premium::gpt-chat"));

        useConfigStore.getState().updateConfig("videoModel", "site:premium::render-alias");
        useConfigStore.getState().updateConfig("imageModel", "site:mixed::image-alias");
        await useConfigStore.persist.rehydrate();
        assert.equal(useConfigStore.getState().config.videoModel, "site:premium::render-alias");
        assert.equal(useConfigStore.getState().config.imageModel, "site:mixed::image-alias");
        assert.equal(resolveModelForCapability({ ...defaultConfig, channels: [], models: [] }, "", "video"), "");
        for (const persisted of storage.values()) assert.ok(!persisted.includes("fixture-token"));

        await t.test("switching image profiles omits stale count and sends only configured exact options", async () => {
            let body;
            globalThis.fetch = async (url, options) => {
                assert.equal(url, "/api/video-studio/jobs/images/generations");
                body = JSON.parse(options.body);
                return Response.json({ success: true, data: { id: "image-job", request_id: "image-request" } });
            };
            const { requestGeneration } = await server.ssrLoadModule("/src/services/api/image.ts");
            // Stop after the durable receipt to inspect the actual submitted payload.
            await assert.rejects(requestGeneration({
                ...defaultConfig, model: "site:mixed::image-alias", count: "4",
                studioAspectRatio: "3:2", quality: "standard", size: "999x999",
            }, "synthetic image", { onJobCreated: () => { throw new Error("receipt captured"); } }), { name: "StudioJobInterruptedError" });
            assert.equal(body.n, undefined);
            assert.equal(body.size, undefined);
            assert.equal(body.aspect_ratio, "3:2");
            assert.equal(body.quality, "standard");
        });

        await t.test("submission carries its selected group while existing tasks remain readable after catalog removal", async () => {
            const requests = [];
            globalThis.fetch = async (url, options) => {
                requests.push({ url, ...options });
                if (url.endsWith("/result")) return new Response(new Blob(["fixture-video"], { type: "video/mp4" }));
                return Response.json({ success: true, data: {
                    id: "task-owned", request_id: "request-owned", kind: "video",
                    group: "premium", model: "render-alias",
                    status: options.method === "POST" ? "queued" : "succeeded",
                } });
            };
            const { createVideoGenerationTask, pollVideoGenerationTask } = await server.ssrLoadModule("/src/services/api/video.ts");
            const task = await createVideoGenerationTask(snapshot, "synthetic request", []);
            assert.equal(task.model, "site:premium::render-alias");
            assert.equal(requests[0].headers.get("X-New-Api-Group"), "premium");
            assert.equal(requests[0].url, "/api/video-studio/jobs/videos");
            assert.equal(requests[0].body.get("model"), "render-alias");
            assert.ok(requests[0].headers.get("Idempotency-Key"));
            assert.equal(task.provider, "studio");
            studio.getStudioGroups().splice(0);
            const result = await pollVideoGenerationTask({ ...snapshot, model: "" }, task);
            assert.equal(result.status, "completed");
            assert.equal(requests[1].url, "/api/video-studio/jobs/task-owned");
            assert.equal(requests[2].url, "/api/video-studio/jobs/task-owned/result");
            for (const request of requests.slice(1)) {
                assert.equal(request.headers.get("Authorization"), "Bearer fixture-token");
                assert.equal(request.headers.get("X-New-Api-Group"), null);
            }
        });
    } finally {
        globalThis.window = original.window;
        globalThis.localStorage = original.localStorage;
        globalThis.fetch = original.fetch;
        await server.close();
    }
});
