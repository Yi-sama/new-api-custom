import assert from "node:assert/strict";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

test("managed profile resolves the selected group override without inferring model limits", async () => {
    const server = await createServer({
        configFile: false,
        resolve: { alias: { "@": fileURLToPath(new URL("../../", import.meta.url)) } },
        server: { middlewareMode: true, hmr: false, ws: false },
        optimizeDeps: { noDiscovery: true, include: [] },
        define: { "import.meta.env.VITE_NEW_API_STUDIO": '"1"' },
    });
    try {
        const studio = await server.ssrLoadModule("/src/lib/new-api-studio.ts");
        studio.getStudioGroups().splice(0, Infinity, {
            id: "premium",
            models: [{
                id: "video-model",
                capabilities: ["video"],
                parameters: {
                    video: { durations: [4, 8], resolutions: ["720p"] },
                    groups: { premium: { video: { durations: [6] } } },
                },
            }],
        });
        const parameters = await server.ssrLoadModule("/src/lib/studio-model-parameters.ts");
        const profile = parameters.getManagedStudioParameterProfile({ model: "site:premium::video-model" }, "video");
        assert.deepEqual(profile.durations, [6]);
        assert.deepEqual(profile.resolutions, ["720p"]);
        assert.equal(parameters.getManagedStudioParameterProfile({ model: "site:premium::unknown" }, "video"), undefined);
    } finally {
        await server.close();
    }
});
