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
            { id: "gpt-image-2.5-web", capabilities: ["image"] },
            ...["gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "illustration-v3", "custom-image-alias"].map((id) => ({ id, capabilities: ["image"] })),
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

        await t.test("web image settings reach generation and edit requests while automatic values are omitted", async () => {
            const parameters = await server.ssrLoadModule("/src/lib/studio-model-parameters.ts");
            const config = { ...defaultConfig, model: "site:mixed::gpt-image-2.5-web", count: "1", size: "", studioAspectRatio: "" };
            assert.deepEqual(parameters.managedImageParameters(config), { n: 1 });
            assert.deepEqual(parameters.managedImageParameters({ ...config, size: "1K" }), { n: 1, size: "1K" });
            assert.deepEqual(parameters.managedImageParameters({ ...config, studioAspectRatio: "9:16" }), { n: 1, size: "9:16" });
            assert.deepEqual(parameters.managedImageParameters({ ...config, size: "2K", studioAspectRatio: "16:9" }), { n: 1, size: "2048x1152" });
            assert.deepEqual(parameters.managedImageParameters({ ...config, size: "0.5K", studioAspectRatio: "1:8" }), { n: 1, size: "64x512" });
            assert.equal(parameters.managedImageParameters({ ...config, count: "15" }).n, undefined);
            assert.equal(parameters.getManagedImageParameterProfile({ model: "site:mixed::unknown" }), undefined);
            assert.deepEqual(parameters.getManagedImageParameterProfile({ model: "site:mixed::image-alias" }).counts, [1]);
            const { buildGenerationConfig, getGenerationCount } = await server.ssrLoadModule("/src/lib/canvas/canvas-generation-helpers.ts");
            const { buildImageGenerationMetadata } = await server.ssrLoadModule("/src/lib/canvas/canvas-node-factory.ts");
            const automatic = buildGenerationConfig({ ...config, size: "4K", studioAspectRatio: "16:9" }, { metadata: { size: "", studioAspectRatio: "", count: 15 } }, "image");
            assert.equal(automatic.size, "");
            assert.equal(automatic.studioAspectRatio, "");
            assert.equal(getGenerationCount(automatic.count), 15);
            assert.equal(buildImageGenerationMetadata("generation", automatic, 15, []).studioAspectRatio, "");

            const { requestGeneration, requestEdit } = await server.ssrLoadModule("/src/services/api/image.ts");
            const options = { onJobCreated: () => { throw new Error("receipt captured"); } };
            let submitted;
            globalThis.fetch = async (url, request) => {
                if (String(url).startsWith("data:")) return original.fetch(url, request);
                submitted = request.body instanceof FormData ? Object.fromEntries(request.body) : JSON.parse(request.body);
                assert.equal(request.headers.get("X-New-Api-Group"), "mixed");
                return Response.json({ success: true, data: { id: "image-job", request_id: "image-request" } });
            };
            await assert.rejects(requestGeneration({ ...config, count: "4", size: "2K", studioAspectRatio: "16:9" }, "fixture", options), { name: "StudioJobInterruptedError" });
            assert.equal(submitted.n, 4);
            assert.equal(submitted.size, "2048x1152");
            assert.equal(submitted.aspect_ratio, undefined);
            const image = { id: "fixture", name: "fixture.png", type: "image/png", dataUrl: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jV1sAAAAASUVORK5CYII=" };
            await assert.rejects(requestEdit({ ...config, size: "2K", studioAspectRatio: "16:9" }, "fixture", [image], options), { name: "StudioJobInterruptedError" });
            assert.equal(submitted.n, "1");
            assert.equal(submitted.size, "2048x1152");
            assert.ok(submitted.image instanceof File);
            await assert.rejects(requestGeneration(config, "automatic fixture", options), { name: "StudioJobInterruptedError" });
            assert.equal(submitted.size, undefined);
            assert.equal(submitted.aspect_ratio, undefined);
            assert.equal(submitted.quality, undefined);
        });

        await t.test("all unconfigured image models expose shared settings and send pixel sizes for generation and edit", async () => {
            const parameters = await server.ssrLoadModule("/src/lib/studio-model-parameters.ts");
            const { requestGeneration, requestEdit } = await server.ssrLoadModule("/src/services/api/image.ts");
            const options = { onJobCreated: () => { throw new Error("receipt captured"); } };
            const image = { id: "fixture", name: "fixture.png", type: "image/png", dataUrl: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jV1sAAAAASUVORK5CYII=" };
            let submitted;
            globalThis.fetch = async (url, request) => {
                if (String(url).startsWith("data:")) return original.fetch(url, request);
                submitted = request.body instanceof FormData ? Object.fromEntries(request.body) : JSON.parse(request.body);
                assert.equal(request.headers.get("X-New-Api-Group"), "mixed");
                return Response.json({ success: true, data: { id: "image-job", request_id: "image-request" } });
            };
            for (const model of ["gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "illustration-v3", "custom-image-alias"]) {
                const config = { ...defaultConfig, model: `site:mixed::${model}`, count: "1", size: "2K", studioAspectRatio: "16:9", quality: "high" };
                const profile = parameters.getManagedImageParameterProfile(config);
                assert.equal(profile, parameters.defaultImageParameterProfile);
                assert.equal(parameters.isImageResolutionProfile(profile), true);
                assert.deepEqual(profile.sizes, ["1K", "2K", "4K"]);
                assert.deepEqual(parameters.managedImageParameters(config), { n: 1, size: "2048x1152", quality: "high" });
                assert.deepEqual(parameters.managedImageParameters({ ...config, size: "", studioAspectRatio: "", quality: "" }), { n: 1 });
                assert.deepEqual(parameters.managedImageParameters({ ...config, size: "auto", studioAspectRatio: "auto", quality: "auto" }), { n: 1 });
                assert.deepEqual(parameters.managedImageParameters({ ...config, size: "0.5K", studioAspectRatio: "1:8", quality: "standard", count: "15" }), {});
                assert.equal(parameters.managedImageParameters({ ...config, studioAspectRatio: "" }).size, "2048x2048");
                assert.equal(parameters.managedImageParameters({ ...config, size: "", studioAspectRatio: "9:16" }).size, "864x1536");
                assert.equal(parameters.getManagedImageParameterProfile({ model: "site:premium::render-alias", imageModel: config.model }), profile);
                for (const size of profile.sizes) for (const studioAspectRatio of profile.aspect_ratios) {
                    const dimensions = parameters.managedImageParameters({ ...config, size, studioAspectRatio }).size;
                    const [width, height] = dimensions.split("x").map(Number);
                    assert.ok(width % 16 === 0 && height % 16 === 0);
                    assert.ok(Math.max(width, height) <= 3840 && Math.max(width, height) / Math.min(width, height) <= 3);
                    assert.ok(width * height >= 655360 && width * height <= 8294400);
                }
                await assert.rejects(requestGeneration(config, "native fixture", options), { name: "StudioJobInterruptedError" });
                assert.equal(submitted.model, model);
                assert.equal(submitted.n, 1);
                assert.equal(submitted.size, "2048x1152");
                assert.equal(submitted.quality, "high");
                assert.equal(submitted.aspect_ratio, undefined);
                await assert.rejects(requestEdit({ ...config, size: "4K", studioAspectRatio: "9:16", quality: "low" }, "native edit fixture", [image], options), { name: "StudioJobInterruptedError" });
                assert.equal(submitted.model, model);
                assert.equal(submitted.n, "1");
                assert.equal(submitted.size, "2160x3840");
                assert.equal(submitted.quality, "low");
                assert.equal(submitted.aspect_ratio, undefined);
                assert.ok(submitted.image instanceof File);
            }
        });

        await t.test("newly catalogued image models receive shared settings without frontend name registration", async () => {
            const parameters = await server.ssrLoadModule("/src/lib/studio-model-parameters.ts");
            const catalog = studio.getStudioGroups()[0].models;
            const model = "future-provider-image";
            catalog.push({ id: model, capabilities: ["image", "text"] });
            try {
                const config = { ...defaultConfig, model: `site:mixed::${model}`, count: "1", size: "4K", studioAspectRatio: "9:16", quality: "medium" };
                assert.equal(parameters.getManagedImageParameterProfile(config), parameters.defaultImageParameterProfile);
                assert.deepEqual(parameters.managedImageParameters(config), { n: 1, size: "2160x3840", quality: "medium" });
                assert.deepEqual(parameters.managedImageParameters({ ...config, size: "", studioAspectRatio: "", quality: "" }), { n: 1 });
                assert.equal(parameters.getManagedImageParameterProfile({ model: "site:mixed::gpt-chat" }), undefined);
                assert.equal(parameters.getManagedImageParameterProfile({ model: "site:mixed::render-alias" }), undefined);
            } finally {
                catalog.splice(catalog.findIndex((entry) => entry.id === model), 1);
            }
            assert.equal(parameters.getManagedImageParameterProfile({ model: `site:mixed::${model}` }), undefined);
        });

        await t.test("administrator metadata remains authoritative and unavailable models do not receive settings", async () => {
            const parameters = await server.ssrLoadModule("/src/lib/studio-model-parameters.ts");
            const native = studio.getStudioGroups()[0].models.find((entry) => entry.id === "gpt-image-2");
            native.parameters = { image: { sizes: ["1024x1024"], qualities: ["low"] }, groups: { mixed: { image: { sizes: ["1536x1024"] } } } };
            try {
                const profile = parameters.getManagedImageParameterProfile({ model: "site:mixed::gpt-image-2" });
                assert.deepEqual(profile.sizes, ["1536x1024"]);
                assert.deepEqual(profile.qualities, ["low"]);
                assert.equal(parameters.isImageResolutionProfile(profile), false);
                assert.deepEqual(parameters.managedImageParameters({ model: "site:mixed::gpt-image-2", count: "1", size: "2K", quality: "high", studioAspectRatio: "16:9" }), {});
                assert.deepEqual(parameters.managedImageParameters({ model: "site:mixed::gpt-image-2", count: "1", size: "1536x1024", quality: "low" }), { size: "1536x1024", quality: "low" });
                assert.equal(parameters.getManagedImageParameterProfile({ model: "site:mixed::unknown", imageModel: "site:mixed::gpt-image-2.5-flare" }), parameters.defaultImageParameterProfile);
                assert.equal(parameters.getManagedImageParameterProfile({ model: "site:premium::gpt-image-2" }), undefined);
            } finally {
                delete native.parameters;
            }
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
