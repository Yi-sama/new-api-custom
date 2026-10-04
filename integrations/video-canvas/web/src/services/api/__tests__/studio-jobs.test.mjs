import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const studioJob = (overrides = {}) => ({
    id: "job-1",
    request_id: "request-1",
    kind: "image",
    operation: "generations",
    group: "premium",
    model: "image",
    prompt: "test",
    parameters: {},
    status: "queued",
    created_at: 1,
    updated_at: 1,
    ...overrides,
});

test("studio jobs preserve durable submission and polling identity", async (t) => {
    const server = await createServer({
        configFile: false,
        resolve: { alias: { "@": fileURLToPath(new URL("../../../", import.meta.url)) } },
        server: { middlewareMode: true, hmr: false, ws: false },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    const original = { window: globalThis.window, fetch: globalThis.fetch, localStorage: globalThis.localStorage };
    const listeners = new Set();
    const session = { user_id: 7, username: "fixture", token: "fixture-token", expires_at: 4102444800 };
    const parent = { postMessage(message) { if (message.type === "new-api-video-request-session") for (const receive of listeners) receive({ origin: "http://localhost:3117", source: parent, data: { type: "new-api-video-session", session } }); } };
    const requests = [];
    let handleRequest = async (url) => {
        if (url === "/api/video-studio/catalog") return Response.json({ success: true, data: { groups: [] } });
        throw new Error(`Unexpected request ${url}`);
    };
    globalThis.window = { parent, location: { origin: "http://localhost:3117" }, addEventListener: (_type, listener) => listeners.add(listener), removeEventListener: (_type, listener) => listeners.delete(listener), setTimeout, clearTimeout };
    globalThis.localStorage = { getItem: () => null, setItem() {}, removeItem() {} };
    globalThis.fetch = async (url, options = {}) => {
        requests.push({ url, options });
        return handleRequest(url, options);
    };

    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        await studio.initializeStudio();
        const jobs = await server.ssrLoadModule("/src/services/api/studio-jobs.ts");

        await t.test("persistence rejection prevents the POST", async () => {
            requests.length = 0;
            handleRequest = async (url) => { throw new Error(`Unexpected request ${url}`); };
            await assert.rejects(
                () => jobs.submitStudioJob("/jobs/images/generations", "{}", { onRequestCreated: async () => { throw new Error("persist failed"); } }),
                /persist failed/,
            );
            assert.equal(requests.length, 0);
        });

        await t.test("lost POST response resolves by request_id without another POST", async () => {
            requests.length = 0;
            const requestId = "request-lost";
            let persistedRequestId;
            handleRequest = async (url, options) => {
                if (url === "/api/video-studio/jobs/images/generations") {
                    assert.equal(options.headers.get("Authorization"), "Bearer fixture-token");
                    assert.equal(options.headers.get("X-New-Api-Group"), "premium");
                    assert.equal(options.headers.get("Idempotency-Key"), requestId);
                    throw new TypeError("response lost");
                }
                if (url === `/api/video-studio/jobs?kind=image&p=1&page_size=20&request_id=${requestId}`) {
                    return Response.json({ success: true, data: { items: [studioJob({ id: "job-lost", request_id: requestId })], total: 1 } });
                }
                throw new Error(`Unexpected request ${url}`);
            };
            await assert.rejects(
                () => jobs.submitStudioJob("/jobs/images/generations", "{}", { group: "premium", contentType: "application/json", idempotencyKey: requestId, onRequestCreated: (value) => { persistedRequestId = value; } }),
                (error) => jobs.isStudioJobSubmissionUncertain(error) && error.requestId === requestId,
            );
            assert.equal(persistedRequestId, requestId);
            assert.equal(await jobs.resolveStudioJobReference("image", { requestId }), "job-lost");
            assert.equal(requests.filter(({ options }) => options.method === "POST").length, 1);
        });

        await t.test("aborted and unreadable polls resume the same job id", async () => {
            requests.length = 0;
            let attempt = 0;
            handleRequest = async (url) => {
                assert.equal(url, "/api/video-studio/jobs/job-resume");
                attempt += 1;
                if (attempt === 1) throw new DOMException("Aborted", "AbortError");
                if (attempt === 2) return new Response("{", { headers: { "Content-Type": "application/json" } });
                return Response.json({ success: true, data: studioJob({ id: "job-resume", status: "succeeded", updated_at: 2 }) });
            };
            await assert.rejects(() => jobs.waitForStudioJob("job-resume"), (error) => jobs.isStudioJobInterrupted(error) && error.jobId === "job-resume");
            await assert.rejects(() => jobs.waitForStudioJob("job-resume"), (error) => jobs.isStudioJobInterrupted(error) && error.jobId === "job-resume");
            assert.equal((await jobs.waitForStudioJob("job-resume")).id, "job-resume");
            assert.deepEqual(requests.map(({ url }) => url), Array(3).fill("/api/video-studio/jobs/job-resume"));
            assert.equal(requests.filter(({ options }) => options.method === "POST").length, 0);
        });

        await t.test("pagination sends p and preserves total", async () => {
            requests.length = 0;
            handleRequest = async (url) => {
                assert.equal(url, "/api/video-studio/jobs?kind=video&p=3&page_size=20");
                return Response.json({ success: true, data: { items: [studioJob({ id: "video-41", kind: "video" })], total: 47 } });
            };
            const page = await jobs.listStudioJobs("video", { page: 3 });
            assert.equal(page.total, 47);
            assert.deepEqual(page.items.map((job) => job.id), ["video-41"]);
            assert.equal(requests.length, 1);
        });
    } finally {
        globalThis.window = original.window;
        globalThis.fetch = original.fetch;
        globalThis.localStorage = original.localStorage;
        await server.close();
    }
});
