import assert from "node:assert/strict";
import test from "node:test";

import { readStudioAccountMessage, readStudioSessionMessage, studioStorageName } from "../new-api-studio.ts";

const origin = "http://localhost:3000";
const parent = {};
const session = { user_id: 17, username: "studio-user", token: "test-scoped-token", expires_at: 4102444800 };
const event = { origin, source: parent, data: { type: "new-api-video-session", session } };

test("only the embedding parent at the same origin can initialize a studio session", () => {
    assert.deepEqual(readStudioSessionMessage(event, origin, parent), session);
    assert.equal(readStudioSessionMessage({ ...event, origin: "https://other.example" }, origin, parent), null);
    assert.equal(readStudioSessionMessage({ ...event, source: {} }, origin, parent), null);
});

test("expired, missing and invalid session identities are rejected", () => {
    for (const value of [{ ...session, user_id: 0 }, { ...session, user_id: 1.5 }, { ...session, token: "" }, { ...session, expires_at: 1 }, null]) {
        assert.equal(readStudioSessionMessage({ ...event, data: { type: "new-api-video-session", session: value } }, origin, parent), null);
    }
    assert.equal(readStudioSessionMessage({ ...event, data: { type: "other", session } }, origin, parent), null);
});

test("separate accounts never use the same project, media or settings storage namespace", () => {
    for (const name of ["infinite-canvas", "infinite-canvas-plugins", "infinite-canvas:ai_config_store", "infinite-canvas:canvas_store"]) {
        assert.notEqual(studioStorageName(name, 17), studioStorageName(name, 18));
        assert.equal(studioStorageName(name, 17), `new-api-17:${name}`);
    }
    assert.equal(studioStorageName("standalone"), "standalone");
});

test("account display accepts only the current user's trusted parent and preserves site currency", () => {
    const account = { user_id: 17, username: "alice", display_name: "Alice", group: "default", balance: "CNY 6.00", used: "CNY 1.00", loading: false };
    const message = { origin, source: parent, data: { type: "new-api-video-account", account } };
    assert.deepEqual(readStudioAccountMessage(message, origin, parent, 17), account);
    assert.equal(readStudioAccountMessage(message, origin, parent, 18), null);
    assert.equal(readStudioAccountMessage({ ...message, source: {} }, origin, parent, 17), null);
    assert.equal(readStudioAccountMessage({ ...message, origin: "https://evil.example" }, origin, parent, 17), null);
    assert.equal(readStudioAccountMessage({ ...message, data: { type: "new-api-video-account", account: { ...account, balance: 999 } } }, origin, parent, 17), null);
    assert.deepEqual(readStudioAccountMessage({ ...message, data: { type: "new-api-video-account", account: { ...account, balance: null, used: null } } }, origin, parent, 17), { ...account, balance: null, used: null });
});
