import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

test("canvas persistence can flush a Studio receipt before the debounce", async () => {
    const server = await createServer({
        configFile: false,
        resolve: { alias: { "@": fileURLToPath(new URL("../../../", import.meta.url)) } },
        server: { middlewareMode: true, hmr: false, ws: false },
        optimizeDeps: { noDiscovery: true, include: [] },
    });
    const original = { window: globalThis.window, localStorage: globalThis.localStorage };
    const storage = new Map();
    globalThis.localStorage = {
        getItem: (key) => storage.get(key) ?? null,
        setItem: (key, value) => storage.set(key, value),
        removeItem: (key) => storage.delete(key),
    };
    globalThis.window = { localStorage: globalThis.localStorage };
    try {
        const { useCanvasStore } = await server.ssrLoadModule("/src/stores/canvas/use-canvas-store.ts");
        const projectId = useCanvasStore.getState().createProject("Receipt fixture");
        await useCanvasStore.getState().flushPersistence();
        storage.clear();

        useCanvasStore.getState().updateProject(projectId, {
            nodes: [{
                id: "image-node",
                type: "image",
                title: "Pending image",
                position: { x: 0, y: 0 },
                width: 320,
                height: 320,
                metadata: { studioJob: { requestId: "request-durable" } },
            }],
        });
        await useCanvasStore.getState().flushPersistence();

        const persisted = [...storage.values()].find((value) => value.includes("request-durable"));
        assert.ok(persisted, "flushPersistence should durably write the latest project nodes immediately");
        const decoded = JSON.parse(persisted);
        const snapshot = typeof decoded === "string" ? JSON.parse(decoded) : decoded;
        assert.equal(snapshot.state.projects[0].nodes[0].metadata.studioJob.requestId, "request-durable");
    } finally {
        globalThis.window = original.window;
        globalThis.localStorage = original.localStorage;
        await server.close();
    }
});
