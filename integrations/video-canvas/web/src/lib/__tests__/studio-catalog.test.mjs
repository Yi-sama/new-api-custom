import assert from "node:assert/strict";
import test from "node:test";

import { filterStudioGroups, studioChannelId, studioGroupHeaders } from "../new-api-studio.ts";

const catalog = [
    { id: "llm", description: "Chat only", ratio: 1, models: [{ id: "video-understanding-llm", capabilities: ["text"] }] },
    { id: "mixed", description: "Mixed", ratio: 1, models: [
        { id: "gpt-chat", capabilities: ["text"] },
        { id: "custom-production-alias", capabilities: ["video"] },
        { id: "image-model", capabilities: ["image"] },
    ] },
    { id: "premium", description: "Video", ratio: 2, models: [{ id: "custom-production-alias", capabilities: ["video"] }] },
    { id: "empty", description: "Empty", ratio: 1, models: [] },
];

test("video choices exclude LLM-only groups and non-video models from mixed groups", () => {
    const result = filterStudioGroups(catalog, "video");
    assert.deepEqual(result.map((group) => group.id), ["mixed", "premium"]);
    assert.deepEqual(result.map((group) => group.models.map((model) => model.id)), [["custom-production-alias"], ["custom-production-alias"]]);
    assert.equal(catalog[1].models.length, 3);
});

test("capability filtering retains independent text and image choices and empty video state", () => {
    assert.deepEqual(filterStudioGroups(catalog, "text").map((group) => group.id), ["llm", "mixed"]);
    assert.deepEqual(filterStudioGroups(catalog, "image").map((group) => group.id), ["mixed"]);
    assert.deepEqual(filterStudioGroups(catalog, "audio"), []);
    assert.deepEqual(filterStudioGroups(catalog.slice(0, 1), "video"), []);
});

test("the same model stays distinct per group and Unicode groups travel as encoded headers", () => {
    const value = (group) => `${studioChannelId(group)}::custom-production-alias`;
    assert.notEqual(value("mixed"), value("premium"));
    const group = "\u89c6\u9891+vip::%";
    assert.ok(!studioChannelId(group).includes("::"));
    assert.equal(decodeURIComponent(studioGroupHeaders(group)["X-New-Api-Group"]), group);
    assert.deepEqual(studioGroupHeaders(), {});
});
