// Local-only provider fixture. No external calls or provider credentials.
import http from "node:http";

const port = Number(process.env.STUDIO_FIXTURE_PORT || 3108);
const image = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jfZkAAAAASUVORK5CYII=";
const pending = new Set();
let submissions = 0;
let released = false;

function json(response, value) {
    response.writeHead(200, { "Content-Type": "application/json" });
    response.end(JSON.stringify(value));
}

const server = http.createServer(async (request, response) => {
    if (request.method === "GET" && request.url === "/__fixture/status") {
        json(response, { submissions, pending: pending.size, released });
        return;
    }
    if (request.method === "POST" && request.url === "/__fixture/release") {
        released = true;
        for (const item of pending) json(item, { created: Math.floor(Date.now() / 1000), data: [{ b64_json: image }] });
        pending.clear();
        json(response, { released: true, submissions });
        return;
    }
    if (request.method === "POST" && request.url === "/__fixture/reset") {
        if (pending.size) {
            response.writeHead(409);
            response.end("Finish pending requests first.");
            return;
        }
        released = false;
        submissions = 0;
        json(response, { reset: true });
        return;
    }
    if (request.method === "POST" && request.url === "/v1/images/generations") {
        // Drain the request without retaining prompts or headers.
        for await (const chunk of request) void chunk;
        submissions += 1;
        if (released) json(response, { data: [{ b64_json: image }] });
        else pending.add(response);
        return;
    }
    response.writeHead(404);
    response.end("Unsupported synthetic provider operation.");
});

server.listen(port, "127.0.0.1", () => console.log(`Synthetic image provider on 127.0.0.1:${port}`));
